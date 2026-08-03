package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"docklite-agent/internal/acme"
	"docklite-agent/internal/store"
)

// SSLManager wraps the ACME manager with store integration
type SSLManager struct {
	acme   *acme.Manager
	store  *store.SQLiteStore
	mu     sync.RWMutex
	config *SSLConfig
}

// SSLConfig holds SSL manager configuration
type SSLConfig struct {
	Email              string
	Production         bool
	CertDir            string
	CloudflareAPIToken string
	CloudflareZoneID   string
	PreferDNSChallenge bool
}

// NewSSLManager creates a new SSL manager
func NewSSLManager(config *SSLConfig, store *store.SQLiteStore) (*SSLManager, error) {
	acmeConfig := &acme.Config{
		Email:              config.Email,
		Production:         config.Production,
		CertDir:            config.CertDir,
		CloudflareAPIToken: config.CloudflareAPIToken,
		CloudflareZoneID:   config.CloudflareZoneID,
		PreferDNSChallenge: config.PreferDNSChallenge,
	}

	manager, err := acme.NewManager(acmeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACME manager: %w", err)
	}

	ctx := context.Background()
	if err := manager.Register(ctx); err != nil {
		fmt.Printf("ACME registration note: %v\n", err)
	}

	if config.CloudflareAPIToken != "" && config.PreferDNSChallenge {
		if err := manager.SetupDNSChallenge(); err != nil {
			fmt.Printf("DNS challenge setup note: %v\n", err)
		}
	}

	return &SSLManager{
		acme:   manager,
		store:  store,
		config: config,
	}, nil
}

// IssueCertificate issues a new SSL certificate
func (m *SSLManager) IssueCertificate(domain string, includeWww bool) (*acme.CertificateInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	domains := []string{domain}
	if includeWww && !strings.HasPrefix(domain, "www.") {
		domains = append(domains, "www."+domain)
	}

	ctx := context.Background()
	return m.acme.IssueCertificate(ctx, domains)
}

// RenewCertificate renews an existing certificate
func (m *SSLManager) RenewCertificate(domain string) (*acme.CertificateInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := context.Background()
	return m.acme.RenewCertificate(ctx, domain)
}

// RevokeCertificate revokes and deletes a certificate
func (m *SSLManager) RevokeCertificate(domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := context.Background()
	return m.acme.RevokeCertificate(ctx, domain)
}

// GetCertificateStatus returns the status of a certificate
func (m *SSLManager) GetCertificateStatus(domain string) (*acme.CertificateInfo, error) {
	return m.acme.GetCertificate(domain)
}

// ListCertificates lists all managed certificates
func (m *SSLManager) ListCertificates() ([]*acme.CertificateInfo, error) {
	return m.acme.ListCertificates()
}

// CheckRenewal checks if a certificate needs renewal
func (m *SSLManager) CheckRenewal(domain string) (bool, error) {
	return m.acme.CheckRenewal(domain)
}

// GetCertificatePEM returns the PEM-encoded certificate and key
func (m *SSLManager) GetCertificatePEM(domain string) ([]byte, []byte, error) {
	return m.acme.GetCertificatePEM(domain)
}

// SSLStatusV2 returns comprehensive SSL status using the ACME manager
func (h *Handlers) SSLStatusV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var certs []*acme.CertificateInfo
	var provider string = "acme"

	if h.sslManager != nil {
		var err error
		certs, err = h.sslManager.ListCertificates()
		if err != nil {
			certs = nil
			provider = "certbot"
		}
	}

	if certs == nil {
		certs = readCertbotCertificatesV2()
		provider = "certbot"
	}

	sites, _ := h.store.ListSites()
	certByDomain := make(map[string]*acme.CertificateInfo)
	for _, cert := range certs {
		certByDomain[cert.Domain] = cert
		for _, d := range cert.Domains {
			certByDomain[d] = cert
		}
	}

	managed := []map[string]interface{}{}
	managedDomains := make(map[string]bool)

	for _, site := range sites {
		domain := site.Domain
		if managedDomains[domain] {
			continue
		}
		managedDomains[domain] = true

		if cert, ok := certByDomain[domain]; ok {
			managed = append(managed, map[string]interface{}{
				"domain":          domain,
				"domains":         cert.Domains,
				"hasSSL":          true,
				"expiryDate":      cert.NotAfter.Format("2006-01-02T15:04:05Z"),
				"daysUntilExpiry": cert.DaysUntilExpiry,
				"status":          cert.Status,
				"certPath":        cert.CertPath,
			})
		} else {
			managed = append(managed, map[string]interface{}{
				"domain": domain,
				"hasSSL": false,
				"status": "none",
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sites":    managed,
		"allCerts": certs,
		"meta": map[string]interface{}{
			"provider":     provider,
			"certCount":    len(certs),
			"managedCount": len(managed),
			"features": map[string]bool{
				"dnsChallenge": h.sslManager != nil && h.sslManager.config.CloudflareAPIToken != "",
				"httpChallenge": true,
			},
		},
	})
}

// SSLIssueV2 issues a new SSL certificate using ACME
func (h *Handlers) SSLIssueV2(w http.ResponseWriter, r *http.Request) {
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
		Method     string `json:"method"`
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

	if h.sslManager == nil {
		h.SSLIssue(w, r)
		return
	}

	if body.Email != "" && h.sslManager.config.Email == "" {
		h.sslManager.config.Email = body.Email
	}

	if body.Method == "dns" {
		if err := h.sslManager.acme.SetupDNSChallenge(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"error":  "DNS challenge not available",
				"detail": err.Error(),
			})
			return
		}
	}

	cert, err := h.sslManager.IssueCertificate(body.Domain, body.IncludeWww)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "certificate issuance failed",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  fmt.Sprintf("SSL certificate issued for %s", body.Domain),
		"cert":     cert,
		"provider": "acme",
	})
}

