package handlers

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"

	_ "modernc.org/sqlite"
)

func writeRealSQLite(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT); INSERT INTO t (v) VALUES ('x')`); err != nil {
		t.Fatal(err)
	}
}

func TestScanFindsOnlyRealSQLiteFilesInsideTheSite(t *testing.T) {
	root := t.TempDir()
	site := filepath.Join(root, "site")
	writeRealSQLite(t, filepath.Join(site, "data", "board.db"))
	writeRealSQLite(t, filepath.Join(site, "app.sqlite3"))
	writeRealSQLite(t, filepath.Join(site, "node_modules", "pkg", "cache.db")) // skipped folder
	writeRealSQLite(t, filepath.Join(site, "notes.txt"))                       // not a database extension
	if err := os.WriteFile(filepath.Join(site, "fake.db"), []byte("this is not a database, just text that is long enough to pass the size check............................................................"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.db")
	writeRealSQLite(t, outside)
	if err := os.Symlink(outside, filepath.Join(site, "link.db")); err != nil { // a link out of the site is never followed
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(site, "up")); err != nil {
		t.Fatal(err)
	}

	got := scanSQLiteFiles(site, "/app")
	var paths []string
	for _, f := range got {
		paths = append(paths, f.ContainerPath)
		if f.Size <= 0 {
			t.Errorf("%s has no size", f.ContainerPath)
		}
	}
	sort.Strings(paths)
	want := []string{"/app/app.sqlite3", "/app/data/board.db"}
	if len(paths) != len(want) || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("found %v, want %v", paths, want)
	}
}

func TestFreeSQLiteNameAvoidsClashes(t *testing.T) {
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	t.Cleanup(func() { s.Close() })
	h := &Handlers{store: s}
	live := map[string]bool{"siteA": true, "siteB": true}

	if n := h.freeSQLiteName("a.com", "/app/data/board.db", "siteA", live); n != "board" {
		t.Fatalf("first: %q", n)
	}
	if _, err := s.UpsertDatabase("board", "sqlite", "siteA", 0, "/app/data/board.db"); err != nil {
		t.Fatal(err)
	}
	// the same file again keeps its name
	if n := h.freeSQLiteName("a.com", "/app/data/board.db", "siteA", live); n != "board" {
		t.Fatalf("same file: %q", n)
	}
	// another live site with a file of the same name gets the site in front, and must not take over the first record
	if n := h.freeSQLiteName("b.com", "/app/data/board.db", "siteB", live); n != "b_com_board" {
		t.Fatalf("clash: %q", n)
	}
	// a record whose site no longer exists is picked up by the recreated site
	if n := h.freeSQLiteName("a.com", "/app/data/board.db", "siteA2", map[string]bool{"siteA2": true}); n != "board" {
		t.Fatalf("recreated site: %q", n)
	}
}

func TestIgnoredSQLiteFilesAreRemembered(t *testing.T) {
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	t.Cleanup(func() { s.Close() })
	testhelpers.AssertNoError(t, s.EnsureSQLiteIgnoreSchema())
	testhelpers.AssertNoError(t, s.IgnoreSQLiteFile("docklite-site-a-com", "/app/x.db"))
	got, err := s.IgnoredSQLiteFiles()
	testhelpers.AssertNoError(t, err)
	if !got["docklite-site-a-com\x00/app/x.db"] {
		t.Fatalf("not remembered: %v", got)
	}
}
