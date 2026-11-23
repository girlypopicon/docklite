package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"docklite-agent/internal/store"
)

func cleanupOldBackups(ctx context.Context, storeHandle *store.SQLiteStore, destinationID int64, retentionDays int) error {
	if retentionDays <= 0 {
		return nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format("2006-01-02 15:04:05")
	records, err := storeHandle.ListOldBackups(destinationID, cutoff)
	if err != nil {
		return err
	}
	for _, record := range records {
		if RemoveRecordArtifacts(ctx, storeHandle, record) == nil {
			_ = storeHandle.DeleteBackup(record.ID)
		}
	}
	return nil
}

func RemoveBackupArtifacts(backupPath string) error {
	if backupPath == "" {
		return nil
	}
	_ = os.Remove(backupPath)
	manifestPath := ManifestPathForArtifact(backupPath)
	_ = os.Remove(manifestPath)
	return nil
}

func ensureRelativePath(baseDir string, targetPath string) string {
	if baseDir == "" {
		return filepath.Base(targetPath)
	}
	rel, err := filepath.Rel(baseDir, targetPath)
	if err != nil {
		return filepath.Base(targetPath)
	}
	if strings.HasPrefix(rel, "..") {
		return filepath.Base(targetPath)
	}
	return rel
}

// pruneKeepLast deletes the oldest successful backups of one site or database so that only the newest keep remain. Only
// backups that finished are counted and removed, so failures never push a good backup out, and the newest is never touched.
func pruneKeepLast(storeHandle *store.SQLiteStore, targetType string, targetID int64, keep int) {
	if keep <= 0 {
		return
	}
	records, err := storeHandle.GetBackupsByTarget(targetType, targetID, 1000) // newest first
	if err != nil {
		return
	}
	kept := 0
	for _, r := range records {
		if r.Status != "success" {
			continue
		}
		kept++
		if kept <= keep {
			continue
		}
		_ = RemoveBackupArtifacts(r.BackupPath)
		_ = storeHandle.DeleteBackup(r.ID)
	}
}

// pruneJobKeepLast keeps only the newest keep successful packages a schedule made. Like pruneKeepLast, failures are never
// counted, so they can't push a good backup out.
func pruneJobKeepLast(ctx context.Context, storeHandle *store.SQLiteStore, jobID int64, keep int) {
	if keep <= 0 {
		return
	}
	records, err := storeHandle.GetBackupsByJob(jobID, 1000) // newest first
	if err != nil {
		return
	}
	kept := 0
	for _, r := range records {
		if r.Status != "success" {
			continue
		}
		kept++
		if kept <= keep {
			continue
		}
		if RemoveRecordArtifacts(ctx, storeHandle, r) == nil { // a copy that can't be deleted yet stays listed, and is tried again next time
			_ = storeHandle.DeleteBackup(r.ID)
		}
	}
}
