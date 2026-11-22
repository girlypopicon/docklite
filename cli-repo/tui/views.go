package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"
	rtruncate "github.com/muesli/reflow/truncate"
)

var (
	// Color scheme
	primaryColor   = lipgloss.Color("#FF6AD5")
	secondaryColor = lipgloss.Color("#FF9A8B")
	accentColor    = lipgloss.Color("#9AD0FF")
	errorColor     = lipgloss.Color("#FF7A8A")
	borderColor    = lipgloss.Color("#C7A4FF")
	textColor      = lipgloss.Color("#F7F1FF")
	dimTextColor   = lipgloss.Color("#CBB9E8")

	// Styles
	sidebarStyle = lipgloss.NewStyle().
			Width(20).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1)

	sidebarItemStyle = lipgloss.NewStyle().
				Foreground(textColor).
				PaddingLeft(2)

	sidebarItemSelectedStyle = lipgloss.NewStyle().
					Foreground(primaryColor).
					Bold(true).
					PaddingLeft(1)

	sidebarFocusedStyle = lipgloss.NewStyle().
				BorderForeground(lipgloss.Color("#B8F2A2"))

	mainPanelStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 2)

	mainPanelFocusedStyle = lipgloss.NewStyle().
				BorderForeground(lipgloss.Color("#FF6AD5"))

	footerStyle = lipgloss.NewStyle().
			Foreground(dimTextColor).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(borderColor).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1)

	statStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(dimTextColor)

	dimTextStyle = lipgloss.NewStyle().
			Foreground(dimTextColor)

	listItemStyle = lipgloss.NewStyle().
			Foreground(textColor).
			PaddingLeft(2)

	listItemSelectedStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true).
				PaddingLeft(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	buttonStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(lipgloss.Color("#374151")).
			Padding(0, 2).
			MarginRight(1)

	buttonActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(primaryColor).
				Padding(0, 2).
				MarginRight(1).
				Bold(true)

	decorStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)
)

func (m Model) View() string {
	// Show splash screen
	if m.showingSplash {
		return RenderSplash()
	}

	if !m.ready {
		return "Initializing..."
	}
	if m.authRequired {
		return m.renderAuthScreen()
	}

	// Build layout
	sidebar := m.renderSidebar()
	mainPanel := m.renderMainPanel()
	footer := m.renderFooter()

	// Calculate dimensions
	sidebarWidth := 22 // sidebar width + borders
	mainWidth := m.width - sidebarWidth - 2
	mainHeight := m.height - 4 // footer height

	// Apply dimensions and focus styles
	focusBorderColor := lipgloss.Color("#FF6AD5")
	sidebarStyled := sidebarStyle.Height(mainHeight)
	if m.focusArea == FocusSidebar {
		sidebarStyled = sidebarStyled.Copy().BorderForeground(focusBorderColor)
	}

	// Lipgloss treats Width and Height as minimums: a long list would grow the panel past the bottom of the terminal
	// and a wide row would wrap onto two lines. Clip the content to the panel's inner size first.
	mainPanel = fitPanel(mainPanel, m.innerWidth(), m.innerHeight())
	mainPanelStyled := mainPanelStyle.Width(mainWidth).Height(mainHeight)
	if m.focusArea == FocusMain {
		mainPanelStyled = mainPanelStyled.Copy().BorderForeground(focusBorderColor)
	}

	// Render layout
	topSection := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebarStyled.Render(sidebar),
		mainPanelStyled.Render(mainPanel),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		topSection,
		footerStyle.Width(m.width-2).Render(footer),
	)
}

