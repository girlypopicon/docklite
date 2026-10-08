package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"docklite-agent/internal/cloudflare"
	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

// fakeCloudflare serves the few DNS endpoints DockLite uses and remembers what was created.
type fakeCloudflare struct {
	mu      sync.Mutex
	records []cloudflare.DNSRecord
	created []cloudflare.DNSRecord
	updated []cloudflare.DNSRecord
}

func (f *fakeCloudflare) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	reply := func(result any) {
		raw, _ := json.Marshal(result)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true, "errors": []any{}, "result": json.RawMessage(raw),
			"result_info": map[string]int{"page": 1, "total_pages": 1},
		})
	}
	switch {
	case r.Method == "GET" && strings.Contains(r.URL.Path, "/dns_records"):
		reply(f.records)
	case r.Method == "POST" && strings.Contains(r.URL.Path, "/dns_records"):
		var rec cloudflare.DNSRecord
		_ = json.NewDecoder(r.Body).Decode(&rec)
		rec.ID = "new-" + rec.Name
		f.created = append(f.created, rec)
		f.records = append(f.records, rec)
		reply(rec)
	case r.Method == "PUT":
		var rec cloudflare.DNSRecord
		_ = json.NewDecoder(r.Body).Decode(&rec)
		rec.ID = r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		f.updated = append(f.updated, rec)
		reply(rec)
	default:
		http.NotFound(w, r)
	}
}

func newCFTest(t *testing.T, existing ...cloudflare.DNSRecord) (*Handlers, *fakeCloudflare) {
	t.Helper()
	fake := &fakeCloudflare{records: existing}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	t.Cleanup(cloudflare.SetBaseURL(srv.URL))
	t.Setenv("DOCKLITE_PUBLIC_IP", "203.0.113.7")

	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	t.Cleanup(func() { s.Close() })
	tok, one := "token", 1
	testhelpers.AssertNoError(t, s.UpdateCloudflareConfig(&tok, nil, &one))
	_, err := s.CreateDNSZone("example.com", "ZID", nil, 1)
	testhelpers.AssertNoError(t, err)
	return &Handlers{store: s}, fake
}

func TestSiteDNSCreatesMissingRecords(t *testing.T) {
	h, fake := newCFTest(t)
	res := h.siteDNS("shop.example.com", true, true, true, false)
	testhelpers.AssertEqual(t, res.Status, "done")
	testhelpers.AssertEqual(t, res.Zone, "example.com")
	testhelpers.AssertEqual(t, len(fake.created), 2)
	testhelpers.AssertEqual(t, fake.created[0].Type, "A")
	testhelpers.AssertEqual(t, fake.created[0].Content, "203.0.113.7")
	testhelpers.AssertTrue(t, fake.created[0].Proxied, "A record is proxied")
	testhelpers.AssertEqual(t, fake.created[1].Type, "CNAME")
	testhelpers.AssertEqual(t, fake.created[1].Name, "www.shop.example.com")
	testhelpers.AssertEqual(t, fake.created[1].Content, "shop.example.com")
}

func TestSiteDNSPreviewChangesNothing(t *testing.T) {
	h, fake := newCFTest(t)
	res := h.siteDNS("shop.example.com", true, true, false, false)
	testhelpers.AssertEqual(t, res.Status, "ready")
	testhelpers.AssertEqual(t, len(fake.created), 0)
	testhelpers.AssertEqual(t, res.Records[0].Action, "create")
}

func TestSiteDNSLeavesCorrectRecordsAlone(t *testing.T) {
	h, fake := newCFTest(t, cloudflare.DNSRecord{ID: "1", Type: "A", Name: "shop.example.com", Content: "203.0.113.7"})
	res := h.siteDNS("shop.example.com", false, true, true, false)
	testhelpers.AssertEqual(t, res.Records[0].Action, "exists")
	testhelpers.AssertEqual(t, len(fake.created), 0)
}

func TestSiteDNSNeverOverwritesWithoutPermission(t *testing.T) {
	h, fake := newCFTest(t, cloudflare.DNSRecord{ID: "9", Type: "A", Name: "shop.example.com", Content: "198.51.100.1"})
	res := h.siteDNS("shop.example.com", false, true, true, false)
	testhelpers.AssertEqual(t, res.Status, "conflict")
	testhelpers.AssertEqual(t, res.Records[0].Action, "conflict")
	testhelpers.AssertEqual(t, res.Records[0].Existing, "A 198.51.100.1")
	testhelpers.AssertEqual(t, len(fake.created)+len(fake.updated), 0)

	// ...but with explicit permission it updates that one record.
	res = h.siteDNS("shop.example.com", false, true, true, true)
	testhelpers.AssertEqual(t, res.Records[0].Action, "updated")
	testhelpers.AssertEqual(t, len(fake.updated), 1)
	testhelpers.AssertEqual(t, fake.updated[0].Content, "203.0.113.7")
}

func TestSiteDNSWithoutCloudflareOrZone(t *testing.T) {
	h, _ := newCFTest(t)
	testhelpers.AssertEqual(t, h.siteDNS("other.org", true, true, true, false).Status, "no-zone")
	off := 0
	testhelpers.AssertNoError(t, h.store.UpdateCloudflareConfig(nil, nil, &off))
	testhelpers.AssertEqual(t, h.siteDNS("shop.example.com", true, true, true, false).Status, "not-configured")
}

func TestZoneForDomainPicksLongestMatch(t *testing.T) {
	zones := []store.DNSZone{{Domain: "example.com", ZoneID: "A", Enabled: 1}, {Domain: "shop.example.com", ZoneID: "B", Enabled: 1}, {Domain: "off.com", ZoneID: "C", Enabled: 0}}
	testhelpers.AssertEqual(t, zoneForDomain(zones, "x.shop.example.com").ZoneID, "B")
	testhelpers.AssertEqual(t, zoneForDomain(zones, "blog.example.com").ZoneID, "A")
	testhelpers.AssertTrue(t, zoneForDomain(zones, "notexample.com") == nil, "suffix must be on a dot boundary")
	testhelpers.AssertTrue(t, zoneForDomain(zones, "off.com") == nil, "disabled zones are ignored")
}
