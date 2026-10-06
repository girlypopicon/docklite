package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type recorded struct {
	Method, Path, Query, Auth, Body string
}

type fakeAgent struct {
	*httptest.Server
	mu       sync.Mutex
	requests []recorded
	// routes maps "METHOD /path" to a handler.
	routes map[string]func(w http.ResponseWriter, body string)
}

func newFakeAgent(t *testing.T) *fakeAgent {
	t.Helper()
	f := &fakeAgent{routes: map[string]func(http.ResponseWriter, string){}}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.requests = append(f.requests, recorded{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"), string(body)})
		f.mu.Unlock()
		if h, ok := f.routes[r.Method+" "+r.URL.Path]; ok {
			h(w, string(body))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeAgent) json(method, path string, status int, body string) {
	f.routes[method+" "+path] = func(w http.ResponseWriter, _ string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func (f *fakeAgent) calls(method, path string) []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recorded
	for _, r := range f.requests {
		if r.Method == method && r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

const sampleContainers = `{"containers":[
 {"id":"aaaaaaaaaaaa1111","name":"docklite-db-main","state":"running","image":"postgres:16","labels":{"docklite.type":"postgres"}},
 {"id":"bbbbbbbbbbbb2222","name":"docklite-site-example-com","state":"running","image":"nginx:alpine","labels":{"docklite.type":"static","docklite.domain":"example.com"}},
 {"id":"cccccccccccc3333","name":"docklite-site-other-org","state":"exited","image":"nginx:alpine","labels":{"docklite.type":"static","docklite.domain":"other.org"}},
 {"id":"dddddddddddd4444","name":"docklite-site-example-net","state":"running","image":"nginx:alpine","labels":{"docklite.type":"static","docklite.domain":"example.net"}}
]}`

// runCLI runs the command line against the fake agent with a clean environment.
func runCLI(t *testing.T, agent *fakeAgent, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOCKLITE_CONFIG", filepath.Join(dir, "config.json"))
	t.Setenv("DOCKLITE_CONF", filepath.Join(dir, "no-such-conf"))
	t.Setenv("DOCKLITE_HOST", "")
	t.Setenv("DOCKLITE_TOKEN", "")
	if agent != nil {
		t.Setenv("DOCKLITE_HOST", agent.URL)
		t.Setenv("DOCKLITE_TOKEN", "test-token")
	}
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestExtractGlobalsAnywhere(t *testing.T) {
	opts, rest, err := extractGlobals([]string{"containers", "--json", "list", "--host=http://x:1", "--timeout", "5s", "-y"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.JSON || !opts.Yes || opts.Host != "http://x:1" || opts.Timeout.Seconds() != 5 {
		t.Fatalf("bad opts: %+v", opts)
	}
	if strings.Join(rest, " ") != "containers list" {
		t.Fatalf("bad rest: %v", rest)
	}
	if _, _, err := extractGlobals([]string{"--timeout", "soon"}); err == nil {
		t.Fatal("a bad timeout should be a usage error")
	}
}

func TestContainersListOrderAndFilters(t *testing.T) {
	agent := newFakeAgent(t)
	agent.json("GET", "/api/containers/all", 200, sampleContainers)

	code, out, _ := runCLI(t, agent, "", "containers", "list", "--json")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var resp struct {
		Containers []containerRow `json:"containers"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("not json: %v\n%s", err, out)
	}
	var names []string
	for _, c := range resp.Containers {
		names = append(names, c.Name)
	}
	// running first; sites before databases; stopped last
	want := "docklite-site-example-com docklite-site-example-net docklite-db-main docklite-site-other-org"
	if strings.Join(names, " ") != want {
		t.Fatalf("order:\n got %v\nwant %s", names, want)
	}

	_, out, _ = runCLI(t, agent, "", "containers", "list", "--kind", "database", "--json")
	if !strings.Contains(out, "docklite-db-main") || strings.Contains(out, "example-com") {
		t.Fatalf("kind filter failed: %s", out)
	}
	if code, _, _ := runCLI(t, agent, "", "containers", "list", "--kind", "bogus"); code != exitUsage {
		t.Fatalf("bad --kind should exit %d, got %d", exitUsage, code)
	}
}

func TestResolveContainerByDomainAmbiguityAndMissing(t *testing.T) {
	agent := newFakeAgent(t)
	agent.json("GET", "/api/containers/all", 200, sampleContainers)
	agent.json("POST", "/api/containers/bbbbbbbbbbbb2222/restart", 200, `{"success":true}`)

	if code, _, errOut := runCLI(t, agent, "", "containers", "restart", "example.com"); code != 0 {
		t.Fatalf("restart by domain exit %d: %s", code, errOut)
	}
	if len(agent.calls("POST", "/api/containers/bbbbbbbbbbbb2222/restart")) != 1 {
		t.Fatal("restart was not sent to the right container")
	}

	// "example" matches two sites: refuse to guess.
	code, _, errOut := runCLI(t, agent, "", "containers", "restart", "example")
	if code != exitUsage || !strings.Contains(errOut, "matches 2") {
		t.Fatalf("ambiguous: exit %d, %q", code, errOut)
	}
	if code, _, _ := runCLI(t, agent, "", "containers", "restart", "nothing-like-this"); code != exitNotFound {
		t.Fatalf("missing container should exit %d, got %d", exitNotFound, code)
	}
}

func TestDestructiveCommandsNeedYes(t *testing.T) {
	agent := newFakeAgent(t)
	agent.json("GET", "/api/containers/all", 200, sampleContainers)
	agent.json("POST", "/api/containers/bbbbbbbbbbbb2222/stop", 200, `{"success":true}`)
	agent.json("DELETE", "/api/containers/bbbbbbbbbbbb2222/delete", 200, `{"success":true}`)

	// Not a terminal and no --yes: refuse, and send nothing.
	for _, args := range [][]string{{"containers", "stop", "example.com"}, {"containers", "delete", "example.com"}} {
		code, _, errOut := runCLI(t, agent, "", args...)
		if code != exitNeedsYes || !strings.Contains(errOut, "--yes") {
			t.Fatalf("%v: exit %d, %q", args, code, errOut)
		}
	}
	if len(agent.calls("POST", "/api/containers/bbbbbbbbbbbb2222/stop")) != 0 || len(agent.calls("DELETE", "/api/containers/bbbbbbbbbbbb2222/delete")) != 0 {
		t.Fatal("a destructive request was sent without --yes")
	}

	if code, _, _ := runCLI(t, agent, "", "containers", "stop", "example.com", "--yes"); code != 0 {
		t.Fatal("stop --yes should work")
	}
	if code, _, _ := runCLI(t, agent, "", "containers", "delete", "example.com", "-y"); code != 0 {
		t.Fatal("delete -y should work")
	}
	if len(agent.calls("DELETE", "/api/containers/bbbbbbbbbbbb2222/delete")) != 1 {
		t.Fatal("delete should use the DELETE method")
	}
}

func TestExitCodesForServerAnswers(t *testing.T) {
	agent := newFakeAgent(t)
	agent.json("GET", "/api/status", 401, `{"error":"unauthorized"}`)
	agent.json("GET", "/api/summary", 403, `{"error":"forbidden"}`)
	agent.json("GET", "/api/tokens", 409, `{"error":"already exists"}`)

	if code, _, errOut := runCLI(t, agent, "", "status"); code != exitAuth || !strings.Contains(errOut, "docklite login") {
		t.Fatalf("401: exit %d %q", code, errOut)
	}
	if code, _, _ := runCLI(t, agent, "", "info"); code != exitAuth {
		t.Fatalf("403: exit %d", code)
	}
	if code, _, _ := runCLI(t, agent, "", "tokens"); code != exitConflict {
		t.Fatalf("409: exit %d", code)
	}

	dead := newFakeAgent(t)
	url := dead.URL
	dead.Close()
	t.Setenv("DOCKLITE_HOST", url)
	var out, errOut bytes.Buffer
	t.Setenv("DOCKLITE_CONFIG", filepath.Join(t.TempDir(), "c.json"))
	if code := run([]string{"status"}, strings.NewReader(""), &out, &errOut); code != exitUnreachable {
		t.Fatalf("unreachable: exit %d %q", code, errOut.String())
	}
}

func TestLoginSavesTokenAndConfigShowHidesIt(t *testing.T) {
	agent := newFakeAgent(t)
	agent.routes["POST /api/auth/login"] = func(w http.ResponseWriter, body string) {
		if !strings.Contains(body, `"password":"s3cret"`) || !strings.Contains(body, `"username":"me"`) {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":"bad"}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"token":{"secret":"SECRET-TOKEN-VALUE"}}`))
	}
	dir := t.TempDir()
	t.Setenv("DOCKLITE_CONFIG", filepath.Join(dir, "config.json"))
	t.Setenv("DOCKLITE_CONF", filepath.Join(dir, "none"))
	t.Setenv("DOCKLITE_TOKEN", "")
	t.Setenv("DOCKLITE_HOST", "")

	var out, errOut bytes.Buffer
	code := run([]string{"--host", agent.URL, "login", "--username", "me", "--password-stdin"}, strings.NewReader("s3cret\n"), &out, &errOut)
	if code != 0 {
		t.Fatalf("login exit %d: %s", code, errOut.String())
	}
	saved, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	if !strings.Contains(string(saved), "SECRET-TOKEN-VALUE") || !strings.Contains(string(saved), agent.URL) {
		t.Fatalf("token/host not saved: %s", saved)
	}
	if info, _ := os.Stat(filepath.Join(dir, "config.json")); info.Mode().Perm() != 0o600 {
		t.Fatalf("config must be private, is %v", info.Mode().Perm())
	}

	out.Reset()
	if code := run([]string{"config", "show"}, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("config show exit %d", code)
	}
	if strings.Contains(out.String(), "SECRET-TOKEN-VALUE") {
		t.Fatalf("config show leaked the token: %s", out.String())
	}
}

func TestLocalAdminAccessAndExplicitHostOptOut(t *testing.T) {
	agent := newFakeAgent(t)
	agent.json("GET", "/api/auth/me", 200, `{"user":{"username":"superadmin","role":"super_admin"}}`)

	port := mustPort(t, agent.URL)
	dir := t.TempDir()
	conf := filepath.Join(dir, "docklite.conf")
	_ = os.WriteFile(conf, []byte("# generated\nAGENT_PORT="+port+"\nDOCKLITE_TOKEN=master-token\n"), 0o600)

	t.Setenv("DOCKLITE_CONFIG", filepath.Join(dir, "config.json"))
	t.Setenv("DOCKLITE_CONF", conf)
	t.Setenv("DOCKLITE_HOST", "")
	t.Setenv("DOCKLITE_TOKEN", "")

	var out, errOut bytes.Buffer
	if code := run([]string{"whoami"}, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("whoami exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "local admin access") {
		t.Fatalf("should say why it works: %s", out.String())
	}
	if got := agent.calls("GET", "/api/auth/me"); len(got) != 1 || got[0].Auth != "Bearer master-token" {
		t.Fatalf("expected the config's token to be used, got %+v", got)
	}

	// Pointing at another server must never send this machine's master token.
	t.Setenv("DOCKLITE_HOST", agent.URL)
	out.Reset()
	errOut.Reset()
	run([]string{"whoami"}, strings.NewReader(""), &out, &errOut)
	last := agent.calls("GET", "/api/auth/me")
	if got := last[len(last)-1]; got.Auth != "" {
		t.Fatalf("an explicit --host must not borrow the local master token, sent %q", got.Auth)
	}
}

func mustPort(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Port()
}

func TestDocsAndCommandsManifest(t *testing.T) {
	code, out, _ := runCLI(t, nil, "", "docs")
	if code != 0 || !strings.Contains(out, "Safety rules") || !strings.Contains(out, "Command reference") {
		t.Fatalf("docs: exit %d\n%.200s", code, out)
	}
	if !strings.Contains(out, "docklite containers delete <name|domain|id>` — Delete a container") || !strings.Contains(out, "[destructive: needs --yes]") {
		t.Fatal("generated reference should list commands and mark destructive ones")
	}

	code, out, _ = runCLI(t, nil, "", "commands", "--json")
	var cmds []map[string]any
	if code != 0 || json.Unmarshal([]byte(out), &cmds) != nil || len(cmds) < 30 {
		t.Fatalf("commands manifest: exit %d, %d entries", code, len(cmds))
	}
	var sawDestructive bool
	for _, c := range cmds {
		if c["destructive"] == true {
			sawDestructive = true
		}
	}
	if !sawDestructive {
		t.Fatal("manifest must flag destructive commands")
	}
}

func TestEveryCommandHasHelpAndASummary(t *testing.T) {
	for _, c := range registry {
		if c.Summary == "" || c.Usage == "" || c.Run == nil {
			t.Errorf("incomplete command: %v", c.Path)
		}
		code, out, _ := runCLI(t, nil, "", append(append([]string{}, c.Path...), "--help")...)
		if code != 0 || !strings.Contains(out, "docklite "+c.Usage) {
			t.Errorf("%s --help: exit %d\n%s", c.name(), code, out)
		}
	}
}

func TestUsersCreateReadsPasswordFromStdin(t *testing.T) {
	agent := newFakeAgent(t)
	agent.json("POST", "/api/users", 201, `{"user":{"id":2}}`)
	code, _, errOut := runCLI(t, agent, "pw-from-stdin\n", "users", "create", "alice", "--admin", "--password-stdin")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	got := agent.calls("POST", "/api/users")
	if len(got) != 1 || !strings.Contains(got[0].Body, `"password":"pw-from-stdin"`) || !strings.Contains(got[0].Body, `"isAdmin":true`) {
		t.Fatalf("bad request: %+v", got)
	}
}

func TestDoctorReportsProblemsWithFixes(t *testing.T) {
	agent := newFakeAgent(t)
	agent.json("GET", "/api/health", 200, `{"status":"ok"}`)
	agent.json("GET", "/api/auth/me", 200, `{"user":{"username":"root","role":"super_admin","isAdmin":true}}`)
	agent.json("GET", "/api/server/services", 200, `{"docker":{"status":"running"},"proxy":{"name":"Nginx","status":"active"},"traefik":{"status":"running","warning":"nginx already serves ports 80/443"}}`)
	agent.json("POST", "/api/nginx/test", 200, `{"ok":true}`)
	agent.json("GET", "/api/ssl/status", 200, `{"allCerts":[{"domain":"old.example.com","status":"expired"}]}`)
	agent.json("GET", "/api/server/overview", 200, `{"disk":{"total":100,"free":5}}`)
	agent.json("GET", "/api/server/updates", 200, `{"pendingUpdates":3,"securityUpdates":0,"rebootRequired":false}`)

	code, out, _ := runCLI(t, agent, "", "doctor")
	if code != 0 {
		t.Fatalf("warnings alone must not fail doctor, exit %d\n%s", code, out)
	}
	for _, want := range []string{"traefik", "old.example.com (expired)", "95% full", "docklite server service traefik stop --yes"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output missing %q:\n%s", want, out)
		}
	}

	agent.json("POST", "/api/nginx/test", 200, `{"ok":false,"error":"nginx: [emerg] bad directive"}`)
	code, out, _ = runCLI(t, agent, "", "doctor", "--json")
	if code != exitFailure || !strings.Contains(out, `"failCount":1`) {
		t.Fatalf("a failing check must give exit 1 and say so: exit %d\n%s", code, out)
	}
}

func TestTableTidiesImagesAndPorts(t *testing.T) {
	if got := shortImage("sha256:b0f7830b6bfaa1258f45d94c240ab668ced1b3651c8a222aefe6683447c7bf55"); got != "b0f7830b6bfa" {
		t.Fatalf("image id: %q", got)
	}
	if got := shortImage("nginx:alpine"); got != "nginx:alpine" {
		t.Fatalf("named image changed: %q", got)
	}
	if got := dedupePorts("5433->5432, 5433->5432, 80->80"); got != "5433->5432, 80->80" {
		t.Fatalf("ports: %q", got)
	}
	if got := dedupePorts(""); got != "-" {
		t.Fatalf("empty ports: %q", got)
	}
}
