package handlers

import (
	"net/http"
	"regexp"
	"strings"
)

var osUsernameRegex = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

// ShellAccess manages which server (OS) users may run `docklite` on the
// server with full admin rights and no login, by membership of the docklite
// group. That group can read the server's master token, so it is effectively
// DockLite admin — hence super admin only, validated twice (here and in the
// root helper), and written to the audit log.
//
//	GET  /api/system/shell-access                       -> {"members": [...]}
//	POST /api/system/shell-access {"username","action"} -> action is grant|revoke
func (h *Handlers) ShellAccess(w http.ResponseWriter, r *http.Request) {
	if !isSuperAdminRole(r) {
		writeError(w, http.StatusForbidden, "only a super admin can manage shell access")
		return
	}
	switch r.Method {
	case http.MethodGet:
		output, err := runRootHelper(nil, "group-list")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not read the docklite group: "+strings.TrimSpace(string(output)))
			return
		}
		members := []string{}
		for _, line := range strings.Split(string(output), "\n") {
			if name := strings.TrimSpace(line); name != "" {
				members = append(members, name)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"members": members})

	case http.MethodPost:
		var body struct {
			Username string `json:"username"`
			Action   string `json:"action"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		body.Username = strings.TrimSpace(body.Username)
		if !osUsernameRegex.MatchString(body.Username) {
			writeError(w, http.StatusBadRequest, "that isn't a valid server username (lowercase letters, digits, - and _)")
			return
		}
		var helperCommand string
		switch body.Action {
		case "grant":
			helperCommand = "group-add"
		case "revoke":
			helperCommand = "group-remove"
		default:
			writeError(w, http.StatusBadRequest, "action must be grant or revoke")
			return
		}
		output, err := runRootHelper(nil, helperCommand, body.Username)
		// Record the attempt whether or not it worked.
		h.audit(r, "shell-access."+body.Action, body.Username, map[string]any{"ok": err == nil})
		if err != nil {
			// The helper's message is written for people ("no such user on
			// this server: ..."), so pass it on.
			message := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(output)), "docklite-helper:"))
			if message == "" {
				message = "the change failed"
			}
			writeError(w, http.StatusBadRequest, message)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "username": body.Username, "action": body.Action})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
