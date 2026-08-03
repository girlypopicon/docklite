package store

import (
	"database/sql"
	"strings"
)

type SiteRecord struct {
	ID           int64   `json:"id"`
	Domain       string  `json:"domain"`
	UserID       int64   `json:"user_id"`
	ContainerID  *string `json:"container_id"`
	TemplateType string  `json:"template_type"`
	CodePath     string  `json:"code_path"`
	Status       string  `json:"status"`
	FolderID     *int64  `json:"folder_id"`
	CreatedAt    string  `json:"created_at"`
}

func (s *SQLiteStore) GetSiteByID(id int64) (*SiteRecord, error) {
	row := s.DB.QueryRow(`
    SELECT id, domain, user_id, container_id, template_type, code_path, status, folder_id, created_at
    FROM sites
    WHERE id = ?
  `, id)
	var record SiteRecord
	var containerID sql.NullString
	var status sql.NullString
	var folderID sql.NullInt64
	if err := row.Scan(&record.ID, &record.Domain, &record.UserID, &containerID, &record.TemplateType, &record.CodePath, &status, &folderID, &record.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if containerID.Valid {
		record.ContainerID = &containerID.String
	}
	if status.Valid {
		record.Status = status.String
	}
	if folderID.Valid {
		record.FolderID = &folderID.Int64
	}
	return &record, nil
}

func (s *SQLiteStore) ListSites() ([]SiteRecord, error) {
	rows, err := s.DB.Query(`
    SELECT id, domain, user_id, container_id, template_type, code_path, status, folder_id, created_at
    FROM sites
    ORDER BY created_at DESC
  `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []SiteRecord
	for rows.Next() {
		var record SiteRecord
		var containerID sql.NullString
		var status sql.NullString
		var folderID sql.NullInt64
		if err := rows.Scan(&record.ID, &record.Domain, &record.UserID, &containerID, &record.TemplateType, &record.CodePath, &status, &folderID, &record.CreatedAt); err != nil {
			return nil, err
		}
		if containerID.Valid {
			record.ContainerID = &containerID.String
		}
		if status.Valid {
			record.Status = status.String
		}
		if folderID.Valid {
			record.FolderID = &folderID.Int64
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *SQLiteStore) GetSiteByDomain(domain string) (*SiteRecord, error) {
	row := s.DB.QueryRow(`
    SELECT id, domain, user_id, container_id, template_type, code_path, status, folder_id, created_at
    FROM sites
    WHERE domain = ?
  `, domain)
	return scanSite(row)
}

func (s *SQLiteStore) GetSiteByContainerIDRecord(containerID string) (*SiteRecord, error) {
	row := s.DB.QueryRow(`
    SELECT id, domain, user_id, container_id, template_type, code_path, status, folder_id, created_at
    FROM sites
    WHERE container_id = ?
  `, containerID)
	return scanSite(row)
}

func (s *SQLiteStore) CreateSite(record SiteRecord) (*SiteRecord, error) {
	columns := []string{"domain", "user_id", "template_type"}
	values := []any{record.Domain, record.UserID, record.TemplateType}

	if record.ContainerID != nil && *record.ContainerID != "" {
		columns = append(columns, "container_id")
		values = append(values, *record.ContainerID)
	}
	if record.CodePath != "" {
		columns = append(columns, "code_path")
		values = append(values, record.CodePath)
	}
	if record.Status != "" {
		columns = append(columns, "status")
		values = append(values, record.Status)
	}
	if record.FolderID != nil {
		columns = append(columns, "folder_id")
		values = append(values, *record.FolderID)
	}

	placeholders := make([]string, len(values))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	query := `INSERT INTO sites (` + strings.Join(columns, ", ") + `) VALUES (` + strings.Join(placeholders, ", ") + `)`
	result, err := s.DB.Exec(query, values...)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetSiteByID(id)
}

func (s *SQLiteStore) UpdateSiteContainerID(id int64, containerID *string) error {
	_, err := s.DB.Exec(`UPDATE sites SET container_id = ? WHERE id = ?`, containerID, id)
	return err
}

func (s *SQLiteStore) UpdateSiteStatus(id int64, status string) error {
	_, err := s.DB.Exec(`UPDATE sites SET status = ? WHERE id = ?`, status, id)
	return err
}

func (s *SQLiteStore) TransferSite(id int64, newUserID int64, newCodePath string) error {
	_, err := s.DB.Exec(`UPDATE sites SET user_id = ?, code_path = ? WHERE id = ?`, newUserID, newCodePath, id)
	return err
}

func (s *SQLiteStore) DeleteSite(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM sites WHERE id = ?`, id)
	return err
}

func scanSite(scanner interface {
	Scan(dest ...any) error
}) (*SiteRecord, error) {
	var record SiteRecord
	var containerID sql.NullString
	var status sql.NullString
	var folderID sql.NullInt64
	if err := scanner.Scan(&record.ID, &record.Domain, &record.UserID, &containerID, &record.TemplateType, &record.CodePath, &status, &folderID, &record.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if containerID.Valid {
		record.ContainerID = &containerID.String
	}
	if status.Valid {
		record.Status = status.String
	}
	if folderID.Valid {
		record.FolderID = &folderID.Int64
	}
	return &record, nil
}
