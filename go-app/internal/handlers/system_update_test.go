package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"docklite-agent/internal/testhelpers"
)

func TestSemverCompare(t *testing.T) {
	a, _ := parseSemver("v1.10.0")
	b, _ := parseSemver("v1.9.7")
	testhelpers.AssertEqual(t, compareSemver(a, b), 1) // numeric, not alphabetical
	testhelpers.AssertEqual(t, compareSemver(b, a), -1)
	testhelpers.AssertEqual(t, compareSemver(a, a), 0)
	for _, bad := range []string{"", "1.2.3", "v1.2", "v1.2.3-beta", "v1.2.3; rm -rf /", "main", "v1.2.3\n"} {
		if _, ok := parseSemver(bad); ok {
			t.Errorf("%q must not be accepted as a release tag", bad)
		}
	}
}

func fakeGitHub(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	old := githubAPI
	githubAPI = srv.URL
	t.Cleanup(func() { githubAPI = old })
}

func TestLookupLatestUsesTheLatestRelease(t *testing.T) {
	fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			_, _ = w.Write([]byte(`{"tag_name":"v1.2.0","body":"New: Cloudflare DNS","html_url":"https://example/r","published_at":"2026-10-08T00:00:00Z"}`))
			return
		}
		http.NotFound(w, r)
	})
	got, err := lookupLatest(context.Background())
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, got.Tag, "v1.2.0")
	testhelpers.AssertEqual(t, got.Version, "1.2.0")
	testhelpers.AssertEqual(t, got.Notes, "New: Cloudflare DNS")
}

func TestLookupLatestFallsBackToTheHighestTag(t *testing.T) {
	fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tags") {
			_, _ = w.Write([]byte(`[{"name":"v1.0.3"},{"name":"nightly"},{"name":"v1.10.0"},{"name":"v1.9.9"},{"name":"v2.0.0-rc1"}]`))
			return
		}
		http.NotFound(w, r) // no releases published
	})
	got, err := lookupLatest(context.Background())
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, got.Tag, "v1.10.0")
}

func TestLookupLatestSaysSoWhenThereIsNothing(t *testing.T) {
	fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tags") {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		http.NotFound(w, r)
	})
	_, err := lookupLatest(context.Background())
	testhelpers.AssertErrorContains(t, err, "no releases")
}

func TestUpdateRunIsGuarded(t *testing.T) {
	started := ""
	old := startUpdateJob
	startUpdateJob = func(tag string) ([]byte, error) { started = tag; return nil, nil }
	t.Cleanup(func() { startUpdateJob = old })
	h := &Handlers{}
	run := func(role, body string) int {
		r := httptest.NewRequest("POST", "/x", strings.NewReader(body))
		r = r.WithContext(context.WithValue(r.Context(), ctxUserRoleKey, role))
		w := httptest.NewRecorder()
		h.SystemUpdateRun(w, r)
		return w.Code
	}
	testhelpers.AssertEqual(t, run("admin", `{"version":"v9.9.9"}`), http.StatusForbidden) // only the super admin
	testhelpers.AssertEqual(t, run("super_admin", `{"version":"main"}`), http.StatusBadRequest)
	testhelpers.AssertEqual(t, run("super_admin", `{"version":"1.2.3; reboot"}`), http.StatusBadRequest)
	testhelpers.AssertEqual(t, started, "")
	testhelpers.AssertEqual(t, run("super_admin", `{"version":"9.9.9"}`), http.StatusAccepted)
	testhelpers.AssertEqual(t, started, "v9.9.9")
}

func TestReadUpdateState(t *testing.T) {
	dir := t.TempDir()
	state, tag, _, msg := readUpdateState(dir + "/missing")
	testhelpers.AssertEqual(t, state, "idle")
	testhelpers.AssertEqual(t, tag, "")
	testhelpers.AssertEqual(t, msg, "")
	testhelpers.AssertNoError(t, writeFileForTest(dir+"/s", "failed v1.2.0 1700000000 build did not finish"))
	state, tag, _, msg = readUpdateState(dir + "/s")
	testhelpers.AssertEqual(t, state, "failed")
	testhelpers.AssertEqual(t, tag, "v1.2.0")
	testhelpers.AssertEqual(t, msg, "build did not finish")
}

func writeFileForTest(path, content string) error { return os.WriteFile(path, []byte(content), 0o644) }
