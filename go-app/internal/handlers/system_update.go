package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"docklite-agent/internal/demo"
)

// Self-update, built on GitHub releases.
//
//   - "Is there a newer version?" asks GitHub for the repository's latest release (or, if there are no
//     releases, its newest vX.Y.Z tag) and compares it with the VERSION file of this install.
//   - "Update" asks the root helper to run the update. The helper accepts only a release tag like
//     v1.2.0, runs as a separate systemd job (so it survives DockLite restarting itself), backs up the
//     data first, installs that exact tag, checks DockLite came back, and rolls back if it didn't.
//   - Progress is a log file plus a small state file, both read here without needing root.

const (
	updateLogFile    = "/var/log/docklite/update.log"
	updateStateFile  = "/var/lib/docklite/update-state"
	updateLogTail    = 150
	updateStaleAfter = 45 * time.Minute
	defaultRepoSlug  = "girlypopicon/docklite"
)

// githubAPI is a variable so tests (and the demo, loopback only) can use a fake.
const defaultGithubAPI = "https://api.github.com"

var githubAPI = defaultGithubAPI

func init() {
	if v := os.Getenv("DOCKLITE_GITHUB_API"); strings.HasPrefix(v, "http://127.0.0.1:") || strings.HasPrefix(v, "http://localhost:") {
		githubAPI = strings.TrimRight(v, "/")
	}
}

// startUpdateJob asks the root helper to start the update. A variable so tests never start a real one.
var startUpdateJob = func(tag string) ([]byte, error) { return runRootHelper(nil, "update-start", tag) }

var releaseTagRE = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

type semver [3]int

func parseSemver(tag string) (semver, bool) {
	m := releaseTagRE.FindStringSubmatch(tag)
	if m == nil {
		return semver{}, false
	}
	var v semver
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return semver{}, false
		}
		v[i] = n
	}
	return v, true
}

// compareSemver returns -1, 0 or 1.
func compareSemver(a, b semver) int {
	for i := 0; i < 3; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

type releaseInfo struct {
	Tag         string `json:"tag"`
	Version     string `json:"version"`
	Notes       string `json:"notes"`
	URL         string `json:"url"`
	PublishedAt string `json:"publishedAt"`
}

var releaseCache struct {
	sync.Mutex
	at   time.Time
	info *releaseInfo
	err  string
}

func repoSlug() string {
	if v := strings.TrimSpace(os.Getenv("DOCKLITE_REPO_SLUG")); v != "" {
		return v
	}
	return defaultRepoSlug
}

func githubGet(ctx context.Context, path string, into any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPI+path, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "docklite-update-check")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("GitHub answered HTTP %d", resp.StatusCode)
	}
	return resp.StatusCode, json.Unmarshal(body, into)
}

// fetchLatestRelease finds the newest version on GitHub. Results are cached for 10 minutes
// (GitHub allows only 60 unauthenticated requests an hour per address); force skips the cache
// but never asks more than once every 30 seconds.
func fetchLatestRelease(ctx context.Context, force bool) (*releaseInfo, error) {
	releaseCache.Lock()
	defer releaseCache.Unlock()
	age := time.Since(releaseCache.at)
	if !releaseCache.at.IsZero() && ((!force && age < 10*time.Minute) || age < 30*time.Second) {
		if releaseCache.info != nil {
			return releaseCache.info, nil
		}
		return nil, fmt.Errorf("%s", releaseCache.err)
	}
	info, err := lookupLatest(ctx)
	releaseCache.at = time.Now()
	releaseCache.info = info
	if err != nil {
		releaseCache.err = err.Error()
	}
	return info, err
}

