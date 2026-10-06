package handlers

import (
	"os"
	"path/filepath"
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
