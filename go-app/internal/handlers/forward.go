package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
)

// Forward: send another domain's visitors to a site. POST /api/containers/{id}/forward {domain, mode, include_www?}
// onboards the new domain like any new site (own container, own DNS, own HTTPS certificate), then points its nginx at
// the source site: mode "redirect" sends visitors to the source's address (301), mode "proxy" fetches the source's
// pages and keeps the new address in the browser. The source and the new container end up in one linked group, so
// forwarding more domains to the same site simply adds to that group.
//
// Forwards are remembered as settings ("forward.<domain>" -> {to, mode}); the upstream sync leaves them alone.

const forwardSettingPrefix = "forward."

type forwardRecord struct {
	To   string `json:"to"`   // the source site's domain
	Mode string `json:"mode"` // "redirect" or "proxy"
}

func (h *Handlers) forwardOf(domain string) (forwardRecord, bool) {
	raw, _ := h.store.GetSetting(forwardSettingPrefix + strings.ToLower(domain))
	if raw == "" {
		return forwardRecord{}, false
	}
	var rec forwardRecord
	if json.Unmarshal([]byte(raw), &rec) != nil || rec.To == "" {
		return forwardRecord{}, false
	}
	return rec, true
}

// forwardVhostConfig is the nginx server block for a forwarded domain. For a redirect, target is the source's address
// ("https://example.com"); for a proxy, upstreamPort is the port the source's container is on and sourceHost its name.
func forwardVhostConfig(domain string, includeWww bool, mode, target, sourceHost string, upstreamPort int) string {
	names := domain
	if includeWww && !strings.HasPrefix(domain, "www.") {
		names += " www." + domain
	}
	if mode == "proxy" {
		return fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;

    client_max_body_size 100M;

    location / {
        proxy_pass http://127.0.0.1:%d;
        proxy_http_version 1.1;
        proxy_set_header Host %s;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
    }
}
`, names, upstreamPort, sourceHost)
	}
	return fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;

    location / {
        return 301 %s$request_uri;
    }
}
`, names, strings.TrimRight(target, "/"))
}

