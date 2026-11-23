package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Where origin certificates live (the nginx editor's allowed list already includes it).
const originCertRoot = "/etc/ssl/cloudflare"

// newOriginCSR makes a fresh private key and a certificate request for domain (and www.domain when asked).
// The key never leaves this server: only the request goes to Cloudflare.
func newOriginCSR(domain string, includeWww bool) (keyPEM, csrPEM string, hostnames []string, err error) {
	hostnames = []string{domain}
	if includeWww && !strings.HasPrefix(domain, "www.") {
		hostnames = append(hostnames, "www."+domain)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", nil, err
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: domain},
		DNSNames: hostnames,
	}, key)
	if err != nil {
		return "", "", nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", "", nil, err
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	csrPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))
	return keyPEM, csrPEM, hostnames, nil
}

var proxyPassPortRe = regexp.MustCompile(`proxy_pass\s+http://127\.0\.0\.1:(\d+)`)
var sslCertKeyPathRe = regexp.MustCompile(`(ssl_certificate(?:_key)?\s+)[^;\s]+;`)

// vhostWithOriginCert returns the site's nginx file using the origin certificate. A file that already serves HTTPS keeps
// every other setting and only has its certificate lines swapped; otherwise the HTTPS template is used.
func vhostWithOriginCert(existing, domain string, includeWww bool, port int) string {
	dir := originCertRoot + "/" + domain
	if nginxConfigHasSSL(existing) && sslCertKeyPathRe.MatchString(existing) {
		return sslCertKeyPathRe.ReplaceAllStringFunc(existing, func(m string) string {
			if strings.HasPrefix(strings.TrimSpace(m), "ssl_certificate_key") {
				return "ssl_certificate_key " + dir + "/privkey.pem;"
			}
			return "ssl_certificate " + dir + "/fullchain.pem;"
		})
	}
	return nginxVhostConfigTLS(domain, includeWww, port, dir)
}

