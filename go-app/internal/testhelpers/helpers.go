package testhelpers

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

// TestStore creates an in-memory SQLite database for testing
func TestStore(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	return db
}

// TestStoreWithTables creates an in-memory database with all required tables
func TestStoreWithTables(t *testing.T) *sql.DB {
	db := TestStore(t)

	_, err := db.Exec(`
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
		t.Fatalf("Failed to create users table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT UNIQUE NOT NULL,
			user_id INTEGER NOT NULL,
			container_id TEXT,
			template_type TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create sites table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS databases (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			type TEXT DEFAULT 'postgres',
			container_id TEXT UNIQUE NOT NULL,
			postgres_port INTEGER NOT NULL,
			db_path TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create databases table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS database_permissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			database_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (database_id) REFERENCES databases(id),
			UNIQUE(user_id, database_id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create database_permissions table: %v", err)
	}

	_, err = db.Exec(`
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
		t.Fatalf("Failed to create tokens table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS folders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			parent_folder_id INTEGER,
			depth INTEGER DEFAULT 0,
			position INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (parent_folder_id) REFERENCES folders(id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create folders table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS folder_containers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			folder_id INTEGER NOT NULL,
			container_id TEXT NOT NULL,
			position INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (folder_id) REFERENCES folders(id) ON DELETE CASCADE,
			UNIQUE(folder_id, container_id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create folder_containers table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS backup_destinations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			config TEXT,
			enabled INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create backup_destinations table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS backup_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			destination_id INTEGER NOT NULL,
			target_type TEXT NOT NULL,
			target_id INTEGER,
			frequency TEXT NOT NULL,
			retention_days INTEGER DEFAULT 30,
			enabled INTEGER DEFAULT 1,
			last_run_at DATETIME,
			next_run_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (destination_id) REFERENCES backup_destinations(id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create backup_jobs table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS backups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id INTEGER,
			destination_id INTEGER NOT NULL,
			target_type TEXT NOT NULL,
			target_id INTEGER NOT NULL,
			backup_path TEXT NOT NULL,
			size_bytes INTEGER DEFAULT 0,
			status TEXT DEFAULT 'in_progress',
			error_message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (job_id) REFERENCES backup_jobs(id),
			FOREIGN KEY (destination_id) REFERENCES backup_destinations(id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create backups table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS dns_zones (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT NOT NULL UNIQUE,
			zone_id TEXT NOT NULL,
			account_id TEXT,
			enabled INTEGER DEFAULT 1,
			last_synced_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create dns_zones table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS dns_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			zone_id INTEGER NOT NULL,
			cloudflare_record_id TEXT,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			content TEXT NOT NULL,
			ttl INTEGER DEFAULT 1,
			priority INTEGER,
			proxied INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (zone_id) REFERENCES dns_zones(id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create dns_records table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cloudflare_config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			api_token TEXT,
			account_id TEXT,
			enabled INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create cloudflare_config table: %v", err)
	}

	_, err = db.Exec(`INSERT INTO cloudflare_config (id) VALUES (1)`)
	if err != nil {
		t.Fatalf("Failed to insert default cloudflare config: %v", err)
	}

	return db
}

// TempDir creates a temporary directory and returns its path along with a cleanup function
func TempDir(t *testing.T) (string, func()) {
	dir, err := os.MkdirTemp("", "docklite-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}

// TempFile creates a temporary file with the given content and returns its path.
// The file is cleaned up when the test finishes.
func TempFile(t *testing.T, content string) string {
	dir, err := os.MkdirTemp("", "docklite-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	filePath := filepath.Join(dir, "testfile")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	return filePath
}

// AssertEqual checks if two values are equal
func AssertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

// AssertNotEqual checks if two values are not equal
func AssertNotEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v to not equal %v", expected, actual)
	}
}

// AssertNoError checks that no error occurred
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// AssertError checks that an error occurred
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected an error but got none")
	}
}

// AssertErrorContains checks that the error message contains the expected substring
func AssertErrorContains(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected an error but got none")
	}
	if !contains(err.Error(), expected) {
		t.Errorf("Expected error to contain %q, got %q", expected, err.Error())
	}
}

// AssertTrue checks that a condition is true
func AssertTrue(t *testing.T, condition bool, message string) {
	t.Helper()
	if !condition {
		t.Errorf("Expected true: %s", message)
	}
}

// AssertFalse checks that a condition is false
func AssertFalse(t *testing.T, condition bool, message string) {
	t.Helper()
	if condition {
		t.Errorf("Expected false: %s", message)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
