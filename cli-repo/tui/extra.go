package tui

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"docklite-tui/api"
)

// The Logs, Network and Settings screens and the dashboard's server details. They read the same endpoints the web
// dashboard uses, and say things in plain words.

// ── what the agent sends ────────────────────────────────────────────────────

type serverOverview struct {
	Hostname  string  `json:"hostname"`
	OS        string  `json:"os"`
	OSVersion string  `json:"osVersion"`
	Kernel    string  `json:"kernel"`
	CPUCount  int     `json:"cpuCount"`
	Uptime    float64 `json:"uptime"`
	LoadAvg   struct {
		One, Five, Fifteen float64
	} `json:"loadAvg"`
	Memory struct{ Total, Free uint64 }       `json:"memory"`
	Disk   struct{ Total, Used, Free uint64 } `json:"disk"`
}

type netOverview struct {
	Hostname       string `json:"hostname"`
	PrimaryIPv4    string `json:"primaryIPv4"`
	DefaultGateway string `json:"defaultGateway"`
	DefaultDevice  string `json:"defaultDevice"`
	Interfaces     []struct {
		Name      string `json:"name"`
		State     string `json:"state"`
		Addresses []struct {
			Address string `json:"address"`
			Family  string `json:"family"`
		} `json:"addresses"`
	} `json:"interfaces"`
}

type netFirewall struct {
	Firewall struct {
		Provider string `json:"provider"`
		Status   string `json:"status"`
	} `json:"firewall"`
	OpenPorts []struct {
		Proto   string `json:"proto"`
		Port    int    `json:"port"`
		Process string `json:"process"`
		Public  bool   `json:"public"`
	} `json:"openPorts"`
	DockerExposed []struct {
		Name  string `json:"name"`
		Ports []struct {
			HostIP   string `json:"hostIp"`
			HostPort int    `json:"hostPort"`
		} `json:"ports"`
	} `json:"dockerExposed"`
}

type netIngress struct {
	Provider string `json:"provider"`
	State    string `json:"state"`
	Status   string `json:"status"`
	Note     string `json:"note"`
}

type netDiagnostics struct {
	PublicIP string `json:"publicIP"`
	Results  []struct {
		Name      string `json:"name"`
		Status    string `json:"status"`
		LatencyMs int64  `json:"latencyMs"`
		Detail    string `json:"detail"`
	} `json:"results"`
}

type sslSites struct {
	Sites []struct {
		Domain          string `json:"domain"`
		HasSSL          bool   `json:"hasSSL"`
		DaysUntilExpiry *int   `json:"daysUntilExpiry"`
		Status          string `json:"status"`
		AutoRenew       bool   `json:"autoRenew"`
		Renewal         *struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		} `json:"renewal"`
	} `json:"sites"`
	Meta struct {
		AutoRenewAll bool `json:"autoRenewAll"`
	} `json:"meta"`
}

// extraState is everything the new screens remember.
type extraState struct {
	// Logs
	logTarget  int
	logLines   []string
	logOffset  int
	logErr     string
	logLoaded  bool
	logFollow  bool
	logLoading bool
	// Network
	netOv       *netOverview
	netFw       *netFirewall
	netIngress  *netIngress
	netDiag     *netDiagnostics
	netDiagBusy bool
	netErrs     []string
	netOffset   int
	netLoaded   bool
	// Settings
	setCursor    int
	setView      int // 0 list, 1 update, 2 about, 3 connection, 4 linked domain
	panel        *panelDomain
	servers      []serverRow
	serversErr   string
	serverCursor int
	viewing      string // name of the other server being shown; "" is the one connected to
	panelEdit    bool   // typing a new domain
	panelText    string // what has been typed
	panelErr     string
	// Dashboard extras
	overview *serverOverview
	ssl      *sslSites
}

var logTargets = []struct{ id, label, hint string }{
	{"docklite", "DockLite", "the agent and dashboard"},
	{"system", "System", "the server's own log"},
	{"proxy", "Web proxy", "nginx"},
	{"traefik", "Traefik", "if you use it"},
	{"legacy-api", "Legacy API", "the older service"},
}

const (
	setItemUpdate = iota
	setItemAbout
	setItemConnection
	setItemAutoRenew
	setItemDomain
	setItemServers
	setItemCount
)

// ── messages and commands ───────────────────────────────────────────────────

