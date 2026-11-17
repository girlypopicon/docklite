package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func TestBackupEstimate(t *testing.T) {
	h, s, alice, _ := groupFixture(t)
	testhelpers.AssertNoError(t, s.EnsureBackupPackageSchema())
	dir := t.TempDir()
	h.backupBaseDir = dir
	site := filepath.Join(dir, "site")
	testhelpers.AssertNoError(t, os.MkdirAll(site, 0o755))
	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(site, "a.bin"), make([]byte, 100_000), 0o644))
	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(site, "b.bin"), make([]byte, 20_000), 0o644))
	if _, err := s.CreateSite(store.SiteRecord{Domain: "example.com", UserID: alice.ID, TemplateType: "static", CodePath: site}); err != nil {
		t.Fatal(err)
	}

	post := func(role, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/backups/estimate", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.BackupEstimate(rec, asRole(req, alice.ID, role))
		return rec
	}

	rec := post("admin", `{"selection":{"all_sites":true}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		RawBytes      int64 `json:"raw_bytes"`
		EstimateBytes int64 `json:"estimate_bytes"`
		Items         []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	testhelpers.AssertNoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	if out.RawBytes != 120_000 {
		t.Errorf("raw bytes = %d, want 120000", out.RawBytes)
	}
	if out.EstimateBytes <= 0 || out.EstimateBytes >= out.RawBytes {
		t.Errorf("a compressed estimate should be between 0 and the raw size, got %d", out.EstimateBytes)
	}
	if len(out.Items) != 1 || out.Items[0].Name != "example.com" {
		t.Errorf("items = %+v", out.Items)
	}

	// An empty selection is an empty answer, and a plain user may not size whole sites.
	if rec := post("admin", `{"selection":{}}`); rec.Code != http.StatusOK {
		t.Errorf("empty selection: %d", rec.Code)
	}
	if rec := post("user", `{"selection":{"all_sites":true}}`); rec.Code != http.StatusForbidden {
		t.Errorf("user sizing all sites: %d, want 403", rec.Code)
	}
}
