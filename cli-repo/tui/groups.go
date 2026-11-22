package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"docklite-tui/api"
)

// Moving a container into a group, and the Groups view (press g on the Containers list) where groups are linked, unlinked,
// renamed and deleted. A linked group is one whose containers are meant to talk to each other (a website and its database);
// an unlinked one just keeps the desktop tidy.

type groupDoneMsg struct {
	Notice string
	Err    error
}

// ── move a container to a group ───────────────────────────────────────────────────

const (
	mgOff = iota
	mgPick
	mgName
	mgLink
)

type moveGroupState struct {
	mode   int
	cursor int
	name   string
}

// moveChoices are the rows of the "move to group" list: every group, a new one, and taking the container out.
func (m Model) moveChoices() []string {
	var out []string
	for _, g := range m.groups {
		out = append(out, "group:"+g.Name)
	}
	out = append(out, "new")
	if m.groupOf[m.selectedContainer.Name] != "" {
		out = append(out, "out")
	}
	return out
}

func (m Model) groupByName(name string) *api.Group {
	for i := range m.groups {
		if m.groups[i].Name == name {
			return &m.groups[i]
		}
	}
	return nil
}

func (m Model) handleMoveGroupInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.selectedContainer
	if c == nil {
		m.mg.mode = mgOff
		return m, nil
	}
	switch m.mg.mode {
	case mgPick:
		choices := m.moveChoices()
		switch msg.Type {
		case tea.KeyUp:
			if m.mg.cursor > 0 {
				m.mg.cursor--
			}
		case tea.KeyDown:
			if m.mg.cursor < len(choices)-1 {
				m.mg.cursor++
			}
		case tea.KeyEnter:
			choice := choices[m.mg.cursor]
			client, id, label := m.client, c.ID, boxTitle(*c)
			switch {
			case choice == "new":
				m.mg.mode, m.mg.name = mgName, ""
			case choice == "out":
				g := m.groupByName(m.groupOf[c.Name])
				if g == nil {
					m.mg.mode = mgOff
					return m, nil
				}
				gid, gname := g.ID, g.Name
				m.mg.mode = mgOff
				return m, func() tea.Msg {
					return groupDoneMsg{Notice: label + " is out of " + gname + ".", Err: client.RemoveFromGroup(gid, id)}
				}
			default:
				name := strings.TrimPrefix(choice, "group:")
				g := m.groupByName(name)
				if g == nil {
					return m, nil
				}
				if m.groupOf[c.Name] == name {
					m.mg.mode = mgOff
					m.notice = label + " is already in " + name + "."
					return m, nil
				}
				gid := g.ID
				m.mg.mode = mgOff
				return m, func() tea.Msg {
					return groupDoneMsg{Notice: label + " moved to " + name + ".", Err: client.AddToGroup(gid, id)}
				}
			}
		}
	case mgName:
		switch {
		case msg.Type == tea.KeyEnter:
			if strings.TrimSpace(m.mg.name) == "" {
				m.err = "Give the group a name"
				return m, nil
			}
			m.err = ""
			m.mg.mode = mgLink
		case msg.Type == tea.KeyBackspace:
			if r := []rune(m.mg.name); len(r) > 0 {
				m.mg.name = string(r[:len(r)-1])
			}
		case msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace:
			if len([]rune(m.mg.name)) < 50 {
				m.mg.name += msg.String()
			}
		}
	case mgLink:
		linked := false
		switch msg.String() {
		case "y", "Y":
			linked = true
		case "n", "N":
		default:
			if msg.Type != tea.KeyEnter {
				return m, nil
			}
		}
		client, id, label, name := m.client, c.ID, boxTitle(*c), strings.TrimSpace(m.mg.name)
		m.mg.mode = mgOff
		return m, func() tea.Msg {
			_, err := client.CreateGroup(name, []string{id}, linked)
			return groupDoneMsg{Notice: label + " is in the new group " + name + ".", Err: err}
		}
	}
	return m, nil
}

func (m Model) renderMoveGroup() string {
	c := m.selectedContainer
	lines := []string{titleStyle.Render("Move to group"), "",
		fmt.Sprintf("%s %s", labelStyle.Render("Container:"), statStyle.Render(boxTitle(*c)))}
	if g := m.groupOf[c.Name]; g != "" {
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Now in:"), g))
	}
	lines = append(lines, "")
	switch m.mg.mode {
	case mgPick:
		for i, ch := range m.moveChoices() {
			text := ""
			switch {
			case ch == "new":
				text = "+ A new group..."
			case ch == "out":
				text = "Take it out of its group"
			default:
				name := strings.TrimPrefix(ch, "group:")
				g := m.groupByName(name)
				text = fmt.Sprintf("%s (%d)", name, len(g.Members))
				if g.Linked {
					text += "  linked"
				}
				if m.groupOf[c.Name] == name {
					text += "  ← here now"
				}
			}
			prefix, style := "  ", listItemStyle
			if i == m.mg.cursor {
				prefix, style = "▶ ", listItemSelectedStyle
			}
			lines = append(lines, style.Render(prefix+text))
		}
		lines = append(lines, "", dimTextStyle.Render("↑/↓ choose • Enter move • Esc cancel"))
	case mgName:
		lines = append(lines, labelStyle.Render("Name for the new group:"), listItemStyle.Render("> "+m.mg.name+"_"))
	case mgLink:
		lines = append(lines,
			statStyle.Render("New group: "+strings.TrimSpace(m.mg.name)), "",
			labelStyle.Render("Link it? Containers in a linked group are meant to talk to each other"),
			labelStyle.Render("(a website and its database). Unlinked just keeps things tidy."), "",
			errorStyle.Render("y = linked • n = not linked • Esc cancel"))
	}
	if m.err != "" {
		lines = append(lines, "", errorStyle.Render("Error: "+m.err))
	}
	return strings.Join(lines, "\n")
}

