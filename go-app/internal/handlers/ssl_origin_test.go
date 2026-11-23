package handlers

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"strings"
	"testing"
)

func TestOriginCSR(t *testing.T) {
	key, csr, hosts, err := newOriginCSR("shop.example.com", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 || hosts[1] != "www.shop.example.com" {
		t.Fatalf("hostnames: %v", hosts)
	}
	if !strings.Contains(key, "BEGIN PRIVATE KEY") {
		t.Fatal("key must be PKCS8 PEM")
	}
	blk, _ := pem.Decode([]byte(csr))
	req, err := x509.ParseCertificateRequest(blk.Bytes)
	if err != nil || req.CheckSignature() != nil || len(req.DNSNames) != 2 {
		t.Fatalf("bad CSR: %v %v", err, req)
	}
}

func TestVhostWithOriginCert(t *testing.T) {
	fresh := vhostWithOriginCert(nginxVhostConfig("a.com", true, 32001), "a.com", true, 32001)
	if !strings.Contains(fresh, "/etc/ssl/cloudflare/a.com/fullchain.pem") || !strings.Contains(fresh, "listen 443") {
		t.Fatalf("plain vhost should become HTTPS:\n%s", fresh)
	}
	if err := validateNginxConfig(fresh); err != nil {
		t.Fatalf("the generated file must pass the nginx editor's own checks: %v", err)
	}
	le := nginxVhostConfigTLS("a.com", false, 32001, "/etc/letsencrypt/live/a.com") + "\n# custom\n"
	swapped := vhostWithOriginCert(le, "a.com", false, 32001)
	if strings.Contains(swapped, "/etc/letsencrypt/") || !strings.Contains(swapped, "# custom") ||
		!strings.Contains(swapped, "ssl_certificate_key /etc/ssl/cloudflare/a.com/privkey.pem;") {
		t.Fatalf("certificate lines should be swapped, rest kept:\n%s", swapped)
	}
}

func TestClassifyChallengeProbe(t *testing.T) {
	h := http.Header{}
	h.Set("server", "cloudflare")
	cases := []struct {
		status int
		hdr    map[string]string
		body   string
		want   string
	}{
		{404, nil, "<html>404 Not Found nginx</html>", "reachable"},
		{403, map[string]string{"cf-mitigated": "challenge"}, "", "blocked"},
		{403, nil, "<title>Just a moment...</title>", "blocked"},
		{403, nil, "error code: 1020", "blocked"},
		{521, nil, "Web server is down", "origin_down"},
		{301, nil, "", "redirect"},
		{418, nil, "", "unknown"},
	}
	for _, c := range cases {
		hh := h.Clone()
		for k, v := range c.hdr {
			hh.Set(k, v)
		}
		if got := classifyChallengeProbe(c.status, hh, c.body).Verdict; got != c.want {
			t.Errorf("%d %q: got %s want %s", c.status, c.body, got, c.want)
		}
	}
}

func TestRecommend(t *testing.T) {
	blocked := &probeResult{Verdict: "blocked", Detail: "x"}
	if r, _ := recommend(dnsCloudflare, blocked, "", true); r != "origin" {
		t.Fatalf("blocked with a Cloudflare token should recommend the origin certificate, got %s", r)
	}
	if r, _ := recommend(dnsCloudflare, blocked, "", false); r != "fix_cloudflare" {
		t.Fatalf("got %s", r)
	}
	if r, _ := recommend(dnsElsewhere, nil, "", true); r != "fix_dns" {
		t.Fatalf("got %s", r)
	}
	if r, m := recommend(dnsCloudflare, &probeResult{Verdict: "reachable"}, "flexible", true); r != "letsencrypt" || !strings.Contains(m, "Flexible") {
		t.Fatalf("got %s %q", r, m)
	}
}
