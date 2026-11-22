package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"docklite-tui/api"
)

func newTestModel() Model {
	return Model{width: 130, height: 40, focusArea: FocusMain, client: api.NewClient()}
}

func TestLogsScreenScrollsAndSwitches(t *testing.T) {
	m := newTestModel()
	m.currentScreen = ScreenLogs
	var text []string
	for i := 0; i < 100; i++ {
		text = append(text, "line "+string(rune('a'+i%26)))
	}
	text[99] = "request failed: boom"
	nm, _, _ := m.handleExtraMsg(logsMsg{Target: 0, Text: strings.Join(text, "\n")})
	m = nm
	if m.x.logOffset != m.logMaxOffset() || m.logMaxOffset() == 0 {
		t.Fatalf("a freshly loaded log should show its end: offset %d max %d", m.x.logOffset, m.logMaxOffset())
	}
	out := m.renderLogsScreen()
	if !strings.Contains(out, "boom") || !strings.Contains(out, "of 100") {
		t.Fatalf("log view: %s", out)
	}
	next, _ := m.handleLogsInput(tea.KeyMsg{Type: tea.KeyHome})
	m = next.(Model)
	if m.x.logOffset != 0 {
		t.Fatalf("Home should go to the top")
	}
	next, cmd := m.handleLogsInput(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if m.x.logTarget != 1 || cmd == nil || m.x.logLoaded {
		t.Fatalf("→ should switch to the next log and fetch it")
	}
	// an answer for a log we've left is ignored
	nm, _, _ = m.handleExtraMsg(logsMsg{Target: 0, Text: "stale"})
	if len(nm.x.logLines) != 0 {
		t.Fatalf("stale answer was shown")
	}
	nm, _, _ = nm.handleExtraMsg(logsMsg{Target: 1, Err: &api.APIError{Status: 403}})
	if !strings.Contains(nm.renderLogsScreen(), "needs an admin account") {
		t.Fatalf("403 should read as plain words: %s", nm.renderLogsScreen())
	}
}

func TestNetworkScreenSaysThingsPlainly(t *testing.T) {
	m := newTestModel()
	days := 5
	m.x.netLoaded = true
	m.x.netOv = &netOverview{Hostname: "smoll", PrimaryIPv4: "10.0.0.5", DefaultGateway: "10.0.0.1", DefaultDevice: "eth0"}
	fw := &netFirewall{}
	fw.Firewall.Provider, fw.Firewall.Status = "ufw", "active"
	fw.OpenPorts = append(fw.OpenPorts, struct {
		Proto   string `json:"proto"`
		Port    int    `json:"port"`
		Process string `json:"process"`
		Public  bool   `json:"public"`
	}{"tcp", 443, "nginx", true})
	m.x.netFw = fw
	m.x.ssl = &sslSites{Sites: []struct {
		Domain          string `json:"domain"`
		HasSSL          bool   `json:"hasSSL"`
		DaysUntilExpiry *int   `json:"daysUntilExpiry"`
		Status          string `json:"status"`
	}{{Domain: "a.example.com", HasSSL: true, DaysUntilExpiry: &days}, {Domain: "b.example.com"}}}
	out := m.renderNetworkScreen()
	for _, want := range []string{"smoll", "443 (secure websites)", "active (ufw)", "expires in 5 days", "no HTTPS", "Press d"} {
		if !strings.Contains(out, want) {
			t.Errorf("network screen missing %q:\n%s", want, out)
		}
	}
}

func TestSettingsMenuOpensUpdateAndGoesBack(t *testing.T) {
	m := newTestModel()
	m.currentScreen = ScreenSettings
	next, cmd := m.handleSettingsInput(keyEnter())
	m = next.(Model)
	if m.x.setView != 1 || cmd == nil {
		t.Fatalf("Enter on Update should open it and check for updates")
	}
	next, _ = m.handleSettingsInput(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.x.setView != 0 || !strings.Contains(m.renderSettings(), "Update DockLite") {
		t.Fatalf("Esc should return to the list")
	}
	m.updRunning = true
	m.x.setView = 1
	next, _ = m.handleSettingsInput(tea.KeyMsg{Type: tea.KeyEsc})
	if next.(Model).x.setView != 1 {
		t.Fatalf("can't leave the update screen while it is installing")
	}
}

func TestDashboardShowsWhatNeedsAttention(t *testing.T) {
	m := newTestModel()
	m.currentScreen = ScreenDashboard
	m.summary = &api.SummaryResponse{ContainersRunning: 2}
	m.updStatus = &api.UpdateStatus{Version: "1.15.0", LatestVersion: "1.16.0", UpdateAvailable: true}
	m.containers = []api.Container{
		{Name: "a", State: "exited", Labels: map[string]string{"docklite.managed": "true", "docklite.domain": "a.com"}},
		{Name: "db", State: "running", Labels: map[string]string{"docklite.managed": "true", "docklite.type": "postgres"}},
	}
	o := &serverOverview{Hostname: "h", CPUCount: 2, Uptime: 90000}
	o.Memory.Total, o.Memory.Free = 8<<30, 2<<30
	o.Disk.Total, o.Disk.Used = 100<<30, 40<<30
	m.x.overview = o
	out := m.renderDashboard()
	for _, want := range []string{"1.16.0 is available", "1 box(es) are stopped", "Sites: 1", "Databases: 1", "Memory", "Disk", "1 days"} {
		if !strings.Contains(strings.ReplaceAll(out, "\x1b", ""), want) && !strings.Contains(stripANSI(out), want) {
			t.Errorf("dashboard missing %q:\n%s", want, out)
		}
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			in = true
		case in && r == 'm':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}
