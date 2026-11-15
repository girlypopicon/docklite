package store

import (
	"database/sql"
	"strings"
	"time"
)

type BackupDestination struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Config    string `json:"config"`
	Enabled   int    `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

type BackupJob struct {
	ID            int64   `json:"id"`
	Destination   int64   `json:"destination_id"`
	TargetType    string  `json:"target_type"`
	TargetID      *int64  `json:"target_id"`
	Frequency     string  `json:"frequency"`
	RetentionDays int     `json:"retention_days"`
	Enabled       int     `json:"enabled"`
	LastRunAt     *string `json:"last_run_at"`
	NextRunAt     *string `json:"next_run_at"`
	CreatedAt     string  `json:"created_at"`
	// Selection is what the schedule backs up into one .litpkg (backup.Selection as JSON). Empty for schedules made
	// before packages; TargetType and TargetID say what those back up.
	Selection string `json:"selection"`
}

const jobColumns = "id, destination_id, target_type, target_id, frequency, retention_days, enabled, last_run_at, next_run_at, created_at, COALESCE(selection, '')"

type BackupRecord struct {
	ID           int64   `json:"id"`
	JobID        *int64  `json:"job_id"`
	Destination  int64   `json:"destination_id"`
	TargetType   string  `json:"target_type"`
	TargetID     int64   `json:"target_id"`
	BackupPath   string  `json:"backup_path"`
	SizeBytes    int64   `json:"size_bytes"`
	Status       string  `json:"status"`
	ErrorMessage *string `json:"error_message"`
	CreatedAt    string  `json:"created_at"`
	// Scope says in a few words what a .litpkg backup holds ("example.com + 2 databases"). Empty for backups made
	// before packages: a single site or database, as TargetType says.
	Scope string `json:"scope"`
}

const backupColumns = "id, job_id, destination_id, target_type, target_id, backup_path, size_bytes, status, error_message, created_at, COALESCE(scope, '')"

func (s *SQLiteStore) GetBackupDestinations() ([]BackupDestination, error) {
	rows, err := s.DB.Query(`SELECT id, name, type, config, enabled, created_at FROM backup_destinations ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []BackupDestination
	for rows.Next() {
		var dest BackupDestination
		if err := rows.Scan(&dest.ID, &dest.Name, &dest.Type, &dest.Config, &dest.Enabled, &dest.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, dest)
	}
	return results, rows.Err()
}

