package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"docklite-agent/internal/store"
)

func TestScopesAreValidatedAndReadOnlyTokensCannotWrite(t *testing.T) {
	read, write, junk := `["read"]`, `["read","write"]`, `["admin-ish"]`
	if validateScopes(&read) != nil || validateScopes(&write) != nil || validateScopes(nil) != nil {
		t.Fatal("read, write and none are fine")
	}
	if validateScopes(&junk) == nil {
		t.Fatal("an unknown scope must be refused, not ignored")
	}
	if !readOnlyToken(&store.TokenRecord{Scopes: &read}) {
		t.Fatal("scopes [read] is read-only")
	}
	if readOnlyToken(&store.TokenRecord{Scopes: &write}) || readOnlyToken(&store.TokenRecord{}) {
		t.Fatal("write scope and no scopes can change things")
	}
	star := `"*"`
	if readOnlyToken(&store.TokenRecord{Scopes: &star}) {
		t.Fatal("* can change things")
	}
	if !isSafeMethod("GET") || isSafeMethod("POST") || isSafeMethod("DELETE") {
		t.Fatal("only GET, HEAD and OPTIONS are safe")
	}
}

func TestFailLimiterBlocksAfterMaxAndExpires(t *testing.T) {
	l := newFailLimiter(3, 50*time.Millisecond)
	for i := 0; i < 3; i++ {
		if l.blocked("a") {
			t.Fatalf("blocked too early at %d", i)
		}
		l.record("a")
	}
	if !l.blocked("a") || l.blocked("b") {
		t.Fatal("a is blocked, b is not")
	}
	time.Sleep(70 * time.Millisecond)
	if l.blocked("a") {
		t.Fatal("the block should end with the window")
	}
}

func TestSprayingManyUsernamesFromOneAddressIsStopped(t *testing.T) {
	h := testHandlers(t)
	h.token = "bootstrap-secret"
	ipFailures = newFailLimiter(5, time.Minute)
	userFailures = newFailLimiter(20, time.Minute)
	defer func() { ipFailures, userFailures = newFailLimiter(40, 5*time.Minute), newFailLimiter(20, 15*time.Minute) }()
	try := func(user string) int {
		r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"`+user+`","password":"wrong-password"}`))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = "198.51.100.9:5555"
		w := httptest.NewRecorder()
		h.AuthLogin(w, r)
		return w.Code
	}
	for i := 0; i < 5; i++ {
		if code := try("user" + string(rune('a'+i))); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i, code)
		}
	}
	if code := try("anotherone"); code != http.StatusTooManyRequests {
		t.Fatalf("the sixth username from the same address should be refused, got %d", code)
	}
}

func TestOneAccountGuessedFromManyAddressesIsStopped(t *testing.T) {
	h := testHandlers(t)
	h.token = "bootstrap-secret"
	ipFailures = newFailLimiter(1000, time.Minute)
	userFailures = newFailLimiter(4, time.Minute)
	defer func() { ipFailures, userFailures = newFailLimiter(40, 5*time.Minute), newFailLimiter(20, 15*time.Minute) }()
	try := func(ip string) int {
		r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"Alice","password":"nope-nope"}`))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = ip + ":1"
		w := httptest.NewRecorder()
		h.AuthLogin(w, r)
		return w.Code
	}
	for i := 0; i < 4; i++ {
		try("203.0.113." + string(rune('1'+i)))
	}
	if code := try("203.0.113.99"); code != http.StatusTooManyRequests {
		t.Fatalf("a fifth guess at one account from a new address should be refused, got %d", code)
	}
}

func TestPasswordChangeEndsOlderSessions(t *testing.T) {
	h := testHandlers(t)
	h.token = "bootstrap-secret"
	user, err := h.store.CreateUser("alice", "first-password-1", "user", nil)
	if err != nil {
		t.Fatal(err)
	}
	cookie, err := createDelegationToken(user, "user", h.token)
	if err != nil {
		t.Fatal(err)
	}
	call := func() int {
		r := httptest.NewRequest("GET", "/api/health", nil)
		r.AddCookie(&http.Cookie{Name: delegationCookieName, Value: cookie})
		w := httptest.NewRecorder()
		h.Auth(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })(w, r)
		return w.Code
	}
	if call() != 200 {
		t.Fatal("a fresh session works")
	}
	if err := h.store.UpdateUserPassword(user.ID, "second-password-2"); err != nil {
		t.Fatal(err)
	}
	if call() != http.StatusUnauthorized {
		t.Fatal("the old session must stop working after the password changed")
	}
	_ = context.Background()
}

func TestLoginTokensExpireByDefault(t *testing.T) {
	h := testHandlers(t)
	user, _ := h.store.CreateUser("bob", "bobs-password-1", "user", nil)
	rec, _, err := h.issueLoginToken(user, "user", loginRequest{})
	if err != nil || rec.ExpiresAt == nil {
		t.Fatalf("a login token needs an expiry: %v %v", rec, err)
	}
}