type logsMsg struct {
	Target int
	Text   string
	Err    error
}
type netMsg struct {
	Ov      *netOverview
	Fw      *netFirewall
	Ingress *netIngress
	Errs    []string
}
type netDiagMsg struct {
	D   *netDiagnostics
	Err error
}
type overviewMsg struct{ O *serverOverview }
type sslMsg struct{ S *sslSites }

func fetchLogsCmd(c *api.Client, target int) tea.Cmd {
	return func() tea.Msg {
		var out struct {
			Logs string `json:"logs"`
		}
		err := c.GetJSON("/api/server/logs?tail=300&target="+url.QueryEscape(logTargets[target].id), &out)
		return logsMsg{Target: target, Text: out.Logs, Err: err}
	}
}

func fetchNetworkCmd(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		var res netMsg
		var ov netOverview
		if err := c.GetJSON("/api/network/overview", &ov); err == nil {
			res.Ov = &ov
		} else {
			res.Errs = append(res.Errs, "Addresses: "+friendlyErr(err))
		}
		var fw netFirewall
		if err := c.GetJSON("/api/network/firewall", &fw); err == nil {
			res.Fw = &fw
		} else {
			res.Errs = append(res.Errs, "Firewall: "+friendlyErr(err))
		}
		var ing netIngress
		if err := c.GetJSON("/api/network/ingress", &ing); err == nil {
			res.Ingress = &ing
		}
		return res
	}
}

func fetchNetDiagCmd(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		var d netDiagnostics
		err := c.PostJSON("/api/network/diagnostics", &d)
		return netDiagMsg{D: &d, Err: err}
	}
}

func fetchOverviewCmd(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		var o serverOverview
		if err := c.GetJSON("/api/server/overview", &o); err != nil {
			return overviewMsg{}
		}
		return overviewMsg{O: &o}
	}
}

func fetchSSLCmd(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		var s sslSites
		if err := c.GetJSON("/api/ssl/status", &s); err != nil {
			return sslMsg{}
		}
		return sslMsg{S: &s}
	}
}

func friendlyErr(err error) string {
	if e, ok := err.(*api.APIError); ok {
		switch e.Status {
		case 403:
			return "needs an admin account"
		case 404:
			return "not available on this DockLite version"
		}
	}
	return err.Error()
}

// handleExtraMsg folds the new messages into the model; ok is false for any other message.
func (m Model) handleExtraMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	if scan, ok := msg.(dbScanMsg); ok {
		switch {
		case scan.err != nil:
			m.notice, m.err = "", scan.err.Error()
		case len(scan.added) > 0:
			m.notice = fmt.Sprintf("Found %d new: %s", len(scan.added), strings.Join(scan.added, ", "))
		default:
			m.notice = fmt.Sprintf("Scan done. Nothing new; %d already listed.", scan.existing)
		}
		return m, fetchDatabaseCatalog(m.client), true
	}
	switch msg := msg.(type) {
	case logsMsg:
		m.x.logLoading = false
		if msg.Target != m.x.logTarget {
			return m, nil, true
		}
		if msg.Err != nil {
			m.x.logErr = friendlyErr(msg.Err)
			m.x.logLines = nil
		} else {
			m.x.logErr = ""
			wasAtEnd := !m.x.logLoaded || m.x.logOffset >= m.logMaxOffset()
			m.x.logLines = splitLogLines(msg.Text)
			if wasAtEnd || m.x.logFollow {
				m.x.logOffset = m.logMaxOffset()
			}
		}
		m.x.logLoaded = true
		return m, nil, true
	case netMsg:
		m.x.netOv, m.x.netFw, m.x.netIngress, m.x.netErrs = msg.Ov, msg.Fw, msg.Ingress, msg.Errs
		m.x.netLoaded = true
		return m, nil, true
	case netDiagMsg:
		m.x.netDiagBusy = false
		if msg.Err == nil {
			m.x.netDiag = msg.D
		} else {
			m.x.netErrs = append(m.x.netErrs, "Checks: "+friendlyErr(msg.Err))
		}
		return m, nil, true
	case groupDoneMsg:
		m.mg.mode = mgOff
		if msg.Err != nil {
			m.err = friendlyErr(msg.Err)
			if e, ok := msg.Err.(*api.APIError); ok {
				m.err = apiMessage(e)
			}
			return m, nil, true
		}
		m.err, m.notice = "", msg.Notice
		if m.currentView == ViewDetails {
			m.currentView, m.selectedContainer = ViewList, nil
		}
		return m, fetchContainers(m.client), true
	case overviewMsg:
		if msg.O != nil {
			m.x.overview = msg.O
		}
		return m, nil, true
	case sslMsg:
		if msg.S != nil {
			m.x.ssl = msg.S
		}
		return m, nil, true
	case serversMsg:
		m.x.servers, m.x.serversErr = msg.Rows, msg.Err
		return m, nil, true
	case panelMsg:
		if msg.P != nil {
			m.x.panel, m.x.panelErr = msg.P, ""
		} else {
			m.x.panelErr = msg.Err
		}
		return m, nil, true
	}
	return m, nil, false
}

