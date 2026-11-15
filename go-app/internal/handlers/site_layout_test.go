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

func TestPlanSiteLayout(t *testing.T) {
	base := t.TempDir()
	old := siteBaseDir
	siteBaseDir = base
	defer func() { siteBaseDir = old }()

	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	defer s.Close()
	h := &Handlers{store: s}

	admin, err := s.CreateUser("admin", "adminpassword1", "super_admin", nil)
	testhelpers.AssertNoError(t, err)

	mk := func(rel string) string {
		p := filepath.Join(base, rel)
		testhelpers.AssertNoError(t, os.MkdirAll(p, 0o755))
		testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(p, "index.html"), []byte("x"), 0o644))
		return p
	}
	flat := mk("flat.com")
	good := mk("admin/good.com")
	mk("stray.org")
	for _, rec := range []store.SiteRecord{
		{Domain: "flat.com", UserID: admin.ID, TemplateType: "static", CodePath: flat, Status: "running"},
		{Domain: "good.com", UserID: admin.ID, TemplateType: "static", CodePath: good, Status: "running"},
		{Domain: "gone.com", UserID: admin.ID, TemplateType: "static", CodePath: filepath.Join(base, "gone.com"), Status: "stopped"},
	} {
		_, err := s.CreateSite(rec)
		testhelpers.AssertNoError(t, err)
	}

	rep, err := h.planSiteLayout()
	testhelpers.AssertNoError(t, err)
	got := map[string]layoutItem{}
	for _, it := range rep.Items {
		got[it.Domain] = it
	}
	testhelpers.AssertEqual(t, got["flat.com"].Action, layoutMove)
	testhelpers.AssertEqual(t, got["flat.com"].To, filepath.Join(base, "admin", "flat.com"))
	testhelpers.AssertEqual(t, got["good.com"].Action, layoutOK)
	testhelpers.AssertEqual(t, got["gone.com"].Action, layoutSkip)
	testhelpers.AssertEqual(t, rep.ToMove, 1)
	testhelpers.AssertEqual(t, len(rep.Strays), 1) // stray.org; flat.com is claimed, "admin" is a user dir

	// Planning must not touch the disk.
	if _, err := os.Stat(filepath.Join(base, "admin", "flat.com")); err == nil {
		t.Fatal("plan created the target folder")
	}
}

func TestApplyLayoutMoveTrashesOldFolder(t *testing.T) {
	base := t.TempDir()
	old := siteBaseDir
	siteBaseDir = base
	defer func() { siteBaseDir = old }()

	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	defer s.Close()
	h := &Handlers{store: s}
	admin, err := s.CreateUser("admin", "adminpassword1", "super_admin", nil)
	testhelpers.AssertNoError(t, err)
	flat := filepath.Join(base, "flat.com")
	testhelpers.AssertNoError(t, os.MkdirAll(flat, 0o755))
	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(flat, "index.html"), []byte("x"), 0o644))
	_, err = s.CreateSite(store.SiteRecord{Domain: "flat.com", UserID: admin.ID, TemplateType: "static", CodePath: flat, Status: "stopped"})
	testhelpers.AssertNoError(t, err)

	rr := httptest.NewRecorder()
	h.SiteLayout(rr, adminReq("POST", `{"apply":true}`))
	testhelpers.AssertEqual(t, rr.Code, http.StatusOK)
	var rep layoutReport
	testhelpers.AssertNoError(t, json.Unmarshal(rr.Body.Bytes(), &rep))
	testhelpers.AssertEqual(t, rep.Items[0].Action, layoutMoved)
	if rep.Items[0].OldFolder == "" || !strings.HasPrefix(rep.Items[0].OldFolder, filepath.Join(base, ".trash")) {
		t.Fatalf("old folder should be in the trash, got %q", rep.Items[0].OldFolder)
	}
	got, _ := os.ReadFile(filepath.Join(base, "admin", "flat.com", "index.html"))
	testhelpers.AssertEqual(t, string(got), "x")
	if _, err := os.Stat(flat); err == nil {
		t.Fatal("the old flat folder should no longer be in place")
	}
	kept, _ := os.ReadFile(filepath.Join(rep.Items[0].OldFolder, "index.html"))
	testhelpers.AssertEqual(t, string(kept), "x") // trashed, not deleted
}
