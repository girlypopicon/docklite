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
	"time"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

// backupFixture is a store with a real site folder on disk and a backup base dir.
func backupFixture(t *testing.T) (*Handlers, *store.SQLiteStore, int64, string) {
	t.Helper()
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	t.Cleanup(func() { s.Close() })

	// The shared test schema's sites table predates code_path/status/folder_id.
	if _, err := db.Exec(`DROP TABLE sites`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE sites (
		id INTEGER PRIMARY KEY AUTOINCREMENT, domain TEXT UNIQUE NOT NULL, user_id INTEGER NOT NULL,
		container_id TEXT, template_type TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		code_path TEXT, status TEXT, folder_id INTEGER)`); err != nil {
		t.Fatal(err)
	}

	siteDir := filepath.Join(t.TempDir(), "example.com")
	if err := os.MkdirAll(siteDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "index.html"), []byte("<h1>hi</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	site, err := s.CreateSite(store.SiteRecord{Domain: "example.com", UserID: 1, TemplateType: "static", CodePath: siteDir})
	if err != nil {
		t.Fatal(err)
	}

	base := t.TempDir()
	return &Handlers{store: s, backupBaseDir: base}, s, site.ID, base
}

func adminRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	ctx := context.WithValue(req.Context(), ctxUserIDKey, int64(1))
	ctx = context.WithValue(ctx, ctxUserRoleKey, "admin")
	return req.WithContext(ctx)
}

func pollProgress(t *testing.T, h *Handlers, id int64) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		rec := httptest.NewRecorder()
		h.BackupProgress(rec, adminRequest(http.MethodGet, "/api/backups/progress?id="+itoa64(id), ""))
		var p map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &p)
		if status, _ := p["status"].(string); status == "success" || status == "failed" {
			return p
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("backup never finished")
	return nil
}

func itoa64(n int64) string { return strings.TrimSpace(strings.Replace(jsonNumber(n), "\"", "", -1)) }

func jsonNumber(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestSiteBackupRunsInTheBackgroundAndReportsVerifiedSuccess(t *testing.T) {
	h, s, siteID, base := backupFixture(t)

	rec := httptest.NewRecorder()
	h.BackupExport(rec, adminRequest(http.MethodPost, "/api/backups/export",
		`{"target_type":"site","target_id":`+itoa64(siteID)+`,"delivery":"local","notes":"before the redesign"}`))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("export should answer 202 immediately, got %d: %s", rec.Code, rec.Body.String())
	}
	var started struct {
		BackupID int64 `json:"backup_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &started)
	if started.BackupID == 0 {
		t.Fatal("no backup id returned")
	}

	done := pollProgress(t, h, started.BackupID)
	if done["status"] != "success" || done["verified"] != true || done["percent"].(float64) != 100 {
		t.Fatalf("expected verified success, got %v", done)
	}
	if !strings.HasSuffix(done["file_name"].(string), ".tar.gz") || done["size"].(float64) <= 0 {
		t.Fatalf("result should name the file and its size: %v", done)
	}

	record, _ := s.GetBackupByID(started.BackupID)
	if record.Status != "success" || !strings.HasPrefix(record.BackupPath, base) {
		t.Fatalf("the saved record must agree with the progress: %+v", record)
	}
	if _, err := os.Stat(record.BackupPath); err != nil {
		t.Fatalf("the backup file must exist: %v", err)
	}
	if _, err := os.Stat(record.BackupPath[:len(record.BackupPath)-len(".tar.gz")] + ".manifest.json"); err != nil {
		t.Fatalf("a manifest should sit beside the backup: %v", err)
	}
}

func TestBackupOfMissingSiteFolderFailsInPlainLanguage(t *testing.T) {
	h, s, siteID, _ := backupFixture(t)
	site, _ := s.GetSiteByID(siteID)
	_ = os.RemoveAll(site.CodePath)

	rec := httptest.NewRecorder()
	h.BackupExport(rec, adminRequest(http.MethodPost, "/api/backups/export",
		`{"target_type":"site","target_id":`+itoa64(siteID)+`,"delivery":"local"}`))
	var started struct {
		BackupID int64 `json:"backup_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &started)

	done := pollProgress(t, h, started.BackupID)
	if done["status"] != "failed" || !strings.Contains(done["error"].(string), "weren't found") {
		t.Fatalf("expected a plain-language failure, got %v", done)
	}
	record, _ := s.GetBackupByID(started.BackupID)
	if record.Status != "failed" || record.ErrorMessage == nil {
		t.Fatalf("the record must say it failed and why: %+v", record)
	}
}

func TestProgressForUnknownBackupAndInterruptedRecords(t *testing.T) {
	h, s, siteID, _ := backupFixture(t)

	rec := httptest.NewRecorder()
	h.BackupProgress(rec, adminRequest(http.MethodGet, "/api/backups/progress?id=99999", ""))
	testhelpers.AssertEqual(t, http.StatusNotFound, rec.Code)

	// A record left "in_progress" by a process that died is reported as failed, not as running forever.
	dest, err := h.ensureLocalDestination()
	testhelpers.AssertNoError(t, err)
	id, err := s.CreateBackup(store.BackupRecord{Destination: dest.ID, TargetType: "site", TargetID: siteID, Status: "in_progress"})
	testhelpers.AssertNoError(t, err)
	done := pollProgress(t, h, id)
	if done["status"] != "failed" {
		t.Fatalf("an orphaned in_progress record must read as failed: %v", done)
	}

	n, err := s.FailStaleBackups("restarted")
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, int64(1), n)
	record, _ := s.GetBackupByID(id)
	testhelpers.AssertEqual(t, "failed", record.Status)
}
