package backup

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func TestEncryptedPackageToS3AndBack(t *testing.T) {
	s, _, sites, _ := packageFixture(t)
	base := t.TempDir()
	fake := newFakeS3()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	rcpt, ident, _ := GenerateKey()
	cfg := `{"endpoint":"` + srv.URL + `","region":"auto","bucket":"bk","prefix":"dl","access_key":"AK","secret_key":"SK","encrypt":true,"recipient":"` + rcpt + `"}`
	destID, err := s.CreateBackupDestination("R2", "s3", cfg, 1)
	testhelpers.AssertNoError(t, err)
	dest, _ := s.GetBackupDestinationByID(destID)
	sel := Selection{Sites: []int64{sites[0]}}
	jobID, err := s.CreateBackupJob(destID, "site", &sites[0], "daily@03:00;keep=1", 0, 1, sel.JSON())
	testhelpers.AssertNoError(t, err)
	job, _ := s.GetBackupJobByID(jobID)

	res, err := RunPackage(context.Background(), s, nil, base, job, *dest, sel, "", "")
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertTrue(t, strings.HasPrefix(res.Path, "s3://bk/dl/packages/"), "reference: "+res.Path)
	testhelpers.AssertTrue(t, strings.HasSuffix(res.Path, ".litpkg.age"), "locked file: "+res.Path)
	testhelpers.AssertEqual(t, 1, len(fake.objects))
	for _, data := range fake.objects {
		testhelpers.AssertTrue(t, !strings.Contains(string(data), "hello from a.test"), "the stored object must not hold plain text")
	}
	// nothing is left in the work folder
	left, _ := os.ReadDir(filepath.Join(base, outboxDir, PackageSubDir))
	testhelpers.AssertEqual(t, 0, len(left))

	rec, _ := s.GetBackupsByJob(jobID, 5)
	testhelpers.AssertEqual(t, 1, len(rec))
	testhelpers.AssertEqual(t, res.Path, rec[0].BackupPath)

	// without the key it can't be opened; with it, it restores like any package
	_, err = FetchRecord(context.Background(), s, base, rec[0], "")
	testhelpers.AssertErrorContains(t, err, "locked")
	_, err = FetchRecord(context.Background(), s, base, rec[0], mustOtherKey(t))
	testhelpers.AssertErrorContains(t, err, "doesn't open")
	plain, err := FetchRecord(context.Background(), s, base, rec[0], ident)
	testhelpers.AssertNoError(t, err)
	info, err := InspectPackage(plain)
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, "a.test", info.Manifest.Description)

	// the schedule keeps one: the second run removes the first from the bucket
	_, _ = s.DB.Exec(`UPDATE backups SET created_at = datetime('now', '-2 hours') WHERE job_id = ?`, jobID)
	res2, err := RunPackage(context.Background(), s, nil, base, job, *dest, sel, "", "")
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, 1, len(fake.objects))
	_, stillThere := fake.objects["/bk/"+strings.TrimPrefix(rec[0].BackupPath, "s3://bk/")]
	testhelpers.AssertTrue(t, !stillThere, "the old backup should be gone from the bucket")
	_, newThere := fake.objects["/bk/"+strings.TrimPrefix(res2.Path, "s3://bk/")]
	testhelpers.AssertTrue(t, newThere, "the new backup should be in the bucket")
}

func mustOtherKey(t *testing.T) string {
	t.Helper()
	_, id, err := GenerateKey()
	testhelpers.AssertNoError(t, err)
	return id
}

func TestFailedUploadLeavesNothingAndIsRecorded(t *testing.T) {
	s, _, sites, _ := packageFixture(t)
	base := t.TempDir()
	srv := httptest.NewServer(nil) // answers 404 to everything
	srv.Close()                    // and now refuses connections
	cfg := `{"endpoint":"` + srv.URL + `","region":"auto","bucket":"bk","access_key":"AK","secret_key":"SK"}`
	destID, _ := s.CreateBackupDestination("Down", "s3", cfg, 1)
	dest, _ := s.GetBackupDestinationByID(destID)
	_, err := RunPackage(context.Background(), s, nil, base, nil, *dest, Selection{Sites: []int64{sites[0]}}, "", "")
	testhelpers.AssertErrorContains(t, err, "couldn't send the backup")
	left, _ := os.ReadDir(filepath.Join(base, outboxDir, PackageSubDir))
	testhelpers.AssertEqual(t, 0, len(left))
	recs, _ := s.GetBackupsByTarget("site", sites[0], 5)
	testhelpers.AssertEqual(t, 1, len(recs))
	testhelpers.AssertEqual(t, "failed", recs[0].Status)
}

