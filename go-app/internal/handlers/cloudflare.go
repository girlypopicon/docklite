package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"docklite-agent/internal/cloudflare"
)

// CloudflareCheck reports what a token can do. POST with api_token tests a
// token before saving it; without one it tests the saved token.
func (h *Handlers) CloudflareCheck(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		APIToken string `json:"api_token"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var client *cloudflare.Client
	if token := strings.TrimSpace(body.APIToken); token != "" {
		client = cloudflare.NewClient(token)
	} else {
		config, err := h.store.GetCloudflareConfig()
		if err != nil || config == nil || !config.APIToken.Valid || config.APIToken.String == "" {
			writeError(w, http.StatusBadRequest, "No Cloudflare token saved yet")
			return
		}
		client = cloudflare.NewClient(config.APIToken.String)
	}
	writeJSON(w, http.StatusOK, client.Check())
}

// CloudflareImportZones adds every Cloudflare zone the token can see that
// DockLite doesn't track yet, so nobody has to copy Zone IDs by hand.
func (h *Handlers) CloudflareImportZones(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	client := h.cloudflareClient()
	if client == nil {
		writeError(w, http.StatusBadRequest, "Save a Cloudflare API token first")
		return
	}
	zones, err := client.ListZones()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	existing, err := h.store.GetDNSZones()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	known := make(map[string]bool, len(existing))
	for _, z := range existing {
		known[z.ZoneID] = true
	}
	imported := []string{}
	for _, zone := range zones {
		if known[zone.ID] {
			continue
		}
		accountID := zone.Account.ID
		if _, err := h.store.CreateDNSZone(zone.Name, zone.ID, &accountID, 1); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		imported = append(imported, zone.Name)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"imported": imported,
		"total":    len(zones),
	})
}

var validSSLModes = map[string]bool{"off": true, "flexible": true, "full": true, "strict": true}

// CloudflareZoneSSL reads (GET ?id=) or changes (PUT) a zone's SSL mode and
// "Always Use HTTPS" setting. id is DockLite's zone id.
func (h *Handlers) CloudflareZoneSSL(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	switch r.Method {
	case http.MethodGet:
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "Missing required parameter: id")
			return
		}
		client, zone := h.cloudflareForZone(id)
		if client == nil {
			writeError(w, http.StatusBadRequest, "Cloudflare isn't configured or the zone is unknown")
			return
		}
		ssl, err := client.GetZoneSetting(zone.ZoneID, "ssl")
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		alwaysHTTPS, err := client.GetZoneSetting(zone.ZoneID, "always_use_https")
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"domain":         zone.Domain,
			"ssl":            ssl,
			"alwaysUseHttps": alwaysHTTPS == "on",
		})
	case http.MethodPut:
		var body struct {
			ID             int64     `json:"id"`
			SSL            string    `json:"ssl"`
			AlwaysUseHTTPS *flexBool `json:"always_use_https"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		client, zone := h.cloudflareForZone(body.ID)
		if client == nil {
			writeError(w, http.StatusBadRequest, "Cloudflare isn't configured or the zone is unknown")
			return
		}
		if body.SSL != "" {
			if !validSSLModes[body.SSL] {
				writeError(w, http.StatusBadRequest, "ssl must be one of: off, flexible, full, strict")
				return
			}
			if err := client.SetZoneSetting(zone.ZoneID, "ssl", body.SSL); err != nil {
				writeError(w, http.StatusBadGateway, err.Error())
				return
			}
		}
		if body.AlwaysUseHTTPS != nil {
			value := "off"
			if *body.AlwaysUseHTTPS {
				value = "on"
			}
			if err := client.SetZoneSetting(zone.ZoneID, "always_use_https", value); err != nil {
				writeError(w, http.StatusBadGateway, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
