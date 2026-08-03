package acme

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"docklite-agent/internal/testhelpers"

	"github.com/go-acme/lego/v4/registration"
)

func TestNewFileStore(t *testing.T) {
	t.Run("creates directory if not exists", func(t *testing.T) {
		dir, cleanup := testhelpers.TempDir(t)
		defer cleanup()

		storePath := filepath.Join(dir, "certs")
		store, err := NewFileStore(storePath)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertNotEqual(t, nil, store)

		_, err = os.Stat(storePath)
		testhelpers.AssertNoError(t, err)
	})

	t.Run("uses existing directory", func(t *testing.T) {
		dir, cleanup := testhelpers.TempDir(t)
		defer cleanup()

		store, err := NewFileStore(dir)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertNotEqual(t, nil, store)
	})
}

func TestFileStore_CertificateOperations(t *testing.T) {
	dir, cleanup := testhelpers.TempDir(t)
	defer cleanup()

	store, err := NewFileStore(dir)
	testhelpers.AssertNoError(t, err)

	t.Run("stores and retrieves certificate", func(t *testing.T) {
		domain := "example.com"
		certPEM := []byte("-----BEGIN CERTIFICATE-----\ntest cert\n-----END CERTIFICATE-----")
		keyPEM := []byte("-----BEGIN PRIVATE KEY-----\ntest key\n-----END PRIVATE KEY-----")

		err := store.StoreCertificate(domain, certPEM, keyPEM)
		testhelpers.AssertNoError(t, err)

		retrievedCert, retrievedKey, err := store.GetCertificate(domain)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, string(certPEM), string(retrievedCert))
		testhelpers.AssertEqual(t, string(keyPEM), string(retrievedKey))
	})

	t.Run("returns error for non-existent certificate", func(t *testing.T) {
		_, _, err := store.GetCertificate("nonexistent.com")
		testhelpers.AssertError(t, err)
	})

	t.Run("deletes certificate", func(t *testing.T) {
		domain := "delete-test.com"
		certPEM := []byte("cert")
		keyPEM := []byte("key")

		err := store.StoreCertificate(domain, certPEM, keyPEM)
		testhelpers.AssertNoError(t, err)

		err = store.DeleteCertificate(domain)
		testhelpers.AssertNoError(t, err)

		_, _, err = store.GetCertificate(domain)
		testhelpers.AssertError(t, err)
	})
}

func TestFileStore_CertInfo(t *testing.T) {
	dir, cleanup := testhelpers.TempDir(t)
	defer cleanup()

	store, err := NewFileStore(dir)
	testhelpers.AssertNoError(t, err)

	t.Run("parses valid certificate", func(t *testing.T) {
		domain := "info-test.com"
		certPEM, keyPEM := generateTestCertificate(t, domain)

		err := store.StoreCertificate(domain, certPEM, keyPEM)
		testhelpers.AssertNoError(t, err)

		info, err := store.getCertInfo(domain)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, domain, info.Domain)
		testhelpers.AssertTrue(t, info.DaysUntilExpiry > 0, "Certificate should not be expired")
		testhelpers.AssertEqual(t, "valid", info.Status)
	})

	t.Run("detects expiring certificate", func(t *testing.T) {
		domain := "expiring-test.com"
		certPEM, keyPEM := generateTestCertificateWithExpiry(t, domain, 15*24*time.Hour)

		err := store.StoreCertificate(domain, certPEM, keyPEM)
		testhelpers.AssertNoError(t, err)

		info, err := store.getCertInfo(domain)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, "expiring", info.Status)
	})

	t.Run("detects expired certificate", func(t *testing.T) {
		domain := "expired-test.com"
		certPEM, keyPEM := generateTestCertificateWithExpiry(t, domain, -24*time.Hour)

		err := store.StoreCertificate(domain, certPEM, keyPEM)
		testhelpers.AssertNoError(t, err)

		info, err := store.getCertInfo(domain)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, "expired", info.Status)
	})
}

func TestCertificateInfo_Status(t *testing.T) {
	tests := []struct {
		name     string
		days     int
		expected string
	}{
		{"valid certificate", 90, "valid"},
		{"expiring soon", 29, "expiring"},
		{"expiring tomorrow", 1, "expiring"},
		{"expired yesterday", -1, "expired"},
		{"expired long ago", -100, "expired"},
		{"at threshold", 30, "valid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := "valid"
			if tt.days < 0 {
				status = "expired"
			} else if tt.days < 30 {
				status = "expiring"
			}
			testhelpers.AssertEqual(t, tt.expected, status)
		})
	}
}

