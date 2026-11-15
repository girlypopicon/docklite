package handlers

import (
	"net/http"
	"strings"

	"docklite-agent/internal/store"
)

// Card colours and icons a box can have. The dashboard (webapp/lib/box-styles.ts) draws them; here they are only an
// allow-list, so whatever is stored is always one of these and nothing else can end up in the page.
var boxColors = map[string]bool{
	"pink": true, "coral": true, "orange": true, "gold": true, "lime": true, "mint": true,
	"teal": true, "sky": true, "blue": true, "lilac": true, "rose": true, "slate": true,
}

var boxIcons = map[string]bool{
	"globe": true, "heart": true, "sparkle": true, "rocket": true, "lightning": true, "planet": true, "cloud": true,
	"flower": true, "moon": true, "star": true, "gamepad": true, "cat": true, "camera": true, "storefront": true, "music": true,
}

// SetContainerStyle is PUT /api/containers/{id}/style {"color": "pink", "icon": "heart"}. Empty strings go back to the
// defaults (a colour by type, the box icon). Anyone who can reach the box may change how its card looks.
func (h *Handlers) SetContainerStyle(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Color string `json:"color"`
		Icon  string `json:"icon"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	color, icon := strings.ToLower(strings.TrimSpace(body.Color)), strings.ToLower(strings.TrimSpace(body.Icon))
	if color != "" && !boxColors[color] {
		writeError(w, http.StatusBadRequest, "unknown colour")
		return
	}
	if icon != "" && !boxIcons[icon] {
		writeError(w, http.StatusBadRequest, "unknown icon")
		return
	}
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
	if err != nil || info.Name == "" {
		writeError(w, http.StatusNotFound, "box not found")
		return
	}
	name := strings.TrimPrefix(info.Name, "/")
	if err := h.store.SetBoxStyle(name, store.BoxStyle{Color: color, Icon: icon}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "style": map[string]string{"color": color, "icon": icon}})
}
