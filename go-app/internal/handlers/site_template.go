package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	oldPort := 80
	if p, err := strconv.Atoi(old.Config.Labels["docklite.internal_port"]); err == nil && p > 0 {
		oldPort = p
	}
	newPort := 80
	if newType == "node" {
		newPort = 3000
		if site.TemplateType == "node" && oldPort > 0 {
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
	if newType == site.TemplateType && newPort == oldPort {
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
	oldType := site.TemplateType

	restoreOld := func(reason error) error {
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
	if w := missingEntryFile(sitePath, newType); w != "" {
		resp["warning"] = w
	}
	writeJSON(w, http.StatusOK, resp)
}

// missingEntryFile says what a site of this kind needs that its folder doesn't have, or "".
func missingEntryFile(sitePath, templateType string) string {
	need := map[string]string{"node": "index.js", "php": "index.php", "static": "index.html"}[templateType]
	if need == "" {
		return ""
	}
	if _, err := os.Stat(filepath.Join(sitePath, need)); err == nil {
		return ""
	}
	return fmt.Sprintf("The site now runs as %s, but its folder has no %s, so visitors will see an error until one is added.", templateType, need)
}
