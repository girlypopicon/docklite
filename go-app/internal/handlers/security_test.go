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

func TestIsValidDomainRejectsNginxInjection(t *testing.T) {
	valid := []string{"example.com", "www.example.com", "a-b.example.co.uk", "x1.io"}
	for _, d := range valid {
		testhelpers.AssertTrue(t, isValidDomain(d), "expected valid: "+d)
	}

	invalid := []string{
		"", "..", ".", "-example.com", "example-.com", "exa mple.com",
		"example.com;", "evil.com; location / { return 200; }", "a.com\nserver {",
		"a/b.com", `a\b.com`, "a..com", strings.Repeat("a", 64) + ".com",
	}
	for _, d := range invalid {
		testhelpers.AssertFalse(t, isValidDomain(d), "expected invalid: "+d)
	}
}

func TestSetupNginxForDomainRejectsInvalidDomain(t *testing.T) {
	err := setupNginxForDomain("evil.com; include x", false, 8080)
	testhelpers.AssertErrorContains(t, err, "invalid domain")
}

func TestUserPasswordAdminCannotResetSuperAdmin(t *testing.T) {
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	defer s.Close()
	h := &Handlers{store: s}

	superAdmin, err := s.CreateUser("root", "superpassword1", "super_admin", nil)
	testhelpers.AssertNoError(t, err)
	admin, err := s.CreateUser("ops", "adminpassword1", "admin", &superAdmin.ID)
	testhelpers.AssertNoError(t, err)
	plain, err := s.CreateUser("bob", "userpassword1", "user", &admin.ID)
	testhelpers.AssertNoError(t, err)

	reset := func(actor *store.UserRecord, role string, targetID int64) int {
		body := `{"userId":` + strconv.FormatInt(targetID, 10) + `,"newPassword":"brandnewpass1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/users/password", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), ctxUserIDKey, actor.ID)
		ctx = context.WithValue(ctx, ctxUserRoleKey, role)
		rec := httptest.NewRecorder()
		h.UserPassword(rec, req.WithContext(ctx))
		return rec.Code
	}

	testhelpers.AssertEqual(t, http.StatusForbidden, reset(admin, "admin", superAdmin.ID))
	testhelpers.AssertEqual(t, http.StatusOK, reset(admin, "admin", plain.ID))
	testhelpers.AssertEqual(t, http.StatusOK, reset(superAdmin, "super_admin", admin.ID))
}

func TestClientIPIgnoresSpoofedHeadersFromNonProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "203.0.113.9:4444"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "5.6.7.8")
	testhelpers.AssertEqual(t, "203.0.113.9", clientIP(req))
}

func TestClientIPTrustsLocalProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "127.0.0.1:5555"
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 198.51.100.7")
	testhelpers.AssertEqual(t, "198.51.100.7", clientIP(req))

	req.Header.Set("X-Real-IP", "198.51.100.8")
	testhelpers.AssertEqual(t, "198.51.100.8", clientIP(req))
}

func TestIsWithinResolvesSymlinks(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(base, "escape")); err != nil {
		t.Fatal(err)
	}
	testhelpers.AssertTrue(t, isWithin(base, filepath.Join(base, "site", "index.html")), "new path inside base")
	testhelpers.AssertFalse(t, isWithin(base, filepath.Join(base, "escape")), "symlink out of base")
	testhelpers.AssertFalse(t, isWithin(base, filepath.Join(base, "escape", "new.txt")), "path under symlink out of base")
}

func TestCSRFMiddlewareBrowserSession(t *testing.T) {
	called := false
	handler := CSRFMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	send := func(origin string) int {
		called = false
		req := httptest.NewRequest(http.MethodPost, "http://74.208.249.126/api/containers/abc/stop", nil)
		req.Host = "74.208.249.126"
		req.AddCookie(&http.Cookie{Name: delegationCookieName, Value: "session"})
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec.Code
	}

	// Same-origin UI request with no CSRF token: allowed.
	testhelpers.AssertEqual(t, http.StatusOK, send("http://74.208.249.126"))
	testhelpers.AssertTrue(t, called, "same-origin request reaches handler")

	// Repeated requests keep working (tokens used to be single-use).
	testhelpers.AssertEqual(t, http.StatusOK, send("http://74.208.249.126"))

	// Cross-site and origin-less cookie requests: rejected.
	testhelpers.AssertEqual(t, http.StatusForbidden, send("https://evil.example"))
	testhelpers.AssertFalse(t, called, "cross-site request blocked")
	testhelpers.AssertEqual(t, http.StatusForbidden, send(""))
	testhelpers.AssertFalse(t, called, "origin-less cookie request blocked")
}

func TestIsSecretColumn(t *testing.T) {
	for _, c := range []string{"password_hash", "token_hash", "token_fingerprint", "api_token", "session_secret", "API_KEY"} {
		testhelpers.AssertTrue(t, isSecretColumn(c), "expected secret: "+c)
	}
	for _, c := range []string{"id", "username", "domain", "created_at", "role"} {
		testhelpers.AssertFalse(t, isSecretColumn(c), "expected visible: "+c)
	}
}
