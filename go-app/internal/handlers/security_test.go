package handlers

import (
	"context"
	"errors"
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

func TestDNSConfigSaveKeepsTokenAndAcceptsNumericEnabled(t *testing.T) {
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	defer s.Close()
	h := &Handlers{store: s}
	saved := "existing-token"
	testhelpers.AssertNoError(t, s.UpdateCloudflareConfig(&saved, nil, nil))

	post := func(body string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/dns/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), ctxUserIDKey, int64(1))
		ctx = context.WithValue(ctx, ctxUserRoleKey, "super_admin")
		rec := httptest.NewRecorder()
		h.DNSConfig(rec, req.WithContext(ctx))
		return rec.Code
	}

	// The web UI sends enabled as a number; that used to fail to decode.
	testhelpers.AssertEqual(t, http.StatusOK, post(`{"api_token":"","enabled":1}`))
	config, err := s.GetCloudflareConfig()
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, "existing-token", config.APIToken.String)
	testhelpers.AssertEqual(t, 1, config.Enabled)

	testhelpers.AssertEqual(t, http.StatusOK, post(`{"enabled":false}`))
	config, _ = s.GetCloudflareConfig()
	testhelpers.AssertEqual(t, "existing-token", config.APIToken.String)
	testhelpers.AssertEqual(t, 0, config.Enabled)
}

func TestServerServiceActionRejectsBadRequests(t *testing.T) {
	h := &Handlers{}
	call := func(role, body string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/server/services/action", strings.NewReader(body))
		ctx := context.WithValue(req.Context(), ctxUserIDKey, int64(1))
		ctx = context.WithValue(ctx, ctxUserRoleKey, role)
		rec := httptest.NewRecorder()
		h.ServerServiceAction(rec, req.WithContext(ctx))
		return rec.Code
	}

	testhelpers.AssertEqual(t, http.StatusForbidden, call("user", `{"service":"traefik","action":"stop"}`))
	testhelpers.AssertEqual(t, http.StatusBadRequest, call("admin", `{"service":"traefik","action":"explode"}`))
	testhelpers.AssertEqual(t, http.StatusBadRequest, call("admin", `{"service":"","action":"stop"}`))
	testhelpers.AssertEqual(t, http.StatusBadRequest, call("admin", `{"service":"mysql","action":"stop"}`))
}

func TestActionNotAllowedIsDistinguishable(t *testing.T) {
	err := notAllowed("nope")
	testhelpers.AssertTrue(t, errors.Is(err, errActionNotAllowed), "refusals wrap errActionNotAllowed")
	testhelpers.AssertFalse(t, errors.Is(errors.New("docker exploded"), errActionNotAllowed), "real failures don't")
}

func TestBootstrapTokenCreatedBeforeFirstUserStillActsAsSuperAdmin(t *testing.T) {
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	defer s.Close()
	h := &Handlers{store: s, token: "master-secret"}

	// The agent starts first: the token exists but there is no user yet.
	testhelpers.AssertNoError(t, EnsureBootstrapToken(s, "master-secret"))

	whoami := func() (int, string) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		req.Header.Set("Authorization", "Bearer master-secret")
		rec := httptest.NewRecorder()
		h.Auth(h.AuthMe)(rec, req)
		return rec.Code, rec.Body.String()
	}

	code, body := whoami()
	testhelpers.AssertEqual(t, http.StatusOK, code)
	testhelpers.AssertTrue(t, strings.Contains(body, `"role":"super_admin"`), "an unlinked token still reports its role: "+body)

	// The web app then creates the first super admin; the token now acts as them.
	owner, err := s.CreateUser("root", "rootpassword1", "super_admin", nil)
	testhelpers.AssertNoError(t, err)
	code, body = whoami()
	testhelpers.AssertEqual(t, http.StatusOK, code)
	testhelpers.AssertTrue(t, strings.Contains(body, `"username":"root"`), "token should act as the super admin: "+body)

	record, err := s.GetTokenByFingerprint(tokenFingerprint("master-secret"))
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertTrue(t, record.UserID != nil && *record.UserID == owner.ID, "the link should be saved")
}

func TestShellAccessIsSuperAdminOnlyAndValidatesNames(t *testing.T) {
	h := &Handlers{}
	call := func(role, method, body string) int {
		req := httptest.NewRequest(method, "/api/system/shell-access", strings.NewReader(body))
		ctx := context.WithValue(req.Context(), ctxUserIDKey, int64(1))
		ctx = context.WithValue(ctx, ctxUserRoleKey, role)
		rec := httptest.NewRecorder()
		h.ShellAccess(rec, req.WithContext(ctx))
		return rec.Code
	}

	for _, role := range []string{"user", "admin"} {
		testhelpers.AssertEqual(t, http.StatusForbidden, call(role, http.MethodGet, ""))
		testhelpers.AssertEqual(t, http.StatusForbidden, call(role, http.MethodPost, `{"username":"alice","action":"grant"}`))
	}
	// Super admin, but the request itself is bad: rejected before the helper is ever run.
	testhelpers.AssertEqual(t, http.StatusBadRequest, call("super_admin", http.MethodPost, `{"username":"Robert; rm -rf /","action":"grant"}`))
	testhelpers.AssertEqual(t, http.StatusBadRequest, call("super_admin", http.MethodPost, `{"username":"-aG","action":"grant"}`))
	testhelpers.AssertEqual(t, http.StatusBadRequest, call("super_admin", http.MethodPost, `{"username":"alice","action":"sudo"}`))
	testhelpers.AssertEqual(t, http.StatusMethodNotAllowed, call("super_admin", http.MethodDelete, ""))
}

func TestAuditWritesOneJSONLinePerEvent(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	testhelpers.AssertNoError(t, os.Chdir(dir))
	defer os.Chdir(old)

	h := &Handlers{}
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.RemoteAddr = "203.0.113.5:9999"
	ctx := context.WithValue(req.Context(), ctxUserIDKey, int64(7))
	ctx = context.WithValue(ctx, ctxUserRoleKey, "super_admin")
	h.audit(req.WithContext(ctx), "shell-access.grant", "alice", map[string]any{"ok": true})
	h.audit(req.WithContext(ctx), "shell-access.revoke", "alice", nil)

	data, err := os.ReadFile(filepath.Join(dir, "logs", "audit.log"))
	testhelpers.AssertNoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	testhelpers.AssertEqual(t, 2, len(lines))
	testhelpers.AssertTrue(t, strings.Contains(lines[0], `"actorId":7`) && strings.Contains(lines[0], `"target":"alice"`) && strings.Contains(lines[0], `"from":"203.0.113.5"`), "audit line has actor, target and source: "+lines[0])
}
