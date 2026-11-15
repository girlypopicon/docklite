package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

// packageFixture is a store with two sites (with files on disk) and two databases, one belonging to the first site.
func packageFixture(t *testing.T) (*store.SQLiteStore, string, []int64, []int64) {
	t.Helper()
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	if err := s.EnsureBackupPackageSchema(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users (username, password_hash) VALUES ('u', 'x')`); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	var siteIDs []int64
	for _, domain := range []string{"a.test", "b.test"} {
		dir := filepath.Join(root, "sites", "u", domain)
		writeFile(t, filepath.Join(dir, "index.html"), "hello from "+domain)
		writeFile(t, filepath.Join(dir, "css", "site.css"), "body{}")
		rec, err := s.CreateSite(store.SiteRecord{Domain: domain, UserID: 1, TemplateType: "static", CodePath: dir})
		if err != nil {
			t.Fatal(err)
		}
		siteIDs = append(siteIDs, rec.ID)
	}
	var dbIDs []int64
	for _, name := range []string{"shop", "loose"} {
		rec, err := s.UpsertDatabase(name, "sqlite", "container-"+name, 0, "/data/"+name+".db")
		if err != nil {
			t.Fatal(err)
		}
		dbIDs = append(dbIDs, rec.ID)
	}
	if err := s.SetDatabaseSite(dbIDs[0], siteIDs[0]); err != nil {
		t.Fatal(err)
	}
	return s, root, siteIDs, dbIDs
}

func packageEntries(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, hdr.Name)
	}
	sort.Strings(names)
	return names
}

func TestSelectionResolve(t *testing.T) {
	s, _, sites, dbs := packageFixture(t)

	r, err := Selection{Sites: []int64{sites[0]}, SiteDatabases: []int64{sites[0]}}.Resolve(s)
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, 1, len(r.Sites))
	testhelpers.AssertEqual(t, 1, len(r.Databases))
	testhelpers.AssertEqual(t, "shop", r.Databases[0].Name)
	testhelpers.AssertEqual(t, "a.test", r.Databases[0].SiteDomain)
	testhelpers.AssertEqual(t, "a.test + 1 database", Selection{Sites: []int64{sites[0]}, SiteDatabases: []int64{sites[0]}}.Describe(r))

	// "all" includes everything, and a site picked twice is still there once
	all := Selection{AllSites: true, AllDatabases: true, Sites: []int64{sites[1]}}
	r, err = all.Resolve(s)
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, 2, len(r.Sites))
	testhelpers.AssertEqual(t, 2, len(r.Databases))
	testhelpers.AssertEqual(t, "All sites + all databases", all.Describe(r))

	// deleted things are reported, not fatal
	r, err = Selection{Sites: []int64{999}, Databases: []int64{dbs[1]}}.Resolve(s)
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, 1, len(r.Missing))
	testhelpers.AssertEqual(t, "loose", Selection{Databases: []int64{dbs[1]}}.Describe(r))
}

func TestSelectionForOldJobs(t *testing.T) {
	id := int64(4)
	for _, c := range []struct {
		job  store.BackupJob
		want Selection
	}{
		{store.BackupJob{TargetType: "site", TargetID: &id}, Selection{Sites: []int64{4}}},
		{store.BackupJob{TargetType: "database", TargetID: &id}, Selection{Databases: []int64{4}}},
		{store.BackupJob{TargetType: "all-sites"}, Selection{AllSites: true}},
		{store.BackupJob{TargetType: "all-databases"}, Selection{AllDatabases: true}},
		{store.BackupJob{TargetType: "site", Selection: `{"sites":[1,2],"all_databases":true}`}, Selection{Sites: []int64{1, 2}, AllDatabases: true}},
	} {
		got, err := SelectionForJob(c.job)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, c.want.JSON(), got.JSON())
	}
}

func TestLegacyTargetFitsTheOldTables(t *testing.T) {
	// backup_jobs only allows site, database, all-sites, all-databases; backups only site and database.
	for _, sel := range []Selection{
		{AllSites: true, AllDatabases: true},
		{AllDatabases: true},
		{Sites: []int64{1, 2}, SiteDatabases: []int64{1}},
		{SiteDatabases: []int64{3}},
		{Databases: []int64{5}},
		{Databases: []int64{5, 6}},
	} {
		jt, _ := sel.LegacyTarget(true)
		if jt != "site" && jt != "database" && jt != "all-sites" && jt != "all-databases" {
			t.Errorf("%s: job target %q", sel.JSON(), jt)
		}
		bt, _ := sel.LegacyTarget(false)
		if bt != "site" && bt != "database" {
			t.Errorf("%s: backup target %q", sel.JSON(), bt)
		}
	}
}

func TestCreatePackageAndRestoreOneSite(t *testing.T) {
	s, root, _, _ := packageFixture(t)
	base := t.TempDir()

	// Every site plus every database. Docker isn't there in a test, so the databases fail to dump: they are left out and
	// listed as problems, and the sites still make a good package.
	res, err := CreatePackage(context.Background(), s, nil, base, PackageSubDir, Selection{AllSites: true, AllDatabases: true}, "before upgrade")
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertTrue(t, strings.HasSuffix(res.Path, ".litpkg"), "package file name: "+res.Path)
	testhelpers.AssertTrue(t, strings.HasPrefix(filepath.Base(res.Path), "everything-"), "package file name: "+res.Path)
	testhelpers.AssertEqual(t, 2, res.Sites)
	testhelpers.AssertEqual(t, 0, res.Databases)
	testhelpers.AssertEqual(t, 2, len(res.Problems))
	testhelpers.AssertNoError(t, CheckAgainstManifest(res.Path))

	names := packageEntries(t, res.Path)
	for _, want := range []string{"manifest.json", "sites/a.test/files/", "sites/a.test/files/index.html", "sites/b.test/files/css/site.css"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		testhelpers.AssertTrue(t, found, "package should hold "+want+"; has "+strings.Join(names, ", "))
	}

	info, err := InspectPackage(res.Path)
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, "All sites + all databases", info.Manifest.Description)
	testhelpers.AssertEqual(t, "before upgrade", info.Manifest.Notes)
	testhelpers.AssertEqual(t, "sites/b.test/files", info.Manifest.SiteFolder("B.test"))

	// Restore only b.test from it; a.test is untouched.
	bDir := filepath.Join(root, "sites", "u", "b.test")
	aDir := filepath.Join(root, "sites", "u", "a.test")
	writeFile(t, filepath.Join(bDir, "index.html"), "changed")
	writeFile(t, filepath.Join(bDir, "new.txt"), "added after the backup")
	writeFile(t, filepath.Join(aDir, "index.html"), "a changed too")
	_, err = RestoreSiteFromPackage(context.Background(), res.Path, info.Manifest.SiteFolder("b.test"), bDir, filepath.Join(base, "safety"))
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, "hello from b.test", readFile(t, filepath.Join(bDir, "index.html")))
	testhelpers.AssertEqual(t, "<missing>", readFile(t, filepath.Join(bDir, "new.txt")))
	testhelpers.AssertEqual(t, "body{}", readFile(t, filepath.Join(bDir, "css", "site.css")))
	testhelpers.AssertEqual(t, "a changed too", readFile(t, filepath.Join(aDir, "index.html")))

	// A site the package doesn't hold is refused before anything changes.
	_, err = RestoreSiteFromPackage(context.Background(), res.Path, "sites/c.test/files", bDir, filepath.Join(base, "safety"))
	testhelpers.AssertErrorContains(t, err, "doesn't hold")
}

func TestCreatePackageFailsWhenNothingWorks(t *testing.T) {
	s, _, _, dbs := packageFixture(t)
	_, err := CreatePackage(context.Background(), s, nil, t.TempDir(), PackageSubDir, Selection{Databases: []int64{dbs[1]}}, "")
	testhelpers.AssertErrorContains(t, err, "nothing could be backed up")
	_, err = CreatePackage(context.Background(), s, nil, t.TempDir(), PackageSubDir, Selection{}, "")
	testhelpers.AssertErrorContains(t, err, "nothing was selected")
}

func TestRunPackageKeepsNewestForSchedule(t *testing.T) {
	s, _, sites, _ := packageFixture(t)
	base := t.TempDir()
	destID, err := s.CreateBackupDestination("Local", "local", `{"path":"`+base+`"}`, 1)
	testhelpers.AssertNoError(t, err)
	dest, _ := s.GetBackupDestinationByID(destID)
	sel := Selection{Sites: []int64{sites[0]}}
	jobID, err := s.CreateBackupJob(destID, "site", &sites[0], "daily@03:00;keep=2", 0, 1, sel.JSON())
	testhelpers.AssertNoError(t, err)
	job, _ := s.GetBackupJobByID(jobID)

	var paths []string
	for i := 0; i < 3; i++ {
		res, err := RunPackage(context.Background(), s, nil, base, job, *dest, sel, "", "")
		testhelpers.AssertNoError(t, err)
		paths = append(paths, res.Path)
		_, _ = s.DB.Exec(`UPDATE backups SET created_at = datetime('now', ?) WHERE backup_path = ?`, []string{"-3 hours", "-2 hours", "-1 hours"}[i], res.Path)
	}
	// runs in the same second still get files of their own
	testhelpers.AssertTrue(t, paths[0] != paths[1] && paths[1] != paths[2], "each run needs its own file")
	_, err = os.Stat(paths[0])
	testhelpers.AssertTrue(t, os.IsNotExist(err), "the oldest package should have been pruned")
	_, err = os.Stat(paths[2])
	testhelpers.AssertNoError(t, err)
	records, _ := s.GetBackupsByJob(jobID, 10)
	testhelpers.AssertEqual(t, 2, len(records))
	testhelpers.AssertEqual(t, "a.test", records[0].Scope)
}

func TestCheckDumpFileSQLite(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.sqlite")
	writeFile(t, good, "SQLite format 3\x00rest of the file")
	size, sum, err := checkDumpFile(good, "sqlite")
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, int64(32), size)
	testhelpers.AssertEqual(t, 64, len(sum))
	bad := filepath.Join(dir, "bad.sqlite")
	writeFile(t, bad, "")
	_, _, err = checkDumpFile(bad, "sqlite")
	testhelpers.AssertErrorContains(t, err, "isn't an SQLite database")
}

func TestIsPackageAcceptsOldExtensions(t *testing.T) {
	for _, name := range []string{"packages/a-20261011-030000.litpkg", "packages/a.litepkg", "downloads/old.dklpkg"} {
		testhelpers.AssertTrue(t, IsPackage(name), "should be a package: "+name)
	}
	for _, name := range []string{"sites/site-a-20261011-030000.tar.gz", "databases/shop.dump.gz", "notes.pkg"} {
		testhelpers.AssertTrue(t, !IsPackage(name), "should not be a package: "+name)
	}
}
