package store

import (
	"os"
	"path/filepath"
	"testing"

	"docklite-agent/internal/testhelpers"
)

func TestNewSQLiteStore(t *testing.T) {
	t.Run("creates in-memory store", func(t *testing.T) {
		store, err := NewSQLiteStore(":memory:")
		testhelpers.AssertNoError(t, err)
		defer store.Close()

		testhelpers.AssertNotEqual(t, nil, store.DB)
		testhelpers.AssertEqual(t, ":memory:", store.Path)
	})

	t.Run("creates file-based store", func(t *testing.T) {
		dir, cleanup := testhelpers.TempDir(t)
		defer cleanup()

		dbPath := filepath.Join(dir, "test.db")
		store, err := NewSQLiteStore(dbPath)
		testhelpers.AssertNoError(t, err)
		defer store.Close()

		testhelpers.AssertNotEqual(t, nil, store.DB)
		testhelpers.AssertEqual(t, dbPath, store.Path)

		_, err = os.Stat(dbPath)
		testhelpers.AssertNoError(t, err)
	})

	t.Run("enables WAL mode for file-based store", func(t *testing.T) {
		dir, cleanup := testhelpers.TempDir(t)
		defer cleanup()

		dbPath := filepath.Join(dir, "test.db")
		store, err := NewSQLiteStore(dbPath)
		testhelpers.AssertNoError(t, err)
		defer store.Close()

		var mode string
		err = store.DB.QueryRow("PRAGMA journal_mode").Scan(&mode)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, "wal", mode)
	})

	t.Run("enables foreign keys", func(t *testing.T) {
		store, err := NewSQLiteStore(":memory:")
		testhelpers.AssertNoError(t, err)
		defer store.Close()

		var enabled int
		err = store.DB.QueryRow("PRAGMA foreign_keys").Scan(&enabled)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, 1, enabled)
	})

	t.Run("enables busy timeout", func(t *testing.T) {
		store, err := NewSQLiteStore(":memory:")
		testhelpers.AssertNoError(t, err)
		defer store.Close()

		var timeout int
		err = store.DB.QueryRow("PRAGMA busy_timeout").Scan(&timeout)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, 5000, timeout)
	})
}

func TestSQLiteStore_Close(t *testing.T) {
	t.Run("closes successfully", func(t *testing.T) {
		store, err := NewSQLiteStore(":memory:")
		testhelpers.AssertNoError(t, err)

		err = store.Close()
		testhelpers.AssertNoError(t, err)
	})

	t.Run("handles nil store", func(t *testing.T) {
		var store *SQLiteStore
		err := store.Close()
		testhelpers.AssertNoError(t, err)
	})

	t.Run("handles nil DB", func(t *testing.T) {
		store := &SQLiteStore{DB: nil}
		err := store.Close()
		testhelpers.AssertNoError(t, err)
	})
}

func TestSQLiteStore_InitializeAgentTables(t *testing.T) {
	t.Run("creates users table", func(t *testing.T) {
		store, err := NewSQLiteStore(":memory:")
		testhelpers.AssertNoError(t, err)
		defer store.Close()

		err = store.InitializeAgentTables()
		testhelpers.AssertNoError(t, err)

		var name string
		err = store.DB.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name='users'",
		).Scan(&name)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, "users", name)
	})

	t.Run("creates tokens table", func(t *testing.T) {
		store, err := NewSQLiteStore(":memory:")
		testhelpers.AssertNoError(t, err)
		defer store.Close()

		err = store.InitializeAgentTables()
		testhelpers.AssertNoError(t, err)

		var name string
		err = store.DB.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name='tokens'",
		).Scan(&name)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, "tokens", name)
	})

	t.Run("is idempotent", func(t *testing.T) {
		store, err := NewSQLiteStore(":memory:")
		testhelpers.AssertNoError(t, err)
		defer store.Close()

		err = store.InitializeAgentTables()
		testhelpers.AssertNoError(t, err)

		err = store.InitializeAgentTables()
		testhelpers.AssertNoError(t, err)
	})
}

func TestSQLiteStore_Concurrent(t *testing.T) {
	t.Run("handles concurrent writes", func(t *testing.T) {
		dir, cleanup := testhelpers.TempDir(t)
		defer cleanup()

		dbPath := filepath.Join(dir, "test.db")
		store, err := NewSQLiteStore(dbPath)
		testhelpers.AssertNoError(t, err)
		defer store.Close()

		err = store.InitializeAgentTables()
		testhelpers.AssertNoError(t, err)

		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(n int) {
				_, err := store.DB.Exec(
					"INSERT INTO users (username, password_hash, role) VALUES (?, ?, 'user')",
					"user"+string(rune('0'+n)), "hash",
				)
				if err != nil {
					t.Errorf("Concurrent write failed: %v", err)
				}
				done <- true
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}

		var count int
		err = store.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, 10, count)
	})
}
