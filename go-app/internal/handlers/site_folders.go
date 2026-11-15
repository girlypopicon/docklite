package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"docklite-agent/internal/store"
)

// Everything under /var/www/sites should be a user folder holding site folders.
// These endpoints show what is actually there, adopt folders DockLite doesn't
// know about, and quarantine orphans. Removing a folder moves it into
// /var/www/sites/.trash; the trash is the only place DockLite ever deletes
// from, either by hand (PurgeTrash) or on its own once an entry is older than
// trashKeepDays (RunTrashPurge).

const (
	trashDirName   = ".trash"
	trashStampSize = len("20060102-150405")
)

// trashKeepDays is how long a trashed folder is kept before it is deleted for good. DOCKLITE_TRASH_KEEP_DAYS overrides
// it; 0 turns the automatic purge off (entries then stay until someone empties the trash).
var trashKeepDays = func() int {
	if v, err := strconv.Atoi(os.Getenv("DOCKLITE_TRASH_KEEP_DAYS")); err == nil && v >= 0 {
		return v
	}
	return 30
}()

type folderEntry struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Depth       int    `json:"depth"`
	Status      string `json:"status"` // user-dir | registered | unregistered | unknown
	Owner       string `json:"owner,omitempty"`
	DetectedTyp string `json:"detected_type,omitempty"`
	Files       int    `json:"files"`
	SizeBytes   int64  `json:"size_bytes"`
	FsUID       int    `json:"fs_uid"`
	Writable    bool   `json:"writable"`
	MountedBy   string `json:"mounted_by,omitempty"`
	SiteID      int64  `json:"site_id,omitempty"`
	CanRemove   bool   `json:"can_remove"`
	Note        string `json:"note,omitempty"`
}

