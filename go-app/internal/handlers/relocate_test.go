package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func relocateFixture(t *testing.T) (*Handlers, *store.SQLiteStore, store.UserRecord, store.UserRecord, store.SiteRecord, string) {
	t.Helper()
	base := t.TempDir()
	old := siteBaseDir
	siteBaseDir = base
	t.Cleanup(func() { siteBaseDir = old })
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	t.Cleanup(func() { s.Close() })
	alice, err := s.CreateUser("alice", "alicepassword1", "user", nil)
	testhelpers.AssertNoError(t, err)
	bob, err := s.CreateUser("bob", "bobpassword1", "user", nil)
	testhelpers.AssertNoError(t, err)
	src := filepath.Join(base, "alice", "shop.example.com")
	testhelpers.AssertNoError(t, os.MkdirAll(filepath.Join(src, "assets", "empty-dir"), 0o755))
	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(src, "index.html"), []byte("hello"), 0o644))
	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(src, "assets", "app.js"), []byte("js"), 0o644))
	testhelpers.AssertNoError(t, os.Symlink("index.html", filepath.Join(src, "home")))
	// Modes an older site (or a file written by the container) might have: no group write anywhere.
	testhelpers.AssertNoError(t, os.Chmod(filepath.Join(src, "assets"), 0o755))
	testhelpers.AssertNoError(t, os.Chmod(filepath.Join(src, "index.html"), 0o644))
	site, err := s.CreateSite(store.SiteRecord{Domain: "shop.example.com", UserID: alice.ID, TemplateType: "static", CodePath: src, Status: "stopped"})
	testhelpers.AssertNoError(t, err)
	return &Handlers{store: s}, s, *alice, *bob, *site, base
}

func TestRelocateSiteCopiesFaithfullyAndNeverDeletes(t *testing.T) {
	h, s, _, bob, site, base := relocateFixture(t)
	from := site.CodePath
	to := filepath.Join(base, "bob", "shop.example.com")

	testhelpers.AssertNoError(t, h.relocateSite(&site, bob.ID, "bob", from, to))

	got, _ := os.ReadFile(filepath.Join(to, "assets", "app.js"))
	testhelpers.AssertEqual(t, string(got), "js")
	if _, err := os.Stat(filepath.Join(to, "assets", "empty-dir")); err != nil {
		t.Fatal("empty folders must be copied too")
	}
	link, err := os.Readlink(filepath.Join(to, "home"))
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, link, "index.html") // a link stays a link, it isn't turned into a copy of its target
	if _, err := os.Stat(filepath.Join(from, "index.html")); err != nil {
		t.Fatal("the old folder must be left exactly as it was")
	}
	// The new owner must be able to write everything: group write on files, 2775 on folders (the dashboard and the
	// container edit as the docklite user/group). The old folder's modes were 644/755.
	mode := func(p string) os.FileMode { st, _ := os.Stat(p); return st.Mode() }
	testhelpers.AssertEqual(t, mode(filepath.Join(to, "index.html")).Perm(), os.FileMode(0o664))
	testhelpers.AssertEqual(t, mode(filepath.Join(to, "assets", "app.js")).Perm(), os.FileMode(0o664))
	for _, d := range []string{to, filepath.Join(to, "assets"), filepath.Join(to, "assets", "empty-dir"), filepath.Join(base, "bob")} {
		if m := mode(d); m.Perm() != 0o775 || m&os.ModeSetgid == 0 {
			t.Errorf("%s should be 2775 for the new owner, got %v", d, m)
		}
	}
	testhelpers.AssertEqual(t, mode(filepath.Join(from, "index.html")).Perm(), os.FileMode(0o644)) // old folder untouched
	rec, _ := s.GetSiteByID(site.ID)
	testhelpers.AssertEqual(t, rec.UserID, bob.ID)
	testhelpers.AssertEqual(t, rec.CodePath, to)
}

