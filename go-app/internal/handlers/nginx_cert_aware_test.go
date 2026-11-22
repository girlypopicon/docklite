package handlers

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// A domain with a certificate on disk but no HTTPS in its vhost (after a transfer or restore) must get an HTTPS vhost
// built from that certificate, and a domain without one must get the plain template.
func TestSetupNginxUsesExistingCertificate(t *testing.T) {
	for _, hasCert := range []bool{true, false} {
		var written string
		rootHelperHook = func(stdin io.Reader, args ...string) ([]byte, error) {
			switch args[0] {
			case "cert-exists":
				if hasCert {
					return nil, nil
				}
				return nil, errors.New("exit 1")
			case "site-write":
				b, _ := io.ReadAll(stdin)
				written = string(b)
			}
			return nil, nil
		}
		if err := setupNginxForDomain("shop.example.com", true, 32001); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(written, "listen 443"); got != hasCert {
			t.Fatalf("hasCert=%v but 443 block present=%v:\n%s", hasCert, got, written)
		}
		if hasCert && !strings.Contains(written, "/etc/letsencrypt/live/shop.example.com/fullchain.pem") {
			t.Fatal("must point at the existing certificate")
		}
		if !strings.Contains(written, "proxy_pass http://127.0.0.1:32001") {
			t.Fatal("must proxy to the site's port")
		}
	}
	rootHelperHook = nil
}
