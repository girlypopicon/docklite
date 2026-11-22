package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Automatic certificate renewal, per domain. Each certificate has an auto-renew switch (a setting per domain, under a master
// switch in Settings that is on by default, so new certificates renew by themselves). Once a day the agent renews every
// certificate whose switch is on and that has less than 30 days left, the way upstream_sync.go loops every minute.
// The outcome of the last attempt is kept per domain, so a failed renewal shows up on the SSL card and in the terminal
// app's "Needs attention" list instead of failing silently.

const (
	settingSSLAutoRenewDefault = "ssl.autorenew.default" // "off" turns the default off; unset means on
	autoRenewDomainPrefix      = "ssl.autorenew.d."      // + domain: "on" or "off"
	renewalResultPrefix        = "ssl.renewal."          // + domain: a renewalRecord as JSON
	autoRenewWithinDays        = 30
	autoRenewEvery             = 24 * time.Hour
)

// renewalRecord is how the last automatic renewal of a domain went.
type renewalRecord struct {
	At    string `json:"at"`              // RFC 3339
	OK    bool   `json:"ok"`              // false: it failed and the certificate is still the old one
	Error string `json:"error,omitempty"` // certbot's last words, when it failed
}

// autoRenewAll is the master switch in Settings: "all" (the default) makes auto-renew available and on for every
// certificate, "none" turns it off everywhere and the per-domain switches stop working.
func (h *Handlers) autoRenewAll() bool {
	v, _ := h.store.GetSetting(settingSSLAutoRenewDefault)
	return v != "off"
}

// autoRenewFor says whether a certificate renews by itself: only while the master switch is on, and then unless the
// domain (or a name its certificate covers) was switched off on its own. One domain's switch never affects another.
func (h *Handlers) autoRenewFor(domains ...string) bool {
	if !h.autoRenewAll() {
		return false
	}
	for _, d := range domains {
		if v, _ := h.store.GetSetting(autoRenewDomainPrefix + d); v == "off" {
			return false
		}
	}
	return true
}

func (h *Handlers) lastRenewal(domains ...string) *renewalRecord {
	var best *renewalRecord
	for _, d := range domains {
		raw, _ := h.store.GetSetting(renewalResultPrefix + d)
		if raw == "" {
			continue
		}
		var rec renewalRecord
		if json.Unmarshal([]byte(raw), &rec) == nil && (best == nil || rec.At > best.At) {
			r := rec
			best = &r
		}
	}
	return best
}

func (h *Handlers) saveRenewal(domain string, rec renewalRecord) {
	b, _ := json.Marshal(rec)
	_ = h.store.SetSetting(renewalResultPrefix+domain, string(b))
}

// certDomains lists the names a certificate answers to (always including its own name).
func certDomains(c sslCertInfo) []string {
	out := append([]string{c.Domain}, c.Domains...)
	return out
}

// dueForRenewal: switched on and fewer than 30 days left (an expired certificate counts).
func (h *Handlers) dueForRenewal(c sslCertInfo) bool {
	return c.DaysUntilExpiry != nil && *c.DaysUntilExpiry < autoRenewWithinDays && h.autoRenewFor(certDomains(c)...)
}

// runSSLRenewal renews every due certificate and records each outcome. It returns the domains it renewed and failed.
func (h *Handlers) runSSLRenewal() (renewed, failed []string) {
	for _, c := range readCertbotCertificates() {
		if !h.dueForRenewal(c) {
			continue
		}
		output, err := runRootHelper(nil, "cert-renew", c.Domain)
		rec := renewalRecord{At: time.Now().UTC().Format(time.RFC3339), OK: err == nil}
		if err != nil {
			rec.Error = lastWords(string(output), 300)
			failed = append(failed, c.Domain)
		} else {
			renewed = append(renewed, c.Domain)
			_ = reloadNginx() // serve the new certificate
		}
		h.saveRenewal(c.Domain, rec)
		auditSystem("ssl.autorenew", c.Domain, map[string]any{"ok": rec.OK, "error": rec.Error})
	}
	return renewed, failed
}

func lastWords(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		s = "..." + s[len(s)-n:]
	}
	return s
}

// RunSSLAutoRenew is the background loop: a first look two minutes after start, then once a day.
func (h *Handlers) RunSSLAutoRenew(ctx context.Context) {
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		h.runSSLRenewal()
		timer.Reset(autoRenewEvery)
	}
}

// SSLAutoRenew: GET gives the master switch; PUT {enabled} sets it (all or none), or with {domain, enabled} sets one
// certificate's switch (only while the master is on); POST renews whatever is due right now.
func (h *Handlers) SSLAutoRenew(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"all": h.autoRenewAll()})
	case http.MethodPut:
		var body struct {
			Domain  string `json:"domain"`
			Enabled bool   `json:"enabled"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		v := map[bool]string{true: "on", false: "off"}[body.Enabled]
		if body.Domain == "" {
			_ = h.store.SetSetting(settingSSLAutoRenewDefault, v)
			h.audit(r, "ssl.autorenew.default", v, nil)
			writeJSON(w, http.StatusOK, map[string]any{"all": body.Enabled})
			return
		}
		if !isValidDomain(body.Domain) {
			writeError(w, http.StatusBadRequest, "invalid domain")
			return
		}
		if !h.autoRenewAll() {
			writeError(w, http.StatusConflict, "automatic renewal is set to none in Settings; set it to all first")
			return
		}
		_ = h.store.SetSetting(autoRenewDomainPrefix+body.Domain, v)
		h.audit(r, "ssl.autorenew.set", body.Domain, map[string]any{"enabled": body.Enabled})
		writeJSON(w, http.StatusOK, map[string]any{"domain": body.Domain, "enabled": body.Enabled})
	case http.MethodPost:
		renewed, failed := h.runSSLRenewal()
		writeJSON(w, http.StatusOK, map[string]any{"renewed": renewed, "failed": failed})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
