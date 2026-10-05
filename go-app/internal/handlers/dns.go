package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"docklite-agent/internal/cloudflare"
	"docklite-agent/internal/store"
)

func (h *Handlers) DNSConfig(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	switch r.Method {
	case http.MethodGet:
		config, err := h.store.GetCloudflareConfig()
		if err != nil || config == nil {
			writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "hasToken": false})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled":  config.Enabled == 1,
			"hasToken": config.APIToken.Valid && config.APIToken.String != "",
			"accountId": func() string {
				if config.AccountID.Valid {
					return config.AccountID.String
				}
				return ""
			}(),
		})
	case http.MethodPost:
		var body struct {
			APIToken  string    `json:"api_token"`
			AccountID string    `json:"account_id"`
			Enabled   *flexBool `json:"enabled"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		body.APIToken = strings.TrimSpace(body.APIToken)
		// Blank fields mean "keep what's stored": the form never shows the
		// saved token, so saving without retyping it used to erase it.
		var apiToken, accountID *string
		if body.APIToken != "" {
			if !cloudflare.NewClient(body.APIToken).VerifyToken() {
				writeError(w, http.StatusBadRequest, "Cloudflare didn't accept this token. Check it was copied completely and has the Zone → Zone → Read permission.")
				return
			}
			apiToken = &body.APIToken
		}
		if strings.TrimSpace(body.AccountID) != "" {
			trimmed := strings.TrimSpace(body.AccountID)
			accountID = &trimmed
		}
		var enabled *int
		if body.Enabled != nil {
			value := 0
			if *body.Enabled {
				value = 1
			}
			enabled = &value
		} else if apiToken != nil {
			value := 1
			enabled = &value
		}
		if err := h.store.UpdateCloudflareConfig(apiToken, accountID, enabled); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "Configuration updated successfully"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) DNSZones(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	switch r.Method {
	case http.MethodGet:
		zones, err := h.store.GetDNSZones()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"zones": zones})
	case http.MethodPost:
		var body struct {
			Domain     string `json:"domain"`
			ZoneID     string `json:"zone_id"`
			AccountID  string `json:"account_id"`
			AutoImport bool   `json:"auto_import"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Domain == "" || body.ZoneID == "" {
			writeError(w, http.StatusBadRequest, "Missing required fields: domain, zone_id")
			return
		}
		account := body.AccountID
		id, err := h.store.CreateDNSZone(body.Domain, body.ZoneID, &account, 1)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "message": "DNS zone created successfully"})
	case http.MethodPut:
		var body struct {
			ID        int64   `json:"id"`
			Domain    *string `json:"domain"`
			ZoneID    *string `json:"zone_id"`
			AccountID *string `json:"account_id"`
			Enabled   *int    `json:"enabled"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.ID <= 0 {
			writeError(w, http.StatusBadRequest, "Missing required field: id")
			return
		}
		params := make(map[string]any)
		if body.Domain != nil {
			params["domain"] = *body.Domain
		}
		if body.ZoneID != nil {
			params["zone_id"] = *body.ZoneID
		}
		if body.AccountID != nil {
			params["account_id"] = *body.AccountID
		}
		if body.Enabled != nil {
			params["enabled"] = *body.Enabled
		}
		if err := h.store.UpdateDNSZone(body.ID, params); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "DNS zone updated successfully"})
	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "Missing required parameter: id")
			return
		}
		if err := h.store.DeleteDNSZone(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "DNS zone deleted successfully"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) DNSRecords(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	switch r.Method {
	case http.MethodGet:
		zoneID := r.URL.Query().Get("zone_id")
		var zonePtr *int64
		if zoneID != "" {
			parsed, err := strconv.ParseInt(zoneID, 10, 64)
			if err != nil || parsed <= 0 {
				writeError(w, http.StatusBadRequest, "invalid zone_id")
				return
			}
			zonePtr = &parsed
		}
		records, err := h.store.GetDNSRecords(zonePtr)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"records": records})
	case http.MethodPost:
		var body struct {
			ZoneID   int64  `json:"zone_id"`
			Type     string `json:"type"`
			Name     string `json:"name"`
			Content  string `json:"content"`
			TTL      int    `json:"ttl"`
			Priority *int   `json:"priority"`
			Proxied  *int   `json:"proxied"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.ZoneID == 0 || body.Type == "" || body.Name == "" || body.Content == "" {
			writeError(w, http.StatusBadRequest, "Missing required fields: zone_id, type, name, content")
			return
		}
		zone, err := h.store.GetDNSZoneByID(body.ZoneID)
		if err != nil || zone == nil {
			writeError(w, http.StatusNotFound, "Zone not found")
			return
		}
		config, _ := h.store.GetCloudflareConfig()

		var cloudflareRecordID string
		if config != nil && config.Enabled == 1 && config.APIToken.Valid {
			client := cloudflare.NewClient(config.APIToken.String)
			ttl := body.TTL
			if ttl == 0 {
				ttl = 1
			}
			proxied := body.Proxied != nil && *body.Proxied == 1
			record, err := client.CreateDNSRecord(zone.ZoneID, cloudflare.DNSRecord{
				Type:     body.Type,
				Name:     body.Name,
				Content:  body.Content,
				TTL:      ttl,
				Proxied:  proxied,
				Priority: body.Priority,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Cloudflare error: "+err.Error())
				return
			}
			cloudflareRecordID = record.ID
		}

		record := store.DNSRecord{
			ZoneID:             body.ZoneID,
			CloudflareRecordID: sql.NullString{String: cloudflareRecordID, Valid: cloudflareRecordID != ""},
			Type:               body.Type,
			Name:               body.Name,
			Content:            body.Content,
			TTL:                fallbackTTL(body.TTL),
			Priority:           nullInt(body.Priority),
			Proxied:            proxiedValue(body.Proxied),
		}
		id, err := h.store.CreateDNSRecord(record)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "message": "DNS record created successfully"})
	case http.MethodPut:
		var body struct {
			ID       int64   `json:"id"`
			Type     *string `json:"type"`
			Name     *string `json:"name"`
			Content  *string `json:"content"`
			TTL      *int    `json:"ttl"`
			Priority *int    `json:"priority"`
			Proxied  *int    `json:"proxied"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.ID <= 0 {
			writeError(w, http.StatusBadRequest, "Missing required field: id")
			return
		}
		params := map[string]any{}
		if body.Type != nil {
			params["type"] = *body.Type
		}
		if body.Name != nil {
			params["name"] = *body.Name
		}
		if body.Content != nil {
			params["content"] = *body.Content
		}
		if body.TTL != nil {
			params["ttl"] = *body.TTL
		}
		if body.Priority != nil {
			params["priority"] = *body.Priority
		}
		if body.Proxied != nil {
			params["proxied"] = *body.Proxied
		}
		// Push the change to Cloudflare first; editing only the local copy
		// left the real DNS record unchanged.
		existing, err := h.store.GetDNSRecordByID(body.ID)
		if err != nil || existing == nil {
			writeError(w, http.StatusNotFound, "DNS record not found")
			return
		}
		if client, zone := h.cloudflareForZone(existing.ZoneID); client != nil && existing.CloudflareRecordID.Valid && existing.CloudflareRecordID.String != "" {
			merged := cloudflare.DNSRecord{
				Type:    pickString(body.Type, existing.Type),
				Name:    pickString(body.Name, existing.Name),
				Content: pickString(body.Content, existing.Content),
				TTL:     existing.TTL,
				Proxied: existing.Proxied == 1,
			}
			if body.TTL != nil {
				merged.TTL = *body.TTL
			}
			if merged.TTL == 0 {
				merged.TTL = 1
			}
			if body.Proxied != nil {
				merged.Proxied = *body.Proxied == 1
			}
			if body.Priority != nil {
				merged.Priority = body.Priority
			} else if existing.Priority.Valid {
				p := int(existing.Priority.Int64)
				merged.Priority = &p
			}
			if _, err := client.UpdateDNSRecord(zone.ZoneID, existing.CloudflareRecordID.String, merged); err != nil {
				writeError(w, http.StatusBadGateway, err.Error())
				return
			}
		}
		if err := h.store.UpdateDNSRecord(body.ID, params); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "DNS record updated successfully"})
	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "Missing required parameter: id")
			return
		}
		existing, err := h.store.GetDNSRecordByID(id)
		if err != nil || existing == nil {
			writeError(w, http.StatusNotFound, "DNS record not found")
			return
		}
		if client, zone := h.cloudflareForZone(existing.ZoneID); client != nil && existing.CloudflareRecordID.Valid && existing.CloudflareRecordID.String != "" {
			// A record already gone from Cloudflare is fine to drop locally.
			if err := client.DeleteDNSRecord(zone.ZoneID, existing.CloudflareRecordID.String); err != nil &&
				!strings.Contains(strings.ToLower(err.Error()), "not exist") && !strings.Contains(strings.ToLower(err.Error()), "not found") {
				writeError(w, http.StatusBadGateway, err.Error())
				return
			}
		}
		if err := h.store.DeleteDNSRecord(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "DNS record deleted successfully"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) DNSSync(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ZoneID *int64 `json:"zone_id"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	config, err := h.store.GetCloudflareConfig()
	if err != nil || config == nil || !config.APIToken.Valid {
		writeError(w, http.StatusBadRequest, "Cloudflare API token not configured")
		return
	}
	if config.Enabled == 0 {
		writeError(w, http.StatusBadRequest, "Cloudflare integration is disabled")
		return
	}

	client := cloudflare.NewClient(config.APIToken.String)
	var zones []*store.DNSZone
	if body.ZoneID != nil {
		zone, err := h.store.GetDNSZoneByID(*body.ZoneID)
		if err != nil || zone == nil {
			writeError(w, http.StatusBadRequest, "No zones to sync")
			return
		}
		zones = append(zones, zone)
	} else {
		all, err := h.store.GetDNSZones()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for i := range all {
			if all[i].Enabled == 1 {
				zones = append(zones, &all[i])
			}
		}
	}

	if len(zones) == 0 {
		writeError(w, http.StatusBadRequest, "No zones to sync")
		return
	}

	totalRecords := 0
	results := make([]map[string]any, 0, len(zones))
	for _, zone := range zones {
		if zone == nil {
			continue
		}
		cfRecords, err := client.ListDNSRecords(zone.ZoneID)
		if err != nil {
			results = append(results, map[string]any{
				"zone":    zone.Domain,
				"records": 0,
				"status":  "failed",
				"error":   err.Error(),
			})
			continue
		}
		_ = h.store.ClearDNSRecords(zone.ID)
		for _, record := range cfRecords {
			rec := store.DNSRecord{
				ZoneID:             zone.ID,
				CloudflareRecordID: sql.NullString{String: record.ID, Valid: record.ID != ""},
				Type:               record.Type,
				Name:               record.Name,
				Content:            record.Content,
				TTL:                record.TTL,
				Priority:           nullInt(record.Priority),
				Proxied:            boolToInt(record.Proxied),
			}
			_, _ = h.store.CreateDNSRecord(rec)
			totalRecords++
		}
		_ = h.store.UpdateDNSZoneSyncTime(zone.ID)
		results = append(results, map[string]any{
			"zone":    zone.Domain,
			"records": len(cfRecords),
			"status":  "success",
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Synced " + strconv.Itoa(totalRecords) + " records from " + strconv.Itoa(len(results)) + " zone(s)",
		"results": results,
	})
}

func fallbackTTL(ttl int) int {
	if ttl == 0 {
		return 1
	}
	return ttl
}

func proxiedValue(value *int) int {
	if value == nil {
		return 0
	}
	if *value != 0 {
		return 1
	}
	return 0
}

func nullInt(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// flexBool accepts true/false as well as 1/0: the web UI sends numbers for
// flags stored as integers, and decoding 1 into a bool failed the request.
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	switch strings.Trim(strings.ToLower(string(data)), `"`) {
	case "true", "1":
		*b = true
	case "false", "0", "null", "":
		*b = false
	default:
		return fmt.Errorf("invalid boolean %s", data)
	}
	return nil
}

// cloudflareClient returns a client when Cloudflare is configured and
// enabled, or nil.
func (h *Handlers) cloudflareClient() *cloudflare.Client {
	config, err := h.store.GetCloudflareConfig()
	if err != nil || config == nil || config.Enabled != 1 || !config.APIToken.Valid || config.APIToken.String == "" {
		return nil
	}
	return cloudflare.NewClient(config.APIToken.String)
}

// cloudflareForZone returns a client and the zone for a DockLite zone id,
// or nil when Cloudflare isn't configured or the zone is unknown.
func (h *Handlers) cloudflareForZone(zoneID int64) (*cloudflare.Client, *store.DNSZone) {
	client := h.cloudflareClient()
	if client == nil {
		return nil, nil
	}
	zone, err := h.store.GetDNSZoneByID(zoneID)
	if err != nil || zone == nil {
		return nil, nil
	}
	return client, zone
}

func pickString(value *string, fallback string) string {
	if value != nil {
		return *value
	}
	return fallback
}