func splitLogLines(s string) []string {
	s = strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// ── scrolling ───────────────────────────────────────────────────────────────

func (m Model) logViewHeight() int { return max(3, m.innerHeight()-6) }

func (m Model) logMaxOffset() int { return max(0, len(m.x.logLines)-m.logViewHeight()) }

// scrollKey moves an offset for the keys every scrolling screen shares; it reports whether the key was one.
func scrollKey(msg tea.KeyMsg, off, max, page int) (int, bool) {
	switch msg.Type {
	case tea.KeyUp:
		off--
	case tea.KeyDown:
		off++
	case tea.KeyPgUp:
		off -= page
	case tea.KeyPgDown:
		off += page
	case tea.KeyHome:
		off = 0
	case tea.KeyEnd:
		off = max
	default:
		return off, false
	}
	if off < 0 {
		off = 0
	}
	if off > max {
		off = max
	}
	return off, true
}

// ── Logs ────────────────────────────────────────────────────────────────────

func (m Model) handleLogsInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if off, ok := scrollKey(msg, m.x.logOffset, m.logMaxOffset(), m.logViewHeight()-1); ok {
		m.x.logOffset = off
		return m, nil
	}
	switch {
	case msg.Type == tea.KeyLeft || msg.Type == tea.KeyRight:
		d := 1
		if msg.Type == tea.KeyLeft {
			d = len(logTargets) - 1
		}
		m.x.logTarget = (m.x.logTarget + d) % len(logTargets)
		m.x.logLines, m.x.logErr, m.x.logLoaded, m.x.logLoading = nil, "", false, true
		return m, fetchLogsCmd(m.client, m.x.logTarget)
	case msg.String() == "r" || msg.String() == "R":
		m.x.logLoading = true
		return m, fetchLogsCmd(m.client, m.x.logTarget)
	case msg.String() == "f" || msg.String() == "F":
		m.x.logFollow = !m.x.logFollow
		if m.x.logFollow {
			m.x.logOffset = m.logMaxOffset()
		}
	}
	return m, nil
}

func (m Model) renderLogsScreen() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Logs"))
	var tabs []string
	for i, t := range logTargets {
		if i == m.x.logTarget {
			tabs = append(tabs, buttonActiveStyle.Render(t.label))
		} else {
			tabs = append(tabs, buttonStyle.Render(t.label))
		}
	}
	lines = append(lines, strings.Join(tabs, " "))
	follow := "off"
	if m.x.logFollow {
		follow = "on (refreshes every 10 seconds)"
	}
	lines = append(lines, labelStyle.Render(fmt.Sprintf("%s · follow: %s", logTargets[m.x.logTarget].hint, follow)), "")
	switch {
	case m.x.logErr != "":
		lines = append(lines, errorStyle.Render("Couldn't read this log: "+m.x.logErr))
	case !m.x.logLoaded:
		lines = append(lines, labelStyle.Render("Loading..."))
	case len(m.x.logLines) == 0:
		lines = append(lines, labelStyle.Render("Nothing in this log yet."))
	default:
		h := m.logViewHeight()
		end := min(len(m.x.logLines), m.x.logOffset+h)
		for _, l := range m.x.logLines[m.x.logOffset:end] {
			lines = append(lines, logLineStyled(truncate(l, m.innerWidth())))
		}
		lines = append(lines, "", labelStyle.Render(fmt.Sprintf("lines %d–%d of %d", m.x.logOffset+1, end, len(m.x.logLines))))
	}
	return strings.Join(lines, "\n")
}

