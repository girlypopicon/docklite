package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	DB   *sql.DB
	Path string
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	dsn := fmt.Sprintf("file:%s?mode=rwc", path)
	if path == ":memory:" {
		dsn = path
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Serialize all writes through a single connection to avoid SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if path != ":memory:" {
		if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLiteStore{DB: db, Path: path}, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

// InitializeAgentTables creates tables needed by the agent if they don't exist
func (s *SQLiteStore) InitializeAgentTables() error {
	// Create users table if it doesn't exist
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			is_admin INTEGER DEFAULT 0,
			role TEXT DEFAULT 'user',
			is_super_admin INTEGER DEFAULT 0,
			managed_by INTEGER,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create tokens table if it doesn't exist
	_, err = s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			token_fingerprint TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_used_at TEXT,
			expires_at TEXT,
			scopes TEXT,
			disabled INTEGER DEFAULT 0,
			revoked_at TEXT,
			issued_for TEXT,
			user_id INTEGER,
			role TEXT
		)
	`)
	return err
}
