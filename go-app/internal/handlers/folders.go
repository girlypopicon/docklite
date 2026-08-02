package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"docklite-agent/internal/store"
)

type createFolderRequest struct {
	Name           string `json:"name"`
	ParentFolderID *int64 `json:"parentFolderId"`
	UserID         *int64 `json:"userId"`
}

type moveFolderRequest struct {
	NewParentID *int64 `json:"newParentId"`
}

type assignContainerRequest struct {
	UserID int64 `json:"user_id"`
}

func (h *Handlers) Folders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listFolders(w, r)
	case http.MethodPost:
		h.createFolder(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) FolderRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/folders/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	folderID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || folderID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid folder id")
		return
	}

	if len(parts) == 1 || parts[1] == "" {
		h.Folder(w, r, folderID)
		return
	}

	switch parts[1] {
	case "move":
		h.MoveFolder(w, r, folderID)
	case "containers":
		if len(parts) > 2 && parts[2] == "reorder" {
			h.ReorderFolderContainers(w, r, folderID)
			return
		}
		h.FolderContainers(w, r, folderID)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *Handlers) Folder(w http.ResponseWriter, r *http.Request, folderID int64) {
	switch r.Method {
	case http.MethodGet:
		h.getFolder(w, r, folderID)
	case http.MethodDelete:
		h.deleteFolder(w, r, folderID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) MoveFolder(w http.ResponseWriter, r *http.Request, folderID int64) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req moveFolderRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	folder, err := h.store.GetFolderByID(folderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if folder == nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	if folder.Name == "Default" {
		writeError(w, http.StatusBadRequest, "cannot move the Default folder")
		return
	}

	userID, _ := readUserID(r)
	if userID != nil && folder.UserID != *userID && !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if req.NewParentID != nil {
		parent, err := h.store.GetFolderByID(*req.NewParentID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if parent == nil {
			writeError(w, http.StatusBadRequest, "invalid parent folder")
			return
		}
		if userID != nil && parent.UserID != *userID && !isAdminRole(r) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	allFolders, err := h.store.GetFoldersByUser(folder.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := validateFolderMove(folderID, req.NewParentID, allFolders); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.store.MoveFolderToParent(folderID, req.NewParentID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) FolderContainers(w http.ResponseWriter, r *http.Request, folderID int64) {
	switch r.Method {
	case http.MethodGet:
		h.listFolderContainers(w, r, folderID)
	case http.MethodPost:
		h.addContainerToFolder(w, r, folderID)
	case http.MethodDelete:
		h.removeContainerFromFolder(w, r, folderID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) ReorderFolderContainers(w http.ResponseWriter, r *http.Request, folderID int64) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ContainerID string `json:"containerId"`
		NewPosition int    `json:"newPosition"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.ContainerID == "" {
		writeError(w, http.StatusBadRequest, "containerId and newPosition are required")
		return
	}
	userID, _ := readUserID(r)
	if userID != nil {
		folder, err := h.store.GetFolderByID(folderID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if folder == nil {
			writeError(w, http.StatusNotFound, "folder not found")
			return
		}
		if folder.UserID != *userID && !isAdminRole(r) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}
	if err := h.store.ReorderContainerInFolder(folderID, body.ContainerID, body.NewPosition); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) AssignContainer(w http.ResponseWriter, r *http.Request, containerID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	var req assignContainerRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID <= 0 {
		writeError(w, http.StatusBadRequest, "target user is required")
		return
	}

	site, err := h.store.GetSiteByContainerIDRecord(containerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if site == nil {
		writeError(w, http.StatusNotFound, "container is not a managed site")
		return
	}

	// If the owner is actually changing, do a full transfer
	// (moves files, recreates container, updates nginx).
	if site.UserID != req.UserID {
		h.transferSiteToUser(w, r, containerID, req.UserID)
		return
	}

	// Same user — just move to their default folder.
	folder, err := h.store.GetDefaultFolderByUser(req.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if folder == nil {
		folder, err = h.store.CreateFolder(req.UserID, "Default", nil)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := h.store.MoveContainerToFolder(containerID, folder.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) TrackContainer(w http.ResponseWriter, r *http.Request, containerID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if containerID == "" {
		writeError(w, http.StatusBadRequest, "container id is required")
		return
	}
	if err := h.store.MarkContainerTracked(containerID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) UntrackContainer(w http.ResponseWriter, r *http.Request, containerID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if containerID == "" {
		writeError(w, http.StatusBadRequest, "container id is required")
		return
	}
	if err := h.store.MarkContainerUntracked(containerID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) listFolders(w http.ResponseWriter, r *http.Request) {
	userID, err := readUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if userID == nil {
		writeError(w, http.StatusBadRequest, "userId is required")
		return
	}
	folders, err := h.store.GetFoldersByUser(*userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"folders": folders})
}

func (h *Handlers) createFolder(w http.ResponseWriter, r *http.Request) {
	var req createFolderRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "folder name is required")
		return
	}
	if !folderNameValid(name) {
		writeError(w, http.StatusBadRequest, "folder name can only contain letters, numbers, spaces, dashes, and underscores")
		return
	}

	userID := req.UserID
	if userID == nil {
		if headerUserID, _ := readUserID(r); headerUserID != nil {
			userID = headerUserID
		}
	}
	if userID == nil {
		writeError(w, http.StatusBadRequest, "userId is required")
		return
	}

	folder, err := h.store.CreateFolder(*userID, name, req.ParentFolderID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, http.StatusConflict, "a folder with this name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"folder": folder})
}

func (h *Handlers) getFolder(w http.ResponseWriter, r *http.Request, folderID int64) {
	folder, err := h.store.GetFolderByID(folderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if folder == nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	userID, _ := readUserID(r)
	if userID != nil && folder.UserID != *userID && !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"folder": folder})
}

func (h *Handlers) deleteFolder(w http.ResponseWriter, r *http.Request, folderID int64) {
	folder, err := h.store.GetFolderByID(folderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if folder == nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	if folder.Name == "Default" {
		writeError(w, http.StatusBadRequest, "cannot delete the Default folder")
		return
	}
	userID, _ := readUserID(r)
	if userID != nil && folder.UserID != *userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := h.store.DeleteFolder(folderID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) listFolderContainers(w http.ResponseWriter, r *http.Request, folderID int64) {
	folder, err := h.store.GetFolderByID(folderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if folder == nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	userID, _ := readUserID(r)
	if userID != nil && folder.UserID != *userID && !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	containers, err := h.store.GetContainersByFolder(folderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"containerIds": containers})
}

func (h *Handlers) addContainerToFolder(w http.ResponseWriter, r *http.Request, folderID int64) {
	var body struct {
		ContainerID string `json:"containerId"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.ContainerID == "" {
		writeError(w, http.StatusBadRequest, "container id is required")
		return
	}
	folder, err := h.store.GetFolderByID(folderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if folder == nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	userID, _ := readUserID(r)
	if userID != nil && folder.UserID != *userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := h.store.MoveContainerToFolder(body.ContainerID, folderID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) removeContainerFromFolder(w http.ResponseWriter, r *http.Request, folderID int64) {
	containerID := r.URL.Query().Get("containerId")
	if containerID == "" {
		writeError(w, http.StatusBadRequest, "container id is required")
		return
	}
	folder, err := h.store.GetFolderByID(folderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if folder == nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	userID, _ := readUserID(r)
	if userID != nil && folder.UserID != *userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := h.store.UnlinkContainerFromFolder(folderID, containerID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func folderNameValid(name string) bool {
	for _, r := range name {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case ' ', '-', '_':
			continue
		default:
			return false
		}
	}
	return true
}

func readUserID(r *http.Request) (*int64, error) {
	if userID, ok := readUserIDFromContext(r); ok {
		return &userID, nil
	}
	if userIDParam := r.URL.Query().Get("userId"); userIDParam != "" {
		value, err := strconv.ParseInt(userIDParam, 10, 64)
		if err != nil {
			return nil, errors.New("invalid user id")
		}
		return &value, nil
	}
	return nil, nil
}

func isAdminRole(r *http.Request) bool {
	role, ok := readUserRoleFromContext(r)
	if !ok {
		return false
	}
	return role == "admin" || role == "super_admin"
}

func validateFolderMove(folderID int64, newParentID *int64, folders []store.Folder) error {
	if newParentID != nil && folderID == *newParentID {
		return errors.New("cannot nest a folder into itself")
	}
	folderMap := make(map[int64]storeFolder, len(folders))
	for _, folder := range folders {
		folderMap[folder.ID] = storeFolder{
			ID:             folder.ID,
			ParentFolderID: folder.ParentFolderID,
			Depth:          folder.Depth,
		}
	}

	if newParentID != nil && isDescendant(*newParentID, folderID, folderMap) {
		return errors.New("cannot create circular folder reference")
	}

	newDepth := calculateDepth(newParentID, folderMap)
	if newDepth > maxFolderDepth {
		return errors.New("maximum nesting depth is 2 layers")
	}

	folder, ok := folderMap[folderID]
	if ok && folder.Depth+newDepth > maxFolderDepth {
		return errors.New("moving this folder would exceed maximum nesting depth for its children")
	}

	return nil
}

const maxFolderDepth = 1

type storeFolder struct {
	ID             int64
	ParentFolderID *int64
	Depth          int
}

func calculateDepth(parentID *int64, folderMap map[int64]storeFolder) int {
	if parentID == nil {
		return 0
	}
	parent, ok := folderMap[*parentID]
	if !ok {
		return 0
	}
	return parent.Depth + 1
}

func isDescendant(folderID int64, potentialAncestorID int64, folderMap map[int64]storeFolder) bool {
	currentID := folderID
	for {
		if currentID == potentialAncestorID {
			return true
		}
		folder, ok := folderMap[currentID]
		if !ok || folder.ParentFolderID == nil {
			return false
		}
		currentID = *folder.ParentFolderID
	}
}

// WriteContainerManifest writes (or overwrites) the .dkl manifest for a
// container that already has a site record in the database.
func (h *Handlers) WriteContainerManifest(w http.ResponseWriter, r *http.Request, containerID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	site, err := h.store.GetSiteByContainerIDRecord(containerID)
	if err != nil || site == nil {
		writeError(w, http.StatusNotFound, "no site record for this container")
		return
	}
	if site.CodePath == "" {
		writeError(w, http.StatusBadRequest, "site has no code path — cannot write manifest")
		return
	}

	username := ""
	if u, err := h.store.GetUserByIDFull(site.UserID); err == nil && u != nil {
		username = u.Username
	}

	if err := WriteDKLManifest(site.CodePath, site.Domain, site.TemplateType, username, 0, false, nil); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write manifest: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": site.CodePath + "/.dkl"})
}

// ClaimContainer registers an existing Docker container as a DockLite-managed
// site, creates a site record, writes a .dkl manifest, and marks it tracked.
func (h *Handlers) ClaimContainer(w http.ResponseWriter, r *http.Request, containerID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Domain       string `json:"domain"`
		TemplateType string `json:"templateType"`
		CodePath     string `json:"codePath"`
	}
	if err := readJSON(w, r, &body); err != nil {
		return
	}
	if body.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}
	if body.TemplateType == "" {
		body.TemplateType = "static"
	}

	// Resolve the user from request context.
	var userID int64 = 1
	if uid, err := readUserID(r); err == nil && uid != nil {
		userID = *uid
	}
	username := ""
	if u, err := h.store.GetUserByIDFull(userID); err == nil && u != nil {
		username = u.Username
	}

	// Default code path to /var/www/sites/{username}/{domain}
	if body.CodePath == "" {
		body.CodePath = siteBaseDir + "/" + username + "/" + body.Domain
	}

	// Create site record pointing at the existing container.
	site, err := h.store.CreateSite(store.SiteRecord{
		Domain:       body.Domain,
		UserID:       userID,
		TemplateType: body.TemplateType,
		CodePath:     body.CodePath,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create site record: "+err.Error())
		return
	}
	_ = h.store.UpdateSiteContainerID(site.ID, &containerID)
	_ = h.store.UpdateSiteStatus(site.ID, "running")
	_ = h.store.MarkContainerTracked(containerID)

	// Write .dkl manifest so future reinstalls can rediscover this site.
	_ = WriteDKLManifest(body.CodePath, body.Domain, body.TemplateType, username, 0, false, nil)

	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":           true,
		"siteId":       site.ID,
		"manifestPath": body.CodePath + "/.dkl",
	})
}
