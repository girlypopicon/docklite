package store

import (
	"database/sql"
	"errors"
	"strings"
)

type CloudflareConfig struct {
	ID        int64          `json:"id"`
	APIToken  sql.NullString `json:"api_token"`
	AccountID sql.NullString `json:"account_id"`
	Enabled   int            `json:"enabled"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

type DNSZone struct {
	ID           int64          `json:"id"`
	Domain       string         `json:"domain"`
	ZoneID       string         `json:"zone_id"`
	AccountID    sql.NullString `json:"account_id"`
	Enabled      int            `json:"enabled"`
	LastSyncedAt sql.NullString `json:"last_synced_at"`
	CreatedAt    string         `json:"created_at"`
}

type DNSRecord struct {
	ID                 int64          `json:"id"`
	ZoneID             int64          `json:"zone_id"`
	CloudflareRecordID sql.NullString `json:"cloudflare_record_id"`
	Type               string         `json:"type"`
	Name               string         `json:"name"`
	Content            string         `json:"content"`
	TTL                int            `json:"ttl"`
	Priority           sql.NullInt64  `json:"priority"`
	Proxied            int            `json:"proxied"`
	CreatedAt          string         `json:"created_at"`
	UpdatedAt          string         `json:"updated_at"`
}

func (s *SQLiteStore) GetCloudflareConfig() (*CloudflareConfig, error) {
	row := s.DB.QueryRow(`
    SELECT id, api_token, account_id, enabled, created_at, updated_at
    FROM cloudflare_config WHERE id = 1
  `)
	var cfg CloudflareConfig
	if err := row.Scan(&cfg.ID, &cfg.APIToken, &cfg.AccountID, &cfg.Enabled, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &cfg, nil
}

func (s *SQLiteStore) UpdateCloudflareConfig(apiToken *string, accountID *string, enabled *int) error {
	fields := []string{"updated_at = CURRENT_TIMESTAMP"}
	values := []any{}

	if apiToken != nil {
		fields = append(fields, "api_token = ?")
		values = append(values, *apiToken)
	}
	if accountID != nil {
		fields = append(fields, "account_id = ?")
		values = append(values, *accountID)
	}
	if enabled != nil {
		fields = append(fields, "enabled = ?")
		values = append(values, *enabled)
	}

	values = append(values, 1)
	query := `UPDATE cloudflare_config SET ` + strings.Join(fields, ", ") + ` WHERE id = ?`
	_, err := s.DB.Exec(query, values...)
	return err
}

func (s *SQLiteStore) GetDNSZones() ([]DNSZone, error) {
	rows, err := s.DB.Query(`SELECT id, domain, zone_id, account_id, enabled, last_synced_at, created_at FROM dns_zones ORDER BY domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DNSZone
	for rows.Next() {
		var zone DNSZone
		if err := rows.Scan(&zone.ID, &zone.Domain, &zone.ZoneID, &zone.AccountID, &zone.Enabled, &zone.LastSyncedAt, &zone.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, zone)
	}
	return results, rows.Err()
}

