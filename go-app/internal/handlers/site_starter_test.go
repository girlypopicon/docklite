package handlers

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestStarterPagesAreSafeToShowTheWorld(t *testing.T) {
	const sitePath = "/var/www/sites/alice/shop.example.com"
	pages := map[string]string{
		"static": staticTemplate(sitePath, "shop.example.com"),
		"php":    phpTemplate(sitePath, "shop.example.com"),
		"node":   nodeTemplate(sitePath, "shop.example.com"),
	}
	for kind, page := range pages {
		if !strings.Contains(page, "shop.example.com") {
			t.Errorf("%s: the page should name the site", kind)
		}
		if strings.Contains(page, "/var/www") || strings.Contains(page, "sites/alice") {
			t.Errorf("%s: a public page must not print server paths", kind)
		}
	}
	// nothing is loaded from anywhere else: the only external address is the footer link
	external := regexp.MustCompile(`(?:src|href)=["']https?://([^/"']+)`)
	for kind, page := range pages {
		for _, m := range external.FindAllStringSubmatch(page, -1) {
			if m[1] != "docklite.net" {
				t.Errorf("%s: loads something from %s", kind, m[1])
			}
		}
		if strings.Contains(page, "<script") || strings.Contains(page, "@import") || strings.Contains(page, "url(http") {
			t.Errorf("%s: should be one self-contained page with no scripts or remote assets", kind)
		}
	}
	if !strings.Contains(pages["php"], "PHP_VERSION") {
		t.Error("the PHP page should show that PHP is running")
	}
}

func TestStarterPageEscapesTheDomain(t *testing.T) {
	evil := `"><script>alert(1)</script>`
	for kind, page := range map[string]string{"static": staticTemplate("", evil), "php": phpTemplate("", evil), "node": nodeTemplate("", evil)} {
		if strings.Contains(page, "<script>alert(1)") {
			t.Errorf("%s: the domain must be escaped", kind)
		}
	}
}

func TestStarterNodeServerIsValidJavaScript(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node isn't installed here")
	}
	// including a domain with characters that would break a naive template
	js := filepath.Join(t.TempDir(), "index.js")
	if err := os.WriteFile(js, []byte(nodeTemplate("/x", "it's`a${b}.example.com")), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, "--check", js).CombinedOutput(); err != nil {
		t.Fatalf("the generated server isn't valid JavaScript: %v\n%s", err, out)
	}
}
