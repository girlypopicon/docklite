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
	if err != nil {
		return err
	}

	// Each user's dashboard look (theme, text size...), so it follows them to every browser and survives updates
	_, err = s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS user_preferences (
			user_id INTEGER PRIMARY KEY,
			appearance TEXT,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	return s.EnsureBackupPackageSchema()
}

// EnsureBackupPackageSchema adds what .litpkg backups need: which databases belong to which site, a scope on each
// backup row and a selection on each schedule. All are additions only, so an older DockLite keeps working with the same
// file. The GUI's migration 016 does the same; whichever starts first wins and the other finds nothing to do.
func (s *SQLiteStore) EnsureBackupPackageSchema() error {
	if _, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS site_databases (
			database_id INTEGER PRIMARY KEY,
			site_id INTEGER NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}
	// On a brand-new server the GUI creates the backup tables; until then there is nothing to add the columns to.
	for _, c := range []struct{ table, column string }{{"backups", "scope"}, {"backup_jobs", "selection"}} {
		if err := s.addColumnIfMissing(c.table, c.column); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) addColumnIfMissing(table, column string) error {
	var exists int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&exists); err != nil || exists == 0 {
		return err
	}
	if has, err := s.hasColumn(table, column); err != nil || has {
		return err
	}
	_, err := s.DB.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` TEXT`)
	return err
}

func (s *SQLiteStore) hasColumn(table, column string) (bool, error) {
	rows, err := s.DB.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
