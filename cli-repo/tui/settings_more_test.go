package tui

import (
	"encoding/json"
	"strings"
	"testing"
)

func sslFixture(t *testing.T, js string) *sslSites {
	t.Helper()
	s := &sslSites{}
	if err := json.Unmarshal([]byte(js), s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestFailedRenewalShowsOnNetworkAndNeedsAttention(t *testing.T) {
	m := newTestModel()
	m.x.netLoaded = true
	m.x.ssl = sslFixture(t, `{"sites":[{"domain":"a.example.com","hasSSL":true,"daysUntilExpiry":9,"autoRenew":true,"renewal":{"ok":false,"error":"dns problem"}}],"meta":{"autoRenewAll":true}}`)
	net := m.renderNetworkScreen()
	if !strings.Contains(net, "auto-renew on") || !strings.Contains(net, "last renewal failed: dns problem") {
		t.Fatalf("network screen:\n%s", net)
	}
	dash := strings.Join(m.dashboardExtras(), "\n")
	if !strings.Contains(dash, "Renewal failed for a.example.com, expires in 9 days.") {
		t.Fatalf("needs attention:\n%s", dash)
	}
}

func TestSettingsShowsAutoRenewAndLinkedDomain(t *testing.T) {
	m := newTestModel()
	m.currentScreen = ScreenSettings
	m.x.ssl = sslFixture(t, `{"sites":[],"meta":{"autoRenewAll":false}}`)
	m.x.panel = &panelDomain{Domain: "smoll.docklite.net", Enabled: true, URL: "https://smoll.docklite.net", FallbackURL: "http://1.2.3.4:3000"}
	out := m.renderSettings()
	for _, want := range []string{"SSL auto-renew", "none (Enter for all)", "Linked domain", "smoll.docklite.net (on)"} {
		if !strings.Contains(out, want) {
			t.Errorf("settings missing %q:\n%s", want, out)
		}
	}
	m.x.setView = 4
	view := m.renderSettings()
	if !strings.Contains(view, "https://smoll.docklite.net") || !strings.Contains(view, "http://1.2.3.4:3000") {
		t.Fatalf("linked domain view:\n%s", view)
	}
}

func TestLinkedDomainEditTypesAndCancels(t *testing.T) {
	m := newTestModel()
	m.currentScreen = ScreenSettings
	m.x.setView = 4
	m.x.panel = &panelDomain{}
	next, _ := m.handlePanelInput(runeKey('c'))
	m = next.(Model)
	for _, r := range "a.io" {
		next, _ = m.handlePanelInput(runeKey(r))
		m = next.(Model)
	}
	if m.x.panelText != "a.io" || !m.x.panelEdit {
		t.Fatalf("typed %q edit=%v", m.x.panelText, m.x.panelEdit)
	}
	next, _ = m.handlePanelInput(keyEsc())
	if next.(Model).x.panelEdit {
		t.Fatal("esc should cancel typing")
	}
}
