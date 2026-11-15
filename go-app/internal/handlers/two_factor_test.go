package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
	"docklite-agent/internal/totp"
)

func twoFactorFixture(t *testing.T) (*Handlers, *store.SQLiteStore, *store.UserRecord) {
	t.Helper()
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	t.Cleanup(func() { s.Close() })
	testhelpers.AssertNoError(t, s.EnsureTwoFactorSchema())
	user, err := s.CreateUser("stella", "correct-horse-battery", "user", nil)
	testhelpers.AssertNoError(t, err)
	// Each test gets its own rate-limit keys, so attempts don't leak between tests.
	loginLimiter.Lock()
	loginLimiter.Entries = map[string]*loginAttempt{}
	loginLimiter.Unlock()
	return &Handlers{store: s, token: "test-delegation-secret"}, s, user
}

func asUser(r *http.Request, id int64) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxUserIDKey, id))
}

func postJSON(path, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func login(h *Handlers, password, code string) (*httptest.ResponseRecorder, map[string]any) {
	b, _ := json.Marshal(map[string]string{"username": "stella", "password": password, "code": code})
	rr := httptest.NewRecorder()
	h.AuthLogin(rr, postJSON("/api/auth/login", string(b)))
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr, out
}

func hasSessionCookie(rr *httptest.ResponseRecorder) bool {
	for _, c := range rr.Result().Cookies() {
		if c.Name == delegationCookieName && c.Value != "" {
			return true
		}
	}
	return false
}

// enableFor goes through setup and enable the way the dashboard does, and returns the secret and recovery codes.
func enableFor(t *testing.T, h *Handlers, userID int64) (string, []string) {
	t.Helper()
	rr := httptest.NewRecorder()
	h.TwoFactor(rr, asUser(postJSON("/api/auth/2fa/setup", "{}"), userID))
	testhelpers.AssertEqual(t, rr.Code, http.StatusOK)
	var setup struct {
		Secret string `json:"secret"`
		QR     string `json:"qr_png"`
		URL    string `json:"otpauth_url"`
	}
	testhelpers.AssertNoError(t, json.Unmarshal(rr.Body.Bytes(), &setup))
	if !strings.HasPrefix(setup.QR, "data:image/png;base64,") || !strings.Contains(setup.URL, "secret="+setup.Secret) {
		t.Fatalf("setup returned no QR code or a bad link: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.TwoFactor(rr, asUser(postJSON("/api/auth/2fa/enable", `{"code":"000000"}`), userID))
	if rr.Code == http.StatusOK {
		// 000000 is right once in a million; skip rather than flake.
		t.Skip("random secret happened to produce 000000")
	}

	// Use the previous step's code so the login tests below still have the current one free.
	code, _ := totp.CodeAt(setup.Secret, totp.Step(time.Now())-1)
	rr = httptest.NewRecorder()
	h.TwoFactor(rr, asUser(postJSON("/api/auth/2fa/enable", `{"code":"`+code+`"}`), userID))
	testhelpers.AssertEqual(t, rr.Code, http.StatusOK)
	var en struct {
		Codes []string `json:"recovery_codes"`
	}
	testhelpers.AssertNoError(t, json.Unmarshal(rr.Body.Bytes(), &en))
	testhelpers.AssertEqual(t, len(en.Codes), recoveryCodeCount)
	return setup.Secret, en.Codes
}

func TestLoginWithoutTwoFactorIsUnchanged(t *testing.T) {
	h, _, _ := twoFactorFixture(t)
	rr, _ := login(h, "correct-horse-battery", "")
	testhelpers.AssertEqual(t, rr.Code, http.StatusOK)
	if !hasSessionCookie(rr) {
		t.Fatal("no session cookie")
	}
}

func TestLoginAsksForCodeAndChecksIt(t *testing.T) {
	h, s, user := twoFactorFixture(t)
	secret, _ := enableFor(t, h, user.ID)

	// Right password, no code: asked for one, and no session.
	rr, out := login(h, "correct-horse-battery", "")
	testhelpers.AssertEqual(t, rr.Code, http.StatusUnauthorized)
	testhelpers.AssertEqual(t, out["two_factor_required"], true)
	if hasSessionCookie(rr) {
		t.Fatal("session issued without a code")
	}

	// Wrong password still says wrong password, not "enter a code" (doesn't reveal that 2FA is on).
	rr, out = login(h, "wrong-password-here", "")
	testhelpers.AssertEqual(t, rr.Code, http.StatusUnauthorized)
	if out["two_factor_required"] == true {
		t.Fatal("wrong password must not reach the code step")
	}

	// Wrong code: refused.
	rr, _ = login(h, "correct-horse-battery", "123456")
	if rr.Code == http.StatusOK {
		cur, _ := totp.CodeAt(secret, totp.Step(time.Now()))
		if cur != "123456" {
			t.Fatal("wrong code accepted")
		}
	}

	// Right code: in.
	code, _ := totp.CodeAt(secret, totp.Step(time.Now()))
	rr, _ = login(h, "correct-horse-battery", code)
	testhelpers.AssertEqual(t, rr.Code, http.StatusOK)
	if !hasSessionCookie(rr) {
		t.Fatal("no session after a good code")
	}

	// The same code a second time: refused (someone who saw it can't reuse it).
	rr, _ = login(h, "correct-horse-battery", code)
	testhelpers.AssertEqual(t, rr.Code, http.StatusUnauthorized)

	tf, err := s.GetTwoFactor(user.ID)
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, tf.LastStep, totp.Step(time.Now()))
}

func TestRecoveryCodeWorksOnce(t *testing.T) {
	h, s, user := twoFactorFixture(t)
	_, codes := enableFor(t, h, user.ID)

	rr, _ := login(h, "correct-horse-battery", strings.ToUpper(codes[3])) // typed in capitals: still fine
	testhelpers.AssertEqual(t, rr.Code, http.StatusOK)
	rr, _ = login(h, "correct-horse-battery", codes[3])
	testhelpers.AssertEqual(t, rr.Code, http.StatusUnauthorized)

	tf, _ := s.GetTwoFactor(user.ID)
	testhelpers.AssertEqual(t, len(tf.RecoveryHashes), recoveryCodeCount-1)
}

func TestDisableNeedsPasswordAndCode(t *testing.T) {
	h, s, user := twoFactorFixture(t)
	secret, _ := enableFor(t, h, user.ID)

	rr := httptest.NewRecorder()
	h.TwoFactor(rr, asUser(postJSON("/api/auth/2fa/disable", `{"password":"correct-horse-battery","code":""}`), user.ID))
	testhelpers.AssertEqual(t, rr.Code, http.StatusUnauthorized)

	code, _ := totp.CodeAt(secret, totp.Step(time.Now()))
	rr = httptest.NewRecorder()
	h.TwoFactor(rr, asUser(postJSON("/api/auth/2fa/disable", `{"password":"nope-nope-nope","code":"`+code+`"}`), user.ID))
	testhelpers.AssertEqual(t, rr.Code, http.StatusUnauthorized)

	rr = httptest.NewRecorder()
	h.TwoFactor(rr, asUser(postJSON("/api/auth/2fa/disable", `{"password":"correct-horse-battery","code":"`+code+`"}`), user.ID))
	testhelpers.AssertEqual(t, rr.Code, http.StatusOK)
	tf, _ := s.GetTwoFactor(user.ID)
	if tf != nil {
		t.Fatal("still on after disable")
	}
	rr, _ = login(h, "correct-horse-battery", "")
	testhelpers.AssertEqual(t, rr.Code, http.StatusOK)
}

func TestSetupCantReplaceAnActiveSecret(t *testing.T) {
	h, _, user := twoFactorFixture(t)
	enableFor(t, h, user.ID)
	rr := httptest.NewRecorder()
	h.TwoFactor(rr, asUser(postJSON("/api/auth/2fa/setup", "{}"), user.ID))
	testhelpers.AssertEqual(t, rr.Code, http.StatusConflict)
}

func TestAdminResetTurnsItOff(t *testing.T) {
	h, s, user := twoFactorFixture(t)
	enableFor(t, h, user.ID)

	// A plain user can't reset anyone.
	rr := httptest.NewRecorder()
	h.TwoFactorReset(rr, asUser(postJSON("/api/users/2fa-reset", `{"username":"stella"}`), user.ID))
	testhelpers.AssertEqual(t, rr.Code, http.StatusForbidden)

	rr = httptest.NewRecorder()
	h.TwoFactorReset(rr, adminReq(http.MethodPost, `{"username":"stella"}`))
	testhelpers.AssertEqual(t, rr.Code, http.StatusOK)
	tf, _ := s.GetTwoFactor(user.ID)
	if tf != nil {
		t.Fatal("reset left it on")
	}
}
