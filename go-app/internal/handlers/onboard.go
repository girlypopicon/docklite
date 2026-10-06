package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"docklite-agent/internal/store"
)

const dklFilename = ".dkl"

// dklManifest is the metadata written into every site directory when a
// container is first provisioned.  The file acts as a breadcrumb so that a
// fresh DockLite installation can discover and re-attach to existing sites.
// It contains enough information to rebuild the container against the existing
// files on disk without needing a full .dklpkg backup.
type dklManifest struct {
	Version      string    `json:"version"`
	Domain       string    `json:"domain"`
	TemplateType string    `json:"templateType"`
	Image        string    `json:"image"`        // Docker image used (e.g. "nginx:alpine")
	InternalPort int       `json:"internalPort"` // port the container listens on internally
	Port         int       `json:"port"`         // user-facing port (node only; same as internalPort)
	IncludeWww   bool      `json:"includeWww"`
	Username     string    `json:"username"`
	Env          []string  `json:"env,omitempty"` // template-defined env vars (no secrets)
	CreatedAt    time.Time `json:"createdAt"`
	// PreviousOwners is the site's ownership history, oldest first. It lives in the
	// manifest so it travels with the folder, backups and .dklpkg packages.
	PreviousOwners []dklPreviousOwner `json:"previousOwners,omitempty"`
}

type dklPreviousOwner struct {
	Username string    `json:"username"`
	Until    time.Time `json:"until"`            // when they stopped owning it
	Reason   string    `json:"reason,omitempty"` // "transfer", "user-deleted", ...
	By       string    `json:"by,omitempty"`     // who made the change, when known
}

func readDKL(sitePath string) (dklManifest, bool) {
	var m dklManifest
	data, err := os.ReadFile(filepath.Join(sitePath, dklFilename))
	if err != nil || json.Unmarshal(data, &m) != nil {
		return m, false
	}
	return m, true
}

// RecordPreviousOwner adds a former owner to the site's .dkl. Best effort: a
// folder with no manifest (or one we can't write) simply has no history file.
func RecordPreviousOwner(sitePath, username, reason, by string) {
	if username == "" || username == "unknown" {
		return
	}
	m, ok := readDKL(sitePath)
	if !ok {
		return
	}
	m.PreviousOwners = append(m.PreviousOwners, dklPreviousOwner{Username: username, Until: time.Now().UTC(), Reason: reason, By: by})
	if data, err := json.MarshalIndent(m, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(sitePath, dklFilename), data, 0o644)
	}
}

// templateImage returns the Docker image for a given template type.
func templateImage(templateType string) string {
	switch templateType {
	case "php":
		return "webdevops/php-nginx:8.2-alpine"
	case "node":
		return "node:20-alpine"
	default:
		return "nginx:alpine"
	}
}

// templateInternalPort returns the container-internal port for a template type.
func templateInternalPort(templateType string, userPort int) int {
	switch templateType {
	case "static", "php":
		return 80
	case "node":
		if userPort > 0 {
			return userPort
		}
		return 3000
	default:
		return 80
	}
}

// WriteDKLManifest writes a .dkl marker file into the site directory.
// Non-fatal: a failure is logged but does not affect the container lifecycle.
func WriteDKLManifest(sitePath, domain, templateType, username string, port int, includeWww bool, env []string) error {
	manifest := dklManifest{
		Version:      "1",
		Domain:       domain,
		TemplateType: templateType,
		Image:        templateImage(templateType),
		InternalPort: templateInternalPort(templateType, port),
		Port:         port,
		IncludeWww:   includeWww,
		Username:     username,
		Env:          env,
		CreatedAt:    time.Now().UTC(),
	}
	// Rewriting a manifest must never lose what it already knows.
	if prev, ok := readDKL(sitePath); ok {
		manifest.PreviousOwners = prev.PreviousOwners
		if !prev.CreatedAt.IsZero() {
			manifest.CreatedAt = prev.CreatedAt
		}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(sitePath, dklFilename), data, 0o644)
}

