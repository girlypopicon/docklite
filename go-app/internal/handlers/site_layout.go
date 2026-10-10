package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"docklite-agent/internal/store"
)

// Site layout normalisation: every site lives in /var/www/sites/<user>/<domain>.
// Older servers have sites at /var/www/sites/<domain> (no user folder). The plan
// is read-only; applying copies the files to the right place, points the site's
// container at them and leaves the old folder untouched, so nothing is lost.

const (
	layoutOK       = "ok"
	layoutMove     = "move"
	layoutSkip     = "skip"
	layoutMoved    = "moved"
	layoutFailed   = "failed"
	layoutNoFolder = "no-folder"
)

type layoutItem struct {
	SiteID  int64  `json:"site_id"`
	Domain  string `json:"domain"`
	Owner   string `json:"owner"`
	From    string `json:"from"`
	To      string `json:"to"`
	Action  string `json:"action"`
	Reason  string `json:"reason,omitempty"`
	Running bool   `json:"running"`
}

type layoutStray struct {
	Path string `json:"path"`
	Note string `json:"note"`
}

type layoutReport struct {
	Items   []layoutItem  `json:"items"`
	Strays  []layoutStray `json:"strays"`
	ToMove  int           `json:"to_move"`
	Applied bool          `json:"applied"`
}

func (h *Handlers) planSiteLayout() (*layoutReport, error) {
	sites, err := h.store.ListSites()
	if err != nil {
		return nil, err
	}
	users, err := h.store.ListUsers()
	if err != nil {
		return nil, err
	}
	names := map[int64]string{}
	for _, u := range users {
		names[u.ID] = u.Username
	}

	report := &layoutReport{Items: []layoutItem{}, Strays: []layoutStray{}}
	claimed := map[string]bool{}
	sort.Slice(sites, func(i, j int) bool { return sites[i].Domain < sites[j].Domain })
	for _, s := range sites {
		owner := names[s.UserID]
		item := layoutItem{SiteID: s.ID, Domain: s.Domain, Owner: owner, Running: s.Status == "running"}
		from := s.CodePath
		if from == "" {
			// Older records have no path; fall back to where DockLite would have looked.
			for _, cand := range []string{filepath.Join(siteBaseDir, owner, s.Domain), filepath.Join(siteBaseDir, s.Domain)} {
				if st, err := os.Stat(cand); err == nil && st.IsDir() {
					from = cand
					break
				}
			}
		}
		item.From = from
		claimed[filepath.Clean(from)] = true
		if owner == "" || !isSafeLayoutName(owner) || !isValidDomain(s.Domain) {
			item.Action, item.Reason = layoutSkip, "owner or domain has an unusual name; fix it by hand"
			report.Items = append(report.Items, item)
			continue
		}
		item.To = getSitePath(owner, s.Domain)
		claimed[item.To] = true
		switch {
		case filepath.Clean(from) == item.To:
			item.Action = layoutOK
		case from == "":
			item.Action, item.Reason = layoutNoFolder, "no site folder found to move"
		case !dirExists(from):
			item.Action, item.Reason = layoutSkip, "folder "+from+" does not exist"
		case !isWithinBase(from):
			item.Action, item.Reason = layoutSkip, "folder is outside "+siteBaseDir+"; left alone"
		case dirHasEntries(item.To):
			item.Action, item.Reason = layoutSkip, "target folder already has files; needs a human decision"
		default:
			item.Action = layoutMove
			report.ToMove++
		}
		report.Items = append(report.Items, item)
	}

	// Folders directly under /var/www/sites that look like domains and belong to no site.
	if entries, err := os.ReadDir(siteBaseDir); err == nil {
		userDirs := map[string]bool{}
		for _, n := range names {
			userDirs[n] = true
		}
		for _, e := range entries {
			if !e.IsDir() || userDirs[e.Name()] || claimed[filepath.Join(siteBaseDir, e.Name())] {
				continue
			}
			note := "not a DockLite user or a registered site"
			if isValidDomain(e.Name()) {
				note = "looks like a site folder with no username and no DockLite record; adopt it, then it can be moved"
			}
			report.Strays = append(report.Strays, layoutStray{Path: filepath.Join(siteBaseDir, e.Name()), Note: note})
		}
	}
	return report, nil
}

