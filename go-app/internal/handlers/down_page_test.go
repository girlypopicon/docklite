package handlers

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func TestDownPageHTML(t *testing.T) {
	plain := downPageHTML("")
	if !strings.Contains(plain, "isn't running") || !strings.Contains(plain, "whoever runs this server") {
		t.Fatal("the default page should say the box isn't running and point at whoever runs the server")
	}
	// Visitors see this page: nothing from other sites, no server paths.
	for _, bad := range []string{"http://", "src=", "<script", "/var/", "/opt/", "@import"} {
		if bad == "http://" {
			continue // the DockLite link is https
		}
		if strings.Contains(plain, bad) {
			t.Errorf("page contains %q", bad)
		}
	}
	if strings.Contains(plain, "{{") {
		t.Fatal("placeholder left in the page")
	}

	mail := downPageHTML("admin@example.com")
	if !strings.Contains(mail, `href="mailto:admin@example.com"`) {
		t.Fatal("an email address should become a mailto link")
	}
	link := downPageHTML("https://example.com/help")
	if !strings.Contains(link, `href="https://example.com/help"`) {
		t.Fatal("a web address should become a link")
	}

	// Whatever an admin types is escaped, whether or not it looks like a link.
	for _, evil := range []string{`<script>alert(1)</script>`, `"><img src=x onerror=alert(1)>`, `https://x.com/"><script>1</script>`, `a@b.co"><i>`} {
		page := downPageHTML(evil)
		if strings.Contains(page, evil) {
			t.Errorf("%q appears unescaped", evil)
		}
		if !strings.Contains(page, html.EscapeString(cleanDownContact(evil))) {
			t.Errorf("%q should appear escaped", evil)
		}
	}
	// The limit applies to what's typed: count a marker the template never uses.
	long := downPageHTML(strings.Repeat("Q", 5000))
	if strings.Count(long, "Q") != maxDownContactLen {
		t.Fatalf("contact line is %d long, want it cut to %d", strings.Count(long, "Q"), maxDownContactLen)
	}
	if cleanDownContact("  two\n  lines  ") != "two lines" {
		t.Fatal("whitespace not tidied")
	}
}

func TestVhostIncludesTheDownPage(t *testing.T) {
	cfg := nginxVhostConfig("shop.example.com", true, 32001)
	if !strings.Contains(cfg, downIncludeLine) {
		t.Fatal("new site configs must include the stopped-box page")
	}
	// the include sits inside the server block, before the location that proxies
	if strings.Index(cfg, downIncludeLine) > strings.Index(cfg, "location /") {
		t.Fatal("include should come before the location block")
	}
}

func TestPlanDownPatches(t *testing.T) {
	listing := strings.Join([]string{
		"/etc/nginx/sites-enabled/a.conf\ta.example.com www.a.example.com\t32001\t-",
		"/etc/nginx/sites-enabled/b.conf\tb.example.com\t32002\tdown",     // already has it
		"/etc/nginx/sites-enabled/panel.conf\tpanel.example.com\t3000\t-", // not a site
		"/etc/nginx/sites-enabled/c.conf\tc.example.com\t\t-",             // redirect only: no upstream
		"/etc/nginx/sites-enabled/d.conf\td.example.com\t32004",           // older helper: no column
		"/etc/nginx/sites-enabled/multi\te.example.com\t32005\t-",
		"/etc/nginx/sites-enabled/multi\te.example.com\t32005\t-", // https twin: same file, one patch call
	}, "\n")
	sites := map[string]bool{"a.example.com": true, "b.example.com": true, "c.example.com": true, "d.example.com": true, "e.example.com": true}
	got := planDownPatches(parseUpstreamBlocks(listing), sites)
	var names []string
	for _, p := range got {
		names = append(names, p.Domain)
	}
	if strings.Join(names, ",") != "a.example.com,e.example.com" {
		t.Fatalf("planned %v, want a.example.com and e.example.com once each", names)
	}
}

func downFixture(t *testing.T) (*Handlers, *store.SQLiteStore, *[][]string) {
	t.Helper()
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	t.Cleanup(func() { s.Close() })
	testhelpers.AssertNoError(t, s.EnsureSettingsSchema())
	var calls [][]string
	var written string
	rootHelperHook = func(stdin io.Reader, args ...string) ([]byte, error) {
		calls = append(calls, args)
		switch args[0] {
		case "nginx-down-write":
			b, _ := io.ReadAll(stdin)
			written = string(b)
		case "nginx-upstreams":
			return []byte("/etc/nginx/sites-enabled/shop.conf\tshop.example.com\t32001\t-\n/etc/nginx/sites-enabled/panel.conf\tpanel.example.com\t3000\t-\n"), nil
		}
		_ = written
		return nil, nil
	}
	t.Cleanup(func() { rootHelperHook = nil })
	return &Handlers{store: s}, s, &calls
}

func TestSyncDownPagePatchesOnlySites(t *testing.T) {
	h, s, calls := downFixture(t)
	admin, err := s.CreateUser("admin", "adminpassword1", "super_admin", nil)
	testhelpers.AssertNoError(t, err)
	// a stopped site is still a site: it is in the database whether or not its box runs
	_, err = s.CreateSite(store.SiteRecord{Domain: "shop.example.com", UserID: admin.ID, TemplateType: "static", CodePath: "/x"})
	testhelpers.AssertNoError(t, err)

	h.SyncDownPage(context.Background())

	var wrote, patched []string
	for _, c := range *calls {
		switch c[0] {
		case "nginx-down-write":
			wrote = append(wrote, "w")
		case "nginx-down-patch":
			patched = append(patched, c[2])
		}
	}
	testhelpers.AssertEqual(t, len(wrote), 1)
	testhelpers.AssertEqual(t, strings.Join(patched, ","), "shop.example.com")
}

func TestDownPageSettingsAPI(t *testing.T) {
	h, s, calls := downFixture(t)

	// anyone signed in can read it
	rr := httptest.NewRecorder()
	h.DownPageSettings(rr, httptest.NewRequest(http.MethodGet, "/api/settings/down-page", nil))
	testhelpers.AssertEqual(t, rr.Code, http.StatusOK)

	// a plain user can't change it
	plain := httptest.NewRequest(http.MethodPut, "/x", strings.NewReader(`{"contact":"me@example.com"}`))
	rr = httptest.NewRecorder()
	h.DownPageSettings(rr, plain)
	testhelpers.AssertEqual(t, rr.Code, http.StatusForbidden)

	rr = httptest.NewRecorder()
	h.DownPageSettings(rr, adminReq(http.MethodPut, `{"contact":"  admin@example.com  "}`))
	testhelpers.AssertEqual(t, rr.Code, http.StatusOK)
	got, _ := s.GetSetting(downContactSetting)
	testhelpers.AssertEqual(t, got, "admin@example.com")
	if len(*calls) == 0 || (*calls)[len(*calls)-1][0] != "nginx-down-write" {
		t.Fatal("saving the contact should rewrite the page")
	}

	// clearing it goes back to the default wording
	rr = httptest.NewRecorder()
	h.DownPageSettings(rr, adminReq(http.MethodPut, `{"contact":""}`))
	testhelpers.AssertEqual(t, rr.Code, http.StatusOK)
	got, _ = s.GetSetting(downContactSetting)
	testhelpers.AssertEqual(t, got, "")
}
