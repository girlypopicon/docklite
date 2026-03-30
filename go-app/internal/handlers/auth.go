package handlers

import (
	"net/http"
	"time"
)

func (h *Handlers) AuthMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := readUserIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	user, err := h.store.GetUserByIDFull(userID)
	if err != nil || user == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"userId":   user.ID,
			"username": user.Username,
			"isAdmin":  user.IsAdmin == 1,
			"role":     user.Role,
		},
	})
}

func (h *Handlers) AuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	clearCookie(w, r, "docklite_session")
	clearCookie(w, r, delegationCookieName)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func clearCookie(w http.ResponseWriter, r *http.Request, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   shouldUseSecureCookies(r),
	})
}
