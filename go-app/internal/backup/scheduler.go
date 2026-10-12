package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"docklite-agent/internal/docker"
	"docklite-agent/internal/store"
)

type DestinationConfig struct {
	Path string `json:"path"`
}

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

	targets := make([]targetRef, 0)
	switch job.TargetType {
	case "site":
		if job.TargetID != nil {
			targets = append(targets, targetRef{Type: "site", ID: *job.TargetID})
		}
	case "database":
		if job.TargetID != nil {
			targets = append(targets, targetRef{Type: "database", ID: *job.TargetID})
		}
	case "all-sites":
		sites, err := storeHandle.ListSites()
		if err != nil {
			return err
		}
		for _, site := range sites {
			targets = append(targets, targetRef{Type: "site", ID: site.ID})
		}
	case "all-databases":
		databases, err := storeHandle.ListDatabases()
		if err != nil {
			return err
		}
		for _, database := range databases {
			targets = append(targets, targetRef{Type: "database", ID: database.ID})
		}
	}

	// One failing site must not stop the others from being backed up, and must not make the job retry every few
	// minutes: every target is tried, the failure is recorded on its backup row, and the schedule moves on.
	var firstErr error
	for _, target := range targets {
		if err := executeDestinationBackup(ctx, storeHandle, dockerClient, backupBaseDir, job, *dest, target); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if err := storeHandle.UpdateBackupJobRunTime(job.ID, calculateNextRunTime(job.Frequency)); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

type targetRef struct {
	Type string
	ID   int64
}

func executeDestinationBackup(ctx context.Context, storeHandle *store.SQLiteStore, dockerClient *docker.Client, backupBaseDir string, job store.BackupJob, dest store.BackupDestination, target targetRef) error {
	switch dest.Type {
	case "local":
		return executeLocalBackup(ctx, storeHandle, dockerClient, backupBaseDir, job, dest, target)
	default:
		return fmt.Errorf("%s backup not implemented", dest.Type)
	}
}

func executeLocalBackup(ctx context.Context, storeHandle *store.SQLiteStore, dockerClient *docker.Client, backupBaseDir string, job store.BackupJob, dest store.BackupDestination, target targetRef) error {
	basePath := backupBaseDir
	if dest.Config != "" {
		var cfg DestinationConfig
		if err := json.Unmarshal([]byte(dest.Config), &cfg); err == nil && cfg.Path != "" {
			basePath = cfg.Path
		}
	}

	record := store.BackupRecord{
		JobID:       &job.ID,
		Destination: dest.ID,
		TargetType:  target.Type,
		TargetID:    target.ID,
		BackupPath:  "",
		SizeBytes:   0,
		Status:      "in_progress",
	}
	backupID, err := storeHandle.CreateBackup(record)
	if err != nil {
		return err
	}

	var artifact *ArtifactResult

	switch target.Type {
	case "site":
		artifact, err = CreateSiteBackup(ctx, storeHandle, basePath, "sites", target.ID, "")
	case "database":
		artifact, err = CreateDatabaseBackup(ctx, storeHandle, dockerClient, basePath, "databases", target.ID, "")
	default:
		err = fmt.Errorf("unsupported target type")
	}

	if err != nil {
		message := err.Error()
		_ = storeHandle.UpdateBackupStatus(backupID, "failed", &message, nil, nil)
		return err
	}

	if artifact != nil {
		_ = storeHandle.UpdateBackupStatus(backupID, "success", nil, &artifact.Size, &artifact.Path)
	} else {
		_ = storeHandle.UpdateBackupStatus(backupID, "success", nil, nil, nil)
	}

	if sched, err := ParseSchedule(job.Frequency); err == nil && sched.KeepLast > 0 {
		pruneKeepLast(storeHandle, target.Type, target.ID, sched.KeepLast)
	}
	if job.RetentionDays > 0 {
		_ = cleanupOldBackups(storeHandle, dest.ID, job.RetentionDays)
	}
	return nil
}