// logLineStyled colours the words a person scans for.
func logLineStyled(l string) string {
	low := strings.ToLower(l)
	if strings.Contains(low, "error") || strings.Contains(low, "fail") || strings.Contains(low, "denied") || strings.Contains(low, "panic") {
		return errorStyle.Render(l)
	}
	return l
}

// ── Network ─────────────────────────────────────────────────────────────────

func (m Model) netLines() []string {
	var out []string
	h := func(s string) { out = append(out, "", statStyle.Render(s)) }
	kv := func(k, v string) {
		out = append(out, fmt.Sprintf("%s %s", labelStyle.Render(fmt.Sprintf("%-18s", k)), v))
	}
	for _, e := range m.x.netErrs {
		out = append(out, errorStyle.Render(e))
	}
	if ov := m.x.netOv; ov != nil {
		h("This server")
		kv("Name", ov.Hostname)
		kv("Address", ov.PrimaryIPv4)
		if m.x.netDiag != nil && m.x.netDiag.PublicIP != "" {
			kv("Public address", m.x.netDiag.PublicIP+"  (what the internet sees)")
		}
		if ov.DefaultGateway != "" {
			kv("Goes out through", fmt.Sprintf("%s on %s", ov.DefaultGateway, ov.DefaultDevice))
		}
		for _, in := range ov.Interfaces {
			// Docker's own plumbing (one veth per container, bridges) isn't something to read through.
			if in.Name == "lo" || strings.HasPrefix(in.Name, "veth") || strings.HasPrefix(in.Name, "br-") || strings.HasPrefix(in.Name, "docker") {
				continue
			}
			var addrs []string
			for _, a := range in.Addresses {
				if a.Family == "inet" || a.Family == "ipv4" || !strings.Contains(a.Address, ":") {
					addrs = append(addrs, a.Address)
				}
			}
			kv("Connection "+in.Name, strings.TrimSpace(in.State+" "+strings.Join(addrs, ", ")))
		}
	}
	if fw := m.x.netFw; fw != nil {
		h("Who can reach your sites")
		status := fw.Firewall.Status
		if status == "" {
			status = "unknown"
		}
		kv("Firewall", fmt.Sprintf("%s (%s)", status, orDash(fw.Firewall.Provider)))
		var open []string
		seen := map[string]bool{}
		for _, p := range fw.OpenPorts {
			if !p.Public {
				continue
			}
			label := portWords(p.Port)
			if seen[label] {
				continue
			}
			seen[label] = true
			open = append(open, label)
		}
		if len(open) > 8 {
			open = append(open[:8], fmt.Sprintf("and %d more", len(open)-8))
		}
		if len(open) == 0 {
			kv("Open to the internet", "nothing found")
		} else {
			kv("Open to the internet", strings.Join(open, ", "))
		}
		var exposed []string
		for _, d := range fw.DockerExposed {
			for _, p := range d.Ports {
				if p.HostIP == "0.0.0.0" || p.HostIP == "::" || p.HostIP == "" {
					exposed = append(exposed, fmt.Sprintf("%s (port %d)", d.Name, p.HostPort))
				}
			}
		}
		if len(exposed) > 0 {
			out = append(out, errorStyle.Render("Containers reachable from the internet directly (not through nginx):"))
			for _, e := range exposed {
				out = append(out, "  "+e)
			}
		}
	}
	if ing := m.x.netIngress; ing != nil && ing.Provider != "" {
		h("Web proxy")
		kv("Provider", ing.Provider)
		kv("State", strings.TrimSpace(ing.State+" "+ing.Status))
		if ing.Note != "" {
			out = append(out, labelStyle.Render(ing.Note))
		}
	}
	if m.x.ssl != nil {
		h("HTTPS certificates")
		if len(m.x.ssl.Sites) == 0 {
			out = append(out, labelStyle.Render("No sites yet."))
		}
		for _, s := range m.x.ssl.Sites {
			line := "  " + sslWords(s.Domain, s.HasSSL, s.DaysUntilExpiry)
			if s.HasSSL {
				if s.AutoRenew {
					line += labelStyle.Render(" · auto-renew on")
				} else {
					line += labelStyle.Render(" · auto-renew off")
				}
			}
			out = append(out, line)
			if s.Renewal != nil && !s.Renewal.OK {
				out = append(out, "    "+errorStyle.Render("last renewal failed: "+truncate(s.Renewal.Error, 60)))
			}
		}
	}
	h("Connection checks")
	switch {
	case m.x.netDiagBusy:
		out = append(out, labelStyle.Render("Running..."))
	case m.x.netDiag == nil:
		out = append(out, labelStyle.Render("Press d to test the server's internet connection."))
	default:
		for _, r := range m.x.netDiag.Results {
			mark := "✓"
			style := statStyle
			if r.Status != "ok" && r.Status != "pass" && r.Status != "success" {
				mark, style = "✗", errorStyle
			}
			out = append(out, "  "+style.Render(mark)+" "+r.Name+labelStyle.Render(fmt.Sprintf("  %dms %s", r.LatencyMs, r.Detail)))
		}
	}
	return out
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func portWords(p int) string {
	switch p {
	case 80:
		return "80 (websites)"
	case 443:
		return "443 (secure websites)"
	case 22:
		return "22 (SSH, remote login)"
	}
	return fmt.Sprintf("%d", p)
}

func sslWords(domain string, has bool, days *int) string {
	switch {
	case !has || days == nil:
		return domain + labelStyle.Render("  no HTTPS")
	case *days < 0:
		return domain + errorStyle.Render("  HTTPS expired")
	case *days <= 14:
		return domain + errorStyle.Render(fmt.Sprintf("  HTTPS expires in %d days", *days))
	}
	return domain + labelStyle.Render(fmt.Sprintf("  HTTPS ok, %d days left", *days))
}

func (m Model) netViewHeight() int { return max(3, m.innerHeight()-4) }

func (m Model) netMaxOffset() int { return max(0, len(m.netLines())-m.netViewHeight()) }

func (m Model) handleNetworkInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if off, ok := scrollKey(msg, m.x.netOffset, m.netMaxOffset(), m.netViewHeight()-1); ok {
		m.x.netOffset = off
		return m, nil
	}
	switch msg.String() {
	case "r", "R":
		return m, tea.Batch(fetchNetworkCmd(m.client), fetchSSLCmd(m.client))
	case "d", "D":
		if !m.x.netDiagBusy {
			m.x.netDiagBusy = true
			return m, fetchNetDiagCmd(m.client)
		}
	}
	return m, nil
}

