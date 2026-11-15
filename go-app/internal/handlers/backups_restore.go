package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"docklite-agent/internal/backup"
	"docklite-agent/internal/store"
)

// restoreTarget works out which site a backup file belongs to and where its folder is. domain may be empty, in which
// case the backup's own manifest says; naming a domain restores the files into that site instead (a copy to another site).
func (h *Handlers) restoreTarget(archive, domain string) (*store.SiteRecord, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		m, err := backup.ReadManifest(archive)
		if err != nil {
			return nil, err
		}
		if m != nil {
			domain = strings.ToLower(m.Source.Domain)
		}
		if domain == "" { // an old or hand-copied backup: the file name says site-<domain>-<timestamp>.tar.gz
			name := strings.TrimSuffix(filepath.Base(archive), ".tar.gz")
			name = strings.TrimPrefix(name, "site-")
			if i := strings.LastIndex(name, "-"); i > 0 {
				i2 := strings.LastIndex(name[:i], "-")
				if i2 > 0 {
					domain = name[:i2]
				}
			}
		}
	}
	if domain == "" {
		return nil, fmt.Errorf("can't tell which site this backup belongs to; name the site")
	}
	site, err := h.store.GetSiteByDomain(domain)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, fmt.Errorf("there is no site called %s here; create it first, then restore into it", domain)
	}
	if site.CodePath == "" || !isWithinBase(site.CodePath) {
		return nil, fmt.Errorf("the folder for %s isn't under %s, so DockLite won't restore into it", domain, siteBaseDir)
	}
	return site, nil
}

// restoreSiteBackup restores a backup file (path relative to the backup folder) into its site. For a package, fromSite
// names which of its sites to restore; it may be left out when the package holds only one.
func (h *Handlers) restoreSiteBackup(r *http.Request, file, domain, fromSite string) (*backup.RestoreResult, *store.SiteRecord, error) {
	archive, ok := h.resolveBackupPath(file)
	if !ok || !(strings.HasSuffix(archive, ".tar.gz") || backup.IsPackage(archive)) {
		return nil, nil, fmt.Errorf("that isn't a site backup file")
	}
	if err := backup.CheckAgainstManifest(archive); err != nil {
		return nil, nil, err
	}
	if backup.IsPackage(archive) {
		info, err := backup.InspectPackage(archive)
		if err != nil {
			return nil, nil, err
		}
		from, err := pickPackageSite(&info.Manifest, fromSite)
		if err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(domain) == "" {
			domain = from.Domain
		}
		site, err := h.restoreTarget(archive, domain)
		if err != nil {
			return nil, nil, err
		}
		res, err := backup.RestoreSiteFromPackage(r.Context(), archive, from.Folder, site.CodePath, filepath.Join(h.backupBaseDir, "safety"))
		if err != nil {
			return nil, nil, err
		}
		return res, site, nil
	}
	site, err := h.restoreTarget(archive, domain)
	if err != nil {
		return nil, nil, err
	}
	res, err := backup.RestoreSiteArchive(r.Context(), archive, site.CodePath, filepath.Join(h.backupBaseDir, "safety"))
	if err != nil {
		return nil, nil, err
	}
	return res, site, nil
}

// pickPackageSite finds the site to restore from a package: the one named, or the only one.
func pickPackageSite(m *backup.PackageManifest, name string) (*backup.PackageSite, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		switch len(m.Sites) {
		case 0:
			return nil, fmt.Errorf("this backup holds no site files (only databases)")
		case 1:
			return &m.Sites[0], nil
		}
		names := make([]string, 0, len(m.Sites))
		for _, s := range m.Sites {
			names = append(names, s.Domain)
		}
		return nil, fmt.Errorf("this backup holds %d sites (%s); say which one to restore", len(m.Sites), strings.Join(names, ", "))
	}
	for i := range m.Sites {
		if strings.EqualFold(m.Sites[i].Domain, name) {
			return &m.Sites[i], nil
		}
	}
	return nil, fmt.Errorf("this backup doesn't hold the files of %s", name)
}

// inspectPackage describes a .litpkg: what is in it, and for each site whether it exists here to restore into.
func (h *Handlers) inspectPackage(w http.ResponseWriter, archive string) {
	info, err := backup.InspectPackage(archive)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	type siteView struct {
		Domain  string `json:"domain"`
		Files   int    `json:"files"`
		Problem string `json:"problem,omitempty"` // why it can't be restored here as it is
	}
	sites := make([]siteView, 0, len(info.Manifest.Sites))
	for _, s := range info.Manifest.Sites {
		v := siteView{Domain: s.Domain, Files: s.Files}
		if _, err := h.restoreTarget(archive, s.Domain); err != nil {
			v.Problem = err.Error()
		}
		sites = append(sites, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"package":     true,
		"description": info.Manifest.Description,
		"created_at":  info.Manifest.CreatedAt,
		"sites":       sites,
		"databases":   info.Manifest.Databases,
		"problems":    info.Manifest.Problems,
		"notes":       info.Manifest.Notes,
		"bytes":       info.Bytes,
		"verified":    backup.CheckAgainstManifest(archive) == nil,
	})
}

// BackupInspect describes a backup file without changing anything: what site it is for, how many files, how big.
func (h *Handlers) BackupInspect(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	archive, ok := h.resolveBackupPath(r.URL.Query().Get("file"))
	if ok && backup.IsPackage(archive) {
		h.inspectPackage(w, archive)
		return
	}
	if !ok || !strings.HasSuffix(archive, ".tar.gz") {
		writeError(w, http.StatusBadRequest, "that isn't a site backup file")
		return
	}
	info, err := backup.InspectSiteArchive(archive)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	out := map[string]any{"files": info.Files, "bytes": info.Bytes, "skipped": info.Skipped, "verified": backup.CheckAgainstManifest(archive) == nil}
	if site, err := h.restoreTarget(archive, r.URL.Query().Get("domain")); err == nil {
		out["domain"] = site.Domain
	} else {
		out["problem"] = err.Error()
	}
	if m, _ := backup.ReadManifest(archive); m != nil {
		out["created_at"] = m.CreatedAt
	}
	writeJSON(w, http.StatusOK, out)
}

// BackupRestore puts a site back the way a backup had it. The caller must confirm: it replaces the site's files.
func (h *Handlers) BackupRestore(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		File     string `json:"file"`
		Domain   string `json:"domain"`    // the site to restore into; by default the one the files came from
		FromSite string `json:"from_site"` // which site of a package to restore
	}
	if err := readJSON(w, r, &body); err != nil || body.File == "" {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	res, site, err := h.restoreSiteBackup(r, body.File, body.Domain, body.FromSite)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"domain": site.Domain, "files": res.Files, "bytes": res.Bytes,
		"safety_backup": res.SafetyBackup, "skipped": res.Skipped,
		"message": "Restored " + site.Domain + ". What was there before was saved first, so this can be undone.",
	})
}
