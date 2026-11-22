package handlers

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func TestMissingEntryFile(t *testing.T) {
	dir := t.TempDir()
	if w := missingEntryFile(dir, "node"); !strings.Contains(w, "index.js") {
		t.Fatalf("node without index.js: %q", w)
	}
	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(dir, "index.js"), []byte("x"), 0o644))
	if w := missingEntryFile(dir, "node"); w != "" {
		t.Fatalf("node with index.js: %q", w)
	}
	if w := missingEntryFile(dir, "php"); !strings.Contains(w, "index.php") {
		t.Fatalf("php: %q", w)
	}
	// plain html pages run fine under PHP and static, so they count as an entry file for both
	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("x"), 0o644))
	if w := missingEntryFile(dir, "php"); w != "" {
		t.Fatalf("php with index.html: %q", w)
	}
}

func TestChangeSiteTemplateChecksBeforeTouchingDocker(t *testing.T) {
	h, s, alice, bob := groupFixture(t)
	cid := "abc123"
	site, err := s.CreateSite(store.SiteRecord{Domain: "a.example.com", UserID: alice.ID, TemplateType: "static", CodePath: t.TempDir()})
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertNoError(t, s.UpdateSiteContainerID(site.ID, &cid))

	call := func(role string, uid int64, body, container string) int {
		req := asRole(httptest.NewRequest(http.MethodPost, "/api/containers/"+container+"/template", strings.NewReader(body)), uid, role)
		rec := httptest.NewRecorder()
		h.ChangeSiteTemplate(rec, req, container)
		return rec.Code
	}
	if c := call("user", alice.ID, `{"template_type":"ruby"}`, cid); c != http.StatusBadRequest {
		t.Errorf("unknown type: %d", c)
	}
	if c := call("user", alice.ID, `{"template_type":"node"}`, "nope"); c != http.StatusNotFound {
		t.Errorf("not a site: %d", c)
	}
	if c := call("user", bob.ID, `{"template_type":"node"}`, cid); c != http.StatusForbidden {
		t.Errorf("someone else's site: %d", c)
	}
	rec := httptest.NewRecorder()
	h.ChangeSiteTemplate(rec, httptest.NewRequest(http.MethodGet, "/", nil), cid)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: %d", rec.Code)
	}
}

func TestJudgeSiteState(t *testing.T) {
	cases := []struct {
		running, restarting bool
		restarts            int
		status, want        string
		elapsed             time.Duration
	}{
		{true, false, 0, "running", "wait", time.Second},
		{true, false, 0, "running", "up", 3 * time.Second},
		{false, true, 0, "restarting", "failed", time.Second}, // a crash loop fails at once
		{false, false, 0, "exited", "failed", time.Second},
		{true, false, 2, "running", "failed", 4 * time.Second},
		{false, false, 0, "created", "wait", time.Second},
	}
	for i, c := range cases {
		if got := judgeSiteState(c.running, c.restarting, c.restarts, c.status, c.elapsed); got != c.want {
			t.Errorf("case %d: %s, want %s", i, got, c.want)
		}
	}
}

func TestCrashReasonQuotesTheError(t *testing.T) {
	logs := "\nnode:internal/modules/cjs/loader:1210\n  throw err;\n  ^\n\nError: Cannot find module '/app/index.js'\n    at Module._resolveFilename\n\nNode.js v20.20.2\n"
	if got := crashReason(logs); !strings.Contains(got, "Cannot find module '/app/index.js'") {
		t.Fatalf("got %q", got)
	}
	if got := crashReason(""); !strings.Contains(got, "stopped as soon as it started") {
		t.Fatalf("empty log: %q", got)
	}
}

func TestNodeStarterServesTheSiteAndHidesTheRest(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node isn't installed here")
	}
	dir := t.TempDir()
	for name, body := range map[string]string{"index.html": "<h1>hi</h1>", "style.css": "body{}", ".dkl": `{"username":"secret"}`, "package.json": "{}"} {
		testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	}
	testhelpers.AssertNoError(t, ensureFile(filepath.Join(dir, "index.js"), nodeStarterIndexJS))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	testhelpers.AssertNoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	cmd := exec.Command("node", "index.js")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PORT="+strconv.Itoa(port))
	testhelpers.AssertNoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })

	get := func(path string) (int, string) {
		for i := 0; i < 40; i++ {
			resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + path)
			if err == nil {
				defer resp.Body.Close()
				b, _ := io.ReadAll(resp.Body)
				return resp.StatusCode, string(b)
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("the starter server never answered")
		return 0, ""
	}
	if c, b := get("/"); c != 200 || !strings.Contains(b, "hi") {
		t.Fatalf("/: %d %q", c, b)
	}
	if c, _ := get("/style.css"); c != 200 {
		t.Fatalf("/style.css: %d", c)
	}
	for _, hidden := range []string{"/.dkl", "/index.js", "/package.json", "/%2e%2e/etc/passwd", "/../../etc/passwd"} {
		if c, b := get(hidden); c == 200 || strings.Contains(b, "secret") || strings.Contains(b, "root:") {
			t.Errorf("%s was served: %d", hidden, c)
		}
	}
}