func (m Model) renderNetworkScreen() string {
	lines := []string{titleStyle.Render("Network")}
	if !m.x.netLoaded {
		return strings.Join(append(lines, labelStyle.Render("Loading...")), "\n")
	}
	all := m.netLines()
	h := m.netViewHeight()
	off := min(m.x.netOffset, max(0, len(all)-h))
	end := min(len(all), off+h)
	for _, l := range all[off:end] {
		lines = append(lines, truncate2(l, m.innerWidth()))
	}
	if len(all) > h {
		lines = append(lines, "", labelStyle.Render(fmt.Sprintf("↑/↓ scroll · %d–%d of %d", off+1, end, len(all))))
	}
	return strings.Join(lines, "\n")
}

// truncate2 cuts a line that may carry colour codes without counting them.
func truncate2(s string, w int) string { return fitPanel(s, w, 1) }

// ── Settings ────────────────────────────────────────────────────────────────

func (m Model) handleSettingsInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.x.setView == 1 {
		if msg.Type == tea.KeyEsc && !m.updRunning {
			m.x.setView = 0
			return m, nil
		}
		return m.handleUpdateInput(msg)
	}
	if m.x.setView == 4 {
		return m.handlePanelInput(msg)
	}
	if m.x.setView == 5 {
		return m.handleServersInput(msg)
	}
	if m.x.setView != 0 {
		if msg.Type == tea.KeyEsc || msg.Type == tea.KeyEnter {
			m.x.setView = 0
		}
		return m, nil
	}
	switch msg.Type {
	case tea.KeyUp:
		if m.x.setCursor > 0 {
			m.x.setCursor--
		}
	case tea.KeyDown:
		if m.x.setCursor < setItemCount-1 {
			m.x.setCursor++
		}
	case tea.KeyEnter:
		switch m.x.setCursor {
		case setItemUpdate:
			m.x.setView = 1
			m.updConfirm, m.updErr, m.updNote = false, "", ""
			return m, fetchUpdateStatus(m.client, true)
		case setItemAbout:
			m.x.setView = 2
			return m, fetchOverviewCmd(m.client)
		case setItemConnection:
			m.x.setView = 3
		case setItemAutoRenew:
			return m, toggleAutoRenewCmd(m.client, !(m.x.ssl != nil && m.x.ssl.Meta.AutoRenewAll))
		case setItemServers:
			m.x.setView, m.x.serverCursor = 5, 0
			return m, fetchServersCmd(m.client)
		case setItemDomain:
			m.x.setView = 4
			m.x.panelEdit, m.x.panelErr = false, ""
			return m, fetchPanelCmd(m.client)
		}
	}
	return m, nil
}

