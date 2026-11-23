package backup

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"docklite-agent/internal/docker"
	"docklite-agent/internal/store"
)

func StartScheduler(storeHandle *store.SQLiteStore, dockerClient *docker.Client, backupBaseDir string) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		time.Sleep(30 * time.Second)
		checkAndRunJobs(storeHandle, dockerClient, backupBaseDir)

		for range ticker.C {
			checkAndRunJobs(storeHandle, dockerClient, backupBaseDir)
		}
	}()
}

func TriggerJob(ctx context.Context, storeHandle *store.SQLiteStore, dockerClient *docker.Client, backupBaseDir string, jobID int64) error {
	job, err := storeHandle.GetBackupJobByID(jobID)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job %d not found", jobID)
	}
	return executeBackupJob(ctx, storeHandle, dockerClient, backupBaseDir, *job)
}

func checkAndRunJobs(storeHandle *store.SQLiteStore, dockerClient *docker.Client, backupBaseDir string) {
	jobs, err := storeHandle.GetEnabledBackupJobs()
	if err != nil {
		return
	}
	for _, job := range jobs {
		if shouldRunJob(job) {
			_ = executeBackupJob(context.Background(), storeHandle, dockerClient, backupBaseDir, job)
		}
	}
}

func shouldRunJob(job store.BackupJob) bool {
	if job.Enabled == 0 {
		return false
	}
	sched, err := ParseSchedule(job.Frequency)
	if err != nil {
		return false // an unreadable schedule never runs; the dashboard shows it as invalid
	}
	var last time.Time
	if job.LastRunAt != nil && *job.LastRunAt != "" {
		if t, err := time.Parse(time.RFC3339, *job.LastRunAt); err == nil {
			last = t
		}
	}
	return sched.Due(last, time.Now())
}

func calculateNextRunTime(frequency string) *string {
	sched, err := ParseSchedule(frequency)
	if err != nil {
		return nil
	}
	next := sched.Next(time.Now(), time.Now()).UTC().Format(time.RFC3339)
	return &next
}

func executeBackupJob(ctx context.Context, storeHandle *store.SQLiteStore, dockerClient *docker.Client, backupBaseDir string, job store.BackupJob) error {
	dest, err := storeHandle.GetBackupDestinationByID(job.Destination)
	if err != nil {
		return err
	}
	if dest == nil || dest.Enabled == 0 {
		return nil
	}

	// A failure is recorded on the backup row and the schedule still moves on, so a broken site can't make the job
	// retry every few minutes.
	if _, running := DefaultTracker.RunningFor("job", job.ID); running {
		return nil // started by hand a moment ago; this run would make the same backup again
	}
	var firstErr error
	sel, err := SelectionForJob(job)
	if err == nil {
		_, err = RunPackage(ctx, storeHandle, dockerClient, backupBaseDir, &job, *dest, sel, "", "")
	}
	if err != nil {
		firstErr = err
	}

	if err := storeHandle.UpdateBackupJobRunTime(job.ID, calculateNextRunTime(job.Frequency)); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// RunPackage makes one .litpkg for a selection and records it in the backup history. A schedule passes its job, so its
// "keep the newest N" applies; a backup made by hand passes nil. subDir "" means the destination's packages folder.
func RunPackage(ctx context.Context, storeHandle *store.SQLiteStore, dockerClient *docker.Client, backupBaseDir string, job *store.BackupJob, dest store.BackupDestination, sel Selection, subDir, notes string) (*PackageResult, error) {
	id, err := StartPackageRecord(storeHandle, dockerClient, job, dest, sel)
	if err != nil {
		return nil, err
	}
	trackType, trackID := "package", id
	if job != nil {
		trackType, trackID = "job", job.ID
	}
	rep := DefaultTracker.Start(id, trackType, trackID)
	if rep == nil {
		msg := "this schedule is already running"
		_ = storeHandle.UpdateBackupStatus(id, "failed", &msg, nil, nil)
		return nil, fmt.Errorf("%s", msg)
	}
	res, err := FinishPackage(WithReporter(ctx, rep), storeHandle, dockerClient, backupBaseDir, id, job, dest, sel, subDir, notes)
	if err != nil {
		rep.Fail(err.Error())
		return nil, err
	}
	rep.Succeed(&res.ArtifactResult, res.Verified)
	return res, nil
}

// StartPackageRecord adds the "in progress" row for a package backup, so its id is known before the work starts.
func StartPackageRecord(storeHandle *store.SQLiteStore, dockerClient *docker.Client, job *store.BackupJob, dest store.BackupDestination, sel Selection) (int64, error) {
	targetType, targetID := sel.LegacyTarget(false)
	scope := "Backup"
	if r, err := sel.ResolveWith(storeHandle, IndexContainers(context.Background(), dockerClient)); err == nil {
		scope = sel.Describe(r)
	}
	record := store.BackupRecord{
		Destination: dest.ID,
		TargetType:  targetType,
		TargetID:    targetID,
		Status:      "in_progress",
		Scope:       scope,
	}
	if job != nil {
		record.JobID = &job.ID
	}
	return storeHandle.CreateBackup(record)
}

// FinishPackage does the work for a row StartPackageRecord made and records the outcome on it.
func FinishPackage(ctx context.Context, storeHandle *store.SQLiteStore, dockerClient *docker.Client, backupBaseDir string, backupID int64, job *store.BackupJob, dest store.BackupDestination, sel Selection, subDir, notes string) (*PackageResult, error) {
	fail := func(err error) (*PackageResult, error) {
		msg := err.Error()
		_ = storeHandle.UpdateBackupStatus(backupID, "failed", &msg, nil, nil)
		return nil, err
	}
	switch dest.Type {
	case "local", "s3", "sftp", "server":
	default:
		return fail(fmt.Errorf("%s backup not implemented", dest.Type))
	}
	if subDir == "" {
		subDir = PackageSubDir
	}
	// A package for another destination is built in a work folder and removed once it is there.
	base := destinationPath(dest, backupBaseDir)
	if dest.Type != "local" {
		base = filepath.Join(backupBaseDir, outboxDir)
	}
	res, err := CreatePackage(ctx, storeHandle, dockerClient, base, subDir, sel, notes)
	if err != nil {
		return fail(err)
	}
	saved, err := Deliver(ctx, storeHandle, dest, res, subDir)
	if err != nil {
		return fail(err)
	}
	// A package that left something out still succeeded; the note says what is missing.
	var note *string
	if len(res.Problems) > 0 {
		text := strings.Join(res.Problems, "; ")
		note = &text
	}
	_ = storeHandle.UpdateBackupStatus(backupID, "success", note, &res.Size, &saved)
	_ = storeHandle.SetBackupScope(backupID, res.Description)

	if job != nil {
		if sched, err := ParseSchedule(job.Frequency); err == nil && sched.KeepLast > 0 {
			pruneJobKeepLast(ctx, storeHandle, job.ID, sched.KeepLast)
		}
		if job.RetentionDays > 0 {
			_ = cleanupOldBackups(ctx, storeHandle, dest.ID, job.RetentionDays)
		}
	}
	return res, nil
}

func destinationPath(dest store.BackupDestination, backupBaseDir string) string {
	if cfg := ParseDestConfig(dest.Config); cfg.Path != "" {
		return cfg.Path
	}
	return backupBaseDir
}
