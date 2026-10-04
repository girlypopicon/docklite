package cloudflare

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeAPI serves canned Cloudflare responses keyed by path prefix.
func fakeAPI(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(handler))
	old := baseURL
	baseURL = server.URL
	t.Cleanup(func() {
		baseURL = old
		server.Close()
	})
}

func ok(w http.ResponseWriter, result any, totalPages int) {
	body := map[string]any{"success": true, "errors": []any{}, "result": result}
	if totalPages > 0 {
		body["result_info"] = map[string]int{"total_pages": totalPages}
	}
	_ = json.NewEncoder(w).Encode(body)
}

func denied(w http.ResponseWriter) {
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "errors": []map[string]any{{"code": 10000, "message": "Authentication error"}}})
}

func TestListZonesFollowsPages(t *testing.T) {
	fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			ok(w, []map[string]any{{"id": "z1", "name": "a.com"}}, 2)
			return
		}
		ok(w, []map[string]any{{"id": "z2", "name": "b.com"}}, 2)
	})
	zones, err := NewClient("t").ListZones()
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 2 || zones[1].Name != "b.com" {
		t.Fatalf("got %+v", zones)
	}
}

func TestCheckReportsMissingSettingsPermission(t *testing.T) {
	fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/zones/z1/dns_records"):
			ok(w, []any{}, 1)
		case strings.HasPrefix(r.URL.Path, "/zones/z1/settings"):
			denied(w)
		case r.URL.Path == "/zones":
			ok(w, []map[string]any{{"id": "z1", "name": "a.com"}}, 1)
		default:
			denied(w)
		}
	})
	check := NewClient("t").Check()
	if !check.Valid || !check.CanListZones || check.ZoneCount != 1 || !check.CanReadDNS || check.CanReadSettings {
		t.Fatalf("got %+v", check)
	}
}

func TestCheckAcceptsTokenThatOnlyVerifies(t *testing.T) {
	fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/tokens/verify" {
			ok(w, map[string]string{"status": "active"}, 0)
			return
		}
		denied(w)
	})
	check := NewClient("t").Check()
	if !check.Valid || check.CanListZones || !strings.Contains(check.Error, "Zone → Zone → Read") {
		t.Fatalf("got %+v", check)
	}
}

func TestAccountTokenVerifiesByListingZones(t *testing.T) {
	// Account-owned tokens fail /user/tokens/verify but can list zones.
	fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/zones" {
			ok(w, []map[string]any{{"id": "z1", "name": "a.com"}}, 1)
			return
		}
		denied(w)
	})
	if !NewClient("t").VerifyToken() {
		t.Fatal("account token should verify")
	}
}
