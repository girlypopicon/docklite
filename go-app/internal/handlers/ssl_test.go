package handlers

import (
	"os/exec"
	"testing"
	"time"

	"docklite-agent/internal/testhelpers"
)

func TestIsValidDomain(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		expected bool
	}{
		{"simple domain", "example.com", true},
		{"subdomain", "sub.example.com", true},
		{"multiple subdomains", "a.b.c.example.com", true},
		{"domain with numbers", "example123.com", true},
		{"domain with hyphen", "my-example.com", true},
		{"short TLD", "example.co", true},
		{"long TLD", "example.technology", true},
		{"empty string", "", false},
		{"just TLD", "com", false},
		{"starts with hyphen", "-example.com", false},
		{"ends with hyphen", "example-.com", false},
		{"double hyphen", "exa--mple.com", false},
		{"starts with dot", ".example.com", false},
		{"ends with dot", "example.com.", false},
		{"double dot", "example..com", false},
		{"space in domain", "exa mple.com", false},
		{"underscore", "example_test.com", false},
		{"special chars", "example!.com", false},
		{"localhost", "localhost", false},
		{"wildcard", "*.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidDomain(tt.domain)
			if result != tt.expected {
				t.Errorf("isValidDomain(%q) = %v, expected %v", tt.domain, result, tt.expected)
			}
		})
	}
}

func TestSSLStatusConstants(t *testing.T) {
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
		{"expiring at 30 days", 30, "valid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := "valid"
			if tt.days < 0 {
				status = "expired"
			} else if tt.days < 30 {
				status = "expiring"
			}

			if status != tt.expected {
				t.Errorf("Status for %d days = %v, expected %v", tt.days, status, tt.expected)
			}
		})
	}
}

func TestParseCertFile(t *testing.T) {
	t.Run("handles non-existent file", func(t *testing.T) {
		info := parseCertFile("nonexistent", "/path/to/nonexistent.pem")
		testhelpers.AssertEqual(t, "nonexistent", info.Domain)
		testhelpers.AssertTrue(t, info.HasSSL, "HasSSL should be true even for unreadable certs")
	})

	t.Run("handles invalid PEM", func(t *testing.T) {
		content := "not a valid PEM file"
		filePath := testhelpers.TempFile(t, content)

		info := parseCertFile("invalid", filePath)
		testhelpers.AssertEqual(t, "invalid", info.Domain)
		testhelpers.AssertTrue(t, info.HasSSL, "HasSSL should be true")
		testhelpers.AssertEqual(t, "unknown", info.Status)
	})
}

func TestParseLiveDirectoryListing(t *testing.T) {
	t.Run("parses directory listing", func(t *testing.T) {
		listing := "example.com\ntest.com\nREADME\n\n  spaced.com  \n"
		certs := parseLiveDirectoryListing(listing)

		if len(certs) != 3 {
			t.Errorf("Expected 3 certs, got %d", len(certs))
		}

		domains := make(map[string]bool)
		for _, cert := range certs {
			domains[cert.Domain] = true
		}

		testhelpers.AssertTrue(t, domains["example.com"], "Should have example.com")
		testhelpers.AssertTrue(t, domains["test.com"], "Should have test.com")
		testhelpers.AssertTrue(t, domains["spaced.com"], "Should have spaced.com (trimmed)")
	})

	t.Run("handles empty listing", func(t *testing.T) {
		certs := parseLiveDirectoryListing("")
		testhelpers.AssertEqual(t, 0, len(certs))
	})

	t.Run("excludes README", func(t *testing.T) {
		certs := parseLiveDirectoryListing("README\nexample.com\nREADME")
		testhelpers.AssertEqual(t, 1, len(certs))
		testhelpers.AssertEqual(t, "example.com", certs[0].Domain)
	})
}

func TestReadFileWithSudo(t *testing.T) {
	t.Run("reads existing file", func(t *testing.T) {
		// Skip if sudo requires a password (not available in CI/test environments)
		if out, err := exec.Command("sudo", "-n", "true").CombinedOutput(); err != nil {
			t.Skipf("sudo not available without password (%s), skipping", string(out))
		}
		content := "test content for sudo read"
		filePath := testhelpers.TempFile(t, content)

		result := readFileWithSudo(filePath)
		if string(result) != content {
			t.Errorf("Expected %q, got %q", content, string(result))
		}
	})

	t.Run("returns nil for non-existent file", func(t *testing.T) {
		result := readFileWithSudo("/nonexistent/path/to/file")
		testhelpers.AssertEqual(t, []byte(nil), result)
	})
}

func TestSSLCertInfo_Structure(t *testing.T) {
	t.Run("has required fields", func(t *testing.T) {
		expiry := "2025-12-31T23:59:59Z"
		days := 365

		info := sslCertInfo{
			Domain:          "example.com",
			Domains:         []string{"example.com", "www.example.com"},
			HasSSL:          true,
			ExpiryDate:      &expiry,
			DaysUntilExpiry: &days,
			Status:          "valid",
			CertPath:        "/etc/letsencrypt/live/example.com/fullchain.pem",
		}

		testhelpers.AssertEqual(t, "example.com", info.Domain)
		testhelpers.AssertEqual(t, 2, len(info.Domains))
		testhelpers.AssertTrue(t, info.HasSSL, "HasSSL should be true")
		testhelpers.AssertEqual(t, "valid", info.Status)
	})
}

func TestDaysUntilExpiry(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		expiry  time.Time
		minDays int
		maxDays int
	}{
		{"expires in 90 days", now.Add(90 * 24 * time.Hour), 89, 91},
		{"expires in 30 days", now.Add(30 * 24 * time.Hour), 29, 31},
		{"expires tomorrow", now.Add(24 * time.Hour), 0, 2},
		{"already expired", now.Add(-24 * time.Hour), -2, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days := int(time.Until(tt.expiry).Hours() / 24)
			if days < tt.minDays || days > tt.maxDays {
				t.Errorf("Days until expiry = %d, expected between %d and %d", days, tt.minDays, tt.maxDays)
			}
		})
	}
}

func BenchmarkIsValidDomain(b *testing.B) {
	domains := []string{
		"example.com",
		"subdomain.example.com",
		"invalid-domain-",
		"-invalid-domain",
		"valid-domain-with-hyphens.example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, d := range domains {
			isValidDomain(d)
		}
	}
}
