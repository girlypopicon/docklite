package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// CertProblem is one thing wrong between a site's certificate and its nginx file, in plain words.
type CertProblem struct {
	Domain string `json:"domain"`
	Kind   string `json:"kind"` // no_https_block | no_renewal | missing_cert_files
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
}

var (
	sslCertLine  = regexp.MustCompile(`\bssl_certificate\s+([^;\s]+);`)
	liveCertPath = regexp.MustCompile(`^/etc/letsencrypt/live/([^/]+)/`)
	serverNameRe = regexp.MustCompile(`(?m)^\s*server_name\s+([^;]+);`)
)

// certProblems compares each nginx vhost with the certificates on disk.
//   - a certificate for the domain exists but the vhost has no HTTPS block (the site serves plain HTTP);
//   - the vhost uses a Let's Encrypt certificate that has no renewal config (it will quietly expire);
//   - the vhost names certificate files that are not there (nginx would refuse to load).
//
// vhosts maps domain -> file text; certExists and renewalExists ask the root helper.
func certProblems(vhosts map[string]string, certExists, renewalExists func(name string) bool) []CertProblem {
	var out []CertProblem
	domains := make([]string, 0, len(vhosts))
	for d := range vhosts {
		domains = append(domains, d)
	}
	sort.Strings(domains)
	for _, d := range domains {
		cfg := vhosts[d]
		if !nginxConfigHasSSL(cfg) {
			if certExists(d) {
				out = append(out, CertProblem{d, "no_https_block", "a certificate exists but the site's nginx file has no HTTPS (port 443) block, so visitors get plain HTTP",
					"docklite ssl issue " + d + "   (or press Issue on the site's HTTPS card)"})
			}
			continue
		}
		for _, m := range sslCertLine.FindAllStringSubmatch(cfg, -1) {
			if lm := liveCertPath.FindStringSubmatch(m[1]); lm != nil {
				name := lm[1]
				if !certExists(name) {
					out = append(out, CertProblem{d, "missing_cert_files", "the nginx file uses " + m[1] + " but that certificate is not on this server",
						"docklite ssl issue " + d})
				} else if !renewalExists(name) {
					out = append(out, CertProblem{d, "no_renewal", "the certificate " + name + " has no renewal config, so it will expire and not renew itself",
						"docklite ssl issue " + d + "   (issues it again with renewal set up)"})
				}
				break
			}
		}
	}
	return out
}

func readVhosts() map[string]string {
	vhosts := map[string]string{}
	files, _ := filepath.Glob(filepath.Join(nginxSitesAvailable, "*.conf"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		cfg := string(b)
		m := serverNameRe.FindStringSubmatch(cfg)
		if m == nil {
			continue
		}
		name := strings.Fields(m[1])[0]
		if !isValidDomain(name) || strings.HasPrefix(name, "_") {
			continue
		}
		vhosts[name] = cfg
	}
	return vhosts
}

// SSLChecks lists certificate/nginx mismatches (admin only). Used by `docklite doctor` and the SSL card.
func (h *Handlers) SSLChecks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	helper := func(cmd string) func(string) bool {
		return func(name string) bool {
			if !isValidDomain(name) {
				return false
			}
			_, err := runRootHelper(nil, cmd, name)
			return err == nil
		}
	}
	problems := certProblems(readVhosts(), helper("cert-exists"), helper("cert-renewal-exists"))
	if problems == nil {
		problems = []CertProblem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"problems": problems})
}
