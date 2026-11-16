package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"docklite-agent/internal/demo"
	"docklite-agent/internal/store"
)

// Groups: the cards on the dashboard that hold boxes. See store/groups.go for what a group is. These endpoints:
//   GET    /api/groups                         the groups this person can see, with their boxes
//   POST   /api/groups                         {name, container_ids?}  make one (optionally with its first boxes)
//   PUT    /api/groups/{id}                    {name?, linked?, color?, icon?}
//   DELETE /api/groups/{id}                    boxes go back to the desktop
//   POST   /api/groups/{id}/members            {container_id}  put a box in (leaving any other group)
//   DELETE /api/groups/{id}/members?container_id=…   take a box out

func (h *Handlers) GroupRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/groups"), "/")
	if path == "" {
		switch r.Method {
		case http.MethodGet:
			h.listGroups(w, r)
		case http.MethodPost:
			h.createGroup(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}
	parts := strings.Split(path, "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	g, ok := h.groupForActor(w, r, id)
	if !ok {
		return
	}
	switch {
	case len(parts) == 1 && r.Method == http.MethodPut:
		h.updateGroup(w, r, g)
	case len(parts) == 1 && r.Method == http.MethodDelete:
		h.deleteGroup(w, r, g)
	case len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodPost:
		h.addGroupMember(w, r, g)
	case len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodDelete:
		h.removeGroupMember(w, r, g)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

// groupForActor loads a group and checks the caller may change it: its owner, or an admin.
func (h *Handlers) groupForActor(w http.ResponseWriter, r *http.Request, id int64) (*store.BoxGroup, bool) {
	g, err := h.store.GetGroup(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if g == nil {
		writeError(w, http.StatusNotFound, "group not found")
		return nil, false
	}
	if !isAdminRole(r) {
		if uid, ok := readUserIDFromContext(r); !ok || uid != g.OwnerID {
			writeError(w, http.StatusForbidden, "forbidden")
			return nil, false
		}
	}
	return g, true
}

type groupView struct {
	store.BoxGroup
	OwnerUsername string `json:"owner_username,omitempty"`
}

func (h *Handlers) groupViews(groups []store.BoxGroup) []groupView {
	names := map[int64]string{}
	out := make([]groupView, 0, len(groups))
	for _, g := range groups {
		if _, ok := names[g.OwnerID]; !ok {
			if u, _ := h.store.GetUserByIDFull(g.OwnerID); u != nil {
				names[g.OwnerID] = u.Username
			} else {
				names[g.OwnerID] = ""
			}
		}
		out = append(out, groupView{BoxGroup: g, OwnerUsername: names[g.OwnerID]})
	}
	return out
}

// visibleGroups is every group for an admin, and the caller's own for anyone else.
func (h *Handlers) visibleGroups(r *http.Request) ([]store.BoxGroup, error) {
	if isAdminRole(r) {
		return h.store.ListGroups(0)
	}
	uid, ok := readUserIDFromContext(r)
	if !ok {
		return nil, errForbidden
	}
	return h.store.ListGroups(uid)
}

func (h *Handlers) listGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.visibleGroups(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": h.groupViews(groups)})
}

// boxName turns a container id into its name after checking the caller may touch that box.
func (h *Handlers) boxName(ctx context.Context, r *http.Request, containerID string) (string, int, error) {
	if _, err := h.authorizeContainerAccess(ctx, r, containerID); err != nil {
		if errors.Is(err, errForbidden) {
			return "", http.StatusForbidden, errors.New("you can't change that box")
		}
		return "", http.StatusInternalServerError, err
	}
	info, err := h.docker.InspectContainer(ctx, containerID)
	if err != nil || info.Name == "" {
		return "", http.StatusNotFound, errors.New("box not found")
	}
	return strings.TrimPrefix(info.Name, "/"), 0, nil
}

// groupNameValid: 1 to 50 characters, any letters (accents and emoji too), digits, spaces and punctuation except slashes.
func groupNameValid(name string) bool {
	n := 0
	for _, r := range name {
		n++
		if r == '/' || r == '\\' || r < 32 || r == 127 {
			return false
		}
	}
	return n >= 1 && n <= 50
}

func validGroupColorIcon(color, icon string) error {
	if color != "" && !boxColors[color] {
		return errors.New("unknown colour")
	}
	if icon != "" && !boxIcons[icon] {
		return errors.New("unknown icon")
	}
	return nil
}

func (h *Handlers) createGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string   `json:"name"`
		ContainerIDs []string `json:"container_ids"`
		Linked       bool     `json:"linked"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name := strings.TrimSpace(body.Name)
	if !groupNameValid(name) {
		writeError(w, http.StatusBadRequest, "give the group a name (up to 50 characters, no slashes)")
		return
	}
	uid, ok := readUserIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "sign in first")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()
	var members []string
	for _, id := range body.ContainerIDs {
		n, status, err := h.boxName(ctx, r, id)
		if err != nil {
			writeError(w, status, err.Error())
			return
		}
		members = append(members, n)
	}
	g, err := h.store.CreateGroup(uid, name)
	if err != nil {
		if errors.Is(err, store.ErrGroupNameTaken) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, m := range members {
		if err := h.store.AddToGroup(g.ID, m); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if body.Linked {
		t := true
		_ = h.store.UpdateGroup(g.ID, nil, &t, nil, nil)
	}
	g, _ = h.store.GetGroup(g.ID)
	h.audit(r, "group.create", name, map[string]any{"members": len(members)})
	writeJSON(w, http.StatusOK, map[string]any{"group": h.groupViews([]store.BoxGroup{*g})[0]})
}

func (h *Handlers) updateGroup(w http.ResponseWriter, r *http.Request, g *store.BoxGroup) {
	var body struct {
		Name   *string `json:"name"`
		Linked *bool   `json:"linked"`
		Color  *string `json:"color"`
		Icon   *string `json:"icon"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name != nil && !groupNameValid(strings.TrimSpace(*body.Name)) {
		writeError(w, http.StatusBadRequest, "give the group a name (up to 50 characters, no slashes)")
		return
	}
	color, icon := "", ""
	if body.Color != nil {
		color = strings.ToLower(strings.TrimSpace(*body.Color))
		body.Color = &color
	}
	if body.Icon != nil {
		icon = strings.ToLower(strings.TrimSpace(*body.Icon))
		body.Icon = &icon
	}
	if err := validGroupColorIcon(color, icon); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.UpdateGroup(g.ID, body.Name, body.Linked, body.Color, body.Icon); err != nil {
		if errors.Is(err, store.ErrGroupNameTaken) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "group.update", g.Name, nil)
	updated, _ := h.store.GetGroup(g.ID)
	writeJSON(w, http.StatusOK, map[string]any{"group": h.groupViews([]store.BoxGroup{*updated})[0]})
}

func (h *Handlers) deleteGroup(w http.ResponseWriter, r *http.Request, g *store.BoxGroup) {
	if err := h.store.DeleteGroup(g.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "group.delete", g.Name, map[string]any{"members": len(g.Members)})
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handlers) addGroupMember(w http.ResponseWriter, r *http.Request, g *store.BoxGroup) {
	var body struct {
		ContainerID string `json:"container_id"`
	}
	if err := readJSON(w, r, &body); err != nil || body.ContainerID == "" {
		writeError(w, http.StatusBadRequest, "container_id is required")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()
	name, status, err := h.boxName(ctx, r, body.ContainerID)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	if err := h.store.AddToGroup(g.ID, name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	updated, _ := h.store.GetGroup(g.ID)
	writeJSON(w, http.StatusOK, map[string]any{"group": h.groupViews([]store.BoxGroup{*updated})[0]})
}

func (h *Handlers) removeGroupMember(w http.ResponseWriter, r *http.Request, g *store.BoxGroup) {
	id := r.URL.Query().Get("container_id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "container_id is required")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()
	name, status, err := h.boxName(ctx, r, id)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	in := false
	for _, m := range g.Members {
		if m == name {
			in = true
		}
	}
	if !in {
		writeError(w, http.StatusNotFound, "that box isn't in this group")
		return
	}
	if err := h.store.RemoveFromGroup(name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	updated, _ := h.store.GetGroup(g.ID)
	writeJSON(w, http.StatusOK, map[string]any{"group": h.groupViews([]store.BoxGroup{*updated})[0]})
}

// ---- folders → groups, once ---------------------------------------------------------------------------------

const groupsMigratedSetting = "groups_migrated"

// migrateFoldersToGroups turns each person's old folders into groups: the "Default" folder is dropped (what was in it is
// now simply on the desktop), nested folders become top-level groups, and boxes are followed by name. idToName maps the
// container ids the old folders hold to container names; boxes that no longer exist are skipped.
func migrateFoldersToGroups(s *store.SQLiteStore, idToName map[string]string) (int, error) {
	users, err := s.ListUsers()
	if err != nil {
		return 0, err
	}
	made := 0
	for _, u := range users {
		folders, err := s.GetFoldersByUser(u.ID)
		if err != nil {
			return made, err
		}
		for _, f := range folders {
			if f.Name == "Default" && f.ParentFolderID == nil {
				continue
			}
			ids, err := s.GetContainersByFolder(f.ID)
			if err != nil {
				return made, err
			}
			name := f.Name
			var g *store.BoxGroup
			for try := 1; try <= 20; try++ {
				candidate := name
				if try > 1 {
					candidate = fmt.Sprintf("%s (%d)", name, try)
				}
				g, err = s.CreateGroup(u.ID, candidate)
				if err == nil {
					break
				}
				if !errors.Is(err, store.ErrGroupNameTaken) {
					return made, err
				}
			}
			if g == nil {
				continue
			}
			made++
			for _, id := range ids {
				if n, ok := idToName[id]; ok {
					if err := s.AddToGroup(g.ID, n); err != nil {
						return made, err
					}
				}
			}
		}
	}
	return made, nil
}

// MigrateFoldersToGroups runs the folder conversion once, after Docker answers (so ids can be turned into names).
func (h *Handlers) MigrateFoldersToGroups(ctx context.Context) {
	if demo.On || h.docker == nil {
		return
	}
	if done, _ := h.store.GetSetting(groupsMigratedSetting); done != "" {
		return
	}
	containers, err := h.docker.ListContainers(ctx, true)
	if err != nil {
		return // try again on the next start
	}
	idToName := map[string]string{}
	for _, c := range containers {
		idToName[c.ID] = strings.TrimPrefix(c.Name, "/")
	}
	n, err := migrateFoldersToGroups(h.store, idToName)
	if err != nil {
		log.Printf("groups: could not convert folders: %v", err)
		return
	}
	_ = h.store.SetSetting(groupsMigratedSetting, "1")
	if n > 0 {
		log.Printf("groups: turned %d folder(s) into groups", n)
	}
}
