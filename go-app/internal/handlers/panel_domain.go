package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// The panel domain: the address people open DockLite's dashboard at (smoll.docklite.net), instead of the server's
// address and port. It is one nginx vhost that proxies the domain to the agent, so turning it off removes the vhost
// and the dashboard is back at http://<server ip>:<port>.

const (
	settingPanelDomain  = "panel.domain"
	settingPanelEnabled = "panel.enabled"
)

type panelDomainState struct {
	Domain      string `json:"domain"`       // the domain that is set (also kept while disabled)
	Enabled     bool   `json:"enabled"`      // nginx currently serves the dashboard at the domain
	HTTPS       bool   `json:"https"`        // the vhost has a certificate
	URL         string `json:"url"`          // where to open the dashboard right now
	FallbackURL string `json:"fallback_url"` // the address that works without a domain: server ip and port
	IP          string `json:"ip"`
	Port        int    `json:"port"`
}

// agentPort is the port this agent listens on (":3000" -> 3000).
func agentPort(listenAddr string) int {
	i := strings.LastIndex(listenAddr, ":")
	if i < 0 {
		return 3000
	}
	n, err := strconv.Atoi(listenAddr[i+1:])
	if err != nil || n <= 0 {
		return 3000
	}
	return n
}

// panelVhostConfig is the plain-HTTP vhost that sends a domain to the agent; certbot adds HTTPS to it later.
func panelVhostConfig(domain string, port int) string {
	return fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;

    client_max_body_size 100M;

    location / {
        proxy_pass http://127.0.0.1:%d;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
        proxy_buffering off;
    }
}
`, domain, port)
}

func (h *Handlers) panelDomainState() panelDomainState {
	st := panelDomainState{Port: agentPort(h.listenAddr)}
	st.Domain, _ = h.store.GetSetting(settingPanelDomain)
	enabled, _ := h.store.GetSetting(settingPanelEnabled)
	st.Enabled = st.Domain != "" && enabled == "true"
	if ip, err := serverPublicIP(); err == nil {
		st.IP = ip
	}
	host := st.IP
	if host == "" {
		host = "<server-ip>"
	}
	st.FallbackURL = fmt.Sprintf("http://%s:%d", host, st.Port)
	st.URL = st.FallbackURL
	if st.Enabled {
		scheme := "http"
		if cfg, err := readNginxSiteConfig(st.Domain); err == nil && nginxConfigHasSSL(cfg) {
			st.HTTPS = true
			scheme = "https"
		}
		st.URL = scheme + "://" + st.Domain
	}
	return st
}

// PanelDomain: GET shows the setting; PUT {domain, enabled} changes it (admins only).
func (h *Handlers) PanelDomain(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if !isAdminRole(r) {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		writeJSON(w, http.StatusOK, h.panelDomainState())
	case http.MethodPut:
		h.setPanelDomain(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) setPanelDomain(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	var body struct {
		Domain  *string `json:"domain"`
		Enabled *bool   `json:"enabled"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	before := h.panelDomainState()
	domain := before.Domain
	if body.Domain != nil {
		domain = strings.ToLower(strings.TrimSpace(*body.Domain))
	}
	enabled := before.Enabled
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	if domain != "" && !isValidDomain(domain) {
		writeError(w, http.StatusBadRequest, "that is not a valid domain")
		return
	}
	if enabled && domain == "" {
		writeError(w, http.StatusBadRequest, "set a domain first")
		return
	}
	if domain != "" {
		if site, _ := h.store.GetSiteByDomain(domain); site != nil {
			writeError(w, http.StatusConflict, domain+" is already a site on this server")
			return
		}
	}

	// A different domain, or turning it off, retires the old vhost first.
	if before.Enabled && (!enabled || domain != before.Domain) {
		_ = removeNginxSiteConfig(before.Domain)
	}
	if enabled {
		config := panelVhostConfig(domain, before.Port)
		if existing, err := readNginxSiteConfig(domain); err == nil && nginxConfigHasSSL(existing) {
			if updated, found := setProxyPort(existing, before.Port); found {
				config = updated // keep the certificate that is already there
			}
		}
		if err := writeNginxSiteConfig(domain, config); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := enableNginxSite(domain); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := reloadNginx(); err != nil {
		if enabled {
			_ = removeNginxSiteConfig(domain) // never leave a config that stops nginx reloading
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = h.store.SetSetting(settingPanelDomain, domain)
	_ = h.store.SetSetting(settingPanelEnabled, map[bool]string{true: "true", false: "false"}[enabled])
	h.audit(r, "panel.domain", domain, map[string]any{"enabled": enabled, "previous": before.Domain})
	writeJSON(w, http.StatusOK, h.panelDomainState())
}
