package store

import (
	"database/sql"
	"errors"
)

// A small key-value table for server-wide settings that are plain text (for now: the contact line on the "this box
// isn't running" page). Created by InitializeAgentTables; additive, so an older DockLite ignores it.

func (s *SQLiteStore) EnsureSettingsSchema() error {
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// GetSetting returns a setting's value, or "" when it was never set.
func (s *SQLiteStore) GetSetting(key string) (string, error) {
	var v string
	err := s.DB.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// SetSetting stores a value; an empty value removes the setting.
func (s *SQLiteStore) SetSetting(key, value string) error {
	if value == "" {
		_, err := s.DB.Exec(`DELETE FROM app_settings WHERE key = ?`, key)
		return err
	}
	_, err := s.DB.Exec(`
		INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, key, value)
	return err
}