type folderGhost struct {
	SiteID int64  `json:"site_id"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// trashEntry is one folder in /var/www/sites/.trash. Its name is "<stamp>-<original path with / as __>", so the
// original location and the time it was trashed can be read back without a database.
type trashEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Original  string `json:"original"` // where it came from, relative to the sites root
	TrashedAt string `json:"trashed_at"`
	AgeDays   int    `json:"age_days"`
	Files     int    `json:"files"`
	SizeBytes int64  `json:"size_bytes"`
	// DeleteOn is when the automatic purge will remove it (empty if the purge is off).
	DeleteOn string `json:"delete_on,omitempty"`
}

type folderReport struct {
	Folders       []folderEntry `json:"folders"`
	Ghosts        []folderGhost `json:"ghosts"`
	Trash         []trashEntry  `json:"trash"`
	TrashKeepDays int           `json:"trash_keep_days"`
}

func detectSiteType(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return "node"
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "*.php")); len(m) > 0 {
		return "php"
	}
	return "static"
}

// measure counts files and bytes without following symlinks, stopping at a cap
// so a huge folder can't make the page slow.
func measure(dir string) (files int, size int64) {
	const cap = 20000
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if files >= cap {
			return filepath.SkipAll
		}
		if !info.IsDir() {
			files++
			size += info.Size()
		}
		return nil
	})
	return
}

// mountedFolders maps each host folder bind-mounted into any container to that container's name.
func (h *Handlers) mountedFolders(ctx context.Context) map[string]string {
	out := map[string]string{}
	if h.docker == nil {
		return out
	}
	list, err := h.docker.ListContainers(ctx, true)
	if err != nil {
		return out
	}
	for _, c := range list {
		info, err := h.docker.InspectContainer(ctx, c.ID)
		if err != nil {
			continue
		}
		for _, m := range info.Mounts {
			if m.Type == "bind" && m.Source != "" {
				out[filepath.Clean(m.Source)] = c.Name
			}
		}
	}
	return out
}

func mountUnder(mounts map[string]string, dir string) string {
	dir = filepath.Clean(dir)
	for src, name := range mounts {
		if src == dir || strings.HasPrefix(src, dir+string(filepath.Separator)) {
			return name
		}
	}
	return ""
}

func (h *Handlers) scanFolders(ctx context.Context) (*folderReport, error) {
	sites, err := h.store.ListSites()
	if err != nil {
		return nil, err
	}
	users, err := h.store.ListUsers()
	if err != nil {
		return nil, err
	}
	userNames := map[string]bool{}
	for _, u := range users {
		userNames[u.Username] = true
	}
	byPath := map[string]store.SiteRecord{}
	for _, s := range sites {
		if s.CodePath != "" {
			byPath[filepath.Clean(s.CodePath)] = s
		}
	}
	mounts := h.mountedFolders(ctx)
	report := &folderReport{Folders: []folderEntry{}, Ghosts: []folderGhost{}, Trash: listTrash(), TrashKeepDays: trashKeepDays}

	describe := func(path string, depth int) folderEntry {
		e := folderEntry{Path: path, Name: filepath.Base(path), Depth: depth}
		if st, err := os.Stat(path); err == nil {
			if sys, ok := st.Sys().(*syscall.Stat_t); ok {
				e.FsUID = int(sys.Uid)
			}
		}
		e.Writable = syscall.Access(path, 2) == nil
		e.MountedBy = mountUnder(mounts, path)
		return e
	}

	top, err := os.ReadDir(siteBaseDir)
	if err != nil {
		return nil, err
	}
	for _, t := range top {
		if !t.IsDir() {
			continue
		}
		if strings.HasPrefix(t.Name(), ".") { // includes the trash, listed separately
			continue
		}
		p := filepath.Join(siteBaseDir, t.Name())
		e := describe(p, 1)
		kids, _ := os.ReadDir(p)
		if userNames[t.Name()] {
			e.Status, e.Owner, e.Files = "user-dir", t.Name(), len(kids)
			e.CanRemove = len(kids) == 0
			report.Folders = append(report.Folders, e)
			for _, k := range kids {
				if !k.IsDir() || strings.HasPrefix(k.Name(), ".") {
					continue
				}
				report.Folders = append(report.Folders, h.classify(describe(filepath.Join(p, k.Name()), 2), t.Name(), byPath))
			}
			continue
		}
		// A folder at the top level that isn't a DockLite user: a site with no
		// username, or a leftover user folder full of sites.
		sub := 0
		for _, k := range kids {
			if k.IsDir() && isValidDomain(k.Name()) {
				sub++
			}
		}
		if _, registered := byPath[filepath.Clean(p)]; !registered && !isValidDomain(t.Name()) && sub > 0 {
			e.Status, e.Note, e.Files = "unknown", "looks like a user folder, but no DockLite user has this name", len(kids)
			report.Folders = append(report.Folders, e)
			for _, k := range kids {
				if k.IsDir() && isValidDomain(k.Name()) {
					c := h.classify(describe(filepath.Join(p, k.Name()), 2), "", byPath)
					report.Folders = append(report.Folders, c)
				}
			}
			continue
		}
		report.Folders = append(report.Folders, h.classify(e, "", byPath))
	}

	for _, s := range sites {
		if s.CodePath == "" {
			continue
		}
		if _, err := os.Stat(s.CodePath); err != nil && (s.ContainerID == nil || *s.ContainerID == "") {
			report.Ghosts = append(report.Ghosts, folderGhost{SiteID: s.ID, Domain: s.Domain, Path: s.CodePath})
		}
	}
	sort.Slice(report.Folders, func(i, j int) bool { return report.Folders[i].Path < report.Folders[j].Path })
	return report, nil
}

func (h *Handlers) classify(e folderEntry, owner string, byPath map[string]store.SiteRecord) folderEntry {
	e.Files, e.SizeBytes = measure(e.Path)
	e.Owner = owner
	if s, ok := byPath[filepath.Clean(e.Path)]; ok {
		e.Status, e.SiteID = "registered", s.ID
		e.DetectedTyp = s.TemplateType
		return e
	}
	e.DetectedTyp = detectSiteType(e.Path)
	if isValidDomain(e.Name) {
		e.Status = "unregistered"
		e.Note = "a site folder DockLite doesn't know about"
	} else {
		e.Status = "unknown"
		e.Note = "not a valid domain name, so it can't be a site"
	}
	e.CanRemove = e.MountedBy == ""
	return e
}

// SiteFolders: GET lists what is on disk.
func (h *Handlers) SiteFolders(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()
	report, err := h.scanFolders(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// folderTarget validates a client-supplied path: it must be a real folder one or
// two levels under the sites root, never the root, a symlink, or the trash.
func folderTarget(p string) (string, int, error) {
	p = filepath.Clean(p)
	rel, err := filepath.Rel(siteBaseDir, p)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", 0, fmt.Errorf("path must be inside %s", siteBaseDir)
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) > 2 {
		return "", 0, fmt.Errorf("only site folders (one or two levels deep) can be managed")
	}
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return "", 0, fmt.Errorf("hidden folders are not managed")
		}
	}
	st, err := os.Lstat(p)
	if err != nil || !st.IsDir() {
		return "", 0, fmt.Errorf("folder not found (symlinks are not followed)")
	}
	return p, len(parts), nil
}

// AdoptFolder registers an existing folder as a site without touching its files,
// its nginx config or any running container.
func (h *Handlers) AdoptFolder(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Path         string `json:"path"`
		UserID       int64  `json:"user_id"`
		TemplateType string `json:"template_type"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p, depth, err := folderTarget(body.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	domain := filepath.Base(p)
	if !isValidDomain(domain) {
		writeError(w, http.StatusBadRequest, "the folder name must be the site's domain, e.g. example.com")
		return
	}
	if existing, _ := h.store.GetSiteByDomain(domain); existing != nil {
		writeError(w, http.StatusConflict, "a site for "+domain+" is already registered")
		return
	}
	var owner *store.UserRecord
	if depth == 2 {
		owner, _ = h.store.GetUserByUsername(filepath.Base(filepath.Dir(p)))
	}
	if owner == nil && body.UserID > 0 {
		owner, _ = h.store.GetUserByIDFull(body.UserID)
	}
	if owner == nil {
		writeError(w, http.StatusBadRequest, "choose which user this site belongs to")
		return
	}
	tmpl := body.TemplateType
	if tmpl != "static" && tmpl != "php" && tmpl != "node" {
		tmpl = detectSiteType(p)
	}
	site, err := h.store.CreateSite(store.SiteRecord{Domain: domain, UserID: owner.ID, TemplateType: tmpl, CodePath: p})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = h.store.UpdateSiteStatus(site.ID, "stopped")
	// Leave a .dkl so a fresh install can re-attach to this folder later. An
	// existing .dkl is kept as is; a folder we can't write to just gets none.
	manifestWritten := false
	if _, err := os.Stat(filepath.Join(p, dklFilename)); err != nil {
		port := 0
		if tmpl == "node" {
			port = 3000
		}
		if WriteDKLManifest(p, domain, tmpl, owner.Username, port, true, nil) == nil {
			chownDocklite(filepath.Join(p, dklFilename))
			manifestWritten = true
		}
	}
	h.audit(r, "site.adopt", domain, map[string]any{"path": p, "owner": owner.Username})
	writeJSON(w, http.StatusCreated, map[string]any{
		"success": true, "manifest_written": manifestWritten, "site_id": site.ID, "domain": domain, "owner": owner.Username, "template_type": tmpl,
		"message": "registered; files, nginx and any running container were not touched",
	})
}

// TrashFolder moves an orphan folder into /var/www/sites/.trash. It refuses
// anything a site record or a container still uses.
func (h *Handlers) TrashFolder(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p, depth, err := folderTarget(body.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sites, err := h.store.ListSites()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, s := range sites {
		cp := filepath.Clean(s.CodePath)
		if cp == p || strings.HasPrefix(cp, p+string(filepath.Separator)) {
			writeError(w, http.StatusConflict, "site "+s.Domain+" still uses this folder; delete the site first")
			return
		}
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()
	if name := mountUnder(h.mountedFolders(ctx), p); name != "" {
		writeError(w, http.StatusConflict, "container "+name+" is using this folder")
		return
	}
	if depth == 1 {
		if u, _ := h.store.GetUserByUsername(filepath.Base(p)); u != nil {
			if kids, _ := os.ReadDir(p); len(kids) > 0 {
				writeError(w, http.StatusConflict, "this is a user's folder and it still has sites in it")
				return
			}
		}
	}
	dest, err := moveToTrash(p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "site.folder-trash", p, map[string]any{"to": dest})
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "moved_to": dest})
}

// moveToTrash moves a site folder into /var/www/sites/.trash (never deletes). The path must be a folder one or two
// levels under the sites root.
func moveToTrash(p string) (string, error) {
	p, _, err := folderTarget(p)
	if err != nil {
		return "", err
	}
	trash := filepath.Join(siteBaseDir, trashDirName)
	if err := os.MkdirAll(trash, 0o775); err != nil {
		return "", fmt.Errorf("cannot create the trash folder: %w", err)
	}
	chownDocklite(trash)
	shareWithGroup(trash)
	rel, _ := filepath.Rel(siteBaseDir, p)
	name := time.Now().UTC().Format("20060102-150405") + "-" + strings.ReplaceAll(rel, string(filepath.Separator), "__")
	dest := filepath.Join(trash, name)
	err = os.Rename(p, dest)
	if err != nil && os.IsPermission(err) && !isTestSitesRoot() {
		// The folder (or its parent) is owned by root, from an older install or a hand-made site. The root helper
		// moves it for us and hands it to the docklite user so the trash can be emptied later without root.
		if out, herr := runRootHelper(nil, "site-trash", rel, name); herr == nil {
			err = nil
		} else {
			err = fmt.Errorf("%w; the root helper also failed: %s", err, strings.TrimSpace(string(out)))
		}
	}
	if err != nil {
		return "", fmt.Errorf("could not move it (run by hand: sudo mv %s %s): %w", p, dest, err)
	}
	return dest, nil
}

// isTestSitesRoot reports whether the sites root is somewhere other than the real one (tests point it at a temp
// folder); the root helper only knows the real root, so it is never asked about anything else.
func isTestSitesRoot() bool { return siteBaseDir != "/var/www/sites" }

// trashName checks a client-supplied trash entry name: exactly what moveToTrash produces, one path component, nothing
// hidden, no separators.
func trashName(name string) (string, error) {
	if len(name) <= trashStampSize+1 || strings.ContainsAny(name, `/\`+"\x00") || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("not a trash entry: %q", name)
	}
	if _, err := time.Parse("20060102-150405", name[:trashStampSize]); err != nil || name[trashStampSize] != '-' {
		return "", fmt.Errorf("not a trash entry: %q", name)
	}
	st, err := os.Lstat(filepath.Join(siteBaseDir, trashDirName, name))
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("%s is not in the trash", name)
	}
	return name, nil
}

// listTrash describes everything in the trash, oldest first.
func listTrash() []trashEntry {
	out := []trashEntry{}
	entries, err := os.ReadDir(filepath.Join(siteBaseDir, trashDirName))
	if err != nil {
		return out
	}
	now := time.Now().UTC()
	for _, e := range entries {
		if _, err := trashName(e.Name()); err != nil {
			continue
		}
		t := trashEntry{Name: e.Name(), Path: filepath.Join(siteBaseDir, trashDirName, e.Name())}
		stamp, _ := time.Parse("20060102-150405", e.Name()[:trashStampSize])
		t.Original = strings.ReplaceAll(e.Name()[trashStampSize+1:], "__", "/")
		t.TrashedAt = stamp.Format(time.RFC3339)
		t.AgeDays = int(now.Sub(stamp).Hours() / 24)
		if trashKeepDays > 0 {
			t.DeleteOn = stamp.AddDate(0, 0, trashKeepDays).Format(time.RFC3339)
		}
		t.Files, t.SizeBytes = measure(t.Path)
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// purgeTrashEntry deletes one trash entry for good. Entries the docklite user cannot delete (root-owned files from
// before the move) go through the root helper.
func purgeTrashEntry(name string) error {
	name, err := trashName(name)
	if err != nil {
		return err
	}
	p := filepath.Join(siteBaseDir, trashDirName, name)
	err = os.RemoveAll(p)
	if err == nil {
		if _, statErr := os.Lstat(p); statErr == nil {
			err = fmt.Errorf("%s is still there", p)
		}
	}
	if err != nil && !isTestSitesRoot() {
		out, herr := runRootHelper(nil, "site-purge", name)
		if herr == nil {
			return nil
		}
		err = fmt.Errorf("%w; the root helper also failed: %s", err, strings.TrimSpace(string(out)))
	}
	if err != nil {
		return fmt.Errorf("could not delete %s: %w", p, err)
	}
	return nil
}

// purgeTrashOlderThan deletes every trash entry trashed at least keepDays ago. It returns what it deleted and the
// first error it hit (the rest are still attempted).
func purgeTrashOlderThan(keepDays int, now time.Time) (deleted []string, firstErr error) {
	if keepDays <= 0 {
		return nil, nil
	}
	cutoff := now.UTC().AddDate(0, 0, -keepDays)
	for _, t := range listTrash() {
		stamp, _ := time.Parse(time.RFC3339, t.TrashedAt)
		if stamp.After(cutoff) {
			continue
		}
		if err := purgeTrashEntry(t.Name); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		deleted = append(deleted, t.Name)
	}
	return deleted, firstErr
}

// RunTrashPurge empties old trash entries once at start and then daily. It runs in the background from main.
func (h *Handlers) RunTrashPurge(ctx context.Context) {
	if trashKeepDays <= 0 {
		return
	}
	run := func() {
		deleted, err := purgeTrashOlderThan(trashKeepDays, time.Now())
		for _, d := range deleted {
			log.Printf("site folders: deleted %s from the trash (older than %d days)", d, trashKeepDays)
		}
		if err != nil {
			log.Printf("site folders: trash purge: %v", err)
		}
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(2 * time.Minute):
		run()
	}
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

// PurgeTrash: POST {"name": "<entry>"} deletes one trash entry for good; {"all": true} empties the trash.
func (h *Handlers) PurgeTrash(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Name string `json:"name"`
		All  bool   `json:"all"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var names []string
	switch {
	case body.All:
		for _, t := range listTrash() {
			names = append(names, t.Name)
		}
	case body.Name != "":
		if _, err := trashName(body.Name); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		names = []string{body.Name}
	default:
		writeError(w, http.StatusBadRequest, "say which trash entry to delete (name), or all")
		return
	}
	deleted := []string{}
	var failed []string
	for _, n := range names {
		if err := purgeTrashEntry(n); err != nil {
			failed = append(failed, err.Error())
			continue
		}
		deleted = append(deleted, n)
	}
	h.audit(r, "site.folder-purge", strings.Join(deleted, ","), map[string]any{"all": body.All, "failed": len(failed)})
	if len(failed) > 0 {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "deleted": deleted, "error": strings.Join(failed, "; ")})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "deleted": deleted, "remaining": len(listTrash())})
}