func TestRelocateSiteRefusesToOverwriteOrLeaveTheSitesRoot(t *testing.T) {
	h, s, alice, bob, site, base := relocateFixture(t)
	to := filepath.Join(base, "bob", "shop.example.com")
	testhelpers.AssertNoError(t, os.MkdirAll(to, 0o755))
	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(to, "precious.txt"), []byte("mine"), 0o644))

	err := h.relocateSite(&site, bob.ID, "bob", site.CodePath, to)
	testhelpers.AssertErrorContains(t, err, "already has files")
	kept, _ := os.ReadFile(filepath.Join(to, "precious.txt"))
	testhelpers.AssertEqual(t, string(kept), "mine")
	rec, _ := s.GetSiteByID(site.ID)
	testhelpers.AssertEqual(t, rec.UserID, alice.ID) // nothing changed in the database

	outside := t.TempDir()
	err = h.relocateSite(&site, bob.ID, "bob", site.CodePath, filepath.Join(outside, "x"))
	testhelpers.AssertErrorContains(t, err, "outside")
}

func TestMoveToTrashNeverDeletes(t *testing.T) {
	base := t.TempDir()
	old := siteBaseDir
	siteBaseDir = base
	defer func() { siteBaseDir = old }()
	src := filepath.Join(base, "alice", "old.example.com")
	testhelpers.AssertNoError(t, os.MkdirAll(src, 0o755))
	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("keep me"), 0o644))

	dest, err := moveToTrash(src)
	testhelpers.AssertNoError(t, err)
	got, _ := os.ReadFile(filepath.Join(dest, "a.txt"))
	testhelpers.AssertEqual(t, string(got), "keep me")
	if _, err := os.Stat(src); err == nil {
		t.Fatal("the folder should have been moved away")
	}
	for _, bad := range []string{base, "/etc", filepath.Join(base, ".trash"), filepath.Join(base, "a", "b", "c")} {
		if _, err := moveToTrash(bad); err == nil {
			t.Errorf("moveToTrash must refuse %q", bad)
		}
	}
}

func TestDeletingAUserMovesTheirSitesToTheNewOwner(t *testing.T) {
	h, s, alice, bob, site, base := relocateFixture(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/users?id="+strconv.FormatInt(alice.ID, 10), nil)
	ctx := context.WithValue(req.Context(), ctxUserIDKey, bob.ID)
	ctx = context.WithValue(ctx, ctxUserRoleKey, "super_admin")
	rec := httptest.NewRecorder()
	h.Users(rec, req.WithContext(ctx))
	testhelpers.AssertEqual(t, rec.Code, http.StatusOK)
	var resp struct {
		SitesMovedTo string   `json:"sites_moved_to"`
		Warnings     []string `json:"warnings"`
	}
	testhelpers.AssertNoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	testhelpers.AssertEqual(t, resp.SitesMovedTo, "bob")
	if len(resp.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", resp.Warnings)
	}

	moved, _ := s.GetSiteByID(site.ID)
	testhelpers.AssertEqual(t, moved.UserID, bob.ID)
	testhelpers.AssertEqual(t, moved.CodePath, filepath.Join(base, "bob", "shop.example.com"))
	got, _ := os.ReadFile(filepath.Join(moved.CodePath, "assets", "app.js"))
	testhelpers.AssertEqual(t, string(got), "js")
	if _, err := os.Stat(site.CodePath); err == nil {
		t.Fatal("the site's old folder should have left alice's folder")
	}
	if _, err := os.Stat(filepath.Join(base, "alice")); err == nil {
		t.Fatal("alice's now-empty user folder should be gone")
	}
	trashed, _ := filepath.Glob(filepath.Join(base, ".trash", "*-alice__shop.example.com"))
	testhelpers.AssertEqual(t, len(trashed), 1) // the old copy is in the trash, not deleted
	if u, _ := s.GetUserByIDFull(alice.ID); u != nil {
		t.Fatal("alice should be deleted")
	}
}
