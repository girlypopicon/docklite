package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

const sitesBasePath = "/var/www/sites"

func (h *Handlers) SystemCheckFolders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	users, err := h.store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	success := 0
	failed := 0
	for _, user := range users {
		if err := ensureUserHomeFolder(user.Username); err != nil {
			failed++
			continue
		}
		success++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   fmt.Sprintf("Checked %d users", len(users)),
		"userCount": len(users),
		"created":   success,
		"failed":    failed,
	})
}

func (h *Handlers) DBCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	containers, err := h.docker.ListContainers(ctx, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	containerIDs := make(map[string]struct{}, len(containers))
	for _, container := range containers {
		if container.Labels != nil && container.Labels["docklite.managed"] == "true" {
			containerIDs[container.ID] = struct{}{}
		}
	}

	sites, err := h.store.ListSites()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	databases, err := h.store.ListDatabases()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	removedSites := 0
	removedDatabases := 0

	for _, site := range sites {
		if site.ContainerID == nil || *site.ContainerID == "" {
			_ = h.store.DeleteSite(site.ID)
			removedSites++
			continue
		}
		if _, ok := containerIDs[*site.ContainerID]; !ok {
			_ = h.store.DeleteSite(site.ID)
			removedSites++
		}
	}

	for _, database := range databases {
		if database.ContainerID == "" {
			_ = h.store.DeleteDatabasePermissionsByDatabaseID(database.ID)
			_ = h.store.DeleteDatabase(database.ID)
			removedDatabases++
			continue
		}
		if _, ok := containerIDs[database.ContainerID]; !ok {
			_ = h.store.DeleteDatabasePermissionsByDatabaseID(database.ID)
			_ = h.store.DeleteDatabase(database.ID)
			removedDatabases++
		}
	}

	_ = h.store.CleanupOrphanedDatabasePermissions()

	writeJSON(w, http.StatusOK, map[string]any{
		"removed": map[string]int{
			"sites":     removedSites,
			"databases": removedDatabases,
		},
	})
}

func ensureUserHomeFolder(username string) error {
	userPath := filepath.Join(sitesBasePath, username)
	if err := os.MkdirAll(userPath, 0o775); err != nil {
		return err
	}
	chownDocklite(userPath)
	return nil
}