func TestLocalDestinationCanBeEncrypted(t *testing.T) {
	s, _, sites, _ := packageFixture(t)
	base := t.TempDir()
	rcpt, ident, _ := GenerateKey()
	destID, _ := s.CreateBackupDestination("Local", "local", `{"path":"`+base+`","encrypt":true,"recipient":"`+rcpt+`"}`, 1)
	dest, _ := s.GetBackupDestinationByID(destID)
	res, err := RunPackage(context.Background(), s, nil, base, nil, *dest, Selection{Sites: []int64{sites[0]}}, "", "")
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertTrue(t, strings.HasSuffix(res.Path, ".litpkg.age"), res.Path)
	files, _ := filepath.Glob(filepath.Join(base, PackageSubDir, "*"))
	for _, f := range files {
		testhelpers.AssertTrue(t, !strings.HasSuffix(f, ".litpkg"), "the plain package must be gone: "+f)
	}
	testhelpers.AssertNoError(t, CheckAgainstManifest(res.Path)) // the locked file has its own manifest
	recs, _ := s.GetBackupsByTarget("site", sites[0], 5)
	plain, err := FetchRecord(context.Background(), s, base, recs[0], ident)
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertNoError(t, CheckAgainstManifest(plain))
}

func TestSelectionByGroupAndContainer(t *testing.T) {
	s, _, sites, _ := packageFixture(t)
	testhelpers.AssertNoError(t, s.EnsureGroupSchema())
	_, _ = s.DB.Exec(`UPDATE sites SET container_id = 'abc123' WHERE id = ?`, sites[0])
	g, err := s.CreateGroup(1, "Client")
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertNoError(t, s.AddToGroup(g.ID, "web-a"))
	testhelpers.AssertNoError(t, s.AddToGroup(g.ID, "db-loose"))
	testhelpers.AssertNoError(t, s.AddToGroup(g.ID, "metrics")) // neither a site nor a database
	idx := ContainerIndex{"web-a": "abc123456789", "db-loose": "container-loose", "metrics": "ffff"}

	r, err := Selection{Groups: []int64{g.ID}}.ResolveWith(s, idx)
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, 1, len(r.Sites))
	testhelpers.AssertEqual(t, "a.test", r.Sites[0].Domain)
	testhelpers.AssertEqual(t, 2, len(r.Databases)) // the site's linked database comes along, plus the loose one
	testhelpers.AssertEqual(t, 1, len(r.Missing))
	testhelpers.AssertTrue(t, strings.Contains(r.Missing[0], "metrics"), r.Missing[0])
	testhelpers.AssertEqual(t, 1, len(r.Groups))
	testhelpers.AssertEqual(t, "Group Client: a.test + 2 databases", Selection{Groups: []int64{g.ID}}.Describe(r))

	// a group means its members when the backup runs, so a container added later is included
	_, _ = s.DB.Exec(`UPDATE sites SET container_id = 'zzz999' WHERE id = ?`, sites[1])
	testhelpers.AssertNoError(t, s.AddToGroup(g.ID, "web-b"))
	idx["web-b"] = "zzz999"
	r, _ = Selection{Groups: []int64{g.ID}}.ResolveWith(s, idx)
	testhelpers.AssertEqual(t, 2, len(r.Sites))

	// a single container, one that is gone, and a deleted group
	r, _ = Selection{Containers: []string{"db-loose", "nope"}, Groups: []int64{999}}.ResolveWith(s, idx)
	testhelpers.AssertEqual(t, 0, len(r.Sites))
	testhelpers.AssertEqual(t, 1, len(r.Databases))
	testhelpers.AssertEqual(t, 2, len(r.Missing))

	// the group goes in the package's manifest, and a schedule for it fits the old tables
	jt, _ := Selection{Groups: []int64{g.ID}}.LegacyTarget(true)
	testhelpers.AssertEqual(t, "site", jt)
	testhelpers.AssertTrue(t, !(Selection{Groups: []int64{1}}).Empty(), "a group is something to back up")
	base := t.TempDir()
	res, err := CreatePackage(context.Background(), s, nil, base, PackageSubDir, Selection{Groups: []int64{g.ID}}, "")
	testhelpers.AssertErrorContains(t, err, "nothing") // without Docker the group can't be resolved to containers
	_ = res
}

var _ = store.BackupRecord{}