func (s *SQLiteStore) GetDNSZoneByID(id int64) (*DNSZone, error) {
	row := s.DB.QueryRow(`SELECT id, domain, zone_id, account_id, enabled, last_synced_at, created_at FROM dns_zones WHERE id = ?`, id)
	var zone DNSZone
	if err := row.Scan(&zone.ID, &zone.Domain, &zone.ZoneID, &zone.AccountID, &zone.Enabled, &zone.LastSyncedAt, &zone.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &zone, nil
}

func (s *SQLiteStore) CreateDNSZone(domain string, zoneID string, accountID *string, enabled int) (int64, error) {
	result, err := s.DB.Exec(`
    INSERT INTO dns_zones (domain, zone_id, account_id, enabled)
    VALUES (?, ?, ?, ?)
  `, domain, zoneID, accountID, enabled)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *SQLiteStore) UpdateDNSZone(id int64, params map[string]any) error {
	fields := []string{}
	values := []any{}

	if val, ok := params["domain"]; ok {
		fields = append(fields, "domain = ?")
		values = append(values, val)
	}
	if val, ok := params["zone_id"]; ok {
		fields = append(fields, "zone_id = ?")
		values = append(values, val)
	}
	if val, ok := params["account_id"]; ok {
		fields = append(fields, "account_id = ?")
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
	query := `UPDATE dns_zones SET ` + strings.Join(fields, ", ") + ` WHERE id = ?`
	_, err := s.DB.Exec(query, values...)
	return err
}

func (s *SQLiteStore) UpdateDNSZoneSyncTime(id int64) error {
	_, err := s.DB.Exec(`UPDATE dns_zones SET last_synced_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) DeleteDNSZone(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM dns_zones WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) GetDNSRecords(zoneID *int64) ([]DNSRecord, error) {
	query := `SELECT id, zone_id, cloudflare_record_id, type, name, content, ttl, priority, proxied, created_at, updated_at FROM dns_records`
	args := []any{}
	if zoneID != nil {
		query += ` WHERE zone_id = ?`
		args = append(args, *zoneID)
		query += ` ORDER BY type, name`
	} else {
		query += ` ORDER BY zone_id, type, name`
	}
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DNSRecord
	for rows.Next() {
		var record DNSRecord
		if err := rows.Scan(&record.ID, &record.ZoneID, &record.CloudflareRecordID, &record.Type, &record.Name, &record.Content, &record.TTL, &record.Priority, &record.Proxied, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, record)
	}
	return results, rows.Err()
}

func (s *SQLiteStore) GetDNSRecordByID(id int64) (*DNSRecord, error) {
	row := s.DB.QueryRow(`SELECT id, zone_id, cloudflare_record_id, type, name, content, ttl, priority, proxied, created_at, updated_at FROM dns_records WHERE id = ?`, id)
	var record DNSRecord
	if err := row.Scan(&record.ID, &record.ZoneID, &record.CloudflareRecordID, &record.Type, &record.Name, &record.Content, &record.TTL, &record.Priority, &record.Proxied, &record.CreatedAt, &record.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (s *SQLiteStore) CreateDNSRecord(record DNSRecord) (int64, error) {
	result, err := s.DB.Exec(`
    INSERT INTO dns_records (zone_id, cloudflare_record_id, type, name, content, ttl, priority, proxied)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?)
  `, record.ZoneID, record.CloudflareRecordID, record.Type, record.Name, record.Content, record.TTL, record.Priority, record.Proxied)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *SQLiteStore) UpdateDNSRecord(id int64, params map[string]any) error {
	fields := []string{"updated_at = CURRENT_TIMESTAMP"}
	values := []any{}

	if val, ok := params["cloudflare_record_id"]; ok {
		fields = append(fields, "cloudflare_record_id = ?")
		values = append(values, val)
	}
	if val, ok := params["type"]; ok {
		fields = append(fields, "type = ?")
		values = append(values, val)
	}
	if val, ok := params["name"]; ok {
		fields = append(fields, "name = ?")
		values = append(values, val)
	}
	if val, ok := params["content"]; ok {
		fields = append(fields, "content = ?")
		values = append(values, val)
	}
	if val, ok := params["ttl"]; ok {
		fields = append(fields, "ttl = ?")
		values = append(values, val)
	}
	if val, ok := params["priority"]; ok {
		fields = append(fields, "priority = ?")
		values = append(values, val)
	}
	if val, ok := params["proxied"]; ok {
		fields = append(fields, "proxied = ?")
		values = append(values, val)
	}
	if len(fields) == 0 {
		return nil
	}
	values = append(values, id)
	query := `UPDATE dns_records SET ` + strings.Join(fields, ", ") + ` WHERE id = ?`
	_, err := s.DB.Exec(query, values...)
	return err
}

func (s *SQLiteStore) DeleteDNSRecord(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM dns_records WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) ClearDNSRecords(zoneID int64) error {
	_, err := s.DB.Exec(`DELETE FROM dns_records WHERE zone_id = ?`, zoneID)
	return err
}
