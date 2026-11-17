package handlers

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestHostPathForContainerFile(t *testing.T) {
	mounts := []mountPair{{"/var/www/sites/a/site.com", "/app"}, {"/var/www/sites/a/site.com/data", "/app/data"}, {"/x", "/usr/share/nginx/html"}}
	cases := []struct {
		in, want string
		fail     bool
	}{
		{"/app/db.sqlite", "/var/www/sites/a/site.com/db.sqlite", false},
		{"/app/data/db.sqlite", "/var/www/sites/a/site.com/data/db.sqlite", false}, // the longer mount wins
		{"/app/../etc/passwd", "", true},
		{"/etc/passwd", "", true},
	}
	for _, c := range cases {
		got, err := hostPathForContainerFile(mounts, c.in)
		if c.fail {
			if err == nil {
				t.Errorf("%q should be refused, got %q", c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%q -> %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestSQLiteEditor(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, q := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, bio TEXT)`,
		`INSERT INTO users (name, bio) VALUES ('ada', 'first'), ('grace', NULL)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}

	tables, err := sqliteSchema(db)
	if err != nil || len(tables) != 1 || tables[0]["name"] != "users" {
		t.Fatalf("schema = %v, %v", tables, err)
	}
	cols := tables[0]["columns"].([]sqliteColumn)
	if len(cols) != 3 || cols[0].Key != "PRIMARY KEY" || cols[1].Nullable || !cols[2].Nullable {
		t.Errorf("columns = %+v", cols)
	}

	_, rows, err := sqliteTable(db, "users")
	if err != nil || len(rows) != 2 || rows[0]["name"] != "ada" {
		t.Fatalf("rows = %v, %v", rows, err)
	}
	if _, _, err := sqliteTable(db, "nope"); err == nil {
		t.Error("a missing table should be an error")
	}

	out, err := sqliteRunSQL(db, "SELECT name FROM users ORDER BY id;")
	if err != nil || out["type"] != "select" || len(out["rows"].([]map[string]any)) != 2 {
		t.Fatalf("select = %v, %v", out, err)
	}
	out, err = sqliteRunSQL(db, "UPDATE users SET bio = 'x'")
	if err != nil || out["type"] != "command" {
		t.Fatalf("update = %v, %v", out, err)
	}

	if err := sqliteUpdateRows(db, "users", []map[string]any{{"id": int64(2), "bio": "navy"}}); err != nil {
		t.Fatal(err)
	}
	_, rows, _ = sqliteTable(db, "users")
	if rows[1]["bio"] != "navy" {
		t.Errorf("update-rows didn't change the row: %v", rows[1])
	}
	if err := sqliteUpdateRows(db, "users", []map[string]any{{"bio": "no id"}}); err == nil {
		t.Error("a row without an id should be refused")
	}
	if err := sqliteUpdateRows(db, "users", []map[string]any{{"id": 1, `b"io`: "x"}}); err == nil {
		t.Error("a bad column name should be refused")
	}
}

func TestPrepareSQLiteFileCreatesInsideMountAndRefusesOutside(t *testing.T) {
	dir := t.TempDir()
	mounts := []mountPair{{Source: dir, Dest: "/usr/share/nginx/html"}}
	if err := prepareSQLiteFile(mounts, "/usr/share/nginx/html/data/db.sqlite"); err != nil {
		t.Fatalf("inside the mount: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "data", "db.sqlite")); err != nil {
		t.Fatalf("file should have been created: %v", err)
	}
	err := prepareSQLiteFile(mounts, "/app/data/db.sqlite")
	if err == nil || !strings.Contains(err.Error(), "Start the path with /usr/share/nginx/html") {
		t.Fatalf("outside the mount should say where to put it, got: %v", err)
	}
}