type scannedSite struct {
	Path       string      `json:"path"`
	Manifest   dklManifest `json:"manifest"`
	Registered bool        `json:"registered"`
}

// ScanSites walks /var/www/sites looking for .dkl files and returns a list of
// found sites, flagging which ones are already registered in the database.
func (h *Handlers) ScanSites(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	found := []scannedSite{}

	_ = filepath.Walk(siteBaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// Only descend up to depth 2: /var/www/sites/username/domain
			rel, _ := filepath.Rel(siteBaseDir, path)
			if rel == "." {
				return nil
			}
			depth := len(strings.Split(rel, string(filepath.Separator)))
			if depth > 2 {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() != dklFilename {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var manifest dklManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil
		}

		site, _ := h.store.GetSiteByDomain(manifest.Domain)
		found = append(found, scannedSite{
			Path:       filepath.Dir(path),
			Manifest:   manifest,
			Registered: site != nil,
		})
		return nil
	})

	writeJSON(w, http.StatusOK, map[string]any{"sites": found})
}

// OnboardSite reads a .dkl file at the given path and creates a site record
// + Docker container, reusing the existing files on disk.
func (h *Handlers) OnboardSite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	var body struct {
		Path   string `json:"path"`
		UserID *int64 `json:"user_id"`
	}
	if err := readJSON(w, r, &body); err != nil {
		return
	}
	if body.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	dklPath := filepath.Join(body.Path, dklFilename)
	data, err := os.ReadFile(dklPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "no .dkl file found at path")
		return
	}

	var manifest dklManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		writeError(w, http.StatusBadRequest, "invalid .dkl file")
		return
	}

	// Already registered — nothing to do.
	if existing, _ := h.store.GetSiteByDomain(manifest.Domain); existing != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "site already registered",
			"site_id": existing.ID,
		})
		return
	}

	// Resolve target user: prefer the username stored in the manifest,
	// fall back to the explicitly supplied user_id.
	var targetUser *store.UserRecord
	if manifest.Username != "" {
		targetUser, _ = h.store.GetUserByUsername(manifest.Username)
	}
	if targetUser == nil && body.UserID != nil {
		targetUser, _ = h.store.GetUserByIDFull(*body.UserID)
	}
	if targetUser == nil {
		// Last resort: superadmin (ID 1).
		targetUser, _ = h.store.GetUserByIDFull(1)
	}
	if targetUser == nil {
		writeError(w, http.StatusInternalServerError, "could not resolve target user")
		return
	}

	port := manifest.Port
	if port <= 0 {
		port = 3000
	}
	templateType := manifest.TemplateType
	if templateType == "" {
		templateType = "static"
	}

	site, err := h.store.CreateSite(store.SiteRecord{
		Domain:       manifest.Domain,
		UserID:       targetUser.ID,
		TemplateType: templateType,
		CodePath:     body.Path,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	createCtx, createCancel := ctxWithLongTimeout()
	defer createCancel()

	containerID, err := h.docker.CreateSiteContainer(
		createCtx, manifest.Domain, templateType, manifest.IncludeWww,
		body.Path, port, site.ID, targetUser.ID, nil,
	)
	if err != nil {
		_ = h.store.UpdateSiteStatus(site.ID, "failed")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = h.store.UpdateSiteContainerID(site.ID, &containerID)
	_ = h.store.UpdateSiteStatus(site.ID, "running")

	nginxInternalPort := port
	if templateType == "static" || templateType == "php" {
		nginxInternalPort = 80
	}
	nginxWarning := ""
	if hostPort, portErr := h.getContainerHostPort(createCtx, containerID, nginxInternalPort); portErr == nil && hostPort > 0 {
		if ngErr := setupNginxForDomain(manifest.Domain, manifest.IncludeWww, hostPort); ngErr != nil {
			nginxWarning = ngErr.Error()
		}
	}

	resp := map[string]any{"success": true, "site_id": site.ID, "container_id": containerID}
	if nginxWarning != "" {
		resp["warning"] = nginxWarning
	}
	writeJSON(w, http.StatusCreated, resp)
}
