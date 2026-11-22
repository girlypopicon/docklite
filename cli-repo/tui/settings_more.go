package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"docklite-tui/api"
)

// Settings → SSL auto-renew (all or none) and Settings → Linked domain (the address the dashboard is reached at).

type panelDomain struct {
	Domain      string `json:"domain"`
	Enabled     bool   `json:"enabled"`
	HTTPS       bool   `json:"https"`
	URL         string `json:"url"`
	FallbackURL string `json:"fallback_url"`
}

type panelMsg struct {
	P   *panelDomain
	Err string
}

func fetchPanelCmd(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		var p panelDomain
		if err := c.GetJSON("/api/panel-domain", &p); err != nil {
			return panelMsg{Err: friendly(err)}
		}
		return panelMsg{P: &p}
	}
}

func savePanelCmd(c *api.Client, payload map[string]any) tea.Cmd {
	return func() tea.Msg {
		var p panelDomain
		if err := c.PutJSON("/api/panel-domain", payload, &p); err != nil {
			return panelMsg{Err: friendly(err)}
		}
		return panelMsg{P: &p}
	}
}

func toggleAutoRenewCmd(c *api.Client, all bool) tea.Cmd {
	return func() tea.Msg {
		if err := c.PutJSON("/api/ssl/autorenew", map[string]any{"enabled": all}, nil); err != nil {
			return sslMsg{}
		}
		var s sslSites
		if err := c.GetJSON("/api/ssl/status", &s); err != nil {
			return sslMsg{}
		}
		return sslMsg{S: &s}
	}
}

func friendly(err error) string {
	if e, ok := err.(*api.APIError); ok {
		return apiMessage(e)
	}
	return err.Error()
}

func (m Model) autoRenewHint() string {
	if m.x.ssl == nil {
		return "renew HTTPS certificates by themselves"
	}
	if m.x.ssl.Meta.AutoRenewAll {
		return "all certificates (Enter for none)"
	}
	return "none (Enter for all)"
}

func (m Model) panelHint() string {
	switch p := m.x.panel; {
	case p == nil:
		return "the address the dashboard is reached at"
	case p.Domain == "":
		return "not set; using " + p.FallbackURL
	case p.Enabled:
		return p.Domain + " (on)"
	default:
		return p.Domain + " (off)"
	}
}

func (m Model) handlePanelInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.x.panelEdit {
		switch msg.Type {
		case tea.KeyEsc:
			m.x.panelEdit = false
		case tea.KeyEnter:
			d := strings.TrimSpace(m.x.panelText)
			m.x.panelEdit = false
			if d != "" {
				m.x.panelErr = ""
				return m, savePanelCmd(m.client, map[string]any{"domain": d, "enabled": true})
			}
		case tea.KeyBackspace:
			if r := []rune(m.x.panelText); len(r) > 0 {
				m.x.panelText = string(r[:len(r)-1])
			}
		case tea.KeyRunes, tea.KeySpace:
			m.x.panelText += string(msg.Runes)
		}
		return m, nil
	}
	switch {
	case msg.Type == tea.KeyEsc || msg.Type == tea.KeyEnter:
		m.x.setView = 0
	case msg.String() == "e":
		if m.x.panel != nil && m.x.panel.Domain != "" {
			return m, savePanelCmd(m.client, map[string]any{"enabled": !m.x.panel.Enabled})
		}
		m.x.panelErr = "Set a domain first (c)."
	case msg.String() == "c":
		m.x.panelEdit, m.x.panelText, m.x.panelErr = true, "", ""
		if m.x.panel != nil {
			m.x.panelText = m.x.panel.Domain
		}
	case msg.String() == "r":
		return m, fetchPanelCmd(m.client)
	}
	return m, nil
}

func (m Model) renderPanel() string {
	lines := []string{titleStyle.Render("Linked domain"), ""}
	p := m.x.panel
	if p == nil {
		lines = append(lines, labelStyle.Render("Loading..."))
	} else {
		state := "no domain set"
		switch {
		case p.Domain != "" && p.Enabled:
			state = "on"
		case p.Domain != "":
			state = "off"
		}
		domain := p.Domain
		if domain == "" {
			domain = "-"
		}
		lines = append(lines,
			fmt.Sprintf("%s %s", labelStyle.Render("Domain:  "), statStyle.Render(domain)),
			fmt.Sprintf("%s %s", labelStyle.Render("Status:  "), statStyle.Render(state)),
		)
		if p.Enabled {
			https := "no, the address is plain http (run: docklite ssl issue " + p.Domain + ")"
			if p.HTTPS {
				https = "yes"
			}
			lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("HTTPS:   "), statStyle.Render(https)))
		}
		lines = append(lines, "", fmt.Sprintf("%s %s", labelStyle.Render("Open DockLite at:"), statStyle.Render(p.URL)))
		if !p.Enabled {
			lines = append(lines, labelStyle.Render("With no domain, DockLite is at the server's address and port."))
		} else {
			lines = append(lines, labelStyle.Render("Without the domain it would be at "+p.FallbackURL))
		}
	}
	if m.x.panelEdit {
		lines = append(lines, "", labelStyle.Render("New domain (Enter saves and turns it on, Esc cancels):"), listItemStyle.Render("> "+m.x.panelText+"_"))
	} else {
		lines = append(lines, "", dimTextStyle.Render("e turn on/off • c set or change the domain • r refresh • Esc back"))
	}
	if m.x.panelErr != "" {
		lines = append(lines, "", errorStyle.Render(m.x.panelErr))
	}
	return strings.Join(lines, "\n")
}
