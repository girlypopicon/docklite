package handlers

import (
	"encoding/json"
	"net/http"
)

// The dashboard's look for the signed-in user. The browser keeps a copy so the page paints in the right theme at
// once; this is the copy that lasts (other browsers, cleared storage, updates).

type userAppearance struct {
	Theme      string `json:"theme"`
	FontSize   string `json:"fontSize"`
	Animations bool   `json:"animations"`
	Glow       int    `json:"glow"`
}

var (
	appearanceThemes    = map[string]bool{"cyberpunk": true, "corpo": true, "corpo-blue": true, "unicorn": true}
	appearanceFontSizes = map[string]bool{"small": true, "medium": true, "large": true, "xlarge": true}
)

func (h *Handlers) AuthAppearance(w http.ResponseWriter, r *http.Request) {
	userID, ok := readUserIDFromContext(r)
	if !ok || userID <= 0 {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	switch r.Method {
	case http.MethodGet:
		saved, err := h.store.GetUserAppearance(userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not read appearance")
			return
		}
		var appearance *userAppearance
		if saved != "" {
			var a userAppearance
			if json.Unmarshal([]byte(saved), &a) == nil {
				appearance = &a
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"appearance": appearance})
	case http.MethodPut:
		var a userAppearance
		if err := readJSON(w, r, &a); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if !appearanceThemes[a.Theme] || !appearanceFontSizes[a.FontSize] || a.Glow < 0 || a.Glow > 200 {
			writeError(w, http.StatusBadRequest, "unknown theme, text size or glow")
			return
		}
		data, _ := json.Marshal(a)
		if err := h.store.SetUserAppearance(userID, string(data)); err != nil {
			writeError(w, http.StatusInternalServerError, "could not save appearance")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"appearance": a})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
