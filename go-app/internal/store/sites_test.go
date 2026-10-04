package store

import (
	"testing"

	"docklite-agent/internal/testhelpers"
)

// A site row with a NULL code_path (the column was added later without a
// default) used to make every site query fail with a Scan error.
func TestSitesWithNullCodePath(t *testing.T) {
	db := testhelpers.TestStore(t)
	s := &SQLiteStore{DB: db, Path: ":memory:"}
	defer s.Close()

	_, err := db.Exec(`
		CREATE TABLE sites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT UNIQUE NOT NULL,
			user_id INTEGER NOT NULL,
			container_id TEXT,
			template_type TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			code_path TEXT, status TEXT, folder_id INTEGER
		)`)
	testhelpers.AssertNoError(t, err)
	_, err = db.Exec(`INSERT INTO sites (domain, user_id, template_type) VALUES ('old.example.com', 1, 'static')`)
	testhelpers.AssertNoError(t, err)

	site, err := s.GetSiteByID(1)
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, "", site.CodePath)

	sites, err := s.ListSites()
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, 1, len(sites))

	byDomain, err := s.GetSiteByDomain("old.example.com")
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, "old.example.com", byDomain.Domain)
}
