package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"docklite-agent/internal/docker"
)

const limitsSettingPrefix = "limits."

// StoredLimits returns the limits saved for a container name (nil = none). The saved copy is what makes a limit
// survive the container being re-created.
func (h *Handlers) StoredLimits(containerName string) *docker.Limits {
	raw, err := h.store.GetSetting(limitsSettingPrefix + containerName)
	if err != nil || raw == "" {
		return nil
	}
	var l docker.Limits
	if json.Unmarshal([]byte(raw), &l) != nil || l.None() {
		return nil
	}
	return &l
}

// handleLimits: GET shows a container's limits and the server's totals; PUT changes them at once (no restart) and
// remembers them. Changing limits is for admins: a limit set too low can take a site down, and one set too high
// defeats the point.
func (h *Handlers) handleLimits(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := dockerContext(r.Context())
	defer cancel()
	if _, err := h.authorizeContainerAccess(ctx, r, id); err != nil {
		if err == errForbidden {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	info, err := h.docker.InspectContainer(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}
	name := strings.TrimPrefix(info.Name, "/")
	host := docker.Host()

	switch r.Method {
	case http.MethodGet:
		cur, err := h.docker.CurrentLimits(ctx, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"limits": cur, "host": host, "canChange": isAdminRole(r)})
	case http.MethodPut, http.MethodPost:
		if !isAdminRole(r) {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		var l docker.Limits
		if err := readJSON(w, r, &l); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		if err := l.Validate(host); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.docker.UpdateLimits(ctx, id, l); err != nil {
			writeError(w, http.StatusInternalServerError, "Docker refused the change: "+err.Error())
			return
		}
		if l.None() {
			_ = h.store.SetSetting(limitsSettingPrefix+name, "")
		} else {
			b, _ := json.Marshal(l)
			_ = h.store.SetSetting(limitsSettingPrefix+name, string(b))
		}
		h.audit(r, "container.limits", name, map[string]any{"cpus": l.CPUs, "memoryMB": l.MemoryMB, "pids": l.Pids})
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "limits": l, "host": host})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