func (m Model) renderSettings() string {
	switch m.x.setView {
	case 1:
		return m.renderUpdate()
	case 2:
		return m.renderAbout()
	case 3:
		return m.renderConnection()
	case 4:
		return m.renderPanel()
	case 5:
		return m.renderServers()
	}
	lines := []string{titleStyle.Render("Settings"), ""}
	items := []struct{ name, hint string }{
		{"Update DockLite", m.updateHint()},
		{"About this server", "version, system, uptime"},
		{"Connection", "which DockLite this terminal app talks to"},
		{"SSL auto-renew", m.autoRenewHint()},
		{"Linked domain", m.panelHint()},
		{"Servers", m.serversHint()},
	}
	for i, it := range items {
		prefix, style := "  ", listItemStyle
		if i == m.x.setCursor && m.focusArea == FocusMain {
			prefix, style = "▶ ", listItemSelectedStyle
		}
		lines = append(lines, style.Render(fmt.Sprintf("%s%-22s %s", prefix, it.name, labelStyle.Render(it.hint))))
	}
	return strings.Join(lines, "\n")
}

func (m Model) updateHint() string {
	if m.updStatus == nil {
		return "check for a newer version"
	}
	if m.updStatus.UpdateAvailable {
		return "version " + m.updStatus.LatestVersion + " is available"
	}
	return "you're on " + m.updStatus.Version + ", up to date"
}

func (m Model) renderAbout() string {
	lines := []string{titleStyle.Render("About this server"), ""}
	ver := "unknown"
	if m.updStatus != nil {
		ver = m.updStatus.Version
	}
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("DockLite version:"), statStyle.Render(ver)))
	if o := m.x.overview; o != nil {
		lines = append(lines,
			fmt.Sprintf("%s %s", labelStyle.Render("Server name:     "), o.Hostname),
			fmt.Sprintf("%s %s %s", labelStyle.Render("System:          "), o.OS, o.OSVersion),
			fmt.Sprintf("%s %s", labelStyle.Render("Kernel:          "), o.Kernel),
			fmt.Sprintf("%s %d", labelStyle.Render("Processors:      "), o.CPUCount),
			fmt.Sprintf("%s %s", labelStyle.Render("Running for:     "), uptimeWords(o.Uptime)),
		)
	} else {
		lines = append(lines, labelStyle.Render("Server details need an admin account."))
	}
	lines = append(lines, "", dimTextStyle.Render("Enter or Esc to go back"))
	return strings.Join(lines, "\n")
}

func (m Model) renderConnection() string {
	lines := []string{titleStyle.Render("Connection"), "",
		fmt.Sprintf("%s %s", labelStyle.Render("Server:"), m.client.BaseURL),
		fmt.Sprintf("%s %s", labelStyle.Render("Token: "), maskToken(m.client.Token)),
		"",
		labelStyle.Render("Set with DOCKLITE_URL and DOCKLITE_TOKEN, or when the app asks you to sign in."),
		"", dimTextStyle.Render("Enter or Esc to go back")}
	return strings.Join(lines, "\n")
}

// ── Dashboard extras ────────────────────────────────────────────────────────

func uptimeWords(sec float64) string {
	d := int(sec) / 86400
	h := int(sec) % 86400 / 3600
	mn := int(sec) % 3600 / 60
	switch {
	case d > 0:
		return fmt.Sprintf("%d days, %d hours", d, h)
	case h > 0:
		return fmt.Sprintf("%d hours, %d minutes", h, mn)
	}
	return fmt.Sprintf("%d minutes", mn)
}

func bar(frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	n := int(frac*float64(width) + 0.5)
	return strings.Repeat("█", n) + strings.Repeat("░", width-n)
}