func (s *SQLiteStore) GetBackupDestinationByID(id int64) (*BackupDestination, error) {
	row := s.DB.QueryRow(`SELECT id, name, type, config, enabled, created_at FROM backup_destinations WHERE id = ?`, id)
	var dest BackupDestination
	if err := row.Scan(&dest.ID, &dest.Name, &dest.Type, &dest.Config, &dest.Enabled, &dest.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &dest, nil
}

func (s *SQLiteStore) CreateBackupDestination(name string, destType string, config string, enabled int) (int64, error) {
	result, err := s.DB.Exec(`
    INSERT INTO backup_destinations (name, type, config, enabled)
    VALUES (?, ?, ?, ?)
  `, name, destType, config, enabled)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *SQLiteStore) UpdateBackupDestination(id int64, params map[string]any) error {
	fields := make([]string, 0, 4)
	values := make([]any, 0, 5)

	if val, ok := params["name"]; ok {
		fields = append(fields, "name = ?")
		values = append(values, val)
	}
	if val, ok := params["type"]; ok {
		fields = append(fields, "type = ?")
		values = append(values, val)
	}
	if val, ok := params["config"]; ok {
		fields = append(fields, "config = ?")
		values = append(values, val)
	}
	if val, ok := params["enabled"]; ok {
		fields = append(fields, "enabled = ?")
		values = append(values, val)
	}

	if len(fields) == 0 {
		return nil
	}
	values = append(values, id)
	query := `UPDATE backup_destinations SET ` + strings.Join(fields, ", ") + ` WHERE id = ?`
	_, err := s.DB.Exec(query, values...)
	return err
}

func (s *SQLiteStore) DeleteBackupDestination(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM backup_destinations WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) CreateBackupJob(destinationID int64, targetType string, targetID *int64, frequency string, retentionDays int, enabled int, selection string) (int64, error) {
	result, err := s.DB.Exec(`
    INSERT INTO backup_jobs (destination_id, target_type, target_id, frequency, retention_days, enabled, selection)
    VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''))
  `, destinationID, targetType, targetID, frequency, retentionDays, enabled, selection)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *SQLiteStore) GetBackupJobs() ([]BackupJob, error) {
	return s.getBackupJobsByQuery(`SELECT ` + jobColumns + ` FROM backup_jobs ORDER BY created_at DESC`)
}

func (s *SQLiteStore) GetBackupJobsByDestination(destinationID int64) ([]BackupJob, error) {
	return s.getBackupJobsByQuery(`SELECT `+jobColumns+` FROM backup_jobs WHERE destination_id = ? ORDER BY created_at DESC`, destinationID)
}

func (s *SQLiteStore) GetBackupJobByID(id int64) (*BackupJob, error) {
	jobs, err := s.getBackupJobsByQuery(`SELECT `+jobColumns+` FROM backup_jobs WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, nil
	}
	return &jobs[0], nil
}

func (s *SQLiteStore) GetEnabledBackupJobs() ([]BackupJob, error) {
	return s.getBackupJobsByQuery(`SELECT ` + jobColumns + ` FROM backup_jobs WHERE enabled = 1`)
}

func (s *SQLiteStore) UpdateBackupJob(id int64, params map[string]any) error {
	fields := make([]string, 0, 6)
	values := make([]any, 0, 7)

	if val, ok := params["destination_id"]; ok {
		fields = append(fields, "destination_id = ?")
		values = append(values, val)
	}
	if val, ok := params["target_type"]; ok {
		fields = append(fields, "target_type = ?")
		values = append(values, val)
	}
	if val, ok := params["target_id"]; ok {
		fields = append(fields, "target_id = ?")
		values = append(values, val)
	}
	if val, ok := params["frequency"]; ok {
		fields = append(fields, "frequency = ?")
		values = append(values, val)
	}
	if val, ok := params["retention_days"]; ok {
		fields = append(fields, "retention_days = ?")
		values = append(values, val)
	}
	if val, ok := params["enabled"]; ok {
		fields = append(fields, "enabled = ?")
		values = append(values, val)
	}
	if val, ok := params["selection"]; ok {
		fields = append(fields, "selection = ?")
		values = append(values, val)
	}

	if len(fields) == 0 {
		return nil
	}
	values = append(values, id)
	query := `UPDATE backup_jobs SET ` + strings.Join(fields, ", ") + ` WHERE id = ?`
	_, err := s.DB.Exec(query, values...)
	return err
}

func (s *SQLiteStore) UpdateBackupJobRunTime(id int64, nextRunAt *string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec(`
    UPDATE backup_jobs
    SET last_run_at = ?, next_run_at = ?
    WHERE id = ?
  `, now, nextRunAt, id)
	return err
}

func (s *SQLiteStore) DeleteBackupJob(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM backup_jobs WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) CreateBackup(record BackupRecord) (int64, error) {
	result, err := s.DB.Exec(`
    INSERT INTO backups (job_id, destination_id, target_type, target_id, backup_path, size_bytes, status, error_message, scope)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
  `, record.JobID, record.Destination, record.TargetType, record.TargetID, record.BackupPath, record.SizeBytes, record.Status, record.ErrorMessage, record.Scope)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *SQLiteStore) SetBackupScope(id int64, scope string) error {
	_, err := s.DB.Exec(`UPDATE backups SET scope = ? WHERE id = ?`, scope, id)
	return err
}

// FailUnfinishedBackups marks backups that were still running when the agent stopped as failed: their progress lived in
// memory and is gone, and the partial file was never given its real name.
func (s *SQLiteStore) FailUnfinishedBackups() error {
	_, err := s.DB.Exec(`UPDATE backups SET status = 'failed', error_message = 'DockLite stopped before this backup finished' WHERE status = 'in_progress'`)
	return err
}

func (s *SQLiteStore) UpdateBackupStatus(id int64, status string, errorMessage *string, sizeBytes *int64, backupPath *string) error {
	fields := []string{"status = ?"}
	values := []any{status}
	if errorMessage != nil {
		fields = append(fields, "error_message = ?")
		values = append(values, *errorMessage)
	}
	if sizeBytes != nil {
		fields = append(fields, "size_bytes = ?")
		values = append(values, *sizeBytes)
	}
	if backupPath != nil {
		fields = append(fields, "backup_path = ?")
		values = append(values, *backupPath)
	}
	values = append(values, id)
	query := `UPDATE backups SET ` + strings.Join(fields, ", ") + ` WHERE id = ?`
	_, err := s.DB.Exec(query, values...)
	return err
}

func (s *SQLiteStore) GetBackups(limit int) ([]BackupRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.getBackupsByQuery(`SELECT `+backupColumns+` FROM backups ORDER BY created_at DESC LIMIT ?`, limit)
}

func (s *SQLiteStore) ListBackups() ([]BackupRecord, error) {
	return s.getBackupsByQuery(`SELECT ` + backupColumns + ` FROM backups ORDER BY created_at DESC`)
}

func (s *SQLiteStore) GetBackupsByJob(jobID int64, limit int) ([]BackupRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.getBackupsByQuery(`SELECT `+backupColumns+` FROM backups WHERE job_id = ? ORDER BY created_at DESC LIMIT ?`, jobID, limit)
}

func (s *SQLiteStore) GetBackupsByTarget(targetType string, targetID int64, limit int) ([]BackupRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.getBackupsByQuery(`SELECT `+backupColumns+` FROM backups WHERE target_type = ? AND target_id = ? ORDER BY created_at DESC LIMIT ?`, targetType, targetID, limit)
}

func (s *SQLiteStore) GetBackupByID(id int64) (*BackupRecord, error) {
	records, err := s.getBackupsByQuery(`SELECT `+backupColumns+` FROM backups WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &records[0], nil
}

func (s *SQLiteStore) ListOldBackups(destinationID int64, cutoff string) ([]BackupRecord, error) {
	return s.getBackupsByQuery(`
    SELECT `+backupColumns+`
    FROM backups
    WHERE destination_id = ? AND created_at < ? AND status = 'success'
  `, destinationID, cutoff)
}

func (s *SQLiteStore) GetBackupByPath(backupPath string) (*BackupRecord, error) {
	records, err := s.getBackupsByQuery(`
    SELECT `+backupColumns+`
    FROM backups
    WHERE backup_path = ?
    LIMIT 1
  `, backupPath)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &records[0], nil
}

func (s *SQLiteStore) DeleteBackup(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM backups WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) ClearBackupHistory() (int64, error) {
	result, err := s.DB.Exec(`DELETE FROM backups`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *SQLiteStore) DeleteOldBackups(destinationID int64, retentionDays int) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	result, err := s.DB.Exec(`
    DELETE FROM backups
    WHERE destination_id = ? AND created_at < ? AND status = 'success'
  `, destinationID, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *SQLiteStore) getBackupJobsByQuery(query string, args ...any) ([]BackupJob, error) {
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []BackupJob
	for rows.Next() {
		var job BackupJob
		var targetID sql.NullInt64
		var lastRun sql.NullString
		var nextRun sql.NullString
		if err := rows.Scan(&job.ID, &job.Destination, &job.TargetType, &targetID, &job.Frequency, &job.RetentionDays, &job.Enabled, &lastRun, &nextRun, &job.CreatedAt, &job.Selection); err != nil {
			return nil, err
		}
		if targetID.Valid {
			job.TargetID = &targetID.Int64
		}
		if lastRun.Valid {
			job.LastRunAt = &lastRun.String
		}
		if nextRun.Valid {
			job.NextRunAt = &nextRun.String
		}
		results = append(results, job)
	}
	return results, rows.Err()
}

func (s *SQLiteStore) getBackupsByQuery(query string, args ...any) ([]BackupRecord, error) {
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []BackupRecord
	for rows.Next() {
		var record BackupRecord
		var jobID sql.NullInt64
		var errorMsg sql.NullString
		if err := rows.Scan(&record.ID, &jobID, &record.Destination, &record.TargetType, &record.TargetID, &record.BackupPath, &record.SizeBytes, &record.Status, &errorMsg, &record.CreatedAt, &record.Scope); err != nil {
			return nil, err
		}
		if jobID.Valid {
			record.JobID = &jobID.Int64
		}
		if errorMsg.Valid {
			record.ErrorMessage = &errorMsg.String
		}
		results = append(results, record)
	}
	return results, rows.Err()
}
