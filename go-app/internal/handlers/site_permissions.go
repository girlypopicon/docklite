package handlers

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Site folders belong to the docklite user and group. People who manage the server (and assistants working in their
// shell) are in the docklite group, so site folders and files must be group-writable (and folders "setgid", so new
// files keep the docklite group). Asking for mode 0775 isn't enough: the process's umask (usually 022) silently removes
// the group write bit, so permissions are set explicitly, after creating things.
//
// This only ever ADDS group read/write (and group-execute on folders) and the setgid bit on folders. It never changes the
// owner, never touches other users' permissions, never follows symlinks, and never makes anything world-writable.

const permsMarkerFile = "data/.site-perms-v1"

// shareWithGroup makes one path usable by the docklite group.
func shareWithGroup(path string) {
	st, err := os.Lstat(path)
	if err != nil || st.Mode()&os.ModeSymlink != 0 {
		return
	}
	mode := st.Mode().Perm() | 0o060
	if st.IsDir() {
		mode = st.Mode().Perm() | 0o070
		_ = os.Chmod(path, mode|os.ModeSetgid)
		return
	}
	_ = os.Chmod(path, mode)
}

// fixSitePermissions walks a folder and shares everything in it. It returns how many entries changed and how many
// could not be changed (for example files owned by root, which only root can fix).
func fixSitePermissions(root string) (changed, skipped int) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			skipped++
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil // links are left alone (chmod would follow them to somewhere else)
		}
		st, err := os.Lstat(path)
		if err != nil {
			skipped++
			return nil
		}
		var want fs.FileMode
		if d.IsDir() {
			want = (st.Mode().Perm() | 0o070) | os.ModeSetgid
		} else {
			want = st.Mode().Perm() | 0o060
		}
		have := st.Mode().Perm()
		if st.IsDir() {
			have |= st.Mode() & os.ModeSetgid
		}
		if have == want {
			return nil
		}
		if err := os.Chmod(path, want); err != nil {
			skipped++
			return nil
		}
		changed++
		return nil
	})
	return changed, skipped
}

type sitePermsResult struct {
	Domain  string `json:"domain"`
	Path    string `json:"path"`
	Changed int    `json:"changed"`
	Skipped int    `json:"skipped"`
}

// fixAllSitePermissions shares the sites root, every user folder and every registered site's folder.
func (h *Handlers) fixAllSitePermissions(only string) ([]sitePermsResult, error) {
	sites, err := h.store.ListSites()
	if err != nil {
		return nil, err
	}
	var out []sitePermsResult
	if only == "" {
		shareWithGroup(siteBaseDir)
		if entries, err := os.ReadDir(siteBaseDir); err == nil {
			for _, e := range entries {
				if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
					shareWithGroup(filepath.Join(siteBaseDir, e.Name()))
				}
			}
		}
	}
	for _, s := range sites {
		if only != "" && !strings.EqualFold(s.Domain, only) {
			continue
		}
		if s.CodePath == "" || !isWithinBase(s.CodePath) {
			continue // never touch a folder outside the sites root
		}
		c, k := fixSitePermissions(s.CodePath)
		out = append(out, sitePermsResult{Domain: s.Domain, Path: s.CodePath, Changed: c, Skipped: k})
	}
	return out, nil
}

// SitePermissions: POST {"domain": "x.com"} (or {} for every site) makes site folders usable by the docklite group.
func (h *Handlers) SitePermissions(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Domain string `json:"domain"`
	}
	_ = readJSON(w, r, &body)
	if body.Domain != "" && !isValidDomain(body.Domain) {
		writeError(w, http.StatusBadRequest, "enter a valid domain")
		return
	}
	res, err := h.fixAllSitePermissions(strings.TrimSpace(body.Domain))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	totalChanged, totalSkipped := 0, 0
	for _, x := range res {
		totalChanged += x.Changed
		totalSkipped += x.Skipped
	}
	h.audit(r, "sites.permissions", body.Domain, map[string]any{"changed": totalChanged, "skipped": totalSkipped})
	writeJSON(w, http.StatusOK, map[string]any{"sites": res, "changed": totalChanged, "skipped": totalSkipped})
}

// RunSitePermissionsOnce shares existing sites once, so servers set up before this fix work without anyone running a command.
func (h *Handlers) RunSitePermissionsOnce() {
	if _, err := os.Stat(permsMarkerFile); err == nil {
		return
	}
	res, err := h.fixAllSitePermissions("")
	if err != nil {
		return
	}
	changed := 0
	for _, x := range res {
		changed += x.Changed
	}
	auditSystem("sites.permissions", "", map[string]any{"changed": changed, "sites": len(res), "once": true})
	_ = os.MkdirAll(filepath.Dir(permsMarkerFile), 0o750)
	_ = os.WriteFile(permsMarkerFile, []byte("done\n"), 0o640)
}

// mkdirAllShared is os.MkdirAll that also makes every folder it creates usable by the docklite group.
func mkdirAllShared(dir string) error {
	var missing []string
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if _, err := os.Stat(d); err == nil {
			break
		}
		missing = append(missing, d)
		if filepath.Dir(d) == d {
			break
		}
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0o775); err != nil && !os.IsExist(err) {
			return err
		}
		shareWithGroup(missing[i])
	}
	return nil
}
