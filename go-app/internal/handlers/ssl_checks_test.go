package handlers

import "testing"

func TestCertProblems(t *testing.T) {
	plain := "server { listen 80; server_name a.com; }"
	tls := "server { listen 443 ssl; server_name b.com; ssl_certificate /etc/letsencrypt/live/b.com/fullchain.pem; }"
	origin := "server { listen 443 ssl; server_name c.com; ssl_certificate /etc/ssl/docklite/c.com.pem; }"
	vh := map[string]string{"a.com": plain, "b.com": tls, "c.com": origin, "d.com": plain}
	has := map[string]bool{"a.com": true, "b.com": true}
	ren := map[string]bool{}
	got := certProblems(vh, func(n string) bool { return has[n] }, func(n string) bool { return ren[n] })
	kinds := map[string]string{}
	for _, p := range got {
		kinds[p.Domain] = p.Kind
	}
	if kinds["a.com"] != "no_https_block" || kinds["b.com"] != "no_renewal" {
		t.Fatalf("unexpected: %v", kinds)
	}
	if _, bad := kinds["c.com"]; bad {
		t.Fatal("a non-Let's-Encrypt certificate has no renewal config and must not be flagged")
	}
	if _, bad := kinds["d.com"]; bad {
		t.Fatal("a plain site with no certificate is fine")
	}
	has["b.com"] = false
	got = certProblems(vh, func(n string) bool { return has[n] }, func(n string) bool { return ren[n] })
	for _, p := range got {
		if p.Domain == "b.com" && p.Kind != "missing_cert_files" {
			t.Fatalf("expected missing_cert_files, got %s", p.Kind)
		}
	}
}
