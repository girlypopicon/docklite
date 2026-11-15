package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"docklite-agent/internal/store"
)

const minPasswordLength = 10

func (h *Handlers) Users(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	switch r.Method {
	case http.MethodGet:
		users, err := h.store.ListUsers()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users})
	case http.MethodPost:
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
			IsAdmin  bool   `json:"isAdmin"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Username == "" || body.Password == "" {
			writeError(w, http.StatusBadRequest, "Username and password are required")
			return
		}
		if len(body.Password) < minPasswordLength {
			writeError(w, http.StatusBadRequest, "Password must be at least 10 characters")
			return
		}

		role := "user"
		if body.IsAdmin && isSuperAdminRole(r) {
			role = "admin"
		}

		managedBy := currentUserID(r)
		user, err := h.store.CreateUser(body.Username, body.Password, role, managedBy)
		if err != nil {
			if isUniqueConstraint(err) {
				writeError(w, http.StatusBadRequest, "Username already exists")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		h.audit(r, "user.create", body.Username, map[string]any{"role": role})
		if err := ensureUserFolder(body.Username); err != nil {
			// User was created but their directory couldn't be made.
			// Return a warning alongside the created user rather than failing.
			writeJSON(w, http.StatusCreated, map[string]any{
				"user": map[string]any{
					"id":       user.ID,
					"username": user.Username,
					"isAdmin":  user.IsAdmin == 1,
				},
				"warning": "user created but home directory could not be created: " + err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"user": map[string]any{
				"id":       user.ID,
				"username": user.Username,
				"isAdmin":  user.IsAdmin == 1,
			},
		})
	case http.MethodDelete:
		targetID, err := readInt64Query(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if targetID <= 0 {
			writeError(w, http.StatusBadRequest, "User ID is required")
			return
		}
		currentID := currentUserID(r)
		if currentID != nil && *currentID == targetID {
			writeError(w, http.StatusBadRequest, "You cannot delete your own account")
			return
		}
		targetUser, err := h.store.GetUserByIDFull(targetID)
		if err != nil || targetUser == nil {
			writeError(w, http.StatusNotFound, "User not found")
			return
		}
		if !canManageUser(r, targetUser) {
			writeError(w, http.StatusForbidden, "Forbidden")
			return
		}

		transferTo := int64(0)
		if targetUser.ManagedBy.Valid {
			transferTo = targetUser.ManagedBy.Int64
		} else if currentID != nil {
			transferTo = *currentID
		}
		if transferTo == 0 {
			writeError(w, http.StatusBadRequest, "Invalid transfer user")
			return
		}
		newOwner, _ := h.store.GetUserByIDFull(transferTo)
		if newOwner == nil {
			writeError(w, http.StatusBadRequest, "Invalid transfer user")
			return
		}
		// The sites go to the new owner, folders included: each one is copied to
		// /var/www/sites/<new owner>/<domain>, its container switched over, and the
		// old folder trashed. A site that can't be moved keeps its folder and only
		// changes owner in the database (reported as a warning, never a failure).
		var ownedPaths, warnings []string
		if sites, lerr := h.store.ListSites(); lerr == nil {
			for i := range sites {
				st := sites[i]
				if st.UserID != targetID || st.CodePath == "" {
					continue
				}
				to := getSitePath(newOwner.Username, st.Domain)
				if !isWithinBase(st.CodePath) || filepath.Clean(st.CodePath) == filepath.Clean(to) {
					ownedPaths = append(ownedPaths, st.CodePath)
					continue
				}
				if err := h.relocateSite(&st, transferTo, newOwner.Username, st.CodePath, to); err != nil {
					warnings = append(warnings, st.Domain+": "+err.Error()+" (its folder stays at "+st.CodePath+")")
					ownedPaths = append(ownedPaths, st.CodePath)
					continue
				}
				ownedPaths = append(ownedPaths, to)
				if _, err := moveToTrash(st.CodePath); err != nil {
					warnings = append(warnings, st.Domain+": "+err.Error())
				}
			}
		}
		if err := h.store.DeleteUserWithTransfer(targetID, transferTo); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		by := actorName(h, r)
		for _, path := range ownedPaths {
			RecordPreviousOwner(path, targetUser.Username, "user-deleted", by)
		}
		// The deleted user's own folder: gone if it is empty now, trashed if something is still in it.
		if dir := filepath.Join(siteBaseDir, targetUser.Username); isSafeLayoutName(targetUser.Username) && dirExists(dir) {
			if dirHasEntries(dir) {
				if _, err := moveToTrash(dir); err != nil {
					warnings = append(warnings, "the folder "+dir+" could not be tidied away: "+err.Error())
				}
			} else {
				_ = os.Remove(dir)
			}
		}
		h.audit(r, "user.delete", targetUser.Username, map[string]any{"userId": targetID, "sitesMovedTo": transferTo, "warnings": warnings})
		resp := map[string]any{"success": true, "sites_moved_to": newOwner.Username}
		if len(warnings) > 0 {
			resp["warnings"] = warnings
		}
		writeJSON(w, http.StatusOK, resp)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) UserPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		UserID          *int64 `json:"userId"`
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "New password is required")
		return
	}
	if len(body.NewPassword) < minPasswordLength {
		writeError(w, http.StatusBadRequest, "Password must be at least 10 characters")
		return
	}

	if body.UserID != nil {
		targetUser, err := h.store.GetUserByIDFull(*body.UserID)
		if err != nil || targetUser == nil {
			writeError(w, http.StatusNotFound, "User not found")
			return
		}
		if !canManageUser(r, targetUser) {
			writeError(w, http.StatusForbidden, "Unauthorized")
			return
		}
		if err := h.store.UpdateUserPassword(*body.UserID, body.NewPassword); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.audit(r, "user.password-reset", targetUser.Username, map[string]any{"userId": *body.UserID})
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}

	userID := currentUserID(r)
	if userID == nil {
		writeError(w, http.StatusForbidden, "Unauthorized")
		return
	}
	if body.CurrentPassword == "" {
		writeError(w, http.StatusBadRequest, "Current password is required")
		return
	}

	user, err := h.store.GetUserByIDFull(*userID)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}
	if err := h.store.VerifyPassword(user, body.CurrentPassword); err != nil {
		writeError(w, http.StatusBadRequest, "Current password is incorrect")
		return
	}
	if err := h.store.UpdateUserPassword(*userID, body.NewPassword); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func ensureUserFolder(username string) error {
	if username == "" {
		return nil
	}
	path := filepath.Join(siteBaseDir, username)
	if err := mkdirAllShared(path); err != nil {
		return err
	}
	chownDocklite(path)
	shareWithGroup(path)
	return nil
}

func currentUserID(r *http.Request) *int64 {
	if userID, ok := readUserIDFromContext(r); ok {
		return &userID
	}
	return nil
}

func readInt64Query(r *http.Request, key string) (int64, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func canManageUser(r *http.Request, target *store.UserRecord) bool {
	if target == nil {
		return false
	}
	if isSuperAdminRole(r) {
		return true
	}
	if !isAdminRole(r) {
		return false
	}
	if target.Role == "user" {
		return true
	}
	current := currentUserID(r)
	if current == nil {
		return false
	}
	return target.ManagedBy.Valid && target.ManagedBy.Int64 == *current
}

func isSuperAdminRole(r *http.Request) bool {
	role, ok := readUserRoleFromContext(r)
	if !ok {
		return false
	}
	return role == "super_admin"
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE")
}
