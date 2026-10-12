package backup

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"docklite-agent/internal/store"
)

func cleanupOldBackups(storeHandle *store.SQLiteStore, destinationID int64, retentionDays int) error {
	if retentionDays <= 0 {
		return nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format("2006-01-02 15:04:05")
	records, err := storeHandle.ListOldBackups(destinationID, cutoff)
	if err != nil {
		return err
	}
	for _, record := range records {
		_ = RemoveBackupArtifacts(record.BackupPath)
		_ = storeHandle.DeleteBackup(record.ID)
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