func lookupLatest(ctx context.Context) (*releaseInfo, error) {
	slug := repoSlug()
	var rel struct {
		TagName     string `json:"tag_name"`
		Body        string `json:"body"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
		Prerelease  bool   `json:"prerelease"`
		Draft       bool   `json:"draft"`
	}
	if code, err := githubGet(ctx, "/repos/"+slug+"/releases/latest", &rel); err == nil {
		if _, ok := parseSemver(rel.TagName); ok && !rel.Draft && !rel.Prerelease {
			return &releaseInfo{Tag: rel.TagName, Version: strings.TrimPrefix(rel.TagName, "v"), Notes: rel.Body, URL: rel.HTMLURL, PublishedAt: rel.PublishedAt}, nil
		}
	} else if code != http.StatusNotFound && code != 0 {
		return nil, err
	} else if code == 0 {
		return nil, fmt.Errorf("couldn't reach GitHub: %v", err)
	}

	// No usable release: fall back to the newest vX.Y.Z tag.
	var tags []struct {
		Name string `json:"name"`
	}
	if _, err := githubGet(ctx, "/repos/"+slug+"/tags?per_page=100", &tags); err != nil {
		return nil, err
	}
	var best semver
	bestTag := ""
	for _, t := range tags {
		if v, ok := parseSemver(t.Name); ok && (bestTag == "" || compareSemver(v, best) > 0) {
			best, bestTag = v, t.Name
		}
	}
	if bestTag == "" {
		return nil, fmt.Errorf("no releases found on GitHub yet")
	}
	return &releaseInfo{Tag: bestTag, Version: strings.TrimPrefix(bestTag, "v"), URL: "https://github.com/" + slug + "/releases/tag/" + bestTag}, nil
}

type updateStatusResponse struct {
	Version         string   `json:"version"`
	LatestVersion   string   `json:"latestVersion"`
	LatestTag       string   `json:"latestTag"`
	UpdateAvailable bool     `json:"updateAvailable"`
	Notes           string   `json:"notes"`
	ReleaseURL      string   `json:"releaseUrl"`
	CheckError      string   `json:"checkError,omitempty"`
	UpdateRunning   bool     `json:"updateRunning"`
	State           string   `json:"state"` // idle | running | success | failed | rolled-back
	StateTag        string   `json:"stateTag,omitempty"`
	StateMessage    string   `json:"stateMessage,omitempty"`
	Log             []string `json:"log"`
	LastUpdated     string   `json:"lastUpdated"`
}

// readUpdateState parses the state file the helper writes: "<state> <tag> <unix time> [message...]".
func readUpdateState(path string) (state, tag string, at time.Time, msg string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "idle", "", time.Time{}, ""
	}
	f := strings.Fields(strings.TrimSpace(string(data)))
	if len(f) < 3 {
		return "idle", "", time.Time{}, ""
	}
	if n, err := strconv.ParseInt(f[2], 10, 64); err == nil {
		at = time.Unix(n, 0)
	}
	if len(f) > 3 {
		msg = strings.Join(f[3:], " ")
	}
	return f[0], f[1], at, msg
}

func (h *Handlers) SystemUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	resp := updateStatusResponse{Version: readVersion(installDir()), State: "idle", Log: tailLog(updateLogFile, updateLogTail)}

	state, tag, at, msg := readUpdateState(updateStateFile)
	if state == "running" && time.Since(at) > updateStaleAfter {
		state = "failed"
		msg = "the update seems to have stopped without finishing"
	}
	resp.State, resp.StateTag, resp.StateMessage = state, tag, msg
	resp.UpdateRunning = state == "running"
	resp.LastUpdated = lastModified(updateLogFile)

	if demo.On && githubAPI == defaultGithubAPI {
		// A demo never calls the real GitHub; it just says it's up to date (unless a pretend GitHub is set up).
		resp.LatestVersion, resp.LatestTag = resp.Version, "v"+resp.Version
		writeJSON(w, http.StatusOK, resp)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	latest, err := fetchLatestRelease(ctx, r.URL.Query().Get("refresh") == "1")
	if err != nil {
		resp.CheckError = err.Error()
	} else {
		resp.LatestVersion, resp.LatestTag, resp.Notes, resp.ReleaseURL = latest.Version, latest.Tag, latest.Notes, latest.URL
		cur, curOK := parseSemver("v" + strings.TrimPrefix(resp.Version, "v"))
		lat, latOK := parseSemver(latest.Tag)
		resp.UpdateAvailable = curOK && latOK && compareSemver(lat, cur) > 0
	}
	writeJSON(w, http.StatusOK, resp)
}

// SystemUpdateRun starts an update to the newest release (or a named older/newer tag). Super admin only.
func (h *Handlers) SystemUpdateRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isSuperAdminRole(r) {
		writeError(w, http.StatusForbidden, "super_admin required")
		return
	}
	if demo.On {
		writeError(w, http.StatusForbidden, "updates are disabled in demo mode")
		return
	}
	var body struct {
		Version string `json:"version"`
		Force   bool   `json:"force"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	tag := strings.TrimSpace(body.Version)
	if tag == "" {
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		latest, err := fetchLatestRelease(ctx, true)
		if err != nil {
			writeError(w, http.StatusBadGateway, "couldn't find the newest version: "+err.Error())
			return
		}
		tag = latest.Tag
	} else if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	target, ok := parseSemver(tag)
	if !ok {
		writeError(w, http.StatusBadRequest, "version must look like 1.2.0")
		return
	}
	if cur, curOK := parseSemver("v" + strings.TrimPrefix(readVersion(installDir()), "v")); curOK && !body.Force && compareSemver(target, cur) <= 0 {
		writeError(w, http.StatusConflict, "you already have "+tag+" or newer")
		return
	}

	out, err := startUpdateJob(tag)
	h.audit(r, "system.update", tag, map[string]any{"from": readVersion(installDir()), "ok": err == nil})
	if err != nil {
		msg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "docklite-helper:"))
		if msg == "" {
			msg = err.Error()
		}
		writeError(w, http.StatusInternalServerError, msg)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started", "version": strings.TrimPrefix(tag, "v")})
}

// --- helpers ---

func installDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "/opt/docklite"
	}
	// the binary lives at <INSTALL_DIR>/bin/docklite-agent
	return filepath.Dir(filepath.Dir(exe))
}

func readVersion(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

func tailLog(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

func lastModified(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return info.ModTime().UTC().Format(time.RFC3339)
}
