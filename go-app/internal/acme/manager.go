package acme

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/registration"
)

// Config holds the ACME manager configuration
type Config struct {
	Email              string
	Production         bool
	CertDir            string
	CloudflareAPIToken string
	CloudflareZoneID   string
	HTTPChallengePort  int
	PreferDNSChallenge bool
}

// Manager handles ACME certificate operations
type Manager struct {
	config      *Config
	client      *lego.Client
	user        *acmeUser
	mu          sync.RWMutex
	store       *FileStore
	dnsProvider challenge.Provider
}

// acmeUser implements registration.User
type acmeUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string                       { return u.Email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.Registration }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey       { return u.key }

// CertificateInfo contains information about a certificate
type CertificateInfo struct {
	Domain          string    `json:"domain"`
	Domains         []string  `json:"domains"`
	NotBefore       time.Time `json:"notBefore"`
	NotAfter        time.Time `json:"notAfter"`
	DaysUntilExpiry int       `json:"daysUntilExpiry"`
	Status          string    `json:"status"`
	CertPath        string    `json:"certPath"`
	KeyPath         string    `json:"keyPath"`
	CertPEM         []byte    `json:"-"`
	KeyPEM          []byte    `json:"-"`
}

// FileStore implements certificate storage using the filesystem
type FileStore struct {
	baseDir string
}

// NewFileStore creates a new file-based certificate store
func NewFileStore(baseDir string) (*FileStore, error) {
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create cert directory: %w", err)
	}
	return &FileStore{baseDir: baseDir}, nil
}

func (s *FileStore) certPath(domain string) string { return filepath.Join(s.baseDir, domain+".crt") }
func (s *FileStore) keyPath(domain string) string  { return filepath.Join(s.baseDir, domain+".key") }

// GetCertificate retrieves a certificate by domain
func (s *FileStore) GetCertificate(domain string) ([]byte, []byte, error) {
	cert, err := os.ReadFile(s.certPath(domain))
	if err != nil {
		return nil, nil, fmt.Errorf("certificate not found: %w", err)
	}
	key, err := os.ReadFile(s.keyPath(domain))
	if err != nil {
		return nil, nil, fmt.Errorf("key not found: %w", err)
	}
	return cert, key, nil
}

// StoreCertificate stores a certificate and key
func (s *FileStore) StoreCertificate(domain string, cert, key []byte) error {
	if err := os.WriteFile(s.certPath(domain), cert, 0644); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}
	if err := os.WriteFile(s.keyPath(domain), key, 0600); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}
	return nil
}

// DeleteCertificate removes a certificate
func (s *FileStore) DeleteCertificate(domain string) error {
	err1 := os.Remove(s.certPath(domain))
	err2 := os.Remove(s.keyPath(domain))
	if err1 != nil && !os.IsNotExist(err1) {
		return err1
	}
	if err2 != nil && !os.IsNotExist(err2) {
		return err2
	}
	return nil
}

// ListCertificates lists all managed certificates
func (s *FileStore) ListCertificates() ([]*CertificateInfo, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var certs []*CertificateInfo
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".crt") {
			continue
		}
		domain := strings.TrimSuffix(entry.Name(), ".crt")
		info, err := s.getCertInfo(domain)
		if err != nil {
			continue
		}
		certs = append(certs, info)
	}
	return certs, nil
}

func (s *FileStore) getCertInfo(domain string) (*CertificateInfo, error) {
	certPEM, keyPEM, err := s.GetCertificate(domain)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	days := int(time.Until(cert.NotAfter).Hours() / 24)
	status := "valid"
	if days < 0 {
		status = "expired"
	} else if days < 30 {
		status = "expiring"
	}

	return &CertificateInfo{
		Domain:          domain,
		Domains:         cert.DNSNames,
		NotBefore:       cert.NotBefore,
		NotAfter:        cert.NotAfter,
		DaysUntilExpiry: days,
		Status:          status,
		CertPath:        s.certPath(domain),
		KeyPath:         s.keyPath(domain),
		CertPEM:         certPEM,
		KeyPEM:          keyPEM,
	}, nil
}

// NewManager creates a new ACME manager
func NewManager(config *Config) (*Manager, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if config.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if config.CertDir == "" {
		config.CertDir = "/var/lib/docklite/certs"
	}
	if config.HTTPChallengePort == 0 {
		config.HTTPChallengePort = 80
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate account key: %w", err)
	}

	user := &acmeUser{
		Email: config.Email,
		key:   privateKey,
	}

	legoConfig := lego.NewConfig(user)
	if config.Production {
		legoConfig.CADirURL = lego.LEDirectoryProduction
	} else {
		legoConfig.CADirURL = lego.LEDirectoryStaging
	}

	client, err := lego.NewClient(legoConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACME client: %w", err)
	}

	store, err := NewFileStore(config.CertDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate store: %w", err)
	}

	var dnsProvider challenge.Provider
	if config.CloudflareAPIToken != "" {
		dnsProvider, err = cloudflare.NewDNSProviderConfig(&cloudflare.Config{
			AuthToken: config.CloudflareAPIToken,
			ZoneToken: config.CloudflareAPIToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Cloudflare DNS provider: %w", err)
		}
	}

	return &Manager{
		config:      config,
		client:      client,
		user:        user,
		store:       store,
		dnsProvider: dnsProvider,
	}, nil
}

// Register registers a new ACME account
func (m *Manager) Register(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	reg, err := m.client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "existing") {
			return nil
		}
		return fmt.Errorf("failed to register account: %w", err)
	}
	m.user.Registration = reg
	return nil
}

