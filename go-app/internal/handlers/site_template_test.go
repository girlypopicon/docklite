package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func TestMissingEntryFile(t *testing.T) {
	dir := t.TempDir()
	if w := missingEntryFile(dir, "node"); !strings.Contains(w, "index.js") {
		t.Fatalf("node without index.js: %q", w)
	}
	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(dir, "index.js"), []byte("x"), 0o644))
	if w := missingEntryFile(dir, "node"); w != "" {
		t.Fatalf("node with index.js: %q", w)
	}
	if w := missingEntryFile(dir, "php"); !strings.Contains(w, "index.php") {
		t.Fatalf("php: %q", w)
	}
}

func TestChangeSiteTemplateChecksBeforeTouchingDocker(t *testing.T) {
	h, s, alice, bob := groupFixture(t)
	cid := "abc123"
	site, err := s.CreateSite(store.SiteRecord{Domain: "a.example.com", UserID: alice.ID, TemplateType: "static", CodePath: t.TempDir()})
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertNoError(t, s.UpdateSiteContainerID(site.ID, &cid))

	call := func(role string, uid int64, body, container string) int {
		req := asRole(httptest.NewRequest(http.MethodPost, "/api/containers/"+container+"/template", strings.NewReader(body)), uid, role)
		rec := httptest.NewRecorder()
		h.ChangeSiteTemplate(rec, req, container)
		return rec.Code
	}
	if c := call("user", alice.ID, `{"template_type":"ruby"}`, cid); c != http.StatusBadRequest {
		t.Errorf("unknown type: %d", c)
	}
	if c := call("user", alice.ID, `{"template_type":"node"}`, "nope"); c != http.StatusNotFound {
		t.Errorf("not a site: %d", c)
	}
	if c := call("user", bob.ID, `{"template_type":"node"}`, cid); c != http.StatusForbidden {
		t.Errorf("someone else's site: %d", c)
	}
	rec := httptest.NewRecorder()
	h.ChangeSiteTemplate(rec, httptest.NewRequest(http.MethodGet, "/", nil), cid)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: %d", rec.Code)
	}
}