// SSLOrigin issues a Cloudflare origin certificate for a site and switches its nginx file to it (admin only).
// Body: {domain, includeWww, strict}. strict also sets the zone's SSL mode to Full (strict), which is what makes
// Cloudflare trust it, but changes the whole domain, so it is only done when asked.
func (h *Handlers) SSLOrigin(w http.ResponseWriter, r *http.Request) {
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
		Strict     bool   `json:"strict"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	body.Domain = strings.ToLower(strings.TrimSpace(body.Domain))
	if !isValidDomain(body.Domain) {
		writeError(w, http.StatusBadRequest, "invalid domain")
		return
	}
	client := h.cloudflareClient()
	if client == nil {
		writeError(w, http.StatusBadRequest, "Cloudflare isn't connected. Add an API token under Settings → Cloudflare.")
		return
	}
	zones, err := h.store.GetDNSZones()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	zone := zoneForDomain(zones, body.Domain)
	if zone == nil {
		writeError(w, http.StatusBadRequest, body.Domain+" isn't in a domain imported from Cloudflare (Settings → Cloudflare).")
		return
	}
	existing, err := readNginxSiteConfig(body.Domain)
	if err != nil {
		writeError(w, http.StatusBadRequest, "this domain has no nginx file yet: create the site first")
		return
	}
	m := proxyPassPortRe.FindStringSubmatch(existing)
	if m == nil {
		writeError(w, http.StatusBadRequest, "the site's nginx file has no proxy_pass to a local port")
		return
	}
	var port int
	fmt.Sscanf(m[1], "%d", &port)

	keyPEM, csrPEM, hostnames, err := newOriginCSR(body.Domain, body.IncludeWww)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create the certificate request")
		return
	}
	certPEM, err := client.CreateOriginCertificate(hostnames, csrPEM, 5475)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error()+" (the token needs Zone → SSL and Certificates → Edit)")
		return
	}
	// Certificate first, then the key, in one stream: the helper splits it and stores both root-only.
	if _, err := runRootHelper(strings.NewReader(strings.TrimSpace(certPEM)+"\n"+keyPEM), "origin-cert-write", body.Domain); err != nil {
		writeError(w, http.StatusInternalServerError, "could not store the certificate")
		return
	}
	cfg := vhostWithOriginCert(existing, body.Domain, body.IncludeWww, port)
	if err := applyNginxConfig(body.Domain, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "nginx rejected the new file, so it was put back: "+err.Error())
		return
	}
	mode := ""
	if body.Strict {
		if err := client.SetZoneSetting(zone.ZoneID, "ssl", "strict"); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"success": true, "warning": "Certificate installed, but couldn't set Cloudflare's SSL mode to Full (strict): " + err.Error() + ". Set it yourself in Cloudflare → SSL/TLS.", "hostnames": hostnames})
			return
		}
		mode = "strict"
	} else if v, err := client.GetZoneSetting(zone.ZoneID, "ssl"); err == nil {
		mode = v
	}
	warning := ""
	if mode != "" && mode != "strict" {
		warning = "Cloudflare's SSL mode for " + zone.Domain + " is \"" + mode + "\". Set it to Full (strict) in Cloudflare → SSL/TLS, or visitors may see errors or loops."
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "hostnames": hostnames, "sslMode": mode, "warning": warning,
		"message": "Cloudflare origin certificate installed for " + body.Domain + " (valid 15 years; no renewal needed)."})
}

// ---- preflight: will a certificate request reach this server? ----

type probeResult struct {
	Verdict string `json:"verdict"` // reachable | blocked | origin_down | redirect | unknown
	Status  int    `json:"status"`
	Detail  string `json:"detail"`
}

// classifyChallengeProbe reads the answer to a request for http://domain/.well-known/acme-challenge/<random>, the same
// kind of request Let's Encrypt makes. Our own nginx answers 404 for a name it does not have, so a 404 means the
// request got through. Cloudflare's firewall or bot checks answer with a challenge page instead.
func classifyChallengeProbe(status int, header http.Header, body string) probeResult {
	lb := strings.ToLower(body)
	switch {
	case header.Get("cf-mitigated") != "" || strings.Contains(lb, "just a moment") || strings.Contains(lb, "attention required") || strings.Contains(lb, "cf-chl"):
		return probeResult{"blocked", status, "Cloudflare answered with a security challenge, so Let's Encrypt would be turned away. Add a rule that skips security checks for /.well-known/acme-challenge/, or use the Cloudflare origin certificate instead."}
	case status == 403 && strings.Contains(strings.ToLower(header.Get("server")), "cloudflare") && !strings.Contains(lb, "nginx"):
		return probeResult{"blocked", status, "Cloudflare refused the request (403). A firewall rule or bot protection is probably blocking certificate checks."}
	case status >= 520 && status <= 527:
		return probeResult{"origin_down", status, fmt.Sprintf("Cloudflare could not reach this server (error %d). Check the site's container, nginx, and Cloudflare's SSL mode.", status)}
	case status == 404 || status == 200:
		return probeResult{"reachable", status, "The request reached this server."}
	case status >= 300 && status < 400:
		return probeResult{"redirect", status, "The request is redirected (probably to HTTPS). That is fine for Let's Encrypt as long as HTTPS answers."}
	}
	return probeResult{"unknown", status, fmt.Sprintf("Unexpected answer (HTTP %d).", status)}
}

func probeChallenge(domain string) probeResult {
	var rnd [6]byte
	_, _ = rand.Read(rnd[:])
	url := fmt.Sprintf("http://%s/.well-known/acme-challenge/docklite-preflight-%x", domain, rnd)
	c := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get(url)
	if err != nil {
		return probeResult{"unknown", 0, "Could not make the request: " + err.Error()}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	return classifyChallengeProbe(resp.StatusCode, resp.Header, string(b))
}

type preflightResult struct {
	Domain         string       `json:"domain"`
	DNS            string       `json:"dns"` // none | here | cloudflare | elsewhere
	Addresses      []string     `json:"addresses"`
	Probe          *probeResult `json:"probe,omitempty"`
	Zone           string       `json:"zone,omitempty"`
	SSLMode        string       `json:"sslMode,omitempty"`
	Recommendation string       `json:"recommendation"` // letsencrypt | origin | fix_dns | fix_cloudflare
	Message        string       `json:"message"`
}

// recommend turns what was found into a plain-words verdict.
func recommend(dns string, probe *probeResult, sslMode string, haveCloudflare bool) (rec, msg string) {
	switch dns {
	case dnsNone:
		return "fix_dns", "This domain has no address record yet, so no certificate can be issued. Point it at this server first (DNS tab, or the Cloudflare DNS card)."
	case dnsElsewhere:
		return "fix_dns", "This domain points somewhere other than this server, so Let's Encrypt would check the wrong machine. Point its A record at this server first."
	}
	if probe != nil && probe.Verdict == "blocked" {
		if haveCloudflare {
			return "origin", probe.Detail
		}
		return "fix_cloudflare", probe.Detail
	}
	if probe != nil && probe.Verdict == "origin_down" {
		return "fix_cloudflare", probe.Detail
	}
	if dns == dnsCloudflare {
		msg = "The domain goes through Cloudflare and certificate requests get through, so Let's Encrypt will work."
		if sslMode == "flexible" {
			msg += " Note: Cloudflare's SSL mode is Flexible; switch to Full (strict) once the site has a certificate, or visitors may get redirect loops."
		}
		return "letsencrypt", msg
	}
	return "letsencrypt", "The domain points at this server and the request gets through. Let's Encrypt will work."
}

// SSLPreflight checks, before issuing, whether a certificate request for the domain will arrive (admin only).
func (h *Handlers) SSLPreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	domain := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("domain")))
	if !isValidDomain(domain) {
		writeError(w, http.StatusBadRequest, "invalid domain")
		return
	}
	ips := lookupIPv4(domain)
	class := classifyDNS(ips, serverAddresses())
	res := preflightResult{Domain: domain, DNS: class, Addresses: ips}
	if res.Addresses == nil {
		res.Addresses = []string{}
	}
	if class == dnsHere || class == dnsCloudflare {
		p := probeChallenge(domain)
		res.Probe = &p
	}
	haveCF := false
	if client := h.cloudflareClient(); client != nil {
		haveCF = true
		if zones, err := h.store.GetDNSZones(); err == nil {
			if z := zoneForDomain(zones, domain); z != nil {
				res.Zone = z.Domain
				if mode, err := client.GetZoneSetting(z.ZoneID, "ssl"); err == nil {
					res.SSLMode = mode
				}
			} else {
				haveCF = false
			}
		}
	}
	res.Recommendation, res.Message = recommend(class, res.Probe, res.SSLMode, haveCF)
	writeJSON(w, http.StatusOK, res)
}
