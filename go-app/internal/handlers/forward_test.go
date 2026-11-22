package handlers

import (
	"strings"
	"testing"

	"docklite-agent/internal/store"
)

func TestForwardVhostRedirect(t *testing.T) {
	cfg := forwardVhostConfig("old.com", true, "redirect", "https://new.com/", "new.com", 0)
	for _, want := range []string{"server_name old.com www.old.com;", "return 301 https://new.com$request_uri;"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("missing %q in:\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "proxy_pass") {
		t.Error("a redirect must not proxy")
	}
}

func TestForwardVhostProxyKeepsTheNewAddress(t *testing.T) {
	cfg := forwardVhostConfig("old.com", false, "proxy", "", "new.com", 8123)
	for _, want := range []string{"server_name old.com;", "proxy_pass http://127.0.0.1:8123;", "proxy_set_header Host new.com;", "X-Forwarded-Host $host"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("missing %q in:\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "return 301") {
		t.Error("a proxy must not redirect")
	}
}

func TestForwardsAreRememberedAndListedPerSource(t *testing.T) {
	h := testHandlers(t)
	u, err := h.store.CreateUser("owner", "password-1234", "admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"b.com", "a.com", "other.com"} {
		cid := "c-" + d
		if _, err := h.store.CreateSite(store.SiteRecord{Domain: d, UserID: u.ID, TemplateType: "static", ContainerID: &cid}); err != nil {
			t.Fatal(err)
		}
	}
	h.store.SetSetting(forwardSettingPrefix+"b.com", mustJSON(forwardRecord{To: "main.com", Mode: "proxy"}))
	h.store.SetSetting(forwardSettingPrefix+"a.com", mustJSON(forwardRecord{To: "main.com", Mode: "redirect"}))
	h.store.SetSetting(forwardSettingPrefix+"gone.com", mustJSON(forwardRecord{To: "main.com", Mode: "redirect"})) // site deleted
	h.store.SetSetting(forwardSettingPrefix+"other.com", mustJSON(forwardRecord{To: "else.com", Mode: "redirect"}))

	got := h.forwardsTo("main.com")
	if len(got) != 2 || got[0].Domain != "a.com" || got[1].Domain != "b.com" || got[1].Mode != "proxy" {
		t.Fatalf("got %+v", got)
	}
	if rec, ok := h.forwardOf("B.com"); !ok || rec.To != "main.com" {
		t.Fatalf("forwardOf: %+v %v", rec, ok)
	}
	if _, ok := h.forwardOf("main.com"); ok {
		t.Fatal("the source itself is not a forward")
	}
}