// ── the Groups view ─────────────────────────────────────────────────────────

const (
	gvList = iota
	gvRename
	gvNew
	gvDelete
)

type groupsViewState struct {
	mode   int
	cursor int
	input  string
}

func (m Model) handleGroupsInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	gv := &m.gv
	client := m.client
	var sel *api.Group
	if gv.cursor >= 0 && gv.cursor < len(m.groups) {
		sel = &m.groups[gv.cursor]
	}
	switch gv.mode {
	case gvRename, gvNew:
		switch {
		case msg.Type == tea.KeyEnter:
			name := strings.TrimSpace(gv.input)
			if name == "" {
				m.err = "Give the group a name"
				return m, nil
			}
			m.err = ""
			mode := gv.mode
			gv.mode = gvList
			if mode == gvNew {
				return m, func() tea.Msg {
					_, err := client.CreateGroup(name, nil, false)
					return groupDoneMsg{Notice: "Group " + name + " created.", Err: err}
				}
			}
			if sel == nil {
				return m, nil
			}
			gid := sel.ID
			return m, func() tea.Msg {
				return groupDoneMsg{Notice: "Renamed to " + name + ".", Err: client.RenameGroup(gid, name)}
			}
		case msg.Type == tea.KeyBackspace:
			if r := []rune(gv.input); len(r) > 0 {
				gv.input = string(r[:len(r)-1])
			}
		case msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace:
			if len([]rune(gv.input)) < 50 {
				gv.input += msg.String()
			}
		}
		return m, nil
	case gvDelete:
		if msg.Type == tea.KeyEnter && sel != nil {
			gid, name := sel.ID, sel.Name
			gv.mode = gvList
			return m, func() tea.Msg {
				return groupDoneMsg{Notice: "Group " + name + " removed. Its containers are still there.", Err: client.DeleteGroup(gid)}
			}
		}
		gv.mode = gvList
		return m, nil
	}
	switch msg.Type {
	case tea.KeyUp:
		if gv.cursor > 0 {
			gv.cursor--
		}
	case tea.KeyDown:
		if gv.cursor < len(m.groups)-1 {
			gv.cursor++
		}
	case tea.KeyEnter, tea.KeySpace:
		if sel != nil {
			gid, linked, name := sel.ID, !sel.Linked, sel.Name
			word := "linked"
			if !linked {
				word = "unlinked"
			}
			return m, func() tea.Msg {
				return groupDoneMsg{Notice: name + " is now " + word + ".", Err: client.SetGroupLinked(gid, linked)}
			}
		}
	}
	switch msg.String() {
	case "n", "N":
		gv.mode, gv.input, m.err = gvNew, "", ""
	case "r", "R":
		if sel != nil {
			gv.mode, gv.input, m.err = gvRename, sel.Name, ""
		}
	case "d", "D":
		if sel != nil {
			gv.mode = gvDelete
		}
	}
	return m, nil
}

func (m Model) renderGroupsView() string {
	lines := []string{titleStyle.Render("Groups")}
	if m.notice != "" {
		lines = append(lines, statStyle.Render(m.notice), "")
	}
	if len(m.groups) == 0 {
		lines = append(lines, labelStyle.Render("No groups yet. Press n to make one, or use Move to group on a container."))
	} else {
		lines = append(lines, labelStyle.Render(fmt.Sprintf("  %-26s %-11s %s", "NAME", "CONTAINERS", "LINKED")))
	}
	for i, g := range m.groups {
		linked := "no"
		if g.Linked {
			linked = "yes: its containers can talk to each other"
		}
		prefix, style := "  ", listItemStyle
		if i == m.gv.cursor {
			prefix, style = "▶ ", listItemSelectedStyle
		}
		lines = append(lines, style.Render(fmt.Sprintf("%s%-26s %-11d %s", prefix, truncate(g.Name, 26), len(g.Members), linked)))
	}
	switch m.gv.mode {
	case gvNew:
		lines = append(lines, "", labelStyle.Render("Name for the new group:"), listItemStyle.Render("> "+m.gv.input+"_"))
	case gvRename:
		lines = append(lines, "", labelStyle.Render("New name:"), listItemStyle.Render("> "+m.gv.input+"_"))
	case gvDelete:
		if m.gv.cursor < len(m.groups) {
			lines = append(lines, "", errorStyle.Render(fmt.Sprintf("Remove the group %s? Its containers are NOT deleted. Enter = yes, any other key = no", m.groups[m.gv.cursor].Name)))
		}
	}
	if m.err != "" {
		lines = append(lines, "", errorStyle.Render("Error: "+m.err))
	}
	return strings.Join(lines, "\n")
}