// Forward handles POST (make a forward) and GET (list the domains forwarded to this site).
func (h *Handlers) Forward(w http.ResponseWriter, r *http.Request, containerID string) {
	site, err := h.store.GetSiteByContainerIDRecord(containerID)
	if err != nil || site == nil {
		writeError(w, http.StatusNotFound, "this isn't one of DockLite's websites, so nothing can be forwarded to it")
		return
	}
	if !isAdminRole(r) {
		uid, uerr := readUserID(r)
		if uerr != nil || uid == nil || *uid != site.UserID {
			writeError(w, http.StatusForbidden, "only the site's owner or an admin can forward domains to it")
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"forwards": h.forwardsTo(site.Domain)})
	case http.MethodPost:
		h.createForward(w, r, containerID, site.Domain, site.UserID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type forwardEntry struct {
	Domain      string `json:"domain"`
	Mode        string `json:"mode"`
	ContainerID string `json:"container_id,omitempty"`
}

// forwardsTo lists the domains forwarded to a site; forwards whose site was deleted are left out.
func (h *Handlers) forwardsTo(source string) []forwardEntry {
	out := []forwardEntry{}
	all, _ := h.store.SettingsWithPrefix(forwardSettingPrefix)
	for key, raw := range all {
		var rec forwardRecord
		if json.Unmarshal([]byte(raw), &rec) != nil || !strings.EqualFold(rec.To, source) {
			continue
		}
		domain := strings.TrimPrefix(key, forwardSettingPrefix)
		site, _ := h.store.GetSiteByDomain(domain)
		if site == nil {
			continue
		}
		e := forwardEntry{Domain: domain, Mode: rec.Mode}
		if site.ContainerID != nil {
			e.ContainerID = *site.ContainerID
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Domain < out[j].Domain })
	return out
}

func (h *Handlers) createForward(w http.ResponseWriter, r *http.Request, sourceID, sourceDomain string, ownerID int64) {
	var req struct {
		Domain     string `json:"domain"`
		Mode       string `json:"mode"`
		IncludeWww *bool  `json:"include_www"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	domain := strings.ToLower(strings.TrimSpace(req.Domain))
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "redirect"
	}
	switch {
	case !isValidDomain(domain):
		writeError(w, http.StatusBadRequest, "enter the new domain, like new-name.com")
		return
	case mode != "redirect" && mode != "proxy":
		writeError(w, http.StatusBadRequest, "mode must be redirect or proxy")
		return
	case domain == strings.ToLower(sourceDomain):
		writeError(w, http.StatusBadRequest, "a site can't be forwarded to itself")
		return
	}
	if existing, _ := h.store.GetSiteByDomain(domain); existing != nil {
		writeError(w, http.StatusConflict, domain+" is already a site on this server")
		return
	}
	includeWww := true
	if req.IncludeWww != nil {
		includeWww = *req.IncludeWww
	}

	// 1. Onboard the new domain exactly like a new site (container, DNS, its own HTTPS certificate), owned by the source's owner.
	body, _ := json.Marshal(map[string]any{"domain": domain, "template_type": "static", "include_www": includeWww, "user_id": ownerID})
	sub := r.Clone(r.Context())
	sub.Method = http.MethodPost
	sub.Body = io.NopCloser(bytes.NewReader(body))
	sub.ContentLength = int64(len(body))
	rec := httptest.NewRecorder()
	h.createContainer(rec, sub)
	if rec.Code != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
		return
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	newSite, _ := h.store.GetSiteByDomain(domain)
	if newSite == nil || newSite.ContainerID == nil {
		writeError(w, http.StatusInternalServerError, "the new site was created but could not be found again")
		return
	}

	// 2. Point its nginx at the source.
	ctx, cancel := ctxWithLongTimeout()
	defer cancel()
	target := "http://" + sourceDomain
	if cfg, err := readNginxSiteConfig(sourceDomain); err == nil && nginxConfigHasSSL(cfg) {
		target = "https://" + sourceDomain
	}
	port := 0
	if mode == "proxy" {
		info, err := h.docker.InspectContainer(ctx, sourceID)
		internal := 80
		if err == nil && info.Config != nil {
			if p := info.Config.Labels["docklite.internal_port"]; p != "" {
				_, _ = fmt.Sscanf(p, "%d", &internal)
			}
		}
		if hp, perr := h.getContainerHostPort(ctx, sourceID, internal); perr == nil {
			port = hp
		}
		if port <= 0 {
			writeError(w, http.StatusInternalServerError, "couldn't find the port the source site is on; the forward's container exists, set it up again once the source is running")
			return
		}
	}
	_ = h.store.SetSetting(forwardSettingPrefix+domain, mustJSON(forwardRecord{To: sourceDomain, Mode: mode}))
	cfg := forwardVhostConfig(domain, includeWww, mode, target, sourceDomain, port)
	warning := ""
	if err := writeNginxSiteConfig(domain, cfg); err != nil {
		warning = err.Error()
	} else if err := enableNginxSite(domain); err != nil {
		warning = err.Error()
	} else if err := reloadNginx(); err != nil {
		warning = err.Error()
	}

	// 3. Put both in one linked group (the source's own, if it has one).
	groupName := h.linkForwardGroup(ctx, ownerID, sourceID, *newSite.ContainerID, sourceDomain)

	h.audit(r, "site.forward", domain, map[string]any{"to": sourceDomain, "mode": mode, "group": groupName})
	resp := map[string]any{"success": true, "domain": domain, "to": sourceDomain, "mode": mode, "group": groupName, "container_id": *newSite.ContainerID}
	if warning != "" {
		resp["warning"] = warning
	} else if w2, ok := created["warning"]; ok {
		resp["warning"] = w2
	}
	if https, ok := created["https"]; ok {
		resp["https"] = https
	}
	// What happened to the new domain's DNS in Cloudflare (created, already right, a conflict, or not connected).
	if dns, ok := created["dns"]; ok {
		resp["dns"] = dns
	}
	writeJSON(w, http.StatusOK, resp)
}

// linkForwardGroup puts the source and the forward in one linked group and returns its name.
func (h *Handlers) linkForwardGroup(ctx context.Context, ownerID int64, sourceID, forwardID, sourceDomain string) string {
	srcInfo, err1 := h.docker.InspectContainer(ctx, sourceID)
	fwdInfo, err2 := h.docker.InspectContainer(ctx, forwardID)
	if err1 != nil || err2 != nil {
		return ""
	}
	srcName, fwdName := strings.TrimPrefix(srcInfo.Name, "/"), strings.TrimPrefix(fwdInfo.Name, "/")
	g, _ := h.store.GroupOfBox(srcName)
	if g == nil {
		name := sourceDomain
		var err error
		g, err = h.store.CreateGroup(ownerID, name)
		if err != nil {
			// A group with that name exists already: use it.
			if groups, lerr := h.store.ListGroups(ownerID); lerr == nil {
				for i := range groups {
					if strings.EqualFold(groups[i].Name, name) {
						g, err = &groups[i], nil
					}
				}
			}
			if g == nil || err != nil {
				return ""
			}
		}
		linked := true
		_ = h.store.UpdateGroup(g.ID, nil, &linked, nil, nil)
		_ = h.store.AddToGroup(g.ID, srcName)
	}
	_ = h.store.AddToGroup(g.ID, fwdName)
	return g.Name
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
