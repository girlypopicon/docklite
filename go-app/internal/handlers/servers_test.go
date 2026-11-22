package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func hubAdminReq(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	return r.WithContext(context.WithValue(r.Context(), ctxUserRoleKey, "admin"))
}

func TestJoinCodeRoundTripAndBadCodes(t *testing.T) {
	j, err := decodeJoin(encodeJoin(joinCode{URL: "https://smoll.docklite.net", Token: "s3cret", Name: "smoll"}))
	if err != nil || j.URL != "https://smoll.docklite.net" || j.Token != "s3cret" || j.Name != "smoll" {
		t.Fatalf("%+v %v", j, err)
	}
	for _, bad := range []string{"", "hello", "dlj1.@@@", "dlj1." + "e30"} { // e30 = {}
		if _, err := decodeJoin(bad); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
}

func TestValidServerURL(t *testing.T) {
	good := map[string]string{"https://a.net/": "https://a.net", "http://203.0.113.7:3000": "http://203.0.113.7:3000"}
	for in, want := range good {
		if got, err := validServerURL(in); err != nil || got != want {
			t.Errorf("%q -> %q %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "ftp://a.net", "a.net", "http://169.254.169.254/", "file:///etc/passwd"} {
		if _, err := validServerURL(bad); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
}

// A pretend second server: it only answers when it gets the right token.
func fakeRemote(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"path":"` + r.URL.Path + `","method":"` + r.Method + `"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAddServerChecksTheTokenThenProxiesReadsOnly(t *testing.T) {
	h := testHandlers(t)
	if err := h.store.EnsureRemoteServerSchema(); err != nil {
		t.Fatal(err)
	}
	remote := fakeRemote(t)

	// a wrong token is refused and nothing is saved
	bad, _ := json.Marshal(map[string]string{"join": encodeJoin(joinCode{URL: remote.URL, Token: "nope", Name: "smoll"})})
	w := httptest.NewRecorder()
	h.Servers(w, hubAdminReq("POST", "/api/servers", string(bad)))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("bad token: %d %s", w.Code, w.Body)
	}
	if list, _ := h.store.ListRemoteServers(); len(list) != 0 {
		t.Fatal("a refused server must not be saved")
	}

	ok, _ := json.Marshal(map[string]string{"join": encodeJoin(joinCode{URL: remote.URL, Token: "good-token", Name: "smoll"})})
	w = httptest.NewRecorder()
	h.Servers(w, hubAdminReq("POST", "/api/servers", string(ok)))
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "good-token") {
		t.Fatalf("add: %d %s (the token must never be sent back)", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	h.Servers(w, hubAdminReq("POST", "/api/servers", string(ok)))
	if w.Code != http.StatusConflict {
		t.Fatalf("same name twice: %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.Servers(w, hubAdminReq("GET", "/api/servers", ""))
	if strings.Contains(w.Body.String(), "good-token") || !strings.Contains(w.Body.String(), "smoll") {
		t.Fatalf("list: %s", w.Body)
	}

	// reads pass through, with the token added on the way
	w = httptest.NewRecorder()
	h.ServerRoutes(w, hubAdminReq("GET", "/api/servers/1/proxy/containers", ""))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"path":"/api/containers"`) {
		t.Fatalf("proxy read: %d %s", w.Code, w.Body)
	}
	// a start is allowed, a delete or a write is not, and a path outside the list is not
	w = httptest.NewRecorder()
	h.ServerRoutes(w, hubAdminReq("POST", "/api/servers/1/proxy/containers/abc123/restart", ""))
	if w.Code != http.StatusOK {
		t.Fatalf("restart: %d %s", w.Code, w.Body)
	}
	for _, c := range []struct{ m, p string }{{"DELETE", "/api/servers/1/proxy/containers/abc123"}, {"POST", "/api/servers/1/proxy/containers"}, {"GET", "/api/servers/1/proxy/users"}, {"GET", "/api/servers/1/proxy/containers/../users"}} {
		w = httptest.NewRecorder()
		h.ServerRoutes(w, hubAdminReq(c.m, c.p, ""))
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s should be refused, got %d", c.m, c.p, w.Code)
		}
	}
	// non-admins get nothing
	w = httptest.NewRecorder()
	h.ServerRoutes(w, httptest.NewRequest("GET", "/api/servers/1/proxy/containers", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("anonymous: %d", w.Code)
	}
	// removing
	w = httptest.NewRecorder()
	h.ServerRoutes(w, hubAdminReq("DELETE", "/api/servers/1", ""))
	if list, _ := h.store.ListRemoteServers(); w.Code != http.StatusOK || len(list) != 0 {
		t.Fatalf("remove: %d %v", w.Code, list)
	}
}
