package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"docklite-tui/api"
)

// Container details → Forwarded domains: send another domain's visitors to this website. The agent onboards the new
// domain as its own container (own DNS and HTTPS certificate), points it here with a redirect or a proxy, and puts both
// in one linked group. Adding another forward is the same steps again.

const (
	fwOff = iota
	fwDomain
	fwHow
	fwConfirm
)

type forwardRow struct {
	Domain string `json:"domain"`
	Mode   string `json:"mode"`
}

type forwardState struct {
	mode     int
	domain   string
	how      int // 0 redirect, 1 proxy
	existing []forwardRow
}

type forwardListMsg struct{ Rows []forwardRow }
type forwardDoneMsg struct {
	Notice string
	Err    error
}

var forwardHows = []struct{ id, label, hint string }{
	{"redirect", "Redirect", "visitors are sent to this site's address (the address bar changes)"},
	{"proxy", "Proxy", "visitors keep the new address and see this site's pages"},
}

func fetchForwardsCmd(c *api.Client, id string) tea.Cmd {
	return func() tea.Msg {
		var res struct {
			Forwards []forwardRow `json:"forwards"`
		}
		if err := c.GetJSON("/api/containers/"+id+"/forward", &res); err != nil {
			return forwardListMsg{}
		}
		return forwardListMsg{Rows: res.Forwards}
	}
}

func (m Model) handleForwardInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.selectedContainer
	if c == nil {
		m.fw.mode = fwOff
		return m, nil
	}
	switch m.fw.mode {
	case fwDomain:
		switch msg.Type {
		case tea.KeyEnter:
			d := strings.ToLower(strings.TrimSpace(m.fw.domain))
			if !strings.Contains(d, ".") || strings.ContainsAny(d, " /") {
				m.err = "Type the new domain, like new-name.com"
				return m, nil
			}
			m.fw.domain, m.err = d, ""
			m.fw.mode = fwHow
		case tea.KeyBackspace:
			if r := []rune(m.fw.domain); len(r) > 0 {
				m.fw.domain = string(r[:len(r)-1])
			}
		case tea.KeyRunes, tea.KeySpace:
			m.fw.domain += string(msg.Runes)
		}
	case fwHow:
		switch msg.Type {
		case tea.KeyUp:
			m.fw.how = 0
		case tea.KeyDown:
			m.fw.how = 1
		case tea.KeyEnter:
			m.fw.mode = fwConfirm
		}
	case fwConfirm:
		if msg.Type == tea.KeyEnter {
			id, domain, how := c.ID, m.fw.domain, forwardHows[m.fw.how].id
			client := m.client
			m.fw.mode = fwOff
			m.notice = "Setting up " + domain + "..."
			return m, func() tea.Msg {
				var res struct {
					Group   string `json:"group"`
					Warning string `json:"warning"`
				}
				err := client.PostBody("/api/containers/"+id+"/forward", map[string]any{"domain": domain, "mode": how}, &res)
				note := domain + " now " + how + "s here."
				if res.Group != "" {
					note += " Both are in the linked group " + res.Group + "."
				}
				note += " Point its DNS at this server; its HTTPS certificate is requested when DNS is ready."
				if res.Warning != "" {
					note += " " + res.Warning
				}
				return forwardDoneMsg{Notice: note, Err: err}
			}
		}
		if msg.Type == tea.KeyUp || msg.Type == tea.KeyDown {
			m.fw.mode = fwHow
		}
	}
	return m, nil
}

func (m Model) renderForward() string {
	c := m.selectedContainer
	lines := []string{titleStyle.Render("Forwarded domains"), "",
		fmt.Sprintf("%s %s", labelStyle.Render("Site:"), statStyle.Render(boxTitle(*c)))}
	if len(m.fw.existing) > 0 {
		lines = append(lines, "", labelStyle.Render("Already forwarded here:"))
		for _, f := range m.fw.existing {
			lines = append(lines, "  "+f.Domain+labelStyle.Render("  ("+f.Mode+")"))
		}
	}
	switch m.fw.mode {
	case fwDomain:
		lines = append(lines, "", labelStyle.Render("Which domain should forward here?"), listItemStyle.Render("> "+m.fw.domain+"_"),
			"", dimTextStyle.Render("Enter next • Esc cancel"))
	case fwHow, fwConfirm:
		lines = append(lines, "", fmt.Sprintf("%s %s", labelStyle.Render("New domain:"), statStyle.Render(m.fw.domain)), "")
		for i, h := range forwardHows {
			prefix, style := "  ", listItemStyle
			if i == m.fw.how {
				prefix, style = "▶ ", listItemSelectedStyle
			}
			lines = append(lines, style.Render(fmt.Sprintf("%s%-10s %s", prefix, h.label, labelStyle.Render(h.hint))))
		}
		if m.fw.mode == fwHow {
			lines = append(lines, "", dimTextStyle.Render("↑/↓ choose • Enter next • Esc cancel"))
		} else {
			lines = append(lines, "",
				labelStyle.Render(m.fw.domain+" gets its own container, DNS record and HTTPS certificate, and joins this site's linked group."),
				errorStyle.Render("Press Enter to create it • Esc cancel"))
		}
	}
	if m.err != "" {
		lines = append(lines, "", errorStyle.Render("Error: "+m.err))
	}
	return strings.Join(lines, "\n")
}
