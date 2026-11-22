package handlers

import (
	"fmt"
	"regexp"
	"strings"
)

// nginx's master process runs as root, so a config an admin writes is carried out as root: `access_log /etc/cron.d/x`
// writes a root-owned file with request-controlled content, `include` and `load_module` pull in files and code, and
// `*_temp_path` and `*_cache_path` create directories anywhere. DockLite's own configs need none of that, so the editor
// accepts a config only if every directive in it is on a short list of harmless ones or points at an allowed place.

var nginxStatementSplit = regexp.MustCompile(`[;{}]`)

// nginxPathAllowed reports whether a path argument stays in a place the directive may use.
func nginxPathAllowed(arg string, prefixes ...string) bool {
	arg = strings.Trim(arg, `"'`)
	if strings.Contains(arg, "..") || strings.ContainsAny(arg, "$\x00") && !strings.HasSuffix(arg, "*.conf") {
		return false
	}
	for _, p := range prefixes {
		if strings.HasPrefix(arg, p) {
			return true
		}
	}
	return false
}

// validateNginxConfig refuses configs that would let an editor run or write things outside DockLite's own sites.
func validateNginxConfig(cfg string) error {
	var lines []string
	for _, l := range strings.Split(cfg, "\n") {
		if i := strings.Index(l, "#"); i >= 0 {
			l = l[:i]
		}
		lines = append(lines, l)
	}
	for _, stmt := range nginxStatementSplit.Split(strings.Join(lines, "\n"), -1) {
		fields := strings.Fields(stmt)
		if len(fields) == 0 {
			continue
		}
		name, args := strings.ToLower(fields[0]), fields[1:]
		bad := func(why string) error { return fmt.Errorf("nginx config refused: %q %s", name, why) }
		switch {
		case name == "load_module" || name == "env" || name == "user" || name == "daemon" || name == "pid" || name == "lock_file" || name == "working_directory":
			return bad("is a server-wide setting and can't be set here")
		case strings.HasPrefix(name, "lua_") || strings.Contains(name, "_by_lua") || strings.HasPrefix(name, "perl") || strings.HasPrefix(name, "js_") || name == "njs":
			return bad("runs code and isn't allowed")
		case strings.HasSuffix(name, "_temp_path") || strings.HasSuffix(name, "_cache_path") || name == "client_body_in_file_only":
			return bad("writes files and isn't allowed")
		case name == "include":
			if len(args) != 1 || !(nginxPathAllowed(args[0], "/etc/letsencrypt/") || args[0] == "/etc/nginx/snippets/docklite-down*.conf") {
				return bad("may only include Let's Encrypt's options or DockLite's stopped-site page")
			}
		case name == "access_log" || name == "error_log":
			if len(args) == 0 || !(args[0] == "off" || nginxPathAllowed(args[0], "/var/log/nginx/")) {
				return bad("may only write under /var/log/nginx/ (or be off)")
			}
		case name == "ssl_certificate" || name == "ssl_certificate_key" || name == "ssl_trusted_certificate" || name == "ssl_dhparam" || name == "ssl_client_certificate" || name == "auth_basic_user_file":
			if len(args) == 0 || !nginxPathAllowed(args[0], "/etc/letsencrypt/", "/etc/ssl/cloudflare/", "/etc/nginx/ssl/") {
				return bad("may only point into /etc/letsencrypt/, /etc/ssl/cloudflare/ or /etc/nginx/ssl/")
			}
		case name == "root" || name == "alias":
			if len(args) == 0 || !nginxPathAllowed(args[0], siteBaseDir+"/", "/usr/share/nginx/html", "/var/www/html", "/etc/nginx/docklite/") {
				return bad("may only serve files under " + siteBaseDir)
			}
		case name == "proxy_pass" || name == "fastcgi_pass" || name == "uwsgi_pass" || name == "scgi_pass" || name == "grpc_pass":
			if len(args) > 0 && strings.Contains(strings.ToLower(args[0]), "unix:") {
				return bad("can't use a unix socket")
			}
		}
	}
	return nil
}

// applyNginxConfig writes a site's config, checks that nginx accepts it, and puts the previous config back if it doesn't, so
// a bad edit can't leave nginx unable to reload for everyone.
func applyNginxConfig(domain, config string) error {
	if err := validateNginxConfig(config); err != nil {
		return err
	}
	previous, readErr := readNginxSiteConfig(domain)
	if err := writeNginxSiteConfig(domain, config); err != nil {
		return err
	}
	if err := enableNginxSite(domain); err != nil {
		return err
	}
	if err := reloadNginx(); err != nil {
		if readErr == nil {
			_ = writeNginxSiteConfig(domain, previous)
		} else {
			_ = removeNginxSiteConfig(domain)
		}
		return err
	}
	return nil
}
