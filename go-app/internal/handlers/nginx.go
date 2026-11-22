package handlers

import (
	"docklite-agent/internal/demo"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
)

const (
	nginxSitesAvailable = "/etc/nginx/sites-available"
	nginxSitesEnabled   = "/etc/nginx/sites-enabled"
)

var domainSanitizeRegex = regexp.MustCompile(`[^a-zA-Z0-9.\-]`)

func sanitizeNginxFilename(domain string) string {
	// nginx.conf's `include sites-enabled/*.conf;` only picks up files
	// ending in .conf — without it, every site this writes is silently
	// never loaded, even though the file itself is created successfully.
	return domainSanitizeRegex.ReplaceAllString(strings.ToLower(domain), "") + ".conf"
}

func nginxVhostConfig(domain string, includeWww bool, upstreamPort int) string {
	serverNames := domain
	if includeWww && !strings.HasPrefix(domain, "www.") {
		serverNames = domain + " www." + domain
	}
	return fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;
    %s

    client_max_body_size 100M;

    location / {
        proxy_pass http://127.0.0.1:%d;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
        proxy_buffering off;
    }
}
`, serverNames, downIncludeLine, upstreamPort)
}

// nginxVhostConfigTLS is the vhost for a domain that already has a certificate on disk (a transfer, a restored
// backup, a re-created container): port 80 sends visitors to HTTPS (except certificate checks), and a 443 block
// uses the existing files. certDir is the folder holding fullchain.pem and privkey.pem.
func nginxVhostConfigTLS(domain string, includeWww bool, upstreamPort int, certDir string) string {
	serverNames := domain
	if includeWww && !strings.HasPrefix(domain, "www.") {
		serverNames = domain + " www." + domain
	}
	return fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %[1]s;
    %[2]s

    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }
    location / {
        return 301 https://$host$request_uri;
    }
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name %[1]s;
    %[2]s

    ssl_certificate %[4]s/fullchain.pem;
    ssl_certificate_key %[4]s/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_session_cache shared:SSL:10m;

    client_max_body_size 100M;

    location / {
        proxy_pass http://127.0.0.1:%[3]d;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
        proxy_buffering off;
    }
}
`, serverNames, downIncludeLine, upstreamPort, certDir)
}

// rootHelper is the only command DockLite runs through sudo; see
// docklite-helper at the repo root and setup_sudoers in install.sh.
const rootHelper = "/usr/local/sbin/docklite-helper"

// rootHelperHook lets tests stand in for the root helper (never set outside tests).
var rootHelperHook func(stdin io.Reader, args ...string) ([]byte, error)

func runRootHelper(stdin io.Reader, args ...string) ([]byte, error) {
	if rootHelperHook != nil {
		return rootHelperHook(stdin, args...)
	}
	if demo.On {
		return demoRootHelper(stdin, args...)
	}
	cmd := exec.Command("sudo", append([]string{"-n", rootHelper}, args...)...)
	cmd.Stdin = stdin
	return cmd.CombinedOutput()
}

func writeNginxSiteConfig(domain string, content string) error {
	output, err := runRootHelper(strings.NewReader(content), "site-write", sanitizeNginxFilename(domain))
	if err != nil {
		return fmt.Errorf("failed to write nginx config: %s: %w", string(output), err)
	}
	return nil
}

func enableNginxSite(domain string) error {
	output, err := runRootHelper(nil, "site-enable", sanitizeNginxFilename(domain))
	if err != nil {
		return fmt.Errorf("failed to enable nginx site: %s: %w", string(output), err)
	}
	return nil
}

func removeNginxSiteConfig(domain string) error {
	_, _ = runRootHelper(nil, "site-remove", sanitizeNginxFilename(domain))
	return nil
}

func testNginxConfig() error {
	output, err := runRootHelper(nil, "nginx-test")
	if err != nil {
		return fmt.Errorf("nginx config test failed: %s", string(output))
	}
	return nil
}

func reloadNginx() error {
	if err := testNginxConfig(); err != nil {
		return err
	}
	output, err := runRootHelper(nil, "nginx-reload")
	if err != nil {
		return fmt.Errorf("nginx reload failed: %s", string(output))
	}
	return nil
}

var proxyPassRegex = regexp.MustCompile(`(proxy_pass\s+http://(?:127\.0\.0\.1|localhost):)\d+`)

// setProxyPort returns cfg with every local proxy_pass pointing at port. found is false when the config has no local
// proxy_pass at all (then there is nothing to update and cfg comes back unchanged).
func setProxyPort(cfg string, port int) (out string, found bool) {
	if !proxyPassRegex.MatchString(cfg) {
		return cfg, false
	}
	return proxyPassRegex.ReplaceAllString(cfg, fmt.Sprintf("${1}%d", port)), true
}

// nginxConfigHasSSL reports whether a vhost already serves HTTPS. certbot --nginx writes the 443 server block and the
// certificate lines into the site's own file, so that file must never be rewritten from the plain template.
func nginxConfigHasSSL(cfg string) bool {
	return strings.Contains(cfg, "ssl_certificate") || nginxListen443.MatchString(cfg)
}

var nginxListen443 = regexp.MustCompile(`(?m)^\s*listen\s+(?:\[::\]:)?443\b`)

// updateNginxProxyPort points the site's existing vhost at newPort, keeping everything else in the file (the SSL
// block above all). A vhost that already names newPort is left alone and counts as success.
func updateNginxProxyPort(domain string, newPort int) error {
	existing, err := readNginxSiteConfig(domain)
	if err != nil || existing == "" {
		return fmt.Errorf("no existing nginx config for %s", domain)
	}
	updated, found := setProxyPort(existing, newPort)
	if !found {
		return fmt.Errorf("could not find proxy_pass to update in nginx config")
	}
	if updated == existing {
		return nil
	}
	if err := writeNginxSiteConfig(domain, updated); err != nil {
		return err
	}
	return reloadNginx()
}

func setupNginxForDomain(domain string, includeWww bool, hostPort int) error {
	if !isValidDomain(domain) {
		return fmt.Errorf("invalid domain %q", domain)
	}
	config := nginxVhostConfig(domain, includeWww, hostPort)
	// A vhost that already carries HTTPS is kept and only re-pointed; the template would drop its SSL block.
	if existing, err := readNginxSiteConfig(domain); err == nil && nginxConfigHasSSL(existing) {
		if updated, found := setProxyPort(existing, hostPort); found {
			config = updated
		}
	}
	// No HTTPS in the vhost but a certificate on disk (after a transfer or restore): use it, so HTTPS isn't lost.
	if !nginxConfigHasSSL(config) {
		if _, err := runRootHelper(nil, "cert-exists", domain); err == nil {
			config = nginxVhostConfigTLS(domain, includeWww, hostPort, "/etc/letsencrypt/live/"+domain)
		}
	}
	if err := writeNginxSiteConfig(domain, config); err != nil {
		return err
	}
	if err := enableNginxSite(domain); err != nil {
		return err
	}
	return reloadNginx()
}
