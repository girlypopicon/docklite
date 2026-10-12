package handlers

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func makeSiteTar(t *testing.T, path, folder string, files map[string]string) {
	t.Helper()
	testhelpers.AssertNoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	f, err := os.Create(path)
	testhelpers.AssertNoError(t, err)
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: folder + "/", Typeflag: tar.TypeDir, Mode: 0o755})
	for n, c := range files {
		_ = tw.WriteHeader(&tar.Header{Name: folder + "/" + n, Typeflag: tar.TypeReg, Size: int64(len(c)), Mode: 0o644})
		_, _ = tw.Write([]byte(c))
	}
	_ = tw.Close()
	_ = gz.Close()
}

func TestRestoreSiteBackupFindsSiteAndKeepsSafetyCopy(t *testing.T) {
	h, _, _, _, site, _ := relocateFixture(t)
	h.backupBaseDir = t.TempDir()
	// an old-style backup (no manifest): the file name says which site it is for
	file := "sites/site-shop.example.com-20261011-030000.tar.gz"
	makeSiteTar(t, filepath.Join(h.backupBaseDir, file), "shop.example.com", map[string]string{"index.html": "from backup"})

	req := httptest.NewRequest("POST", "/api/backups/restore", nil)
	res, got, err := h.restoreSiteBackup(req, file, "")
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, got.Domain, "shop.example.com")
	b, _ := os.ReadFile(filepath.Join(site.CodePath, "index.html"))
	testhelpers.AssertEqual(t, string(b), "from backup")
	if res.SafetyBackup == "" || !strings.HasPrefix(res.SafetyBackup, filepath.Join(h.backupBaseDir, "safety")) {
		t.Fatalf("the previous files should be saved under backups/safety: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(site.CodePath, "assets", "app.js")); err == nil {
		t.Fatal("files that weren't in the backup should be gone")
	}
}

func TestRestoreSiteBackupRefusesWrongTargets(t *testing.T) {
	h, _, _, _, site, _ := relocateFixture(t)
	h.backupBaseDir = t.TempDir()
	makeSiteTar(t, filepath.Join(h.backupBaseDir, "sites", "site-other.org-1-2.tar.gz"), "other.org", map[string]string{"a": "b"})
	req := httptest.NewRequest("POST", "/", nil)

	if _, _, err := h.restoreSiteBackup(req, "sites/site-other.org-1-2.tar.gz", ""); err == nil || !strings.Contains(err.Error(), "no site called other.org") {
		t.Fatalf("a backup for an unknown site must say so: %v", err)
	}
	if _, _, err := h.restoreSiteBackup(req, "../../etc/passwd", ""); err == nil {
		t.Fatal("paths outside the backup folder must be refused")
	}
	if _, _, err := h.restoreSiteBackup(req, "sites/site-other.org-1-2.tar.gz", "../../etc"); err == nil {
		t.Fatal("a made-up site name must be refused")
	}
	// a site whose folder is outside the sites base is never restored into
	outside := t.TempDir()
	_, err := h.store.CreateSite(store.SiteRecord{Domain: "evil.test", UserID: site.UserID, TemplateType: "static", CodePath: outside, Status: "stopped"})
	testhelpers.AssertNoError(t, err)
	if _, _, err := h.restoreSiteBackup(req, "sites/site-other.org-1-2.tar.gz", "evil.test"); err == nil || !strings.Contains(err.Error(), "isn't under") {
		t.Fatalf("restore outside the sites folder must be refused: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "a")); err == nil {
		t.Fatal("something was written outside the sites folder")
	}
}

func TestBackupJobsValidateAndDescribeSchedules(t *testing.T) {
	h, s, _, _, _, _ := relocateFixture(t)
	dest, err := h.ensureLocalDestination()
	testhelpers.AssertNoError(t, err)
	post := func(freq string) int {
		body := `{"destination_id":` + fmt.Sprint(dest.ID) + `,"target_type":"all-sites","frequency":"` + freq + `"}`
		req := httptest.NewRequest("POST", "/api/backups/jobs", strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), ctxUserRoleKey, "admin"))
		rec := httptest.NewRecorder()
		h.BackupJobs(rec, req)
		return rec.Code
	}
	testhelpers.AssertEqual(t, post("daily@03:00;keep=7"), 200)
	testhelpers.AssertEqual(t, post("daily@25:99"), 400)
	testhelpers.AssertEqual(t, post("whenever"), 400)
	jobs, _ := s.GetBackupJobs()
	views := describeJobs(jobs)
	testhelpers.AssertEqual(t, len(views), 1)
	testhelpers.AssertEqual(t, views[0].ScheduleText, "every day at 03:00, keeping the last 7")
}