func (m Model) renderSidebar() string {
	items := []string{"Dashboard", "Containers", "Databases", "Files", "Logs", "Network", "Settings", "Quit"}
	var lines []string

	lines = append(lines, m.centerLine(titleStyle.Render("D O C K L I T E")))
	lines = append(lines, m.centerLine(m.renderRainbowBar()))
	lines = append(lines, "")

	for i, item := range items {
		prefix := "  "
		style := sidebarItemStyle
		if i == m.sidebarSelected {
			prefix = "▶ "
			style = sidebarItemSelectedStyle
		}
		lines = append(lines, style.Render(prefix+item))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderRainbowBar() string {
	colors := []lipgloss.Color{
		lipgloss.Color("#FF6AD5"),
		lipgloss.Color("#FF9A8B"),
		lipgloss.Color("#FFD166"),
		lipgloss.Color("#B8F2A2"),
		lipgloss.Color("#9AD0FF"),
		lipgloss.Color("#C7A4FF"),
		lipgloss.Color("#FF6AD5"),
		lipgloss.Color("#FF9A8B"),
	}
	chunk := "█"
	var parts []string
	for _, color := range colors {
		parts = append(parts, lipgloss.NewStyle().Foreground(color).Render(chunk))
	}
	return strings.Join(parts, " ")
}

func (m Model) centerLine(text string) string {
	return lipgloss.PlaceHorizontal(20, lipgloss.Center, text)
}

func (m Model) renderMainPanel() string {
	switch m.currentScreen {
	case ScreenDashboard:
		return m.renderDashboard()
	case ScreenContainers:
		if m.currentView == ViewDetails && m.selectedContainer != nil {
			return m.renderContainerDetails()
		}
		if m.currentView == ViewCreateSite {
			return m.renderCreateSite()
		}
		return m.renderContainers()
	case ScreenDatabases:
		if m.currentView == ViewCreateDatabase {
			return m.renderCreateDatabase()
		}
		if m.currentView == ViewDatabaseDetails {
			return m.renderDatabaseDetails()
		}
		return m.renderDatabases()
	case ScreenFiles:
		if m.currentView == ViewFileEdit {
			return m.renderFileEditor()
		}
		return m.renderFiles()
	case ScreenLogs:
		return m.renderLogsScreen()
	case ScreenNetwork:
		return m.renderNetworkScreen()
	case ScreenSettings:
		return m.renderSettings()
	default:
		return "Unknown screen"
	}
}

func (m Model) renderDashboard() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Dashboard"))
	lines = append(lines, "")

	if m.err != "" {
		lines = append(lines, errorStyle.Render("Error: "+m.err))
		lines = append(lines, "")
		lines = append(lines, labelStyle.Render("Make sure Docklite API is running"))
		return strings.Join(lines, "\n")
	}

	lines = append(lines, labelStyle.Render("Health Status: ")+statStyle.Render(m.healthStatus))
	lines = append(lines, "")

	if m.summary != nil {
		lines = append(lines, titleStyle.Render("System Summary"))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Running Containers:"), statStyle.Render(fmt.Sprintf("%d", m.summary.ContainersRunning))))
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Stopped Containers:"), statStyle.Render(fmt.Sprintf("%d", m.summary.ContainersStopped))))
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Images:           "), statStyle.Render(fmt.Sprintf("%d", m.summary.ImagesCount))))
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Volumes:          "), statStyle.Render(fmt.Sprintf("%d", m.summary.VolumesCount))))
	} else {
		lines = append(lines, labelStyle.Render("Loading summary..."))
	}
	lines = append(lines, m.dashboardExtras()...)

	return strings.Join(lines, "\n")
}

func (m Model) renderContainers() string {
	var lines []string

	title := "Containers"
	if m.searchMode {
		title = fmt.Sprintf("Containers (Search: %s_)", m.searchQuery)
	} else if m.searchQuery != "" {
		title = fmt.Sprintf("Containers (Filter: %s)", m.searchQuery)
	}
	lines = append(lines, titleStyle.Render(title))
	lines = append(lines, "")

	if m.err != "" {
		lines = append(lines, errorStyle.Render("Error: "+m.err))
		return strings.Join(lines, "\n")
	}
	if m.notice != "" {
		lines = append(lines, statStyle.Render(m.notice))
		lines = append(lines, "")
	}

	// Columns share the panel's width: the box's name, what it is, and its status. Names come first and get the room,
	// because the point is to tell the boxes apart; the container ID and image are in the details.
	avail := m.innerWidth() - 4 // list items are indented by the cursor and the style's padding
	kindW, statusW := 14, 14
	nameW := avail - kindW - statusW - 2
	if nameW < 16 {
		kindW = 0
		nameW = max(8, avail-statusW-1)
	}
	row := func(name, kind, status string) string {
		if kindW == 0 {
			return fmt.Sprintf("%-*s %s", nameW, truncate(name, nameW), truncate(status, statusW))
		}
		return fmt.Sprintf("%-*s %-*s %s", nameW, truncate(name, nameW), kindW, truncate(kind, kindW), truncate(status, statusW))
	}
	header := row("NAME", "KIND", "STATUS")
	lines = append(lines, labelStyle.Render(header))
	lines = append(lines, labelStyle.Render(strings.Repeat("─", min(len(header), avail))))

	if len(m.filteredContainers) == 0 {
		if m.searchQuery != "" {
			lines = append(lines, labelStyle.Render("No containers match your search"))
		} else {
			lines = append(lines, labelStyle.Render("No containers found"))
		}
	} else {
		// Only as many rows as fit, keeping the cursor in view. Fixed lines: title and its margin, the blank after it,
		// the header and rule, then the blank, toggle, blank and total at the bottom; plus two for a notice. Group
		// headings take a line each, so leave room for them.
		fixed := 9 + len(m.groups) + 1
		if m.notice != "" {
			fixed += 2
		}
		start, end, above, below := listWindow(len(m.filteredContainers), m.containersCursor, m.innerHeight()-fixed)
		if above > 0 {
			lines = append(lines, labelStyle.Render(fmt.Sprintf("  ↑ %d more", above)))
		}
		count := map[string]int{}
		for _, c := range m.filteredContainers {
			count[m.groupOf[c.Name]]++
		}
		prevGroup := "\x00"
		if start > 0 {
			prevGroup = m.groupOf[m.filteredContainers[start-1].Name]
		}
		for i := start; i < end; i++ {
			container := m.filteredContainers[i]
			g := m.groupOf[container.Name]
			if g != prevGroup {
				if g != "" {
					lines = append(lines, accentHeading(fmt.Sprintf("▾ %s (%d)", g, count[g])))
				} else if len(m.groups) > 0 {
					lines = append(lines, accentHeading(fmt.Sprintf("▾ Not in a group (%d)", count[""])))
				}
				prevGroup = g
			}
			title := boxTitle(container)
			indent := ""
			if g != "" {
				indent = "  "
			}
			line := row(indent+title, boxKind(container), container.Status)

			style := listItemStyle
			prefix := "  "
			if i == m.containersCursor && m.focusArea == FocusMain {
				style = listItemSelectedStyle
				prefix = "▶ "
			}

			lines = append(lines, style.Render(prefix+line))
		}
		if below > 0 {
			lines = append(lines, labelStyle.Render(fmt.Sprintf("  ↓ %d more", below)))
		}
	}

	toggleLabel := "View all containers"
	if m.showAllContainers {
		toggleLabel = "View Docklite containers"
	}
	toggleStyle := listItemStyle
	togglePrefix := "  "
	if m.containersCursor == len(m.filteredContainers) && m.focusArea == FocusMain {
		toggleStyle = listItemSelectedStyle
		togglePrefix = "▶ "
	}

	lines = append(lines, "")
	lines = append(lines, toggleStyle.Render(togglePrefix+toggleLabel))
	lines = append(lines, "")
	lines = append(lines, labelStyle.Render(fmt.Sprintf("Total: %d containers", len(m.filteredContainers))))

	return strings.Join(lines, "\n")
}

func (m Model) renderDatabases() string {
	var lines []string
	title := "Databases"
	if m.dbSearchMode {
		title = fmt.Sprintf("Databases (Search: %s_)", m.dbSearchQuery)
	} else if m.dbSearchQuery != "" {
		title = fmt.Sprintf("Databases (Filter: %s)", m.dbSearchQuery)
	}
	lines = append(lines, titleStyle.Render(title))
	lines = append(lines, "")

	if m.err != "" {
		lines = append(lines, errorStyle.Render("Error: "+m.err))
		return strings.Join(lines, "\n")
	}
	if m.notice != "" {
		lines = append(lines, statStyle.Render(m.notice))
		lines = append(lines, "")
	}

	if len(m.filteredDatabases) == 0 {
		if m.dbSearchQuery != "" {
			lines = append(lines, labelStyle.Render("No databases match your search"))
		} else {
			lines = append(lines, labelStyle.Render("No databases found"))
		}
		return strings.Join(lines, "\n")
	}

	header := fmt.Sprintf("%-20s %-11s %-6s %-12s %s", "NAME", "KIND", "PORT", "USER", "STATUS")
	lines = append(lines, labelStyle.Render(header))
	lines = append(lines, labelStyle.Render(strings.Repeat("─", len(header))))

	fixed := 7 // title block, header and rule, blank and total
	if m.notice != "" {
		fixed += 2
	}
	start, end, above, below := listWindow(len(m.filteredDatabases), m.databasesCursor, m.innerHeight()-fixed)
	if above > 0 {
		lines = append(lines, labelStyle.Render(fmt.Sprintf("  ↑ %d more", above)))
	}
	for i := start; i < end; i++ {
		db := m.filteredDatabases[i]
		name := truncate(db.Name, 20)
		user := truncate(db.Username, 12)
		port := fmt.Sprintf("%d", db.Port)
		status := truncate(db.Status, 20)

		line := fmt.Sprintf("%-20s %-11s %-6s %-12s %s", name, databaseLabel(db.Type), port, user, status)
		style := listItemStyle
		prefix := "  "
		if i == m.databasesCursor && m.focusArea == FocusMain {
			style = listItemSelectedStyle
			prefix = "▶ "
		}
		lines = append(lines, style.Render(prefix+line))
	}
	if below > 0 {
		lines = append(lines, labelStyle.Render(fmt.Sprintf("  ↓ %d more", below)))
	}

	lines = append(lines, "")
	lines = append(lines, labelStyle.Render(fmt.Sprintf("Total: %d databases", len(m.filteredDatabases))))
	return strings.Join(lines, "\n")
}

func (m Model) renderFiles() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Files"))
	lines = append(lines, "")
	pathLabel := "/var/www/sites"
	if m.filePath != "" {
		pathLabel += "/" + m.filePath
	}
	lines = append(lines, labelStyle.Render("Path: ")+statStyle.Render(pathLabel))
	lines = append(lines, "")

	if m.err != "" {
		lines = append(lines, errorStyle.Render("Error: "+m.err))
		return strings.Join(lines, "\n")
	}
	if m.notice != "" {
		lines = append(lines, statStyle.Render(m.notice))
		lines = append(lines, "")
	}

	if len(m.fileEntries) == 0 {
		lines = append(lines, labelStyle.Render("No files found"))
	} else {
		header := fmt.Sprintf("%-30s %-8s %s", "NAME", "TYPE", "SIZE")
		lines = append(lines, labelStyle.Render(header))
		lines = append(lines, labelStyle.Render(strings.Repeat("─", len(header))))

		fixed := 9 // title block, path line and blank, header and rule, blank and hint at the bottom
		if m.notice != "" {
			fixed += 2
		}
		start, end, above, below := listWindow(len(m.fileEntries), m.fileListCursor, m.innerHeight()-fixed)
		if above > 0 {
			lines = append(lines, labelStyle.Render(fmt.Sprintf("  ↑ %d more", above)))
		}
		for i := start; i < end; i++ {
			entry := m.fileEntries[i]
			name := entry.Name
			if entry.IsDir {
				name += "/"
			}
			name = truncate(name, 30)
			entryType := "file"
			if entry.IsDir {
				entryType = "dir"
			}
			size := fmt.Sprintf("%d", entry.Size)
			if entry.IsDir {
				size = "-"
			}
			line := fmt.Sprintf("%-30s %-8s %s", name, entryType, size)

			style := listItemStyle
			prefix := "  "
			if i == m.fileListCursor && m.focusArea == FocusMain {
				style = listItemSelectedStyle
				prefix = "▶ "
			}
			lines = append(lines, style.Render(prefix+line))
		}
		if below > 0 {
			lines = append(lines, labelStyle.Render(fmt.Sprintf("  ↓ %d more", below)))
		}
	}

	if m.filePromptMode != FilePromptNone {
		lines = append(lines, "")
		lines = append(lines, m.renderFilePrompt())
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderFileEditor() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Edit File"))
	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("File: ")+statStyle.Render(m.fileActivePath))
	if m.fileDirty {
		lines = append(lines, dimTextStyle.Render("Unsaved changes"))
	}
	lines = append(lines, "")

	if m.err != "" {
		lines = append(lines, errorStyle.Render("Error: "+m.err))
		lines = append(lines, "")
	}

	content := m.renderFileWithCursor()
	if content == "" {
		content = "(empty)"
	}
	lines = append(lines, content)
	return strings.Join(lines, "\n")
}

