package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"docklite-tui/api"
)

// Settings → Servers: pick which DockLite server the terminal app shows. "This server" is the one it is connected to;
// the others are servers that one has been given with `docklite servers add`. Another server is seen through the hub,
// which passes on reads and start/stop/restart only, so that view is read-only apart from those.

type serverRow struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type serversMsg struct {
	Rows []serverRow
	Err  string
}

func fetchServersCmd(c *api.Client) tea.Cmd {
	return func() tea.Msg {
		var res struct {
			Servers []serverRow `json:"servers"`
		}
		if err := c.GetJSON("/api/servers", &res); err != nil {
			if e, ok := err.(*api.APIError); ok && (e.Status == 403 || e.Status == 404) {
				return serversMsg{Err: "Only admins can see other servers, and the agent must be DockLite 1.20 or newer."}
			}
			return serversMsg{Err: err.Error()}
		}
		return serversMsg{Rows: res.Servers}
	}
}

func (m Model) serversHint() string {
	if m.x.viewing != "" {
		return "viewing " + m.x.viewing + " (read-only)"
	}
	if len(m.x.servers) > 0 {
		return fmt.Sprintf("this server and %d more", len(m.x.servers))
	}
	return "show other DockLite servers"
}

func (m Model) handleServersInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := len(m.x.servers) + 1 // "This server" first
	switch msg.Type {
	case tea.KeyEsc:
		m.x.setView = 0
	case tea.KeyUp:
		if m.x.serverCursor > 0 {
			m.x.serverCursor--
		}
	case tea.KeyDown:
		if m.x.serverCursor < rows-1 {
			m.x.serverCursor++
		}
	case tea.KeyEnter:
		if m.x.serverCursor == 0 {
			m.client.Via, m.x.viewing = "", ""
		} else {
			s := m.x.servers[m.x.serverCursor-1]
			m.client.Via, m.x.viewing = fmt.Sprintf("/api/servers/%d/proxy", s.ID), s.Name
		}
		// Everything on screen belongs to the server it came from: start clean and load the chosen one.
		m.containers, m.groups, m.databases, m.filteredContainers, m.filteredDatabases = nil, nil, nil, nil, nil
		m.x.ssl, m.x.overview, m.x.netOv, m.x.netFw, m.x.netIngress, m.x.netLoaded = nil, nil, nil, nil, nil, false
		m.summary, m.err, m.notice = nil, "", ""
		m.updateFilteredContainers()
		m.x.setView, m.currentScreen, m.currentView, m.sidebarSelected, m.focusArea = 0, ScreenDashboard, ViewList, 0, FocusSidebar
		return m, tea.Batch(fetchSummary(m.client), fetchContainers(m.client), fetchDatabases(m.client), fetchOverviewCmd(m.client), fetchSSLCmd(m.client))
	}
	return m, nil
}

func (m Model) renderServers() string {
	lines := []string{titleStyle.Render("Servers"), ""}
	names := []string{"This server"}
	hints := []string{m.client.BaseURL}
	for _, s := range m.x.servers {
		names = append(names, s.Name)
		hints = append(hints, s.URL)
	}
	for i, n := range names {
		prefix, style := "  ", listItemStyle
		if i == m.x.serverCursor {
			prefix, style = "▶ ", listItemSelectedStyle
		}
		mark := ""
		if (i == 0 && m.x.viewing == "") || (i > 0 && m.x.viewing == n) {
			mark = "  (showing)"
		}
		lines = append(lines, style.Render(fmt.Sprintf("%s%-18s %s%s", prefix, truncate(n, 18), labelStyle.Render(truncate(hints[i], 40)), mark)))
	}
	if m.x.serversErr != "" {
		lines = append(lines, "", labelStyle.Render(m.x.serversErr))
	} else if len(m.x.servers) == 0 {
		lines = append(lines, "", labelStyle.Render("No other servers yet. On the other server run: docklite share"), labelStyle.Render("then on this one: docklite servers add <code>"))
	}
	lines = append(lines, "", dimTextStyle.Render("↑/↓ choose • Enter show it • Esc back"),
		dimTextStyle.Render(strings.Repeat(" ", 0)+"Other servers are read-only here, except start, stop and restart."))
	return strings.Join(lines, "\n")
}
