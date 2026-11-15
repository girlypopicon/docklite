package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docklite-agent/internal/testhelpers"
)

// adminJSON calls a handler as an admin and decodes its JSON answer.
func adminJSON(t *testing.T, fn http.HandlerFunc, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), ctxUserRoleKey, "admin"))
	rec := httptest.NewRecorder()
	fn(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestBackupPackageEndToEnd(t *testing.T) {
	h, s, _, _, site, _ := relocateFixture(t)
	testhelpers.AssertNoError(t, s.EnsureBackupPackageSchema())
	h.backupBaseDir = t.TempDir()
	db, err := s.UpsertDatabase("shopdb", "postgres", "c-shopdb", 5432, "")
	testhelpers.AssertNoError(t, err)

	// Link the database to the site; the picker then lists it under the site.
	code, _ := adminJSON(t, h.BackupLinks, "POST", "/api/backups/links", `{"database_id":`+itoa(db.ID)+`,"site_id":`+itoa(site.ID)+`}`)
	testhelpers.AssertEqual(t, 200, code)
	code, targets := adminJSON(t, h.BackupTargets, "GET", "/api/backups/targets", "")
	testhelpers.AssertEqual(t, 200, code)
	sites := targets["sites"].([]any)
	testhelpers.AssertEqual(t, 1, len(sites))
	testhelpers.AssertEqual(t, 1, len(sites[0].(map[string]any)["databases"].([]any)))

	// Back up the site and its databases, waiting for it. The database can't be dumped without Docker, so the package
	// holds the site and says the database was left out.
	code, out := adminJSON(t, h.BackupExport, "POST", "/api/backups/export",
		`{"selection":{"sites":[`+itoa(site.ID)+`],"site_databases":[`+itoa(site.ID)+`]},"delivery":"local","wait":true}`)
	testhelpers.AssertEqual(t, 200, code)
	file := out["file"].(string)
	testhelpers.AssertTrue(t, strings.HasPrefix(file, "packages/shop.example.com-") && strings.HasSuffix(file, ".litpkg"), "file: "+file)
	testhelpers.AssertEqual(t, "shop.example.com + 1 database", out["description"])
	testhelpers.AssertEqual(t, 1, len(out["problems"].([]any)))

	rec, err := s.GetBackupByID(int64(out["backup_id"].(float64)))
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, "success", rec.Status)
	testhelpers.AssertEqual(t, "shop.example.com + 1 database", rec.Scope)
	testhelpers.AssertEqual(t, "site", rec.TargetType)

	// It shows up in the file list, inspects as a package, and restores.
	files, err := h.listBackupFiles()
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, 1, len(files))
	code, info := adminJSON(t, h.BackupInspect, "GET", "/api/backups/inspect?file="+file, "")
	testhelpers.AssertEqual(t, 200, code)
	testhelpers.AssertEqual(t, true, info["package"])
	testhelpers.AssertEqual(t, true, info["verified"])
	testhelpers.AssertEqual(t, "shop.example.com", info["sites"].([]any)[0].(map[string]any)["domain"])

	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(site.CodePath, "index.html"), []byte("broken"), 0o644))
	code, res := adminJSON(t, h.BackupRestore, "POST", "/api/backups/restore", `{"file":"`+file+`"}`)
	testhelpers.AssertEqual(t, 200, code)
	testhelpers.AssertEqual(t, "shop.example.com", res["domain"])
	got, _ := os.ReadFile(filepath.Join(site.CodePath, "index.html"))
	testhelpers.AssertEqual(t, "hello", string(got))

	// Progress for a finished backup comes from the history once the tracker has nothing.
	code, prog := adminJSON(t, h.BackupProgress, "GET", "/api/backups/progress?id="+itoa(rec.ID), "")
	testhelpers.AssertEqual(t, 200, code)
	testhelpers.AssertEqual(t, "success", prog["status"])
}

func TestBackupScheduleWithSelection(t *testing.T) {
	h, s, _, _, site, _ := relocateFixture(t)
	testhelpers.AssertNoError(t, s.EnsureBackupPackageSchema())
	h.backupBaseDir = t.TempDir()

	code, _ := adminJSON(t, h.BackupJobs, "POST", "/api/backups/jobs",
		`{"selection":{"all_sites":true,"all_databases":true},"frequency":"daily@03:00;keep=7"}`)
	testhelpers.AssertEqual(t, 200, code)
	code, _ = adminJSON(t, h.BackupJobs, "POST", "/api/backups/jobs", `{"selection":{},"frequency":"daily@03:00"}`)
	testhelpers.AssertEqual(t, 400, code)

	jobs, _ := s.GetBackupJobs()
	testhelpers.AssertEqual(t, 1, len(jobs))
	testhelpers.AssertEqual(t, "all-sites", jobs[0].TargetType) // what an older DockLite would do with it
	views := h.describeJobs(jobs)
	testhelpers.AssertEqual(t, "All sites + all databases", views[0].What)

	// Editing the selection changes what it backs up.
	code, _ = adminJSON(t, h.BackupJobs, "PUT", "/api/backups/jobs", `{"id":`+itoa(jobs[0].ID)+`,"selection":{"sites":[`+itoa(site.ID)+`]}}`)
	testhelpers.AssertEqual(t, 200, code)
	jobs, _ = s.GetBackupJobs()
	testhelpers.AssertEqual(t, "site", jobs[0].TargetType)
	testhelpers.AssertEqual(t, "shop.example.com", h.describeJobs(jobs)[0].What)
}