// SSLRenewV2 renews an SSL certificate using ACME
func (h *Handlers) SSLRenewV2(w http.ResponseWriter, r *http.Request) {
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

	if h.sslManager == nil {
		h.SSLRenew(w, r)
		return
	}

	cert, err := h.sslManager.RenewCertificate(body.Domain)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "renewal failed",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  fmt.Sprintf("SSL certificate renewed for %s", body.Domain),
		"cert":     cert,
		"provider": "acme",
	})
}

// SSLDeleteV2 deletes an SSL certificate using ACME
func (h *Handlers) SSLDeleteV2(w http.ResponseWriter, r *http.Request) {
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

	if h.sslManager == nil {
		h.SSLDelete(w, r)
		return
	}

	if err := h.sslManager.RevokeCertificate(body.Domain); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "delete failed",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  fmt.Sprintf("SSL certificate deleted for %s", body.Domain),
		"provider": "acme",
	})
}

// SSLCheckRenewal checks which certificates need renewal and optionally auto-renews
func (h *Handlers) SSLCheckRenewal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.sslManager == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"certificates": []interface{}{},
			"renewed":      []interface{}{},
			"provider":     "certbot",
			"message":      "ACME manager not available, using certbot",
		})
		return
	}

	autoRenew := r.URL.Query().Get("auto") == "true"

	certs, err := h.sslManager.ListCertificates()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	needsRenewal := []map[string]interface{}{}
	renewed := []map[string]interface{}{}
	failed := []map[string]interface{}{}

	for _, cert := range certs {
		if cert.DaysUntilExpiry < 30 {
			info := map[string]interface{}{
				"domain":          cert.Domain,
				"daysUntilExpiry": cert.DaysUntilExpiry,
				"status":          cert.Status,
			}
			needsRenewal = append(needsRenewal, info)

			if autoRenew {
				_, err := h.sslManager.RenewCertificate(cert.Domain)
				if err != nil {
					info["error"] = err.Error()
					failed = append(failed, info)
				} else {
					info["status"] = "renewed"
					renewed = append(renewed, info)
				}
			}
		}
	}

	response := map[string]interface{}{
		"certificates":    certs,
		"needsRenewal":    needsRenewal,
		"renewalCount":    len(needsRenewal),
		"provider":        "acme",
	}

	if autoRenew {
		response["renewed"] = renewed
		response["failed"] = failed
		response["autoRenewed"] = len(renewed)
		response["autoRenewFailed"] = len(failed)
	}

	writeJSON(w, http.StatusOK, response)
}

// readCertbotCertificatesV2 reads certbot certificates (fallback)
func readCertbotCertificatesV2() []*acme.CertificateInfo {
	certs := readCertbotCertificates()
	result := make([]*acme.CertificateInfo, 0, len(certs))

	for _, c := range certs {
		info := &acme.CertificateInfo{
			Domain:  c.Domain,
			Domains: c.Domains,
			Status:  c.Status,
		}
		if c.ExpiryDate != nil {
			info.CertPath = c.CertPath
		}
		if c.DaysUntilExpiry != nil {
			info.DaysUntilExpiry = *c.DaysUntilExpiry
		}
		result = append(result, info)
	}

	return result
}
