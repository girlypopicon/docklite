package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"docklite-agent/internal/models"
	"docklite-agent/internal/store"

	"github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
)

const siteBaseDir = "/var/www/sites"

var dockliteUID, dockliteGID int

func init() {
	u, err := user.Lookup("docklite")
	if err != nil {
		dockliteUID = os.Getuid()
		dockliteGID = os.Getgid()
		return
	}
	dockliteUID, _ = strconv.Atoi(u.Uid)
	dockliteGID, _ = strconv.Atoi(u.Gid)
}

func chownDocklite(path string) {
	_ = os.Chown(path, dockliteUID, dockliteGID)
}

func (h *Handlers) getContainerHostPort(ctx context.Context, containerID string, containerPort int) (int, error) {
	portKey := nat.Port(fmt.Sprintf("%d/tcp", containerPort))
	// Docker may not have fully populated port bindings immediately after
	// ContainerStart, so retry a few times before giving up.
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}
		container, err := h.docker.InspectContainer(ctx, containerID)
		if err != nil {
			lastErr = err
			continue
		}
		if container.NetworkSettings == nil || container.NetworkSettings.Ports == nil {
			lastErr = fmt.Errorf("no port bindings found")
			continue
		}
		bindings := container.NetworkSettings.Ports[portKey]
		if len(bindings) == 0 || bindings[0].HostPort == "" {
			lastErr = fmt.Errorf("no host port binding for %s", portKey)
			continue
		}
		port, err := strconv.Atoi(bindings[0].HostPort)
		if err != nil {
			return 0, fmt.Errorf("invalid host port: %s", bindings[0].HostPort)
		}
		return port, nil
	}
	return 0, lastErr
}

type createContainerRequest struct {
	Domain       string `json:"domain"`
	TemplateType string `json:"template_type"`
	IncludeWww   *bool  `json:"include_www"`
	Port         *int   `json:"port"`
	UserID       *int64 `json:"user_id"`
	CodePath     string `json:"code_path"`
	FolderID     *int64 `json:"folder_id"`
}