// dashboardExtras are the lines under the summary: what needs attention first, then how the server is doing.
func (m Model) dashboardExtras() []string {
	var out []string
	var attention []string
	if m.updStatus != nil && m.updStatus.UpdateAvailable {
		attention = append(attention, fmt.Sprintf("DockLite %s is available. Settings → Update DockLite.", m.updStatus.LatestVersion))
	}
	if m.x.ssl != nil {
		soon, none := 0, 0
		for _, s := range m.x.ssl.Sites {
			if s.Renewal != nil && !s.Renewal.OK {
				left := "an unknown number of"
				if s.DaysUntilExpiry != nil {
					left = fmt.Sprintf("%d", *s.DaysUntilExpiry)
				}
				attention = append(attention, fmt.Sprintf("Renewal failed for %s, expires in %s days.", s.Domain, left))
			}
			switch {
			case !s.HasSSL || s.DaysUntilExpiry == nil:
				none++
			case *s.DaysUntilExpiry <= 14:
				soon++
			}
		}
		if soon > 0 {
			attention = append(attention, fmt.Sprintf("%d site(s) have HTTPS expiring within two weeks or already expired.", soon))
		}
		if none > 0 {
			attention = append(attention, fmt.Sprintf("%d site(s) have no HTTPS.", none))
		}
	}
	stopped := 0
	for _, c := range m.containers {
		if c.Labels["docklite.managed"] == "true" && c.State != "running" {
			stopped++
		}
	}
	if stopped > 0 {
		attention = append(attention, fmt.Sprintf("%d container(s) are stopped.", stopped))
	}
	out = append(out, "", titleStyle.Render("Needs attention"))
	if len(attention) == 0 {
		out = append(out, statStyle.Render("  Nothing. All good."))
	}
	for _, a := range attention {
		out = append(out, errorStyle.Render("  • "+a))
	}

	sites, dbs := 0, 0
	for _, c := range m.containers {
		if c.Labels["docklite.managed"] != "true" {
			continue
		}
		if c.Labels["docklite.domain"] != "" {
			sites++
		} else if databaseTypeSet[c.Labels["docklite.type"]] {
			dbs++
		}
	}
	out = append(out, "", titleStyle.Render("Your containers"))
	out = append(out, fmt.Sprintf("  %s %s   %s %s   %s %s",
		labelStyle.Render("Sites:"), statStyle.Render(fmt.Sprint(sites)),
		labelStyle.Render("Databases:"), statStyle.Render(fmt.Sprint(dbs)),
		labelStyle.Render("Groups:"), statStyle.Render(fmt.Sprint(len(m.groups)))))

	if o := m.x.overview; o != nil {
		out = append(out, "", titleStyle.Render("This server"))
		w := 24
		if o.Memory.Total > 0 {
			used := o.Memory.Total - min(o.Memory.Free, o.Memory.Total)
			out = append(out, fmt.Sprintf("  %s %s %s of %s", labelStyle.Render("Memory"), bar(float64(used)/float64(o.Memory.Total), w), formatBytes(used), formatBytes(o.Memory.Total)))
		}
		if o.Disk.Total > 0 {
			out = append(out, fmt.Sprintf("  %s %s %s of %s", labelStyle.Render("Disk  "), bar(float64(o.Disk.Used)/float64(o.Disk.Total), w), formatBytes(o.Disk.Used), formatBytes(o.Disk.Total)))
		}
		if o.CPUCount > 0 {
			out = append(out, fmt.Sprintf("  %s %s %.2f (of %d processors)", labelStyle.Render("Load  "), bar(o.LoadAvg.One/float64(o.CPUCount), w), o.LoadAvg.One, o.CPUCount))
		}
		out = append(out, fmt.Sprintf("  %s %s", labelStyle.Render("Up for"), uptimeWords(o.Uptime)))
	}
	return out
}

var databaseTypeSet = map[string]bool{"postgres": true, "mysql": true, "mariadb": true, "mongodb": true, "redis": true, "sqlite": true}

// refreshExtras is what the Dashboard wants fresh.
func (m Model) refreshExtras() tea.Cmd {
	return tea.Batch(fetchOverviewCmd(m.client), fetchSSLCmd(m.client), fetchUpdateStatus(m.client, false))
}

var _ = time.Second