func (m Model) renderFilePrompt() string {
	prompt := ""
	switch m.filePromptMode {
	case FilePromptCreateFile:
		prompt = "New file name: " + m.filePromptInput + "_"
	case FilePromptCreateFolder:
		prompt = "New folder name: " + m.filePromptInput + "_"
	case FilePromptRename:
		prompt = "Rename to: " + m.filePromptInput + "_"
	case FilePromptDelete:
		prompt = "Press Enter to delete selected path"
	}
	return dimTextStyle.Render(prompt)
}

func (m Model) renderFileWithCursor() string {
	runes := []rune(m.fileContent)
	cursor := m.fileEditCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	cursorStyle := lipgloss.NewStyle().Foreground(accentColor).Bold(true).Reverse(true)
	var out strings.Builder
	for i, r := range runes {
		if i == cursor {
			if r == '\n' {
				out.WriteString(cursorStyle.Render(" "))
				out.WriteRune('\n')
			} else {
				out.WriteString(cursorStyle.Render(string(r)))
			}
			continue
		}
		out.WriteRune(r)
	}
	if cursor == len(runes) {
		out.WriteString(cursorStyle.Render(" "))
	}
	return out.String()
}

func (m Model) renderDatabaseDetails() string {
	if m.selectedDatabase == nil {
		return "No database selected"
	}

	var lines []string
	db := m.selectedDatabase

	lines = append(lines, titleStyle.Render("Database Details"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Name:    "), statStyle.Render(db.Name)))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Kind:    "), statStyle.Render(databaseLabel(db.Type))))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Port:    "), statStyle.Render(fmt.Sprintf("%d", db.Port))))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("User:    "), statStyle.Render(db.Username)))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Password:"), statStyle.Render(db.Password)))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Host:    "), statStyle.Render("localhost")))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Status:  "), statStyle.Render(db.Status)))
	lines = append(lines, "")
	if url := dbConnectURL(*db); url != "" {
		lines = append(lines, dimTextStyle.Render(truncate("Connect: "+url, m.innerWidth())))
	}
	lines = append(lines, "")
	lines = append(lines, titleStyle.Render("Actions"))
	lines = append(lines, "")

	actions := []string{"Start", "Stop", "Restart", "Delete"}
	for i, action := range actions {
		style := buttonStyle
		if i == m.dbActionCursor {
			style = buttonActiveStyle
		}
		lines = append(lines, style.Render(action))
	}

	if m.dbConfirmDelete {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render("Press Enter again to delete this database"))
	}

	if m.err != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render("Error: "+m.err))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderCreateDatabase() string {
	var lines []string

	lines = append(lines, titleStyle.Render("Create Database"))
	lines = append(lines, "")

	lines = append(lines, labelStyle.Render("Step 1: Database Name"))
	nameLine := "> " + m.createDbName
	if m.createDbStep == CreateDbStepName {
		nameLine += "_"
	}
	lines = append(lines, listItemStyle.Render(nameLine))
	lines = append(lines, "")

	lines = append(lines, labelStyle.Render("Step 2: Kind of database (↑/↓ to choose)"))
	var kinds []string
	for i, t := range databaseTypes {
		label := databaseLabel(t)
		if i == m.createDbTypeIdx {
			label = "[" + label + "]"
		}
		kinds = append(kinds, label)
	}
	typeLine := "> " + strings.Join(kinds, "  ")
	if m.createDbStep == CreateDbStepType {
		typeLine += "_"
	}
	lines = append(lines, listItemStyle.Render(typeLine))
	lines = append(lines, "")

	lines = append(lines, labelStyle.Render("Step 3: Username (optional)"))
	userLine := "> " + m.createDbUser
	if m.createDbStep == CreateDbStepUser {
		userLine += "_"
	}
	lines = append(lines, listItemStyle.Render(userLine))
	lines = append(lines, "")

	lines = append(lines, labelStyle.Render("Step 4: Password (optional)"))
	passLine := "> " + maskToken(m.createDbPassword)
	if m.createDbStep == CreateDbStepPassword {
		passLine += "_"
	}
	lines = append(lines, listItemStyle.Render(passLine))
	lines = append(lines, "")

	lines = append(lines, labelStyle.Render("Step 5: Port (optional)"))
	portValue := m.createDbPort
	if strings.TrimSpace(portValue) == "" {
		portValue = "auto"
	}
	portLine := "> " + portValue
	if m.createDbStep == CreateDbStepPort {
		portLine += "_"
	}
	lines = append(lines, listItemStyle.Render(portLine))
	lines = append(lines, "")

	if m.creatingDatabase {
		lines = append(lines, labelStyle.Render("Creating database..."))
	} else {
		lines = append(lines, dimTextStyle.Render("Press Enter to advance or create"))
	}

	if m.err != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render("Error: "+m.err))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderAuthScreen() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Agent Login"))
	lines = append(lines, "")
	lines = append(lines, dimTextStyle.Render("Enter the agent URL and token. Leave token empty if auth is disabled."))
	lines = append(lines, "")

	urlLine := "Agent URL: " + m.authURLInput
	tokenLine := "Token: " + maskToken(m.authTokenInput)

	if m.authFieldIndex == 0 {
		urlLine = "> " + urlLine + "_"
	} else {
		urlLine = "  " + urlLine
	}
	if m.authFieldIndex == 1 {
		tokenLine = "> " + tokenLine + "_"
	} else {
		tokenLine = "  " + tokenLine
	}

	lines = append(lines, listItemStyle.Render(urlLine))
	lines = append(lines, listItemStyle.Render(tokenLine))
	lines = append(lines, "")
	lines = append(lines, dimTextStyle.Render("Press Enter to save • ↑/↓ to move • Ctrl+C to quit"))

	return strings.Join(lines, "\n")
}