// SetupHTTPChallenge sets up HTTP-01 challenge on the specified port
func (m *Manager) SetupHTTPChallenge(port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.client.Challenge.SetHTTP01Provider(NewHTTPProvider(port))
	return nil
}

// SetupDNSChallenge sets up DNS-01 challenge with Cloudflare
func (m *Manager) SetupDNSChallenge() error {
	if m.dnsProvider == nil {
		return fmt.Errorf("DNS provider not configured (Cloudflare API token required)")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.client.Challenge.SetDNS01Provider(m.dnsProvider)
	return nil
}

// IssueCertificate issues a new certificate for the given domains
func (m *Manager) IssueCertificate(ctx context.Context, domains []string) (*CertificateInfo, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("at least one domain is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate certificate key: %w", err)
	}

	request := certificate.ObtainRequest{
		Domains:    domains,
		Bundle:     true,
		PrivateKey: privateKey,
	}

	certificates, err := m.client.Certificate.Obtain(request)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain certificate: %w", err)
	}

	domain := domains[0]
	if err := m.store.StoreCertificate(domain, certificates.Certificate, certificates.PrivateKey); err != nil {
		return nil, fmt.Errorf("failed to store certificate: %w", err)
	}

	return m.store.getCertInfo(domain)
}

// RenewCertificate renews an existing certificate
func (m *Manager) RenewCertificate(ctx context.Context, domain string) (*CertificateInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	certPEM, keyPEM, err := m.store.GetCertificate(domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing certificate: %w", err)
	}

	certResource := &certificate.Resource{
		Domain:      domain,
		Certificate: certPEM,
		PrivateKey:  keyPEM,
	}

	newCert, err := m.client.Certificate.Renew(*certResource, true, false, "")
	if err != nil {
		return nil, fmt.Errorf("failed to renew certificate: %w", err)
	}

	if err := m.store.StoreCertificate(domain, newCert.Certificate, newCert.PrivateKey); err != nil {
		return nil, fmt.Errorf("failed to store renewed certificate: %w", err)
	}

	return m.store.getCertInfo(domain)
}

// RevokeCertificate revokes a certificate
func (m *Manager) RevokeCertificate(ctx context.Context, domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	certPEM, _, err := m.store.GetCertificate(domain)
	if err != nil {
		return fmt.Errorf("failed to get certificate: %w", err)
	}

	if err := m.client.Certificate.Revoke(certPEM); err != nil {
		fmt.Printf("Warning: revocation failed: %v\n", err)
	}

	return m.store.DeleteCertificate(domain)
}

// GetCertificate retrieves certificate information
func (m *Manager) GetCertificate(domain string) (*CertificateInfo, error) {
	return m.store.getCertInfo(domain)
}

// ListCertificates lists all managed certificates
func (m *Manager) ListCertificates() ([]*CertificateInfo, error) {
	return m.store.ListCertificates()
}

// CheckRenewal checks if a certificate needs renewal
func (m *Manager) CheckRenewal(domain string) (bool, error) {
	info, err := m.store.getCertInfo(domain)
	if err != nil {
		return false, err
	}
	return info.DaysUntilExpiry < 30, nil
}

// GetCertificatePEM returns the PEM-encoded certificate and key
func (m *Manager) GetCertificatePEM(domain string) ([]byte, []byte, error) {
	return m.store.GetCertificate(domain)
}

// HTTPProvider implements challenge.Provider for HTTP-01 challenge
type HTTPProvider struct {
	port   int
	tokens map[string][]byte
	mu     sync.RWMutex
}

// NewHTTPProvider creates a new HTTP provider
func NewHTTPProvider(port int) *HTTPProvider {
	return &HTTPProvider{
		port:   port,
		tokens: make(map[string][]byte),
	}
}

// Present presents the challenge token
func (p *HTTPProvider) Present(domain, token, keyAuth string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tokens[token] = []byte(keyAuth)
	return nil
}

// CleanUp removes the challenge token
func (p *HTTPProvider) CleanUp(domain, token, keyAuth string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.tokens, token)
	return nil
}

// GetToken returns the key authorization for a token
func (p *HTTPProvider) GetToken(token string) ([]byte, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	val, ok := p.tokens[token]
	return val, ok
}
