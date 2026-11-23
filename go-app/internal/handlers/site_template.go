package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ChangeSiteTemplate is POST /api/containers/{id}/template {template_type, port?}: run the same site as another kind
// (static, php or node). The site's files, owner, HTTPS and address stay as they are; only the container it runs in is
// replaced. Like moving a site to another owner, a failure puts the old container back.
//
// Nothing in the site's folder is changed: switching to node needs the site's own index.js, to php an index.php, and
// the answer says so when the file is missing.
func (h *Handlers) ChangeSiteTemplate(w http.ResponseWriter, r *http.Request, containerID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		TemplateType string `json:"template_type"`
		Port         *int   `json:"port"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	newType := strings.ToLower(strings.TrimSpace(req.TemplateType))
	if newType != "static" && newType != "php" && newType != "node" {
		writeError(w, http.StatusBadRequest, "template_type must be static, php or node")
		return
	}

	site, err := h.store.GetSiteByContainerIDRecord(containerID)
	if err != nil || site == nil {
		writeError(w, http.StatusNotFound, "this isn't one of DockLite's websites, so it has no type to change")
		return
	}
	if !isAdminRole(r) {
		uid, uerr := readUserID(r)
		if uerr != nil || uid == nil || *uid != site.UserID {
			writeError(w, http.StatusForbidden, "only the site's owner or an admin can change its type")
			return
		}
	}

	ctx, cancel := ctxWithLongTimeout()
	defer cancel()
	old, err := h.docker.InspectContainer(ctx, containerID)
	if err != nil || old.Config == nil {
		writeError(w, http.StatusNotFound, "couldn't look at this site's container")
		return
	}
	includeWww := old.Config.Labels["docklite.include_www"] == "true"
	// What the container really is is the truth. The saved type can be wrong (an earlier switch that was rolled back used
	// to leave it on the new type), and trusting it made "switch to node" answer "already node" for a static container.
	currentType := currentSiteType(site.TemplateType, old.Config.Labels["docklite.type"])
	if currentType != site.TemplateType {
		_ = h.store.UpdateSiteTemplate(site.ID, currentType)
	}
	oldPort := 80
	if p, err := strconv.Atoi(old.Config.Labels["docklite.internal_port"]); err == nil && p > 0 {
		oldPort = p
	}
	newPort := 80
	if newType == "node" {
		newPort = 3000
		if currentType == "node" && oldPort > 0 {
			newPort = oldPort
		}
		if req.Port != nil {
			newPort = *req.Port
		}
		if newPort < 1 || newPort > 65535 {
			writeError(w, http.StatusBadRequest, "invalid port")
			return
		}
	}
	if newType == currentType && newPort == oldPort {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "template_type": newType, "message": "already " + newType})
		return
	}

	owner, _ := h.store.GetUserByIDFull(site.UserID)
	username := "unknown"
	if owner != nil {
		username = owner.Username
	}
	sitePath := site.CodePath
	if sitePath == "" {
		sitePath = getSitePath(username, site.Domain)
	}
	if st, err := os.Stat(sitePath); err != nil || !st.IsDir() {
		writeError(w, http.StatusConflict, "this site's folder "+sitePath+" doesn't exist, so there is nothing to run")
		return
	}
	wasRunning := old.State != nil && old.State.Running
	oldType := currentType

	// Check before touching anything: a site that can't run as the new kind would only crash. Switching plain files to
	// Node is the common case, so DockLite adds a small index.js that serves them first (never replacing a file), and
	// the switch carries on from there.
	var added []string
	if need := missingEntryFile(sitePath, newType); need != "" {
		if !(newType == "node" && hasAnyFile(sitePath, "index.html", "index.htm")) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": need, "code": "missing_entry", "needs": entryFileFor(newType)})
			return
		}
		if err := ensureFile(filepath.Join(sitePath, "index.js"), nodeStarterIndexJS); err != nil {
			writeError(w, http.StatusInternalServerError, "couldn't add index.js: "+err.Error())
			return
		}
		added = append(added, "index.js")
		if _, err := os.Stat(filepath.Join(sitePath, "package.json")); os.IsNotExist(err) {
			if ensureFile(filepath.Join(sitePath, "package.json"), nodePackageJSON(site.Domain)) == nil {
				added = append(added, "package.json")
			}
		}
	}

	restoreOld := func(reason error) error {
		// The saved type must go back with the container, or the database says "node" while a static container runs.
		_ = h.store.UpdateSiteTemplate(site.ID, oldType)
		if backID, backErr := h.docker.CreateSiteContainer(ctx, site.Domain, oldType, includeWww, sitePath, oldPort, site.ID, site.UserID, site.FolderID); backErr == nil {
			_ = h.store.UpdateSiteContainerID(site.ID, &backID)
			h.repointNginx(ctx, site.Domain, backID, oldPort, includeWww)
			return fmt.Errorf("%v (the site is back as %s)", reason, oldType)
		}
		return fmt.Errorf("%v, and the old container could not be restored automatically", reason)
	}

	if err := h.docker.RemoveContainer(ctx, containerID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not stop the current container: "+err.Error())
		return
	}
	newID, err := h.docker.CreateSiteContainer(ctx, site.Domain, newType, includeWww, sitePath, newPort, site.ID, site.UserID, site.FolderID)
	if err != nil {
		rerr := restoreOld(fmt.Errorf("could not start the site as %s: %w", newType, err))
		h.audit(r, "site.template", site.Domain, map[string]any{"from": oldType, "to": newType, "ok": false})
		writeError(w, http.StatusInternalServerError, rerr.Error())
		return
	}
	if err := h.store.UpdateSiteTemplate(site.ID, newType); err != nil {
		_ = h.docker.RemoveContainer(ctx, newID)
		writeError(w, http.StatusInternalServerError, restoreOld(fmt.Errorf("could not save the change: %w", err)).Error())
		return
	}
	// A site that crashes as its new kind (a Node site with no index.js, say) would otherwise sit in a restart loop,
	// offline. Give it a few seconds and, if it can't stay up, put the old kind back and say why.
	if wasRunning {
		if ok, why := h.siteStaysUp(ctx, newID); !ok {
			_ = h.docker.RemoveContainer(ctx, newID)
			rerr := restoreOld(fmt.Errorf("the site crashed as %s and was not switched. %s", newType, why))
			h.audit(r, "site.template", site.Domain, map[string]any{"from": oldType, "to": newType, "ok": false, "reason": why})
			writeError(w, http.StatusConflict, rerr.Error())
			return
		}
	}
	_ = h.store.UpdateSiteContainerID(site.ID, &newID)
	h.repointNginx(ctx, site.Domain, newID, newPort, includeWww)
	_ = WriteDKLManifest(sitePath, site.Domain, newType, username, newPort, includeWww, nil)
	if !wasRunning {
		_ = h.docker.StopContainer(ctx, newID)
		_ = h.store.UpdateSiteStatus(site.ID, "stopped")
	} else {
		_ = h.store.UpdateSiteStatus(site.ID, "running")
	}
	h.audit(r, "site.template", site.Domain, map[string]any{"from": oldType, "to": newType, "ok": true})

	resp := map[string]any{"success": true, "domain": site.Domain, "template_type": newType, "port": newPort, "container_id": newID}
	if len(added) > 0 {
		resp["added_files"] = added
		resp["note"] = "Added " + strings.Join(added, " and ") + " to the site's folder: a small server that serves your existing files. Replace it with your own whenever you like."
	}
	writeJSON(w, http.StatusOK, resp)
}

// entryFiles are the files a site of each kind can start from (any one is enough).
var entryFiles = map[string][]string{
	"node":   {"index.js"},
	"php":    {"index.php", "index.html"},
	"static": {"index.html", "index.htm"},
}

func entryFileFor(templateType string) string {
	if f := entryFiles[templateType]; len(f) > 0 {
		return f[0]
	}
	return ""
}

func hasAnyFile(dir string, names ...string) bool {
	for _, n := range names {
		if st, err := os.Stat(filepath.Join(dir, n)); err == nil && st.Mode().IsRegular() {
			return true
		}
	}
	return false
}

// missingEntryFile says what a site of this kind needs that its folder doesn't have, or "".
func missingEntryFile(sitePath, templateType string) string {
	files := entryFiles[templateType]
	if len(files) == 0 || hasAnyFile(sitePath, files...) {
		return ""
	}
	return fmt.Sprintf("A %s site needs %s in its folder, and this one has none.", map[string]string{"node": "Node.js", "php": "PHP", "static": "static"}[templateType], strings.Join(files, " or "))
}

// siteStaysUp watches a freshly started site container for a few seconds. It is "up" once it has run for three seconds
// without restarting; it has failed the moment it is restarting or has exited, and the reason is what its log ended with.
func (h *Handlers) siteStaysUp(ctx context.Context, id string) (bool, string) {
	start := time.Now()
	for {
		info, err := h.docker.InspectContainer(ctx, id)
		if err != nil {
			return false, "Couldn't look at the new container: " + err.Error()
		}
		running, restarting, restarts, status := false, false, 0, ""
		if info.State != nil {
			running, restarting, status = info.State.Running, info.State.Restarting, info.State.Status
		}
		restarts = info.RestartCount
		switch judgeSiteState(running, restarting, restarts, status, time.Since(start)) {
		case "up":
			return true, ""
		case "failed":
			logs, _ := h.docker.ContainerLogs(ctx, id, "20")
			return false, crashReason(logs)
		}
		select {
		case <-ctx.Done():
			return false, "Timed out waiting for the site to start."
		case <-time.After(400 * time.Millisecond):
		}
	}
}

// judgeSiteState: "up", "failed" or "wait".
func judgeSiteState(running, restarting bool, restarts int, status string, elapsed time.Duration) string {
	if restarting || restarts > 0 || status == "exited" || status == "dead" {
		return "failed"
	}
	if running && elapsed >= 3*time.Second {
		return "up"
	}
	if !running && elapsed >= 3*time.Second {
		return "failed"
	}
	return "wait"
}

// crashReason is the most telling line at the end of a container's log, in a sentence.
func crashReason(logs string) string {
	var last, errLine string
	for _, l := range strings.Split(strings.ReplaceAll(logs, "\r", ""), "\n") {
		l = strings.TrimSpace(strings.TrimLeft(l, "\x00\x01\x02\x03\x04\x05\x06\x07\x08"))
		if l == "" {
			continue
		}
		last = l
		if strings.Contains(l, "Error") || strings.Contains(l, "error") {
			errLine = l
		}
	}
	if errLine == "" {
		errLine = last
	}
	if errLine == "" {
		return "It stopped as soon as it started."
	}
	if len(errLine) > 200 {
		errLine = errLine[:200] + "..."
	}
	return "Its log says: " + errLine
}

// currentSiteType is the kind a site really runs as: its container's label when that is one of DockLite's kinds,
// otherwise the saved type.
func currentSiteType(saved, label string) string {
	if label == "static" || label == "php" || label == "node" {
		return label
	}
	return saved
}
