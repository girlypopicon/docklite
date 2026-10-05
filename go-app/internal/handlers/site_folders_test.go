package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func adminReq(method, body string) *http.Request {
	r := httptest.NewRequest(method, "/x", strings.NewReader(body))
	return r.WithContext(context.WithValue(r.Context(), ctxUserRoleKey, "super_admin"))
}

func TestScanAdoptAndTrashFolders(t *testing.T) {
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

	mk := func(rel, file string) string {
		p := filepath.Join(base, rel)
		testhelpers.AssertNoError(t, os.MkdirAll(p, 0o755))
		testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(p, file), []byte("x"), 0o644))
		return p
	}
	flat := mk("flat.com", "index.php")
	reg := mk("admin/reg.com", "index.html")
	mk("junk", "a.txt")
	_, err = s.CreateSite(store.SiteRecord{Domain: "reg.com", UserID: admin.ID, TemplateType: "static", CodePath: reg})
	testhelpers.AssertNoError(t, err)

	rep, err := h.scanFolders(context.Background())
	testhelpers.AssertNoError(t, err)
	st := map[string]folderEntry{}
	for _, f := range rep.Folders {
		st[f.Name] = f
	}
	testhelpers.AssertEqual(t, st["admin"].Status, "user-dir")
	testhelpers.AssertEqual(t, st["reg.com"].Status, "registered")
	testhelpers.AssertEqual(t, st["flat.com"].Status, "unregistered")
	testhelpers.AssertEqual(t, st["flat.com"].DetectedTyp, "php")
	testhelpers.AssertEqual(t, st["junk"].Status, "unknown")

	// Registered folders and the root can't be trashed.
	rr := httptest.NewRecorder()
	h.TrashFolder(rr, adminReq("POST", `{"path":"`+reg+`"}`))
	testhelpers.AssertEqual(t, rr.Code, http.StatusConflict)
	rr = httptest.NewRecorder()
	h.TrashFolder(rr, adminReq("POST", `{"path":"`+base+`"}`))
	testhelpers.AssertEqual(t, rr.Code, http.StatusBadRequest)
	rr = httptest.NewRecorder()
	h.TrashFolder(rr, adminReq("POST", `{"path":"`+base+`/../etc"}`))
	testhelpers.AssertEqual(t, rr.Code, http.StatusBadRequest)

	// Adopt needs an owner, then registers without touching files.
	rr = httptest.NewRecorder()
	h.AdoptFolder(rr, adminReq("POST", `{"path":"`+flat+`"}`))
	testhelpers.AssertEqual(t, rr.Code, http.StatusBadRequest)
	rr = httptest.NewRecorder()
	h.AdoptFolder(rr, adminReq("POST", `{"path":"`+flat+`","user_id":`+strconv.FormatInt(admin.ID, 10)+`}`))
	testhelpers.AssertEqual(t, rr.Code, http.StatusCreated)
	rec, _ := s.GetSiteByDomain("flat.com")
	testhelpers.AssertEqual(t, rec.TemplateType, "php")
	if _, err := os.Stat(filepath.Join(flat, ".dkl")); err != nil {
		t.Fatal("adopt did not write a .dkl")
	}

	// An orphan goes to the trash, not the void.
	junk := filepath.Join(base, "junk")
	rr = httptest.NewRecorder()
	h.TrashFolder(rr, adminReq("POST", `{"path":"`+junk+`"}`))
	testhelpers.AssertEqual(t, rr.Code, http.StatusOK)
	if _, err := os.Stat(junk); err == nil {
		t.Fatal("junk still in place")
	}
	m, _ := filepath.Glob(filepath.Join(base, ".trash", "*-junk"))
	testhelpers.AssertEqual(t, len(m), 1)
}

func itoa(n int64) string {
	return strings.TrimSpace(strings.Repeat(" ", 0) + func() string { return strconv.FormatInt(n, 10) }())
}