func (m Model) renderCreateSite() string {
	var lines []string

	lines = append(lines, titleStyle.Render("Create Web Container"))
	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("Step 1: Domain"))
	domainLine := "> " + m.createDomain
	if m.createStep == CreateStepDomain {
		domainLine += "_"
	}
	lines = append(lines, listItemStyle.Render(domainLine))
	lines = append(lines, "")

	lines = append(lines, labelStyle.Render("Step 2: Template"))
	template := strings.ToUpper(m.selectedTemplate())
	templateLine := fmt.Sprintf("> %s", template)
	if m.createStep == CreateStepTemplate {
		templateLine += " (use ↑/↓)"
	}
	lines = append(lines, listItemStyle.Render(templateLine))
	lines = append(lines, "")

	if m.selectedTemplate() == "node" {
		lines = append(lines, labelStyle.Render("Step 3: Node Port"))
		portLine := "> " + m.createPortInput
		if m.createStep == CreateStepPort {
			portLine += "_"
		}
		lines = append(lines, listItemStyle.Render(portLine))
		lines = append(lines, "")
	}

	lines = append(lines, labelStyle.Render("Step 4: SSL Hosts"))
	wwwStatus := "www + @"
	if !m.createIncludeWWW {
		wwwStatus = "@ only"
	}
	sslLine := fmt.Sprintf("> %s", wwwStatus)
	if m.createStep == CreateStepSSL {
		sslLine += " (space toggles)"
	}
	lines = append(lines, listItemStyle.Render(sslLine))
	lines = append(lines, "")

	if m.creatingSite {
		lines = append(lines, labelStyle.Render("Creating container..."))
	} else {
		lines = append(lines, dimTextStyle.Render("Press Enter to advance or create"))
	}

	if m.err != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render("Error: "+m.err))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderContainerDetails() string {
	if m.showingLogs {
		return m.renderContainerLogs()
	}
	if m.showingStats {
		return m.renderContainerStats()
	}

	var lines []string
	c := m.selectedContainer

	lines = append(lines, titleStyle.Render("Container Details"))
	lines = append(lines, "")

	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Name:   "), statStyle.Render(boxTitle(*c))))
	if g := m.groupOf[c.Name]; g != "" {
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Group:  "), g))
	}
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Kind:   "), boxKind(*c)))
	if c.Owner != "" {
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Owner:  "), c.Owner))
	}
	if c.Ports != "" {
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Ports:  "), c.Ports))
	}
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Box:    "), c.Name))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("ID:     "), truncate(c.ID, 12)))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Image:  "), c.Image))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Status: "), c.Status))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("State:  "), c.State))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Created:"), time.Unix(c.Created, 0).Local().Format("2006-01-02 15:04")))
	lines = append(lines, "")
	lines = append(lines, titleStyle.Render("Actions"))
	lines = append(lines, "")

	// Action buttons
	if m.ctMode != 0 {
		return m.renderChangeType()
	}
	actions := m.detailActions()
	for i, action := range actions {
		style := buttonStyle
		if i == m.actionCursor {
			style = buttonActiveStyle
		}
		lines = append(lines, style.Render(action))
	}

	if m.containerConfirmDelete {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render("Press Enter again to delete this container"))
	}

	if m.err != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render("Error: "+m.err))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderContainerLogs() string {
	var lines []string
	lines = append(lines, titleStyle.Render(fmt.Sprintf("Logs: %s", m.selectedContainer.Name)))
	lines = append(lines, "")

	if m.containerLogs == "" {
		lines = append(lines, labelStyle.Render("No logs available"))
	} else {
		// Split logs into lines and display
		logLines := strings.Split(m.containerLogs, "\n")
		maxLines := 50 // Show last 50 lines
		start := 0
		if len(logLines) > maxLines {
			start = len(logLines) - maxLines
		}
		for _, line := range logLines[start:] {
			if line != "" {
				lines = append(lines, dimTextStyle.Render(line))
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("Press Esc to go back"))

	return strings.Join(lines, "\n")
}

func (m Model) renderContainerStats() string {
	var lines []string
	name := "container"
	if m.selectedContainer != nil {
		name = m.selectedContainer.Name
	}
	lines = append(lines, titleStyle.Render(fmt.Sprintf("Stats: %s", name)))
	lines = append(lines, "")

	if m.containerStats == nil {
		lines = append(lines, labelStyle.Render("No stats available"))
		lines = append(lines, "")
		lines = append(lines, labelStyle.Render("Press Esc to go back"))
		return strings.Join(lines, "\n")
	}

	cpu := formatPercent(m.containerStats.CPUUsage)
	memUsed := formatBytes(m.containerStats.MemoryUsage)
	memLimit := formatBytes(m.containerStats.MemoryLimit)
	memPct := formatPercent(m.containerStats.MemoryPct)

	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("CPU Usage:"), statStyle.Render(cpu)))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Memory:"), statStyle.Render(memUsed+" / "+memLimit)))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Memory %:"), statStyle.Render(memPct)))
	lines = append(lines, "")
	lines = append(lines, dimTextStyle.Render("CPU Trend:  "+trendLine(m.statsHistoryCPU, 10)))
	lines = append(lines, dimTextStyle.Render("Memory %:  "+trendLine(m.statsHistoryMem, 10)))
	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("Press Esc to go back"))

	return strings.Join(lines, "\n")
}

