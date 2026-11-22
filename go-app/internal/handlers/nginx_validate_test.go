package handlers

import (
	"errors"
	"io"
	"strings"
	"testing"
)

const certbotStyle = `server {
    server_name example.com www.example.com;
    location / { proxy_pass http://127.0.0.1:20001; }
    listen 443 ssl; # managed by Certbot
    ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem; # managed by Certbot
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem; # managed by Certbot
    include /etc/letsencrypt/options-ssl-nginx.conf; # managed by Certbot
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem; # managed by Certbot
}
server {
    if ($host = example.com) { return 301 https://$host$request_uri; } # managed by Certbot
    listen 80;
    server_name example.com;
    return 404;
}`

func TestDockLiteOwnConfigsPassValidation(t *testing.T) {
	for name, cfg := range map[string]string{
		"site":       nginxVhostConfig("example.com", true, 20001),
		"panel":      panelVhostConfig("panel.example.com", 3000),
		"redirect":   forwardVhostConfig("old.com", true, "redirect", "https://new.com", "new.com", 0),
		"proxy":      forwardVhostConfig("old.com", true, "proxy", "", "new.com", 20002),
		"certbot":    certbotStyle,
		"withLogOff": "server { listen 80; server_name a.com; access_log off; error_log /var/log/nginx/a.err warn; root /var/www/sites/u/a.com; }",
	} {
		if err := validateNginxConfig(cfg); err != nil {
			t.Errorf("%s should be accepted: %v", name, err)
		}
	}
}

func TestDangerousNginxDirectivesAreRefused(t *testing.T) {
	for name, directive := range map[string]string{
		"log into cron":     "access_log /etc/cron.d/evil;",
		"error log":         "error_log /root/x;",
		"include any file":  "include /etc/shadow;",
		"include traversal": "include /etc/letsencrypt/../shadow;",
		"load module":       "load_module /tmp/evil.so;",
		"lua":               "content_by_lua_block { os.execute('id') }",
		"temp path":         "client_body_temp_path /etc/x;",
		"cache path":        "proxy_cache_path /etc/y keys_zone=z:1m;",
		"root outside":      "root /etc;",
		"alias outside":     "alias /root/;",
		"root traversal":    "root /var/www/sites/../../etc;",
		"key outside":       "ssl_certificate_key /root/.ssh/id_rsa;",
		"unix socket":       "proxy_pass http://unix:/var/run/docker.sock:/;",
		"user":              "user root;",
		"pid":               "pid /etc/x.pid;",
		"njs":               "js_import main from /tmp/m.js;",
	} {
		cfg := "server { listen 80; server_name a.com; location / { " + directive + " } }"
		if err := validateNginxConfig(cfg); err == nil {
			t.Errorf("%s should be refused: %s", name, directive)
		}
	}
	// a directive hidden after a comment-looking line or on a second line is still seen
	if validateNginxConfig("server {\n listen 80; # fine\n access_log /etc/cron.d/x;\n}") == nil {
		t.Error("a directive on a later line must be checked")
	}
	if validateNginxConfig("server { # access_log /etc/cron.d/x;\n listen 80; }") != nil {
		t.Error("a commented-out directive is not a directive")
	}
}

func TestBadNginxEditIsRolledBack(t *testing.T) {
	var written []string
	rootHelperHook = func(stdin io.Reader, args ...string) ([]byte, error) {
		switch args[0] {
		case "site-write":
			b, _ := io.ReadAll(stdin)
			written = append(written, string(b))
		case "nginx-test":
			return []byte("nginx: configuration file test failed"), errors.New("exit 1")
		}
		return nil, nil
	}
	defer func() { rootHelperHook = nil }()

	good := "server { listen 80; server_name a.com; }"
	err := applyNginxConfig("a.com", good)
	if err == nil {
		t.Fatal("nginx -t failed, so the edit must report an error")
	}
	// no earlier config existed on this machine, so the bad one is removed rather than left behind
	if len(written) != 1 || !strings.Contains(written[0], "a.com") {
		t.Fatalf("written: %v", written)
	}
	if err := applyNginxConfig("a.com", "server { access_log /etc/cron.d/x; }"); err == nil || len(written) != 1 {
		t.Fatalf("a dangerous config must be refused before anything is written (%v, writes %d)", err, len(written))
	}
}
