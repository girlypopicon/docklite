package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"strings"

	"docklite-agent/internal/cloudflare"
	"docklite-agent/internal/store"
)

// Cloudflare DNS for sites: when a website is added, create the DNS records it needs in the
// connected Cloudflare account. It only ever creates what is missing. A record that already
// points somewhere else is reported as a conflict and left alone unless the caller says to
// overwrite it. Only admins trigger this, because it edits the account-wide Cloudflare zones.

type siteDNSRecord struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	Proxied  bool   `json:"proxied"`
	Action   string `json:"action"`             // create | exists | conflict | update | created | updated
	Existing string `json:"existing,omitempty"` // what is there now, for conflicts
}

type siteDNSResult struct {
	Status  string          `json:"status"` // not-configured | no-zone | no-ip | ready | done | conflict | error
	Zone    string          `json:"zone,omitempty"`
	IP      string          `json:"ip,omitempty"`
	Records []siteDNSRecord `json:"records,omitempty"`
	Message string          `json:"message,omitempty"`
}

// serverPublicIP is this server's public address: DOCKLITE_PUBLIC_IP if set, otherwise looked up.
func serverPublicIP() (string, error) {
	if ip := strings.TrimSpace(os.Getenv("DOCKLITE_PUBLIC_IP")); ip != "" {
		return ip, nil
	}
	return fetchPublicIP()
}

// zoneForDomain picks the enabled zone that contains domain (the longest matching suffix).
func zoneForDomain(zones []store.DNSZone, domain string) *store.DNSZone {
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	var best *store.DNSZone
	for i := range zones {
		z := zones[i]
		name := strings.ToLower(z.Domain)
		if z.Enabled != 1 || name == "" {
			continue
		}
		if domain == name || strings.HasSuffix(domain, "."+name) {
			if best == nil || len(name) > len(best.Domain) {
				zc := z
				best = &zc
			}
		}
	}
	return best
}

// siteDNS works out (and, if apply is true, creates) the DNS records for a site.
func (h *Handlers) siteDNS(domain string, includeWww, proxied, apply, overwrite bool) siteDNSResult {
	domain = strings.ToLower(strings.TrimSpace(domain))
	client := h.cloudflareClient()
	if client == nil {
		return siteDNSResult{Status: "not-configured", Message: "Cloudflare isn't connected. Add an API token under Settings → Cloudflare."}
	}
	zones, err := h.store.GetDNSZones()
	if err != nil {
		return siteDNSResult{Status: "error", Message: err.Error()}
	}
	zone := zoneForDomain(zones, domain)
	if zone == nil {
		return siteDNSResult{Status: "no-zone", Message: domain + " isn't in a domain you've imported from Cloudflare. Import your domains under Settings → Cloudflare."}
	}
	ip, err := serverPublicIP()
	if err != nil || ip == "" {
		return siteDNSResult{Status: "no-ip", Zone: zone.Domain, Message: "Couldn't work out this server's public IP address."}
	}
	existing, err := client.ListDNSRecords(zone.ZoneID)
	if err != nil {
		return siteDNSResult{Status: "error", Zone: zone.Domain, IP: ip, Message: "Cloudflare: " + err.Error()}
	}

	desired := []cloudflare.DNSRecord{{Type: "A", Name: domain, Content: ip, TTL: 1, Proxied: proxied}}
	if includeWww {
		desired = append(desired, cloudflare.DNSRecord{Type: "CNAME", Name: "www." + domain, Content: domain, TTL: 1, Proxied: proxied})
	}

	res := siteDNSResult{Status: "ready", Zone: zone.Domain, IP: ip}
	conflicts := 0
	for _, want := range desired {
		out := siteDNSRecord{Name: want.Name, Type: want.Type, Content: want.Content, Proxied: want.Proxied, Action: "create"}
		var same *cloudflare.DNSRecord
		for i := range existing {
			have := existing[i]
			if !strings.EqualFold(have.Name, want.Name) || (have.Type != "A" && have.Type != "AAAA" && have.Type != "CNAME") {
				continue
			}
			if have.Type == want.Type && strings.EqualFold(have.Content, want.Content) {
				same = &have
				break
			}
			out.Action, out.Existing = "conflict", have.Type+" "+have.Content
			if overwrite && have.Type == want.Type {
				hc := have
				same = nil
				out.Action = "update"
				if apply {
					if _, uerr := client.UpdateDNSRecord(zone.ZoneID, hc.ID, want); uerr != nil {
						out.Action = "conflict"
						res.Message = "Cloudflare: " + uerr.Error()
					} else {
						out.Action = "updated"
					}
				}
			}
		}
		if same != nil {
			out.Action = "exists"
		} else if out.Action == "create" && apply {
			created, cerr := client.CreateDNSRecord(zone.ZoneID, want)
			if cerr != nil {
				out.Action = "conflict"
				out.Existing = cerr.Error()
			} else {
				out.Action = "created"
				h.cacheDNSRecord(zone, created)
			}
		}
		if out.Action == "conflict" {
			conflicts++
		}
		res.Records = append(res.Records, out)
	}
	switch {
	case conflicts > 0:
		res.Status = "conflict"
		if res.Message == "" {
			res.Message = "A record for this name already exists and points somewhere else, so I left it alone."
		}
	case apply:
		res.Status = "done"
	}
	return res
}

func (h *Handlers) cacheDNSRecord(zone *store.DNSZone, rec *cloudflare.DNSRecord) {
	if rec == nil {
		return
	}
	proxied := 0
	if rec.Proxied {
		proxied = 1
	}
	_, _ = h.store.CreateDNSRecord(store.DNSRecord{
		ZoneID: zone.ID, CloudflareRecordID: nullString(rec.ID), Type: rec.Type, Name: rec.Name,
		Content: rec.Content, TTL: rec.TTL, Proxied: proxied,
	})
}

// SiteDNS: GET previews what would happen for a domain; POST does it. Admin only.
func (h *Handlers) SiteDNS(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		domain := strings.TrimSpace(r.URL.Query().Get("domain"))
		if !isValidDomain(domain) {
			writeError(w, http.StatusBadRequest, "enter a valid domain")
			return
		}
		writeJSON(w, http.StatusOK, h.siteDNS(domain, r.URL.Query().Get("www") != "0", r.URL.Query().Get("proxied") != "0", false, false))
	case http.MethodPost:
		var body struct {
			Domain     string `json:"domain"`
			IncludeWww *bool  `json:"include_www"`
			Proxied    *bool  `json:"proxied"`
			Overwrite  bool   `json:"overwrite"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		body.Domain = strings.TrimSpace(body.Domain)
		if !isValidDomain(body.Domain) {
			writeError(w, http.StatusBadRequest, "enter a valid domain")
			return
		}
		www, proxied := boolOr(body.IncludeWww, true), boolOr(body.Proxied, true)
		res := h.siteDNS(body.Domain, www, proxied, true, body.Overwrite)
		h.audit(r, "dns.site", body.Domain, map[string]any{"status": res.Status, "zone": res.Zone, "overwrite": body.Overwrite})
		writeJSON(w, http.StatusOK, res)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func boolOr(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

func nullString(v string) sql.NullString {
	return sql.NullString{String: v, Valid: v != ""}
}