func (m Model) renderUpdate() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Update DockLite"))
	lines = append(lines, "")
	st := m.updStatus
	if st == nil && m.updErr == "" {
		lines = append(lines, labelStyle.Render("Checking for a newer version..."))
		return strings.Join(lines, "\n")
	}
	if st != nil {
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Installed:"), statStyle.Render(st.Version)))
		latest := st.LatestVersion
		if latest == "" {
			latest = "unknown"
		}
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Newest:   "), statStyle.Render(latest)))
		lines = append(lines, "")
		switch {
		case m.updRunning:
			lines = append(lines, statStyle.Render("Updating..."))
		case st.UpdateAvailable:
			lines = append(lines, statStyle.Render("A newer version is available."))
			lines = append(lines, labelStyle.Render("One update covers the agent, the web dashboard (if installed) and this terminal app."))
			lines = append(lines, "")
			if m.updConfirm {
				lines = append(lines, errorStyle.Render("Press Enter again to install "+latest+" (your sites keep running)"))
			} else {
				lines = append(lines, buttonActiveStyle.Render("Install "+latest))
			}
		case st.CheckError != "":
			lines = append(lines, errorStyle.Render("Couldn't check for updates: "+st.CheckError))
		default:
			lines = append(lines, statStyle.Render("You're up to date."))
		}
	}
	if m.updNote != "" {
		lines = append(lines, "")
		lines = append(lines, statStyle.Render(m.updNote))
	}
	if m.updErr != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(m.updErr))
	}
	if st != nil && len(st.Log) > 0 && (m.updRunning || st.State == "failed" || st.State == "rolled-back") {
		lines = append(lines, "", labelStyle.Render("Progress"))
		log := st.Log
		room := max(3, m.innerHeight()-len(lines)-2)
		if len(log) > room {
			log = log[len(log)-room:]
		}
		for _, l := range log {
			lines = append(lines, dimTextStyle.Render(truncate(l, m.innerWidth())))
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderFooter() string {
	focusIndicator := ""
	if m.focusArea == FocusSidebar {
		focusIndicator = "[SIDEBAR FOCUSED] "
	} else {
		focusIndicator = "[MAIN PANEL FOCUSED] "
	}

	help := focusIndicator + "↑/↓ move • Enter select • Tab focus • Esc back"
	if m.currentScreen == ScreenContainers && m.currentView == ViewList && !m.searchMode {
		help += " • / search • n new site"
	}
	if m.currentScreen == ScreenContainers && m.currentView == ViewCreateSite {
		help = focusIndicator + "Type input • ↑/↓ choose • Space toggle • Enter next/create • Esc cancel"
	}
	if m.currentScreen == ScreenContainers && m.currentView == ViewDetails {
		help = focusIndicator + "↑/↓ move • Enter action • Esc back"
		if m.containerConfirmDelete {
			help = focusIndicator + "Enter confirms delete • Esc cancel"
		}
	}
	if m.currentScreen == ScreenDatabases && m.currentView == ViewList {
		if m.dbSearchMode {
			help = focusIndicator + "Type to search • Enter to apply • Esc cancel"
		} else {
			help += " • / search • n new db"
		}
	}
	if m.currentScreen == ScreenDatabases && m.currentView == ViewCreateDatabase {
		help = focusIndicator + "Type input • Enter next/create • Esc cancel"
	}
	if m.currentScreen == ScreenDatabases && m.currentView == ViewDatabaseDetails {
		help = focusIndicator + "↑/↓ move • Enter action • Esc back"
		if m.dbConfirmDelete {
			help = focusIndicator + "Enter confirms delete • Esc cancel"
		}
	}
	switch m.currentScreen {
	case ScreenSettings:
		help = focusIndicator + "↑/↓ move • Enter open • Esc back"
		if m.x.setView == 1 {
			help = focusIndicator + "Enter install • r recheck • Esc back"
		}
	case ScreenLogs:
		help = focusIndicator + "←/→ which log • ↑/↓ PgUp/PgDn scroll • f follow • r refresh • Esc back"
	case ScreenNetwork:
		help = focusIndicator + "↑/↓ scroll • d test the connection • r refresh • Esc back"
	}
	if m.currentScreen == ScreenFiles && m.currentView == ViewList {
		if m.filePromptMode != FilePromptNone {
			help = focusIndicator + "Type name • Enter confirm • Esc cancel"
		} else {
			help = focusIndicator + "↑/↓ move • Enter open • Backspace up • c file • C folder • r rename • d delete"
		}
	}
	if m.currentScreen == ScreenFiles && m.currentView == ViewFileEdit {
		help = focusIndicator + "Type to edit • Ctrl+O save • Ctrl+X exit • Esc back"
	}
	help += " • Ctrl+C quit"

	return help
}

// innerWidth and innerHeight are the main panel's content area: the terminal minus the sidebar, the borders, the
// footer and the panel's own padding. Never below 1, so a tiny terminal degrades instead of panicking.
func (m Model) innerWidth() int {
	return max(1, m.width-22-2-4) // sidebar with borders, panel borders, panel padding
}

func (m Model) innerHeight() int {
	return max(1, m.height-4-2) // footer, panel's vertical padding
}

// fitPanel clips content to width columns and height lines so lipgloss never wraps a long row or grows the panel
// past the bottom of the terminal. Truncation is ANSI-aware, so coloured text keeps its styling.
func fitPanel(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		if ansi.PrintableRuneWidth(line) > width {
			lines[i] = rtruncate.String(line, uint(width))
		}
	}
	return strings.Join(lines, "\n")
}

// listWindow picks which rows of a list to draw so the cursor is always on screen: rows [start, end), with above
// and below the counts hidden off each edge. An indicator line is reserved for each hidden edge.
func listWindow(total, cursor, visible int) (start, end, above, below int) {
	if visible < 1 {
		visible = 1
	}
	if total <= visible {
		return 0, total, 0, 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= total {
		cursor = total - 1
	}
	// Reserve a line for each "more" indicator so the rows plus indicators still fit. With fewer than three lines
	// there is no room for indicators: draw what fits and say nothing about the rest.
	rows := max(1, visible-2)
	indicators := visible >= 3
	if !indicators {
		rows = visible
	}
	start = cursor - rows/2
	if start < 0 {
		start = 0
	}
	end = start + rows
	if end > total {
		end = total
		start = max(0, end-rows)
	}
	if !indicators {
		return start, end, 0, 0
	}
	return start, end, start, total - end
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(value)/float64(div), "KMGTPE"[exp])
}

func trendLine(series []float64, maxSamples int) string {
	if len(series) == 0 {
		return "no data"
	}
	if maxSamples <= 0 {
		maxSamples = 10
	}
	start := 0
	if len(series) > maxSamples {
		start = len(series) - maxSamples
	}
	var out []string
	for _, v := range series[start:] {
		out = append(out, formatPercent(v))
	}
	return strings.Join(out, " ")
}

func formatPercent(value float64) string {
	if value <= 0 {
		return "00.0%"
	}
	if value < 1 {
		rounded := math.Round(value*10) / 10
		return fmt.Sprintf("00.%d%%", int(rounded*10))
	}
	if value >= 100 {
		return "99.9%"
	}
	rounded := math.Round(value*10) / 10
	return fmt.Sprintf("%04.1f%%", rounded)
}

func maskToken(token string) string {
	if token == "" {
		return "(empty)"
	}
	return strings.Repeat("*", len(token))
}

// RenderSplash renders the startup splash screen
func RenderSplash() string {
	logo := `
 ██████╗  ██████╗  ██████╗██╗  ██╗     ██╗ ██╗    ██╗     ██╗████████╗███████╗
 ██╔══██╗██╔═══██╗██╔════╝██║ ██╔╝    ██╔╝██╔╝    ██║     ██║╚══██╔══╝██╔════╝
 ██║  ██║██║   ██║██║     █████╔╝    ██╔╝██╔╝     ██║     ██║   ██║   █████╗  
 ██║  ██║██║   ██║██║     ██╔═██╗   ██╔╝██╔╝      ██║     ██║   ██║   ██╔══╝  
 ██████╔╝╚██████╔╝╚██████╗██║  ██╗ ██╔╝██╔╝       ███████╗██║   ██║   ███████╗
 ╚═════╝  ╚═════╝  ╚═════╝╚═╝  ╚═╝ ╚═╝ ╚═╝        ╚══════╝╚═╝   ╚═╝   ╚══════╝
`

	starfield := []string{
		"     *     .     *     .     *",
		"  .     *     .     *     .   ",
		"*     .     *     .     *     ",
	}

	// Rainbow gradient for the logo lines
	rainbow := []lipgloss.Color{
		lipgloss.Color("#FF6AD5"),
		lipgloss.Color("#FF9A8B"),
		lipgloss.Color("#FFD166"),
		lipgloss.Color("#B8F2A2"),
		lipgloss.Color("#9AD0FF"),
		lipgloss.Color("#C7A4FF"),
	}

	logoLines := strings.Split(strings.Trim(logo, "\n"), "\n")
	var styledLogo []string
	for i, line := range logoLines {
		color := rainbow[i%len(rainbow)]
		styledLogo = append(styledLogo, lipgloss.NewStyle().Foreground(color).Bold(true).Render(line))
	}
	logoRender := strings.Join(styledLogo, "\n")

	taglineStyle := lipgloss.NewStyle().
		Foreground(dimTextColor).
		Italic(true)

	// Combine elements
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		strings.Join(starfield, "\n"),
		"",
		logoRender,
		"",
		taglineStyle.Render("Lightweight Docker Management"),
	)

	// Center everything on screen
	return lipgloss.Place(
		100, 35,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func accentHeading(text string) string { return statStyle.Render(text) }

func (m Model) renderChangeType() string {
	c := m.selectedContainer
	lines := []string{titleStyle.Render("Change site type"), "",
		fmt.Sprintf("%s %s", labelStyle.Render("Site:"), statStyle.Render(boxTitle(*c))),
		fmt.Sprintf("%s %s", labelStyle.Render("Now: "), siteTypes[siteTypeIndex(siteTypeOf(*c))].label), ""}
	for i, t := range siteTypes {
		prefix, style := "  ", listItemStyle
		if i == m.ctCursor {
			prefix, style = "▶ ", listItemSelectedStyle
		}
		lines = append(lines, style.Render(fmt.Sprintf("%s%-16s %s", prefix, t.label, labelStyle.Render(t.hint))))
	}
	switch m.ctMode {
	case 1:
		lines = append(lines, "", dimTextStyle.Render("↑/↓ choose • Enter next • Esc cancel"))
	case 2:
		port := m.ctPort
		if port == "" {
			port = "3000 (default)"
		}
		lines = append(lines, "", labelStyle.Render("Which port does your app listen on?"), listItemStyle.Render("> "+port+"_"))
	case 3:
		lines = append(lines, "",
			labelStyle.Render("Your files, address and HTTPS stay as they are. The site restarts as the new kind"),
			labelStyle.Render("(a few seconds offline), and goes back to the old kind if that fails."),
			errorStyle.Render("Press Enter to switch to "+siteTypes[m.ctCursor].label+" • Esc cancel"))
	}
	if m.err != "" {
		lines = append(lines, "", errorStyle.Render("Error: "+m.err))
	}
	return strings.Join(lines, "\n")
}