func (h *Handlers) ListContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.createContainer(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userID, err := readUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if userID == nil {
		writeError(w, http.StatusBadRequest, "userId is required")
		return
	}
	isAdmin := isAdminRole(r)

	folders, err := h.store.GetFoldersByUser(*userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(folders) == 0 {
		folder, err := h.store.CreateFolder(*userID, "Default", nil)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		folders = []store.Folder{*folder}
	}

	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	containers, err := h.docker.ListContainers(ctx, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	untracked, err := h.store.GetUntrackedContainerIDs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	untrackedSet := make(map[string]struct{}, len(untracked))
	for _, id := range untracked {
		untrackedSet[id] = struct{}{}
	}

	containersWithOwners := make([]models.ContainerInfo, 0, len(containers))
	userCache := make(map[int64]string)

	for _, container := range containers {
		if container.Labels == nil || container.Labels["docklite.managed"] != "true" {
			continue
		}
		if _, ok := untrackedSet[container.ID]; ok {
			continue
		}
		ownerID, ownerName, err := h.resolveContainerOwner(container, userCache)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !isAdmin {
			if ownerID == 0 || ownerID != *userID {
				continue
			}
		}
		container.OwnerUsername = ownerName
		containersWithOwners = append(containersWithOwners, container)
	}

	containersByFolderID := make(map[int64][]models.ContainerInfo, len(folders))
	assignedIDs := make(map[string]struct{})

	for _, folder := range folders {
		containerIDs, err := h.store.GetContainersByFolder(folder.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		idSet := make(map[string]struct{}, len(containerIDs))
		for _, id := range containerIDs {
			idSet[id] = struct{}{}
		}
		var folderContainers []models.ContainerInfo
		for _, container := range containersWithOwners {
			if _, ok := idSet[container.ID]; ok {
				folderContainers = append(folderContainers, container)
				assignedIDs[container.ID] = struct{}{}
			}
		}
		containersByFolderID[folder.ID] = folderContainers
	}

	unassignedContainers := make([]models.ContainerInfo, 0)
	for _, container := range containersWithOwners {
		if _, ok := assignedIDs[container.ID]; !ok {
			unassignedContainers = append(unassignedContainers, container)
		}
	}

	for i, folder := range folders {
		if folder.Name == "Default" && len(unassignedContainers) > 0 {
			containersByFolderID[folder.ID] = append(containersByFolderID[folder.ID], unassignedContainers...)
			folders[i] = folder
			break
		}
	}

	folderTree := buildFolderTree(folders, containersByFolderID)

	writeJSON(w, http.StatusOK, map[string]any{
		"folders":         folderTree,
		"totalContainers": len(containersWithOwners),
	})
}

func (h *Handlers) ListAllContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	containers, err := h.docker.ListContainers(ctx, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	untracked, err := h.store.GetUntrackedContainerIDs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	untrackedSet := make(map[string]struct{}, len(untracked))
	for _, id := range untracked {
		untrackedSet[id] = struct{}{}
	}

	type containerResult struct {
		models.ContainerInfo
		Tracked     bool   `json:"tracked"`
		SiteID      *int64 `json:"siteId,omitempty"`
		Domain      string `json:"domain,omitempty"`
		CodePath    string `json:"codePath,omitempty"`
		HasManifest bool   `json:"hasManifest"`
	}

	results := make([]containerResult, 0, len(containers))
	for _, c := range containers {
		_, isUntracked := untrackedSet[c.ID]
		r := containerResult{
			ContainerInfo: c,
			Tracked:       !isUntracked,
		}
		if site, err := h.store.GetSiteByContainerIDRecord(c.ID); err == nil && site != nil {
			r.SiteID = &site.ID
			r.Domain = site.Domain
			r.CodePath = site.CodePath
			if site.CodePath != "" {
				_, statErr := os.Stat(filepath.Join(site.CodePath, dklFilename))
				r.HasManifest = statErr == nil
			}
		}
		results = append(results, r)
	}

	writeJSON(w, http.StatusOK, map[string]any{"containers": results})
}

func (h *Handlers) Container(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/containers/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "start":
		h.handleLifecycle(w, r, id, h.docker.StartContainer)
	case "stop":
		h.handleLifecycle(w, r, id, h.docker.StopContainer)
	case "restart":
		h.handleLifecycle(w, r, id, h.docker.RestartContainer)
	case "delete":
		h.handleDelete(w, r, id)
	case "logs":
		h.handleLogs(w, r, id)
	case "stats":
		h.handleStats(w, r, id)
	case "inspect":
		h.handleInspect(w, r, id)
	case "export":
		h.ExportSite(w, r)
	case "assign":
		h.AssignContainer(w, r, id)
	case "track":
		h.TrackContainer(w, r, id)
	case "untrack":
		h.UntrackContainer(w, r, id)
	case "write-manifest":
		h.WriteContainerManifest(w, r, id)
	case "claim":
		h.ClaimContainer(w, r, id)
	case "transfer":
		h.TransferSite(w, r, id)
	case "terminal":
		h.ContainerTerminal(w, r, id)
	case "":
		h.handleApp(w, r, id)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *Handlers) handleLifecycle(w http.ResponseWriter, r *http.Request, id string, action func(context.Context, string) error) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()
	site, err := h.authorizeContainerAccess(ctx, r, id)
	if err != nil {
		if err == errForbidden {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := action(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	isStopping := strings.Contains(r.URL.Path, "/stop")
	if site != nil {
		status := "running"
		if isStopping {
			status = "stopped"
		}
		_ = h.store.UpdateSiteStatus(site.ID, status)
	}
	// After a start or restart, the container may have received a new random
	// host port (Docker re-draws from the ephemeral range each time).
	// Re-detect the port and rewrite the nginx upstream so the site stays live.
	if !isStopping {
		if info, inspErr := h.docker.InspectContainer(ctx, id); inspErr == nil {
			labels := info.Config.Labels
			if labels["docklite.managed"] == "true" {
				domain := labels["docklite.domain"]
				includeWww := labels["docklite.include_www"] == "true"
				internalPortStr := labels["docklite.internal_port"]
				internalPort := 80
				if p, err := strconv.Atoi(internalPortStr); err == nil && p > 0 {
					internalPort = p
				}
				if domain != "" {
					if hostPort, portErr := h.getContainerHostPort(ctx, id, internalPort); portErr == nil && hostPort > 0 {
						_ = setupNginxForDomain(domain, includeWww, hostPort)
					}
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) handleLogs(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
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
	trail := r.URL.Query().Get("tail")
	if trail == "" {
		trail = "200"
	}
	logs, err := h.docker.ContainerLogs(ctx, id, trail)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"logs": logs})
}

func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
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
	stats, err := h.docker.ContainerStats(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]*models.ContainerStats{"stats": stats})
}

func (h *Handlers) handleInspect(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()
	container, err := h.docker.InspectContainer(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.authorizeContainerAccess(ctx, r, id, container.Config.Labels); err != nil {
		if err == errForbidden {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"container": inspectInfo(container)})
}

func (h *Handlers) handleApp(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method == http.MethodDelete {
		h.handleDelete(w, r, id)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()
	container, err := h.docker.InspectContainer(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.authorizeContainerAccess(ctx, r, id, container.Config.Labels); err != nil {
		if err == errForbidden {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var stats *models.ContainerStats
	if container.State != nil && container.State.Running {
		stats, _ = h.docker.ContainerStats(ctx, id)
	}
	writeJSON(w, http.StatusOK, map[string]any{"container": containerInfo(container), "stats": stats})
}

func (h *Handlers) handleDelete(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()
	site, err := h.authorizeContainerAccess(ctx, r, id)
	if err != nil {
		if err == errForbidden {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	container, inspErr := h.docker.InspectContainer(ctx, id)
	var domain string
	if inspErr == nil && container.Config != nil && container.Config.Labels != nil {
		domain = container.Config.Labels["docklite.domain"]
	}

	if err := h.docker.RemoveContainer(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if site != nil {
		if domain == "" {
			domain = site.Domain
		}
		_ = h.store.DeleteSite(site.ID)
	}
	_ = h.store.UnlinkContainerFromAllFolders(id)

	if domain != "" {
		_ = removeNginxSiteConfig(domain)
		_ = reloadNginx()
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) TransferSite(w http.ResponseWriter, r *http.Request, containerID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	var req struct {
		NewUserID int64 `json:"new_user_id"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.NewUserID <= 0 {
		writeError(w, http.StatusBadRequest, "new_user_id is required")
		return
	}

	h.transferSiteToUser(w, r, containerID, req.NewUserID)
}

func (h *Handlers) transferSiteToUser(w http.ResponseWriter, r *http.Request, containerID string, newUserID int64) {

	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	site, err := h.store.GetSiteByContainerIDRecord(containerID)
	if err != nil || site == nil {
		writeError(w, http.StatusNotFound, "site not found for this container")
		return
	}

	if site.UserID == newUserID {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "site already owned by this user"})
		return
	}

	newUser, err := h.store.GetUserByIDFull(newUserID)
	if err != nil || newUser == nil {
		writeError(w, http.StatusNotFound, "target user not found")
		return
	}

	oldUser, _ := h.store.GetUserByIDFull(site.UserID)
	oldUsername := "unknown"
	if oldUser != nil {
		oldUsername = oldUser.Username
	}

	oldPath := site.CodePath
	if oldPath == "" {
		oldPath = getSitePath(oldUsername, site.Domain)
	}
	newPath := getSitePath(newUser.Username, site.Domain)

	if err := ensureSiteDirectory(newUser.Username, site.Domain); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create target directory: "+err.Error())
		return
	}

	if oldPath != newPath {
		if info, statErr := os.Stat(oldPath); statErr == nil && info.IsDir() {
			if err := copyFileOrDir(oldPath, newPath); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to copy files: "+err.Error())
				return
			}
			_ = os.RemoveAll(oldPath)
		}
	}

	if err := h.store.TransferSite(site.ID, newUserID, newPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update site record: "+err.Error())
		return
	}

	container, inspErr := h.docker.InspectContainer(ctx, containerID)
	if inspErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"warning": "site transferred but container could not be inspected — restart it manually",
		})
		return
	}

	includeWww := container.Config.Labels["docklite.include_www"] == "true"
	templateType := site.TemplateType
	internalPort := 80
	if portStr := container.Config.Labels["docklite.internal_port"]; portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			internalPort = p
		}
	}

	_ = h.docker.RemoveContainer(ctx, containerID)

	createCtx, createCancel := ctxWithLongTimeout()
	defer createCancel()

	newContainerID, err := h.docker.CreateSiteContainer(
		createCtx, site.Domain, templateType, includeWww,
		newPath, internalPort, site.ID, newUserID, site.FolderID,
	)
	if err != nil {
		_ = h.store.UpdateSiteStatus(site.ID, "failed")
		writeError(w, http.StatusInternalServerError, "failed to create new container: "+err.Error())
		return
	}

	_ = h.store.UpdateSiteContainerID(site.ID, &newContainerID)
	_ = h.store.UpdateSiteStatus(site.ID, "running")

	nginxWarning := ""
	if hostPort, portErr := h.getContainerHostPort(createCtx, newContainerID, internalPort); portErr == nil && hostPort > 0 {
		if ngErr := updateNginxProxyPort(site.Domain, hostPort); ngErr != nil {
			// Fall back to full config generation if no existing config exists.
			if ngErr2 := setupNginxForDomain(site.Domain, includeWww, hostPort); ngErr2 != nil {
				nginxWarning = ngErr2.Error()
			}
		}
	}

	_ = WriteDKLManifest(newPath, site.Domain, templateType, newUser.Username, internalPort, includeWww, nil)

	resp := map[string]any{
		"success":          true,
		"new_user":         newUser.Username,
		"new_path":         newPath,
		"new_container_id": newContainerID,
	}
	if nginxWarning != "" {
		resp["warning"] = nginxWarning
	}
	writeJSON(w, http.StatusOK, resp)
}

func copyFileOrDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, 0o775); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyFileOrDir(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
				return err
			}
		}
		chownDocklite(dst)
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0o664); err != nil {
		return err
	}
	chownDocklite(dst)
	return nil
}

func (h *Handlers) createContainer(w http.ResponseWriter, r *http.Request) {
	var req createContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, err := readUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	isAdmin := isAdminRole(r)

	domain := strings.TrimSpace(req.Domain)
	if domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}
	if strings.Contains(domain, "/") || strings.Contains(domain, "\\") {
		writeError(w, http.StatusBadRequest, "domain contains invalid characters")
		return
	}

	templateType := strings.ToLower(strings.TrimSpace(req.TemplateType))
	if templateType == "" {
		templateType = "static"
	}
	if templateType != "static" && templateType != "php" && templateType != "node" {
		writeError(w, http.StatusBadRequest, "unsupported template type")
		return
	}

	includeWww := true
	if req.IncludeWww != nil {
		includeWww = *req.IncludeWww
	}

	port := 3000
	if req.Port != nil {
		port = *req.Port
	}
	if templateType == "node" {
		if port < 1 || port > 65535 {
			writeError(w, http.StatusBadRequest, "invalid port")
			return
		}
	}

	var targetUserID int64
	if isAdmin && req.UserID != nil {
		targetUserID = *req.UserID
	} else if userID != nil {
		targetUserID = *userID
	} else {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	targetUser, err := h.store.GetUserByIDFull(targetUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if targetUser == nil {
		writeError(w, http.StatusNotFound, "target user not found")
		return
	}

	existingSite, err := h.store.GetSiteByDomain(domain)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var existingContainerID string
	if existingSite != nil && existingSite.ContainerID != nil {
		checkCtx, checkCancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer checkCancel()
		if _, err := h.docker.InspectContainer(checkCtx, *existingSite.ContainerID); err == nil {
			existingContainerID = *existingSite.ContainerID
		}
	}

	sitePath := req.CodePath
	if sitePath == "" {
		if existingSite != nil && existingSite.CodePath != "" {
			sitePath = existingSite.CodePath
		} else {
			sitePath = getSitePath(targetUser.Username, domain)
		}
	}

	if err := ensureSiteDirectory(targetUser.Username, domain); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if req.CodePath == "" && existingSite == nil {
		if err := ensureDefaultSiteFiles(sitePath, domain, templateType); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	site := existingSite
	if site == nil {
		siteRecord := store.SiteRecord{
			Domain:       domain,
			UserID:       targetUserID,
			TemplateType: templateType,
			CodePath:     sitePath,
			FolderID:     req.FolderID,
		}
		site, err = h.store.CreateSite(siteRecord)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Use a long-lived context for image pull + container creation.
	// Image pulls on a cold host can take several minutes; the short
	// dockerContext (30 s) would expire while the pull is still running,
	// leaving ContainerCreate/Start with an already-cancelled context.
	createCtx, createCancel := ctxWithLongTimeout()
	defer createCancel()

	if existingContainerID != "" {
		_ = h.docker.RemoveContainer(createCtx, existingContainerID)
	}
	containerID, err := h.docker.CreateSiteContainer(createCtx, domain, templateType, includeWww, sitePath, port, site.ID, targetUserID, req.FolderID)
	if err != nil {
		_ = h.store.UpdateSiteStatus(site.ID, "failed")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := h.store.UpdateSiteContainerID(site.ID, &containerID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = h.store.UpdateSiteStatus(site.ID, "running")

	// Write .dkl manifest so a future re-install can discover and re-attach
	// this site without needing to recreate it from scratch.
	_ = WriteDKLManifest(sitePath, domain, templateType, targetUser.Username, port, includeWww, nil)

	// Determine the actual internal port the container listens on.
	// Static and PHP containers always use port 80; Node uses the user-specified port.
	nginxInternalPort := port
	if templateType == "static" || templateType == "php" {
		nginxInternalPort = 80
	}

	nginxWarning := ""
	hostPort, portErr := h.getContainerHostPort(createCtx, containerID, nginxInternalPort)
	if portErr == nil && hostPort > 0 {
		if ngErr := setupNginxForDomain(domain, includeWww, hostPort); ngErr != nil {
			nginxWarning = fmt.Sprintf("Container created but nginx config failed: %v", ngErr)
		}
	} else if portErr != nil {
		nginxWarning = fmt.Sprintf("Container created but could not detect host port: %v", portErr)
	}

	resp := map[string]any{"success": true, "site_id": site.ID}
	if nginxWarning != "" {
		resp["warning"] = nginxWarning
	}
	writeJSON(w, http.StatusOK, resp)
}

func ensureFile(path string, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o664); err != nil {
		return err
	}
	chownDocklite(path)
	return nil
}

var errForbidden = errors.New("forbidden")

func (h *Handlers) resolveContainerOwner(container models.ContainerInfo, userCache map[int64]string) (int64, string, error) {
	var ownerID int64
	if container.Labels != nil {
		if label := container.Labels["docklite.user.id"]; label != "" {
			if parsed, err := strconv.ParseInt(label, 10, 64); err == nil {
				ownerID = parsed
			}
		}
	}

	site, err := h.store.GetSiteByContainerIDRecord(container.ID)
	if err != nil {
		return 0, "", err
	}
	if site != nil {
		ownerID = site.UserID
	}
	if ownerID == 0 {
		return 0, "", nil
	}
	if cached, ok := userCache[ownerID]; ok {
		return ownerID, cached, nil
	}
	user, err := h.store.GetUserByIDFull(ownerID)
	if err != nil {
		return 0, "", err
	}
	if user == nil {
		return ownerID, "", nil
	}
	userCache[ownerID] = user.Username
	return ownerID, user.Username, nil
}

func (h *Handlers) authorizeContainerAccess(ctx context.Context, r *http.Request, containerID string, labels ...map[string]string) (*store.SiteRecord, error) {
	site, err := h.store.GetSiteByContainerIDRecord(containerID)
	if err != nil {
		return nil, err
	}
	if isAdminRole(r) {
		return site, nil
	}
	userID, ok := readUserIDFromContext(r)
	if !ok || userID <= 0 {
		return nil, errForbidden
	}
	if site != nil {
		if site.UserID != userID {
			return nil, errForbidden
		}
		return site, nil
	}

	labelSet := map[string]string(nil)
	if len(labels) > 0 {
		labelSet = labels[0]
	}
	if labelSet == nil {
		container, err := h.docker.InspectContainer(ctx, containerID)
		if err != nil {
			return nil, err
		}
		labelSet = container.Config.Labels
	}

	labelUserID := ""
	if labelSet != nil {
		labelUserID = labelSet["docklite.user.id"]
	}
	if labelUserID == "" || labelUserID != fmt.Sprintf("%d", userID) {
		return nil, errForbidden
	}
	return nil, nil
}

func containerInfo(container types.ContainerJSON) models.ContainerInfo {
	createdAt, err := time.Parse(time.RFC3339Nano, container.Created)
	uptime := "-"
	if err == nil && container.State != nil && container.State.Running {
		uptime = formatUptime(time.Since(createdAt))
	}
	state := "stopped"
	status := ""
	if container.State != nil {
		status = container.State.Status
		if container.State.Running {
			state = "running"
		}
	}
	labels := map[string]string{}
	image := ""
	if container.Config != nil {
		image = container.Config.Image
		if container.Config.Labels != nil {
			labels = container.Config.Labels
		}
	}
	ports := "-"
	if container.NetworkSettings != nil {
		ports = formatPortsFromInspect(container.NetworkSettings.Ports)
	}
	name := strings.TrimPrefix(container.Name, "/")
	return models.ContainerInfo{
		ID:      container.ID,
		Name:    name,
		Image:   image,
		Created: createdAt.Unix(),
		State:   state,
		Status:  status,
		Uptime:  uptime,
		Ports:   ports,
		Labels:  labels,
	}
}

func inspectInfo(container types.ContainerJSON) map[string]any {
	labels := map[string]string{}
	env := []string{}
	image := ""
	if container.Config != nil {
		image = container.Config.Image
		if container.Config.Labels != nil {
			labels = container.Config.Labels
		}
		if container.Config.Env != nil {
			env = container.Config.Env
		}
	}
	networkSettings := map[string]any{
		"networks": map[string]any{},
		"ports":    nat.PortMap{},
		"ipAddress": func() string {
			if container.NetworkSettings != nil {
				return container.NetworkSettings.IPAddress
			}
			return ""
		}(),
		"gateway": func() string {
			if container.NetworkSettings != nil {
				return container.NetworkSettings.Gateway
			}
			return ""
		}(),
	}
	if container.NetworkSettings != nil {
		networkSettings["networks"] = container.NetworkSettings.Networks
		networkSettings["ports"] = container.NetworkSettings.Ports
	}
	resources := map[string]any{
		"memory":     0,
		"memorySwap": 0,
		"cpuShares":  0,
		"cpuQuota":   0,
	}
	restartPolicy := map[string]any{}
	if container.HostConfig != nil {
		restartPolicy = map[string]any{
			"Name":              container.HostConfig.RestartPolicy.Name,
			"MaximumRetryCount": container.HostConfig.RestartPolicy.MaximumRetryCount,
		}
		resources = map[string]any{
			"memory":     container.HostConfig.Memory,
			"memorySwap": container.HostConfig.MemorySwap,
			"cpuShares":  container.HostConfig.CPUShares,
			"cpuQuota":   container.HostConfig.CPUQuota,
		}
	}
	return map[string]any{
		"id":              container.ID,
		"name":            strings.TrimPrefix(container.Name, "/"),
		"image":           image,
		"created":         container.Created,
		"state":           container.State,
		"env":             env,
		"labels":          labels,
		"mounts":          container.Mounts,
		"networkSettings": networkSettings,
		"restartPolicy":   restartPolicy,
		"resources":       resources,
	}
}

func formatUptime(duration time.Duration) string {
	seconds := int64(duration.Seconds())
	if seconds < 0 {
		seconds = 0
	}
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours%24)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes%60)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

func formatPortsFromInspect(ports nat.PortMap) string {
	if ports == nil {
		return "-"
	}
	mappings := make([]string, 0)
	for containerPort, hostPorts := range ports {
		if hostPorts == nil || len(hostPorts) == 0 {
			continue
		}
		hostPort := hostPorts[0].HostPort
		cleanPort := strings.TrimSuffix(string(containerPort), "/tcp")
		cleanPort = strings.TrimSuffix(cleanPort, "/udp")
		mappings = append(mappings, fmt.Sprintf("%s->%s", hostPort, cleanPort))
	}
	if len(mappings) == 0 {
		return "-"
	}
	return strings.Join(mappings, ", ")
}

func buildFolderTree(folders []store.Folder, containersByFolderID map[int64][]models.ContainerInfo) []*folderNode {
	folderMap := make(map[int64]*folderNode, len(folders))
	for _, folder := range folders {
		containers := containersByFolderID[folder.ID]
		if containers == nil {
			containers = []models.ContainerInfo{}
		}
		node := &folderNode{
			Folder:     folder,
			Children:   []*folderNode{},
			Containers: containers,
		}
		folderMap[folder.ID] = node
	}

	var roots []*folderNode
	for _, folder := range folders {
		node := folderMap[folder.ID]
		if folder.ParentFolderID == nil {
			roots = append(roots, node)
			continue
		}
		parent := folderMap[*folder.ParentFolderID]
		if parent != nil {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node)
		}
	}

	sortByPosition(roots)
	return roots
}

type folderNode struct {
	store.Folder
	Children   []*folderNode          `json:"children"`
	Containers []models.ContainerInfo `json:"containers"`
}

func sortByPosition(nodes []*folderNode) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Position < nodes[j].Position
	})
	for _, node := range nodes {
		sortByPosition(node.Children)
	}
}

func getSitePath(username string, domain string) string {
	return filepath.Join(siteBaseDir, username, domain)
}

func ensureSiteDirectory(username string, domain string) error {
	sitePath := getSitePath(username, domain)
	if err := os.MkdirAll(sitePath, 0o775); err != nil {
		return fmt.Errorf("failed to create site directory: %w", err)
	}
	chownDocklite(filepath.Join(siteBaseDir, username))
	chownDocklite(sitePath)
	return nil
}

func ensureDefaultSiteFiles(sitePath string, domain string, templateType string) error {
	candidateFiles := []string{"index.html", "index.php", "index.js"}
	for _, name := range candidateFiles {
		if _, err := os.Stat(filepath.Join(sitePath, name)); err == nil {
			return nil
		}
	}

	switch templateType {
	case "php":
		return ensureFile(filepath.Join(sitePath, "index.php"), phpTemplate(sitePath, domain))
	case "node":
		if err := ensureFile(filepath.Join(sitePath, "package.json"), nodePackageJSON(domain)); err != nil {
			return err
		}
		return ensureFile(filepath.Join(sitePath, "index.js"), nodeTemplate(sitePath, domain))
	default:
		return ensureFile(filepath.Join(sitePath, "index.html"), staticTemplate(sitePath, domain))
	}
}

func staticTemplate(sitePath string, domain string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + domain + `</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #333; }
        .info { background: #f0f0f0; padding: 15px; border-radius: 5px; }
    </style>
</head>
<body>
    <h1>Welcome to ` + domain + `</h1>
    <div class="info">
        <p>This is a static site managed by DockLite.</p>
        <p><strong>Site Path:</strong> <code>` + sitePath + `</code></p>
        <p>You can edit this file to customize your site.</p>
    </div>
</body>
</html>`
}

func phpTemplate(sitePath string, domain string) string {
	return `<?php
/**
 * ` + domain + `
 * PHP site managed by DockLite
 * Site path: ` + sitePath + `
 */
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title><?php echo '` + domain + `'; ?></title>
</head>
<body>
    <h1>Welcome to <?php echo '` + domain + `'; ?></h1>
    <p>This is a PHP site managed by DockLite.</p>
</body>
</html>`
}

func nodePackageJSON(domain string) string {
	payload := map[string]any{
		"name":        strings.ReplaceAll(domain, ".", "-"),
		"version":     "1.0.0",
		"description": fmt.Sprintf("Node.js site for %s", domain),
		"main":        "index.js",
		"scripts": map[string]string{
			"start": "node index.js",
		},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

func nodeTemplate(sitePath string, domain string) string {
	return `const http = require('http');

const hostname = '0.0.0.0';
const port = process.env.PORT || 3000;

const server = http.createServer((req, res) => {
  res.statusCode = 200;
  res.setHeader('Content-Type', 'text/html');
  res.end(` + "`" + `
    <!DOCTYPE html>
    <html>
      <head><title>` + domain + `</title></head>
      <body>
        <h1>Welcome to ` + domain + `</h1>
        <p>Node.js site managed by DockLite</p>
        <p>Site path: ` + sitePath + `</p>
      </body>
    </html>
  ` + "`" + `);
});

server.listen(port, hostname, () => {
  console.log(` + "`" + `Server running at http://${hostname}:${port}/` + "`" + `);
});`
}
