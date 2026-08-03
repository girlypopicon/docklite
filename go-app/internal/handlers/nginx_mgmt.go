package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"docklite-agent/internal/store"
)

// nginxSiteEntry is the per-site status returned by GET /api/nginx/sites.
type nginxSiteEntry struct {
	Domain       string `json:"domain"`
	TemplateType string `json:"templateType"`
	Enabled      bool   `json:"enabled"`   // symlink exists in sites-enabled
	HasConfig    bool   `json:"hasConfig"` // file exists in sites-available
	Config       string `json:"config,omitempty"`
}

// NginxSites handles:
//
//	GET /api/nginx/sites          — list all DockLite-managed nginx configs
//	GET /api/nginx/sites/{domain} — read config for one domain
//	PUT /api/nginx/sites/{domain} — write + reload config for one domain
func (h *Handlers) NginxSites(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	domain := strings.TrimSpace(
		strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/api/nginx/sites"), "/"),
	)

	if domain == "" {
		h.nginxListSites(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.nginxGetConfig(w, r, domain)
	case http.MethodPut:
		h.nginxPutConfig(w, r, domain)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// NginxActions handles:
//
//	POST /api/nginx/test   — run nginx -t and return pass/fail
//	POST /api/nginx/reload — run nginx -t then nginx -s reload
func (h *Handlers) NginxActions(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	switch strings.TrimPrefix(r.URL.Path, "/api/nginx/") {
	case "test":
		if err := testNginxConfig(); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case "reload":
		if err := reloadNginx(); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	default:
		writeError(w, http.StatusNotFound, "unknown action")
	}
}

// ── private helpers ───────────────────────────────────────────────────────────

func (h *Handlers) nginxListSites(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sites, err := h.store.ListSites()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	entries := make([]nginxSiteEntry, 0, len(sites))
	for _, s := range sites {
		entries = append(entries, nginxEntryStatus(s.Domain, s.TemplateType))
	}
	writeJSON(w, http.StatusOK, map[string]any{"sites": entries})
}

func (h *Handlers) nginxGetConfig(w http.ResponseWriter, r *http.Request, domain string) {
	site, _ := h.store.GetSiteByDomain(domain)
	if site == nil {
		writeError(w, http.StatusNotFound, "domain not managed by DockLite")
		return
	}

	entry := nginxEntryStatus(domain, site.TemplateType)

	if entry.HasConfig {
		content, err := readNginxSiteConfig(domain)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		entry.Config = content
	} else {
		entry.Config = h.defaultVhostForSite(site)
	}

	writeJSON(w, http.StatusOK, entry)
}

func (h *Handlers) nginxPutConfig(w http.ResponseWriter, r *http.Request, domain string) {
	site, _ := h.store.GetSiteByDomain(domain)
	if site == nil {
		writeError(w, http.StatusNotFound, "domain not managed by DockLite")
		return
	}

	var body struct {
		Config string `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(body.Config) == "" {
		writeError(w, http.StatusBadRequest, "config must not be empty")
		return
	}

	if err := writeNginxSiteConfig(domain, body.Config); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := enableNginxSite(domain); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := reloadNginx(); err != nil {
		// Config saved but nginx rejected it — surface the error
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// nginxEntryStatus returns a site entry with status flags but no config text.
func nginxEntryStatus(domain, templateType string) nginxSiteEntry {
	filename := sanitizeNginxFilename(domain)
	_, errA := os.Stat(filepath.Join(nginxSitesAvailable, filename))
	_, errE := os.Stat(filepath.Join(nginxSitesEnabled, filename))
	return nginxSiteEntry{
		Domain:       domain,
		TemplateType: templateType,
		HasConfig:    errA == nil,
		Enabled:      errE == nil,
	}
}

// readNginxSiteConfig reads the raw text of a site's nginx config file.
func readNginxSiteConfig(domain string) (string, error) {
	path := filepath.Join(nginxSitesAvailable, sanitizeNginxFilename(domain))
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("could not read nginx config: %w", err)
	}
	return string(data), nil
}

// defaultVhostForSite generates the standard DockLite vhost config for a site,
// looking up the actual host-bound port from Docker when possible.
func (h *Handlers) defaultVhostForSite(site *store.SiteRecord) string {
	port := 80
	if site.ContainerID != nil && *site.ContainerID != "" {
		ctx, cancel := dockerContext(nil)
		defer cancel()
		internalPort := 80
		if site.TemplateType == "node" {
			internalPort = 3000
		}
		if hostPort, err := h.getContainerHostPort(ctx, *site.ContainerID, internalPort); err == nil && hostPort > 0 {
			port = hostPort
		}
	}
	return nginxVhostConfig(site.Domain, false, port)
}