func isSafeLayoutName(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, `/\`+"\x00")
}

func isWithinBase(p string) bool {
	rel, err := filepath.Rel(siteBaseDir, filepath.Clean(p))
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func dirHasEntries(p string) bool {
	entries, err := os.ReadDir(p)
	return err == nil && len(entries) > 0
}

// SiteLayout: GET returns the plan; POST {"apply":true} carries it out.
func (h *Handlers) SiteLayout(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		report, err := h.planSiteLayout()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, report)
	case http.MethodPost:
		var body struct {
			Apply  bool   `json:"apply"`
			Domain string `json:"domain"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		report, err := h.planSiteLayout()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !body.Apply {
			writeJSON(w, http.StatusOK, report)
			return
		}
		for i := range report.Items {
			it := &report.Items[i]
			if it.Action != layoutMove || (body.Domain != "" && body.Domain != it.Domain) {
				continue
			}
			if err := h.applyLayoutMove(r.Context(), it); err != nil {
				it.Action, it.Reason = layoutFailed, err.Error()
				continue
			}
			it.Action = layoutMoved
			h.audit(r, "site.layout", it.Domain, map[string]any{"from": it.From, "to": it.To})
		}
		report.Applied = true
		writeJSON(w, http.StatusOK, report)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// applyLayoutMove normalizes one site's folder (same owner, new location).
func (h *Handlers) applyLayoutMove(parent context.Context, it *layoutItem) error {
	site, err := h.store.GetSiteByID(it.SiteID)
	if err != nil || site == nil {
		return fmt.Errorf("site record not found")
	}
	return h.relocateSite(site, site.UserID, it.Owner, it.From, it.To)
}

// relocateSite moves a site's files to `to` (a folder under newUsername) and re-points its container, nginx and record,
// optionally under a different owner. It works copy-then-switch: the old folder is never touched, and the old container
// is only given up once the new one is running; any failure puts the old container back. It never deletes anything: the
// caller decides what to do with the old folder.
func (h *Handlers) relocateSite(site *store.SiteRecord, newUserID int64, newUsername, from, to string) error {
	samePath := filepath.Clean(from) == filepath.Clean(to)
	if !samePath {
		if !isWithinBase(to) {
			return fmt.Errorf("refusing to move a site outside %s", siteBaseDir)
		}
		if err := ensureSiteDirectory(newUsername, site.Domain); err != nil {
			return err
		}
		if dirHasEntries(to) {
			return fmt.Errorf("the target folder %s already has files; not overwriting them", to)
		}
		// --no-preserve=ownership: we run as the docklite user and just want a faithful copy (links, modes, empty folders).
		if out, err := exec.Command("cp", "-a", "--no-preserve=ownership", from+"/.", to+"/").CombinedOutput(); err != nil {
			clearFolderContents(to) // it was empty before, so everything in it is our partial copy
			return fmt.Errorf("copy failed: %s", strings.TrimSpace(string(out)))
		}
		chownDocklite(to)
	}

	finish := func(ctx context.Context, containerID string, internalPort int, includeWww bool) error {
		if err := h.store.TransferSite(site.ID, newUserID, to); err != nil {
			return err
		}
		_ = WriteDKLManifest(to, site.Domain, site.TemplateType, newUsername, internalPort, includeWww, nil)
		return nil
	}

	if site.ContainerID == nil || *site.ContainerID == "" {
		return finish(context.Background(), "", 80, true)
	}
	ctx, cancel := ctxWithLongTimeout()
	defer cancel()
	old, err := h.docker.InspectContainer(ctx, *site.ContainerID)
	if err != nil || old.Config == nil {
		// No live container to disturb; just record the new location.
		return finish(ctx, "", 80, true)
	}
	includeWww := old.Config.Labels["docklite.include_www"] == "true"
	internalPort := 80
	if p, err := strconv.Atoi(old.Config.Labels["docklite.internal_port"]); err == nil && p > 0 {
		internalPort = p
	}
	wasRunning := old.State != nil && old.State.Running

	// Put the old container back (serving from the old folder) if anything below fails.
	restoreOld := func(reason error) error {
		if backID, backErr := h.docker.CreateSiteContainer(ctx, site.Domain, site.TemplateType, includeWww, from, internalPort, site.ID, site.UserID, site.FolderID); backErr == nil {
			_ = h.store.UpdateSiteContainerID(site.ID, &backID)
			h.repointNginx(ctx, site.Domain, backID, internalPort, includeWww)
			return fmt.Errorf("%v (restored the old one)", reason)
		}
		return fmt.Errorf("%v, and the old container could not be restored automatically", reason)
	}

	if err := h.docker.RemoveContainer(ctx, *site.ContainerID); err != nil {
		return fmt.Errorf("could not stop the old container: %w", err)
	}
	newID, err := h.docker.CreateSiteContainer(ctx, site.Domain, site.TemplateType, includeWww, to, internalPort, site.ID, newUserID, site.FolderID)
	if err != nil {
		return restoreOld(fmt.Errorf("could not start the site from the new folder: %w", err))
	}
	_ = h.store.UpdateSiteContainerID(site.ID, &newID)
	if wasRunning {
		_ = h.store.UpdateSiteStatus(site.ID, "running")
	}
	h.repointNginx(ctx, site.Domain, newID, internalPort, includeWww)
	if err := finish(ctx, newID, internalPort, includeWww); err != nil {
		_ = h.docker.RemoveContainer(ctx, newID)
		return restoreOld(fmt.Errorf("could not save the change: %w", err))
	}
	return nil
}

// clearFolderContents empties a folder (used only on a folder we just created empty).
func clearFolderContents(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil || !isWithinBase(dir) {
		return
	}
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}

func (h *Handlers) repointNginx(ctx context.Context, domain, containerID string, internalPort int, includeWww bool) {
	hostPort, err := h.getContainerHostPort(ctx, containerID, internalPort)
	if err != nil || hostPort <= 0 {
		return
	}
	if updateNginxProxyPort(domain, hostPort) != nil {
		_ = setupNginxForDomain(domain, includeWww, hostPort)
	}
}