func TestNewManager(t *testing.T) {
	t.Run("requires config", func(t *testing.T) {
		_, err := NewManager(nil)
		testhelpers.AssertError(t, err)
		testhelpers.AssertTrue(t, containsString(err.Error(), "config is required"), "Should require config")
	})

	t.Run("requires email", func(t *testing.T) {
		_, err := NewManager(&Config{})
		testhelpers.AssertError(t, err)
		testhelpers.AssertTrue(t, containsString(err.Error(), "email is required"), "Should require email")
	})

	t.Run("creates manager with valid config", func(t *testing.T) {
		dir, cleanup := testhelpers.TempDir(t)
		defer cleanup()

		config := &Config{
			Email:      "test@example.com",
			CertDir:    dir,
			Production: false,
		}

		manager, err := NewManager(config)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertNotEqual(t, nil, manager)
		testhelpers.AssertNotEqual(t, nil, manager.client)
		testhelpers.AssertNotEqual(t, nil, manager.store)
	})

	t.Run("uses default cert directory", func(t *testing.T) {
		config := &Config{
			Email:      "test@example.com",
			Production: false,
		}

		manager, err := NewManager(config)
		testhelpers.AssertNoError(t, err)
		testhelpers.AssertEqual(t, "/var/lib/docklite/certs", config.CertDir)
		_ = manager
	})
}

func TestHTTPProvider(t *testing.T) {
	provider := NewHTTPProvider(80)

	t.Run("presents and cleans up token", func(t *testing.T) {
		domain := "example.com"
		token := "test-token"
		keyAuth := "test-key-auth"

		err := provider.Present(domain, token, keyAuth)
		testhelpers.AssertNoError(t, err)

		data, ok := provider.GetToken(token)
		testhelpers.AssertTrue(t, ok, "Token should exist")
		testhelpers.AssertEqual(t, keyAuth, string(data))

		err = provider.CleanUp(domain, token, keyAuth)
		testhelpers.AssertNoError(t, err)

		_, ok = provider.GetToken(token)
		testhelpers.AssertFalse(t, ok, "Token should be removed")
	})
}

func TestACMEUser(t *testing.T) {
	t.Run("implements registration.User interface", func(t *testing.T) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		testhelpers.AssertNoError(t, err)

		user := &acmeUser{
			Email: "test@example.com",
			key:   key,
		}

		testhelpers.AssertEqual(t, "test@example.com", user.GetEmail())
		testhelpers.AssertEqual(t, key, user.GetPrivateKey())
		testhelpers.AssertEqual(t, (*registration.Resource)(nil), user.GetRegistration())
	})
}

func TestFileStore_PathGeneration(t *testing.T) {
	dir, cleanup := testhelpers.TempDir(t)
	defer cleanup()

	store, err := NewFileStore(dir)
	testhelpers.AssertNoError(t, err)

	t.Run("generates correct cert path", func(t *testing.T) {
		path := store.certPath("example.com")
		expected := filepath.Join(dir, "example.com.crt")
		testhelpers.AssertEqual(t, expected, path)
	})

	t.Run("generates correct key path", func(t *testing.T) {
		path := store.keyPath("example.com")
		expected := filepath.Join(dir, "example.com.key")
		testhelpers.AssertEqual(t, expected, path)
	})
}

func generateTestCertificate(t *testing.T, domain string) ([]byte, []byte) {
	return generateTestCertificateWithExpiry(t, domain, 365*24*time.Hour)
}

func generateTestCertificateWithExpiry(t *testing.T, domain string, duration time.Duration) ([]byte, []byte) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	notBefore := time.Now().Add(-1 * time.Hour)
	notAfter := notBefore.Add(duration)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{domain},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("Failed to marshal key: %v", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})

	return certPEM, keyPEM
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func BenchmarkFileStore_StoreCertificate(b *testing.B) {
	dir, err := os.MkdirTemp("", "bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store, err := NewFileStore(dir)
	if err != nil {
		b.Fatal(err)
	}

	certPEM := []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----")
	keyPEM := []byte("-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		domain := fmt.Sprintf("domain%d.com", i)
		_ = store.StoreCertificate(domain, certPEM, keyPEM)
	}
}
