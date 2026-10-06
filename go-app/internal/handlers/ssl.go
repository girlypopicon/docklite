package handlers

import (
	"crypto/x509"
	"docklite-agent/internal/demo"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const letsencryptLiveDir = "/etc/letsencrypt/live"

type sslCertInfo struct {
	Domain          string   `json:"domain"`
	Domains         []string `json:"domains"`
	HasSSL          bool     `json:"hasSSL"`
	ExpiryDate      *string  `json:"expiryDate"`
	DaysUntilExpiry *int     `json:"daysUntilExpiry"`
	Status          string   `json:"status"`
	CertPath        string   `json:"certPath,omitempty"`
}

func (h *Handlers) SSLStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	certs := readCertbotCertificates()

	certByDomain := map[string]*sslCertInfo{}
	for i := range certs {
		certByDomain[certs[i].Domain] = &certs[i]
		for _, d := range certs[i].Domains {
			certByDomain[d] = &certs[i]
		}
	}

	sites, _ := h.store.ListSites()
	managed := []sslCertInfo{}
	managedDomains := map[string]bool{}

	for _, site := range sites {
		domain := site.Domain
		if managedDomains[domain] {
			continue
		}
		managedDomains[domain] = true

		if cert, ok := certByDomain[domain]; ok {
			entry := *cert
			entry.Domain = domain
			managed = append(managed, entry)
		} else {
			managed = append(managed, sslCertInfo{
				Domain: domain,
				HasSSL: false,
				Status: "none",
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sites":    managed,
		"allCerts": certs,
		"meta": map[string]any{
			"provider":     "certbot",
			"certCount":    len(certs),
			"managedCount": len(managed),
		},
	})
}

func (h *Handlers) SSLIssue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	var body struct {
		Domain     string `json:"domain"`
		IncludeWww bool   `json:"includeWww"`
		Email      string `json:"email"`
	}
	if err := readJSON(w, r, &body); err != nil {
		return
	}
	if body.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}
	if !isValidDomain(body.Domain) {
		writeError(w, http.StatusBadRequest, "invalid domain")
		return
	}

	args := []string{"cert-issue", body.Domain}
	if body.IncludeWww && !strings.HasPrefix(body.Domain, "www.") {
		args = append(args, "--www")
	}
	if body.Email != "" {
		args = append(args, "--email", body.Email)
	}

	output, err := runRootHelper(nil, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":  "certbot failed",
			"detail": string(output),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("SSL certificate issued for %s", body.Domain),
		"output":  string(output),
	})
}

func (h *Handlers) SSLRenew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	var body struct {
		Domain string `json:"domain"`
	}
	if err := readJSON(w, r, &body); err != nil {
		return
	}
	if body.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}

	output, err := runRootHelper(nil, "cert-renew", body.Domain)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":  "renewal failed",
			"detail": string(output),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("SSL certificate renewed for %s", body.Domain),
		"output":  string(output),
	})
}

func (h *Handlers) SSLDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	var body struct {
		Domain string `json:"domain"`
	}
	if err := readJSON(w, r, &body); err != nil {
		return
	}
	if body.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}

	output, err := runRootHelper(nil, "cert-delete", body.Domain)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":  "delete failed",
			"detail": string(output),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("SSL certificate deleted for %s", body.Domain),
		"output":  string(output),
	})
}

// SSLBasic returns a simplified view for backward compatibility.
func (h *Handlers) SSLBasic(w http.ResponseWriter, r *http.Request) {
	certs := readCertbotCertificates()
	status := make([]map[string]string, 0, len(certs))
	for _, cert := range certs {
		expiry := "Unknown"
		if cert.ExpiryDate != nil {
			expiry = *cert.ExpiryDate
		}
		status = append(status, map[string]string{
			"domain": cert.Domain,
			"expiry": expiry,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sslStatus": status})
}

// SSLRepair kept for backward compatibility - now renews via certbot.
func (h *Handlers) SSLRepair(w http.ResponseWriter, r *http.Request) {
	h.SSLRenew(w, r)
}

func readCertbotCertificates() []sslCertInfo {
	if demo.On {
		return nil // never list the real host's certificates in demo mode
	}
	entries, err := os.ReadDir(letsencryptLiveDir)
	if err != nil {
		output, cmdErr := runRootHelper(nil, "cert-list")
		if cmdErr != nil {
			return nil
		}
		return parseLiveDirectoryListing(string(output))
	}

	certs := []sslCertInfo{}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "README" {
			continue
		}
		certPath := filepath.Join(letsencryptLiveDir, entry.Name(), "fullchain.pem")
		info := parseCertFile(entry.Name(), certPath)
		certs = append(certs, info)
	}
	return certs
}

func parseLiveDirectoryListing(output string) []sslCertInfo {
	certs := []sslCertInfo{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || name == "README" {
			continue
		}
		certPath := filepath.Join(letsencryptLiveDir, name, "fullchain.pem")
		info := parseCertFile(name, certPath)
		certs = append(certs, info)
	}
	return certs
}

func parseCertFile(name string, certPath string) sslCertInfo {
	raw, err := os.ReadFile(certPath)
	if err != nil {
		raw = readCertWithSudo(name)
	}
	if len(raw) == 0 {
		return sslCertInfo{
			Domain: name,
			HasSSL: true,
			Status: "unknown",
		}
	}

	block, _ := pem.Decode(raw)
	if block == nil {
		return sslCertInfo{Domain: name, HasSSL: true, Status: "unknown"}
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return sslCertInfo{Domain: name, HasSSL: true, Status: "unknown"}
	}

	expiry := cert.NotAfter.UTC()
	expiryStr := expiry.Format(time.RFC3339)
	days := int(time.Until(expiry).Hours() / 24)

	status := "valid"
	if days < 0 {
		status = "expired"
	} else if days < 30 {
		status = "expiring"
	}

	domains := cert.DNSNames
	if len(domains) == 0 {
		domains = []string{cert.Subject.CommonName}
	}

	return sslCertInfo{
		Domain:          name,
		Domains:         domains,
		HasSSL:          true,
		ExpiryDate:      &expiryStr,
		DaysUntilExpiry: &days,
		Status:          status,
		CertPath:        certPath,
	}
}

// readCertWithSudo reads a live certificate's cert.pem, whose leaf is the
// same certificate fullchain.pem starts with.
func readCertWithSudo(name string) []byte {
	output, err := runRootHelper(nil, "cert-read", name)
	if err != nil {
		return nil
	}
	return output
}

// Requires at least two labels (one dot), no consecutive hyphens, no wildcards.
var validDomainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)+$`)

func isValidDomain(domain string) bool {
	if len(domain) > 253 || len(domain) == 0 {
		return false
	}
	if strings.Contains(domain, "--") {
		return false
	}
	return validDomainRegex.MatchString(domain)
}
