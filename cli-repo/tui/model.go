package tui

import (
	"docklite-tui/api"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type Screen int

const (
	ScreenDashboard Screen = iota
	ScreenContainers
	ScreenDatabases
	ScreenFiles
	ScreenLogs
	ScreenNetwork
	ScreenSettings
)

type FocusArea int

const (
	FocusSidebar FocusArea = iota
	FocusMain
)

type View int

const (
	ViewList View = iota
	ViewDetails
	ViewCreateSite
	ViewCreateDatabase
	ViewDatabaseDetails
	ViewFileEdit
)

type FilePromptMode int

const (
	FilePromptNone FilePromptMode = iota
	FilePromptCreateFile
	FilePromptCreateFolder
	FilePromptRename
	FilePromptDelete
)

type CreateStep int

const (
	CreateStepDomain CreateStep = iota
	CreateStepTemplate
	CreateStepPort
	CreateStepSSL
)

type CreateDbStep int

const (
	CreateDbStepName CreateDbStep = iota
	CreateDbStepType
	CreateDbStepUser
	CreateDbStepPassword
	CreateDbStepPort
)

type Model struct {
	// API client
	client *api.Client

	// Navigation state
	currentScreen   Screen
	sidebarSelected int
	focusArea       FocusArea
	currentView     View

	// Containers screen
	containers         []api.Container
	containersCursor   int
	selectedContainer  *api.Container
	searchMode         bool
	searchQuery        string
	filteredContainers []api.Container
	actionCursor       int // For selecting actions in details view

	// Change site type (details of a website)
	ctMode                 int // 0 off, 1 choosing the kind, 2 typing the port, 3 confirming
	ctCursor               int
	ctPort                 string
	containerLogs          string
	showingLogs            bool
	containerStats         *api.ContainerStats
	showingStats           bool
	statsHistoryCPU        []float64
	statsHistoryMem        []float64
	statsContainerID       string
	containerConfirmDelete bool
	showAllContainers      bool
	groups                 []api.Group
	groupOf                map[string]string // container name -> the group it sits in

	// Logs, Network, Settings and the dashboard extras (extra.go)
	x extraState

	// Update screen
	updStatus  *api.UpdateStatus
	updConfirm bool
	updRunning bool // we started an update (or one was already running) and are watching it
	updNote    string
	updErr     string

	// Databases screen
	databases         []api.Database
	filteredDatabases []api.Database
	databasesCursor   int
	selectedDatabase  *api.Database
	dbSearchMode      bool
	dbSearchQuery     string
	dbActionCursor    int
	dbConfirmDelete   bool

	// Files screen
	fileEntries     []api.FileEntry
	fileListCursor  int
	filePath        string
	fileActivePath  string
	fileContent     string
	fileDirty       bool
	fileEditCursor  int
	filePromptMode  FilePromptMode
	filePromptInput string

	// Create site flow
	createDomain        string
	createIncludeWWW    bool
	createTemplateIndex int
	createPortInput     string
	createStep          CreateStep
	creatingSite        bool

	// Create database flow
	createDbName     string
	createDbTypeIdx  int
	createDbUser     string
	createDbPassword string
	createDbPort     string
	createDbStep     CreateDbStep
	creatingDatabase bool

	// Auth flow
	authRequired   bool
	authURLInput   string
	authTokenInput string
	authFieldIndex int

	// Dashboard data
	summary      *api.SummaryResponse
	healthStatus string

	// UI state
	width         int
	height        int
	ready         bool
	showingSplash bool

	// Error state
	err    string
	notice string
}

type tickMsg time.Time
type splashDoneMsg struct{}
type summaryMsg *api.SummaryResponse
type containersMsg *api.ContainersResponse
type healthMsg *api.HealthResponse
type errMsg error
type updateStatusMsg struct {
	Status *api.UpdateStatus
	Err    error
}
type updateStartedMsg struct{ Err error }
type updatePollMsg struct{}
type siteTypeDoneMsg struct {
	Notice  string
	Warning string
	Err     error
}
type containerActionSuccessMsg struct {
	Notice       string
	ReturnToList bool
}
type containerLogsMsg string
type containerStatsMsg *api.ContainerStats
type createSiteSuccessMsg string
type databasesMsg *api.DatabasesResponse
type createDatabaseSuccessMsg struct {
	Name     string
	Port     int
	Username string
	Password string
}
type databaseActionSuccessMsg string
type filesMsg struct {
	Path    string
	Entries []api.FileEntry
}
type fileContentMsg struct {
	Path    string
	Content string
}
type fileActionSuccessMsg struct {
	Notice     string
	ClearDirty bool
}

func InitialModel() Model {
	model := Model{
		client:           api.NewClient(),
		currentScreen:    ScreenDashboard,
		sidebarSelected:  0,
		focusArea:        FocusSidebar,
		currentView:      ViewList,
		healthStatus:     "unknown",
		showingSplash:    true,
		createIncludeWWW: true,
		createPortInput:  "3000",
		createDbPort:     "",
		filePath:         "",
	}
	if cfg, err := LoadConfig(); err == nil {
		if os.Getenv("DOCKLITE_URL") == "" && cfg.BaseURL != "" {
			model.client.SetBaseURL(cfg.BaseURL)
		}
		if os.Getenv("DOCKLITE_TOKEN") == "" && cfg.Token != "" {
			model.client.SetToken(cfg.Token)
		}
	}
	model.authURLInput = model.client.BaseURL
	model.authTokenInput = model.client.Token
	return model
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		splashCmd(),
		fetchHealth(m.client),
		fetchSummary(m.client),
		fetchContainers(m.client),
		fetchDatabases(m.client),
		fetchFiles(m.client, m.filePath),
		m.refreshExtras(),
		tickCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if nm, cmd, ok := m.handleExtraMsg(msg); ok {
		return nm, cmd
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't process keys during splash
		if m.showingSplash {
			return m, nil
		}
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.showingSplash {
			m.ready = true
		}
		return m, nil

	case splashDoneMsg:
		m.showingSplash = false
		m.ready = true
		return m, nil

	case tickMsg:
		// Refresh data every 10 seconds
		if m.authRequired {
			return m, tickCmd()
		}
		cmds := []tea.Cmd{
			fetchSummary(m.client),
			fetchContainers(m.client),
			fetchDatabases(m.client),
			tickCmd(),
		}
		if m.showingStats && m.selectedContainer != nil {
			cmds = append(cmds, fetchContainerStats(m.client, m.selectedContainer.ID))
		}
		if m.currentScreen == ScreenDashboard {
			cmds = append(cmds, fetchOverviewCmd(m.client), fetchSSLCmd(m.client))
		}
		if m.currentScreen == ScreenLogs && m.x.logFollow {
			cmds = append(cmds, fetchLogsCmd(m.client, m.x.logTarget))
		}
		if m.currentScreen == ScreenFiles && m.currentView == ViewList && m.filePromptMode == FilePromptNone {
			cmds = append(cmds, fetchFiles(m.client, m.filePath))
		}
		return m, tea.Batch(cmds...)

	case healthMsg:
		m.healthStatus = msg.Status
		return m, nil

	case updateStatusMsg:
		if msg.Err != nil {
			// While an update restarts the agent, requests fail for a few seconds: that is expected, not an error.
			if m.updRunning {
				m.updNote = "The server is restarting with the new version..."
				return m, updatePollCmd()
			}
			m.updErr = msg.Err.Error()
			return m, nil
		}
		m.updErr = ""
		m.updStatus = msg.Status
		if msg.Status.UpdateRunning || msg.Status.State == "running" {
			m.updRunning = true
			return m, updatePollCmd()
		}
		if m.updRunning {
			m.updRunning = false
			switch msg.Status.State {
			case "success":
				m.updNote = "Updated to " + msg.Status.Version + ": the agent, the web dashboard and this terminal app. Quit (Ctrl+C) and run docklite tui again to use the new terminal app."
			case "failed", "rolled-back":
				m.updErr = "The update didn't finish: " + msg.Status.StateMessage
			}
		}
		return m, nil

	case updateStartedMsg:
		if msg.Err != nil {
			m.updRunning = false
			m.updErr = msg.Err.Error()
			return m, nil
		}
		m.updRunning = true
		m.updNote = "Update started. This takes a minute or two; your sites keep running."
		return m, updatePollCmd()

	case updatePollMsg:
		if m.updRunning {
			return m, fetchUpdateStatus(m.client, false)
		}
		return m, nil

	case summaryMsg:
		m.summary = msg
		m.err = ""
		return m, nil

	case containersMsg:
		m.containers = msg.Containers
		m.groups = msg.Groups
		m.updateFilteredContainers()
		m.err = ""
		return m, nil

	case databasesMsg:
		m.databases = msg.Databases
		m.updateFilteredDatabases()
		m.err = ""
		return m, nil

	case filesMsg:
		m.fileEntries = msg.Entries
		m.filePath = msg.Path
		m.err = ""
		if m.fileListCursor >= len(m.fileEntries) {
			m.fileListCursor = 0
		}
		return m, nil

	case errMsg:
		if apiErr, ok := error(msg).(*api.APIError); ok && apiErr.Status == 401 {
			m.authRequired = true
			m.authURLInput = m.client.BaseURL
			m.authTokenInput = m.client.Token
			m.authFieldIndex = 0
			m.err = ""
			m.creatingSite = false
			m.creatingDatabase = false
			return m, nil
		}
		m.err = msg.Error()
		m.creatingSite = false
		m.creatingDatabase = false
		return m, nil

	case siteTypeDoneMsg:
		m.ctMode = 0
		if msg.Err != nil {
			m.err = friendlyErr(msg.Err)
			if e, ok := msg.Err.(*api.APIError); ok {
				m.err = apiMessage(e)
			}
			return m, nil
		}
		m.err = ""
		m.notice = msg.Notice
		if msg.Warning != "" {
			m.notice += " " + msg.Warning
		}
		m.currentView = ViewList
		m.selectedContainer = nil
		return m, fetchContainers(m.client)

	case containerActionSuccessMsg:
		if msg.Notice != "" {
			m.notice = msg.Notice
		}
		m.containerConfirmDelete = false
		if msg.ReturnToList {
			m.currentView = ViewList
			m.selectedContainer = nil
			m.actionCursor = 0
		}
		// Refresh containers after action
		return m, fetchContainers(m.client)

	case containerLogsMsg:
		m.containerLogs = string(msg)
		m.showingLogs = true
		m.showingStats = false
		m.err = ""
		return m, nil

	case containerStatsMsg:
		if m.selectedContainer != nil {
			if m.statsContainerID == "" || m.statsContainerID != m.selectedContainer.ID {
				m.statsContainerID = m.selectedContainer.ID
				m.statsHistoryCPU = nil
				m.statsHistoryMem = nil
			}
		}
		m.containerStats = msg
		if msg != nil {
			m.statsHistoryCPU = appendStatSample(m.statsHistoryCPU, msg.CPUUsage)
			m.statsHistoryMem = appendStatSample(m.statsHistoryMem, msg.MemoryPct)
		}
		m.showingStats = true
		m.showingLogs = false
		m.err = ""
		return m, nil

	case createSiteSuccessMsg:
		m.notice = "Site created: " + string(msg)
		m.err = ""
		m.creatingSite = false
		m.currentView = ViewList
		m.createDomain = ""
		m.createIncludeWWW = true
		m.createTemplateIndex = 0
		m.createPortInput = "3000"
		m.createStep = CreateStepDomain
		return m, fetchContainers(m.client)

	case createDatabaseSuccessMsg:
		m.notice = fmt.Sprintf("DB created: %s (port %d)", msg.Name, msg.Port)
		m.err = ""
		m.creatingDatabase = false
		m.currentView = ViewList
		m.createDbName = ""
		m.createDbUser = ""
		m.createDbPassword = ""
		m.createDbPort = ""
		m.createDbStep = CreateDbStepName
		m.createDbTypeIdx = 0
		return m, fetchDatabases(m.client)

	case databaseActionSuccessMsg:
		m.notice = string(msg)
		m.err = ""
		m.dbConfirmDelete = false
		m.currentView = ViewList
		m.selectedDatabase = nil
		m.dbActionCursor = 0
		return m, fetchDatabases(m.client)

	case fileContentMsg:
		m.fileActivePath = msg.Path
		m.fileContent = msg.Content
		m.fileDirty = false
		m.fileEditCursor = len([]rune(m.fileContent))
		m.currentView = ViewFileEdit
		m.err = ""
		return m, nil

	case fileActionSuccessMsg:
		m.notice = msg.Notice
		m.err = ""
		if msg.ClearDirty {
			m.fileDirty = false
		}
		return m, fetchFiles(m.client, m.filePath)
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global shortcuts
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if m.authRequired {
		return m.handleAuthInput(msg)
	}

	// Search mode for containers screen
	if m.currentScreen == ScreenContainers && m.currentView == ViewList {
		if m.searchMode {
			return m.handleSearchInput(msg)
		}
		if msg.String() == "/" {
			m.searchMode = true
			m.searchQuery = ""
			return m, nil
		}
	}
	if m.currentScreen == ScreenDatabases && m.currentView == ViewList {
		if m.dbSearchMode {
			return m.handleDbSearchInput(msg)
		}
		if msg.String() == "/" {
			m.dbSearchMode = true
			m.dbSearchQuery = ""
			return m, nil
		}
	}

	// Handle Esc - back navigation
	if msg.Type == tea.KeyEsc && m.currentScreen == ScreenSettings && m.focusArea == FocusMain && m.x.setView != 0 {
		return m.handleSettingsInput(msg)
	}
	if msg.Type == tea.KeyEsc {
		if m.showingLogs {
			m.showingLogs = false
			m.containerLogs = ""
			return m, nil
		}
		if m.showingStats {
			m.showingStats = false
			m.containerStats = nil
			m.statsHistoryCPU = nil
			m.statsHistoryMem = nil
			m.statsContainerID = ""
			return m, nil
		}
		if m.currentView == ViewCreateSite {
			m.currentView = ViewList
			m.createDomain = ""
			m.createIncludeWWW = true
			m.createTemplateIndex = 0
			m.createPortInput = "3000"
			m.createStep = CreateStepDomain
			m.creatingSite = false
			m.err = ""
			return m, nil
		}
		if m.currentView == ViewCreateDatabase {
			m.currentView = ViewList
			m.createDbName = ""
			m.createDbUser = ""
			m.createDbPassword = ""
			m.createDbPort = ""
			m.createDbStep = CreateDbStepName
			m.creatingDatabase = false
			m.err = ""
			return m, nil
		}
		if m.currentScreen == ScreenFiles {
			if m.filePromptMode != FilePromptNone {
				m.filePromptMode = FilePromptNone
				m.filePromptInput = ""
				return m, nil
			}
			if m.currentView == ViewFileEdit {
				m.currentView = ViewList
				m.fileActivePath = ""
				m.fileContent = ""
				m.fileDirty = false
				m.fileEditCursor = 0
				return m, nil
			}
		}
		if m.currentView == ViewDatabaseDetails {
			if m.dbConfirmDelete {
				m.dbConfirmDelete = false
				return m, nil
			}
			m.currentView = ViewList
			m.selectedDatabase = nil
			m.dbActionCursor = 0
			return m, nil
		}
		if m.currentView == ViewDetails && m.ctMode != 0 {
			m.ctMode = 0
			return m, nil
		}
		if m.currentView == ViewDetails {
			m.currentView = ViewList
			m.selectedContainer = nil
			m.actionCursor = 0
			m.containerConfirmDelete = false
			return m, nil
		}
		if m.searchMode {
			m.searchMode = false
			m.searchQuery = ""
			m.updateFilteredContainers()
			return m, nil
		}
		if m.dbSearchMode {
			m.dbSearchMode = false
			m.dbSearchQuery = ""
			m.updateFilteredDatabases()
			return m, nil
		}
		// If in main panel, go back to sidebar
		if m.focusArea == FocusMain {
			m.focusArea = FocusSidebar
			return m, nil
		}
		// If already in sidebar, jump to Quit
		if m.focusArea == FocusSidebar {
			m.sidebarSelected = 7
			return m, nil
		}
		return m, nil
	}

	// Handle Tab - cycle focus
	if msg.Type == tea.KeyTab {
		if m.focusArea == FocusSidebar {
			m.focusArea = FocusMain
		} else {
			m.focusArea = FocusSidebar
		}
		return m, nil
	}

	// Handle navigation based on focus
	if m.focusArea == FocusSidebar {
		return m.handleSidebarInput(msg)
	} else {
		return m.handleMainPanelInput(msg)
	}
}

func (m Model) handleSidebarInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.sidebarSelected > 0 {
			m.sidebarSelected--
		}
	case tea.KeyDown:
		if m.sidebarSelected < 7 { // 8 items (0-7)
			m.sidebarSelected++
		}
	case tea.KeyEnter:
		// Handle selection
		if m.sidebarSelected == 7 { // Quit
			return m, tea.Quit
		}
		m.currentScreen = Screen(m.sidebarSelected)
		m.currentView = ViewList
		m.selectedContainer = nil
		m.selectedDatabase = nil
		m.searchMode = false
		m.searchQuery = ""
		m.dbSearchMode = false
		m.dbSearchQuery = ""
		m.containersCursor = 0
		m.databasesCursor = 0
		m.updateFilteredContainers()
		m.updateFilteredDatabases()
		// Auto-switch focus to main panel for Containers/Databases/Files screens
		if m.currentScreen == ScreenContainers || m.currentScreen == ScreenDatabases || m.currentScreen == ScreenFiles {
			m.focusArea = FocusMain
		}
		if m.currentScreen == ScreenFiles {
			m.fileListCursor = 0
			return m, fetchFiles(m.client, m.filePath)
		}
		switch m.currentScreen {
		case ScreenLogs:
			m.focusArea = FocusMain
			m.x.logLoaded, m.x.logLoading, m.x.logErr = false, true, ""
			return m, fetchLogsCmd(m.client, m.x.logTarget)
		case ScreenNetwork:
			m.focusArea = FocusMain
			m.x.netOffset = 0
			return m, tea.Batch(fetchNetworkCmd(m.client), fetchSSLCmd(m.client))
		case ScreenSettings:
			m.focusArea = FocusMain
			m.x.setView, m.x.setCursor = 0, 0
			return m, fetchUpdateStatus(m.client, false)
		case ScreenDashboard:
			return m, m.refreshExtras()
		}
	}
	return m, nil
}

func (m Model) handleUpdateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.updRunning {
		return m, nil
	}
	switch {
	case msg.Type == tea.KeyEnter:
		if m.updStatus == nil || !m.updStatus.UpdateAvailable {
			m.updConfirm = false
			return m, fetchUpdateStatus(m.client, true)
		}
		if !m.updConfirm {
			m.updConfirm = true
			return m, nil
		}
		m.updConfirm = false
		m.updRunning = true
		m.updNote = "Starting the update..."
		m.updErr = ""
		return m, runUpdateCmd(m.client)
	case msg.String() == "r" || msg.String() == "R":
		m.updConfirm = false
		m.updNote = ""
		return m, fetchUpdateStatus(m.client, true)
	default:
		m.updConfirm = false
	}
	return m, nil
}

func (m Model) handleMainPanelInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.currentScreen {
	case ScreenSettings:
		return m.handleSettingsInput(msg)
	case ScreenLogs:
		return m.handleLogsInput(msg)
	case ScreenNetwork:
		return m.handleNetworkInput(msg)
	}

	// Only containers screen has interactive main panel for now
	if m.currentScreen == ScreenContainers {
		if m.showingLogs {
			// In logs view, just allow Esc to exit
			return m, nil
		}

		if m.currentView == ViewList {
			switch msg.Type {
			case tea.KeyUp:
				if m.containersCursor > 0 {
					m.containersCursor--
				}
			case tea.KeyDown:
				maxIdx := len(m.filteredContainers)
				if m.containersCursor < maxIdx {
					m.containersCursor++
				}
			case tea.KeyEnter:
				if m.containersCursor == len(m.filteredContainers) {
					m.showAllContainers = !m.showAllContainers
					m.containersCursor = 0
					m.updateFilteredContainers()
					return m, nil
				}
				if len(m.filteredContainers) > 0 && m.containersCursor < len(m.filteredContainers) {
					m.selectedContainer = &m.filteredContainers[m.containersCursor]
					m.currentView = ViewDetails
					m.actionCursor = 0
				}
			}
			if msg.String() == "n" || msg.String() == "N" {
				m.currentView = ViewCreateSite
				m.createDomain = ""
				m.createIncludeWWW = true
				m.createTemplateIndex = 0
				m.createPortInput = "3000"
				m.createStep = CreateStepDomain
				m.creatingSite = false
				m.err = ""
				m.notice = ""
				m.searchMode = false
				return m, nil
			}
		} else if m.currentView == ViewDetails {
			// Handle action selection in details view
			if m.ctMode != 0 {
				return m.handleChangeTypeInput(msg)
			}
			actions := m.detailActions()
			if m.containerConfirmDelete {
				if msg.Type == tea.KeyEnter {
					return m, m.executeContainerAction()
				}
				if msg.Type != tea.KeyUp && msg.Type != tea.KeyDown {
					return m, nil
				}
			}
			switch msg.Type {
			case tea.KeyUp:
				if m.actionCursor > 0 {
					m.actionCursor--
				}
				m.containerConfirmDelete = false
			case tea.KeyDown:
				if m.actionCursor < len(actions)-1 {
					m.actionCursor++
				}
				m.containerConfirmDelete = false
			case tea.KeyEnter:
				if m.actionCursor >= len(actions) {
					return m, nil
				}
				if actions[m.actionCursor] == "Delete" {
					m.containerConfirmDelete = true
					return m, nil
				}
				if actions[m.actionCursor] == "Change site type" {
					m.ctMode = 1
					m.ctCursor = siteTypeIndex(siteTypeOf(*m.selectedContainer))
					m.ctPort = ""
					m.err = ""
					return m, nil
				}
				if actions[m.actionCursor] == "View Stats" && m.selectedContainer != nil {
					m.statsContainerID = m.selectedContainer.ID
					m.statsHistoryCPU = nil
					m.statsHistoryMem = nil
					m.containerStats = nil
				}
				return m, m.executeContainerAction()
			}
		} else if m.currentView == ViewCreateSite {
			return m.handleCreateInput(msg)
		}

		return m, nil
	}

	if m.currentScreen == ScreenDatabases {
		if m.currentView == ViewList {
			switch msg.Type {
			case tea.KeyUp:
				if m.databasesCursor > 0 {
					m.databasesCursor--
				}
			case tea.KeyDown:
				maxIdx := len(m.filteredDatabases) - 1
				if m.databasesCursor < maxIdx {
					m.databasesCursor++
				}
			case tea.KeyEnter:
				if len(m.filteredDatabases) > 0 && m.databasesCursor < len(m.filteredDatabases) {
					m.selectedDatabase = &m.filteredDatabases[m.databasesCursor]
					m.currentView = ViewDatabaseDetails
					m.dbActionCursor = 0
					m.dbConfirmDelete = false
				}
			}
			if msg.String() == "n" || msg.String() == "N" {
				m.currentView = ViewCreateDatabase
				m.createDbName = ""
				m.createDbUser = ""
				m.createDbPassword = ""
				m.createDbPort = ""
				m.createDbStep = CreateDbStepName
				m.creatingDatabase = false
				m.err = ""
				m.notice = ""
				return m, nil
			}
		} else if m.currentView == ViewDatabaseDetails {
			if m.dbConfirmDelete {
				if msg.Type == tea.KeyEnter {
					return m, m.executeDatabaseAction()
				}
				if msg.Type != tea.KeyUp && msg.Type != tea.KeyDown {
					return m, nil
				}
			}
			switch msg.Type {
			case tea.KeyUp:
				if m.dbActionCursor > 0 {
					m.dbActionCursor--
				}
				m.dbConfirmDelete = false
			case tea.KeyDown:
				if m.dbActionCursor < 3 {
					m.dbActionCursor++
				}
				m.dbConfirmDelete = false
			case tea.KeyEnter:
				if m.dbActionCursor == 3 {
					m.dbConfirmDelete = true
					return m, nil
				}
				return m, m.executeDatabaseAction()
			}
		} else if m.currentView == ViewCreateDatabase {
			return m.handleCreateDatabaseInput(msg)
		}
	}

	if m.currentScreen == ScreenFiles {
		if m.filePromptMode != FilePromptNone {
			return m.handleFilePromptInput(msg)
		}
		if m.currentView == ViewFileEdit {
			return m.handleFileEditInput(msg)
		}

		switch msg.Type {
		case tea.KeyUp:
			if m.fileListCursor > 0 {
				m.fileListCursor--
			}
		case tea.KeyDown:
			maxIdx := len(m.fileEntries) - 1
			if m.fileListCursor < maxIdx {
				m.fileListCursor++
			}
		case tea.KeyEnter:
			if len(m.fileEntries) == 0 || m.fileListCursor >= len(m.fileEntries) {
				return m, nil
			}
			entry := m.fileEntries[m.fileListCursor]
			if entry.IsDir {
				m.filePath = entry.Path
				m.fileListCursor = 0
				return m, fetchFiles(m.client, m.filePath)
			}
			return m, m.openFileCmd(entry.Path)
		case tea.KeyBackspace:
			parent := path.Dir(m.filePath)
			if parent == "." {
				parent = ""
			}
			m.filePath = parent
			m.fileListCursor = 0
			return m, fetchFiles(m.client, m.filePath)
		}

		switch msg.String() {
		case "c":
			m.filePromptMode = FilePromptCreateFile
			m.filePromptInput = ""
			return m, nil
		case "C":
			m.filePromptMode = FilePromptCreateFolder
			m.filePromptInput = ""
			return m, nil
		case "r":
			if len(m.fileEntries) > 0 && m.fileListCursor < len(m.fileEntries) {
				m.filePromptMode = FilePromptRename
				m.filePromptInput = ""
				return m, nil
			}
		case "d":
			if len(m.fileEntries) > 0 && m.fileListCursor < len(m.fileEntries) {
				m.filePromptMode = FilePromptDelete
				m.filePromptInput = ""
				return m, nil
			}
		}
	}

	return m, nil
}

func (m Model) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter, tea.KeyEsc:
		m.searchMode = false
		m.containersCursor = 0
		return m, nil
	case tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.updateFilteredContainers()
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.searchQuery += msg.String()
			m.updateFilteredContainers()
		}
	}
	return m, nil
}

func (m Model) handleDbSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter, tea.KeyEsc:
		m.dbSearchMode = false
		m.databasesCursor = 0
		return m, nil
	case tea.KeyBackspace:
		if len(m.dbSearchQuery) > 0 {
			m.dbSearchQuery = m.dbSearchQuery[:len(m.dbSearchQuery)-1]
			m.updateFilteredDatabases()
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.dbSearchQuery += msg.String()
			m.updateFilteredDatabases()
		}
	}
	return m, nil
}

func (m Model) selectedTemplate() string {
	templates := []string{"static", "php", "node"}
	if m.createTemplateIndex < 0 || m.createTemplateIndex >= len(templates) {
		return "static"
	}
	return templates[m.createTemplateIndex]
}

func (m Model) createPortValue() int {
	port := 0
	for _, r := range m.createPortInput {
		if r < '0' || r > '9' {
			return 0
		}
		port = port*10 + int(r-'0')
	}
	if port == 0 {
		return 3000
	}
	return port
}

func (m Model) handleCreateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.creatingSite {
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEnter:
		switch m.createStep {
		case CreateStepDomain:
			trimmed := strings.TrimSpace(m.createDomain)
			if trimmed == "" {
				m.err = "Domain is required"
				return m, nil
			}
			m.err = ""
			m.createStep = CreateStepTemplate
			return m, nil
		case CreateStepTemplate:
			if m.selectedTemplate() == "node" {
				m.createStep = CreateStepPort
				return m, nil
			}
			m.createStep = CreateStepSSL
			return m, nil
		case CreateStepPort:
			port := m.createPortValue()
			if port < 1 || port > 65535 {
				m.err = "Port must be between 1 and 65535"
				return m, nil
			}
			m.err = ""
			m.createStep = CreateStepSSL
			return m, nil
		case CreateStepSSL:
			trimmed := strings.TrimSpace(m.createDomain)
			if trimmed == "" {
				m.err = "Domain is required"
				m.createStep = CreateStepDomain
				return m, nil
			}
			m.creatingSite = true
			m.err = ""
			template := m.selectedTemplate()
			port := 0
			if template == "node" {
				port = m.createPortValue()
			}
			return m, m.createSiteCmd(trimmed, template, m.createIncludeWWW, port)
		}
	case tea.KeyBackspace:
		switch m.createStep {
		case CreateStepDomain:
			if len(m.createDomain) > 0 {
				m.createDomain = m.createDomain[:len(m.createDomain)-1]
			}
			return m, nil
		case CreateStepPort:
			if len(m.createPortInput) > 0 {
				m.createPortInput = m.createPortInput[:len(m.createPortInput)-1]
			}
			return m, nil
		}
	}

	if msg.String() == " " && m.createStep == CreateStepSSL {
		m.createIncludeWWW = !m.createIncludeWWW
		return m, nil
	}

	if m.createStep == CreateStepTemplate {
		switch msg.Type {
		case tea.KeyUp:
			m.createTemplateIndex = (m.createTemplateIndex + 2) % 3
			return m, nil
		case tea.KeyDown:
			m.createTemplateIndex = (m.createTemplateIndex + 1) % 3
			return m, nil
		}
	}

	if msg.Type == tea.KeyRunes {
		switch m.createStep {
		case CreateStepDomain:
			m.createDomain += msg.String()
		case CreateStepPort:
			if len(msg.String()) > 0 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
				m.createPortInput += msg.String()
			}
		}
	}

	return m, nil
}

func (m Model) createDbPortValue() int {
	if strings.TrimSpace(m.createDbPort) == "" {
		return 0
	}
	port := 0
	for _, r := range m.createDbPort {
		if r < '0' || r > '9' {
			return 0
		}
		port = port*10 + int(r-'0')
	}
	return port
}

func (m Model) handleCreateDatabaseInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.creatingDatabase {
		return m, nil
	}

	if m.createDbStep == CreateDbStepType {
		switch msg.Type {
		case tea.KeyUp, tea.KeyLeft:
			m.createDbTypeIdx = (m.createDbTypeIdx + len(databaseTypes) - 1) % len(databaseTypes)
			return m, nil
		case tea.KeyDown, tea.KeyRight:
			m.createDbTypeIdx = (m.createDbTypeIdx + 1) % len(databaseTypes)
			return m, nil
		}
	}

	switch msg.Type {
	case tea.KeyEnter:
		switch m.createDbStep {
		case CreateDbStepName:
			trimmed := strings.TrimSpace(m.createDbName)
			if trimmed == "" {
				m.err = "Database name is required"
				return m, nil
			}
			m.err = ""
			m.createDbStep = CreateDbStepType
			return m, nil
		case CreateDbStepType:
			m.createDbStep = CreateDbStepUser
			return m, nil
		case CreateDbStepUser:
			m.createDbStep = CreateDbStepPassword
			return m, nil
		case CreateDbStepPassword:
			m.createDbStep = CreateDbStepPort
			return m, nil
		case CreateDbStepPort:
			port := m.createDbPortValue()
			if port < 0 || port > 65535 {
				m.err = "Port must be empty or between 1 and 65535"
				return m, nil
			}
			m.creatingDatabase = true
			m.err = ""
			return m, m.createDatabaseCmd(m.createDbName, databaseTypes[m.createDbTypeIdx], m.createDbUser, m.createDbPassword, port)
		}
	case tea.KeyBackspace:
		switch m.createDbStep {
		case CreateDbStepName:
			if len(m.createDbName) > 0 {
				m.createDbName = m.createDbName[:len(m.createDbName)-1]
			}
			return m, nil
		case CreateDbStepUser:
			if len(m.createDbUser) > 0 {
				m.createDbUser = m.createDbUser[:len(m.createDbUser)-1]
			}
			return m, nil
		case CreateDbStepPassword:
			if len(m.createDbPassword) > 0 {
				m.createDbPassword = m.createDbPassword[:len(m.createDbPassword)-1]
			}
			return m, nil
		case CreateDbStepPort:
			if len(m.createDbPort) > 0 {
				m.createDbPort = m.createDbPort[:len(m.createDbPort)-1]
			}
			return m, nil
		}
	}

	if msg.Type == tea.KeyRunes {
		switch m.createDbStep {
		case CreateDbStepName:
			m.createDbName += msg.String()
		case CreateDbStepUser:
			m.createDbUser += msg.String()
		case CreateDbStepPassword:
			m.createDbPassword += msg.String()
		case CreateDbStepPort:
			if len(msg.String()) > 0 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
				m.createDbPort += msg.String()
			}
		}
	}

	return m, nil
}

func (m Model) handleFilePromptInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		selected := ""
		if len(m.fileEntries) > 0 && m.fileListCursor < len(m.fileEntries) {
			selected = m.fileEntries[m.fileListCursor].Path
		}
		input := strings.TrimSpace(m.filePromptInput)
		mode := m.filePromptMode
		m.filePromptMode = FilePromptNone
		m.filePromptInput = ""

		switch mode {
		case FilePromptCreateFile:
			if input == "" {
				m.err = "File name is required"
				return m, nil
			}
			return m, m.createPathCmd(path.Join(m.filePath, input), "file")
		case FilePromptCreateFolder:
			if input == "" {
				m.err = "Folder name is required"
				return m, nil
			}
			return m, m.createPathCmd(path.Join(m.filePath, input), "folder")
		case FilePromptRename:
			if selected == "" {
				return m, nil
			}
			if input == "" {
				m.err = "New name is required"
				return m, nil
			}
			return m, m.renamePathCmd(selected, input)
		case FilePromptDelete:
			if selected == "" {
				return m, nil
			}
			return m, m.deletePathCmd(selected)
		}
	case tea.KeyBackspace:
		if len(m.filePromptInput) > 0 {
			m.filePromptInput = m.filePromptInput[:len(m.filePromptInput)-1]
		}
		return m, nil
	}

	if msg.Type == tea.KeyRunes {
		m.filePromptInput += msg.String()
	}

	return m, nil
}

func (m Model) handleFileEditInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlO:
		if m.fileActivePath == "" {
			return m, nil
		}
		return m, m.saveFileCmd(m.fileActivePath, m.fileContent)
	case tea.KeyCtrlX:
		m.currentView = ViewList
		m.fileActivePath = ""
		m.fileContent = ""
		m.fileDirty = false
		m.fileEditCursor = 0
		return m, nil
	case tea.KeyBackspace:
		m.deleteRuneAtCursor()
		m.fileDirty = true
		return m, nil
	case tea.KeyEnter:
		m.insertRuneAtCursor('\n')
		m.fileDirty = true
		return m, nil
	case tea.KeyLeft:
		if m.fileEditCursor > 0 {
			m.fileEditCursor--
		}
		return m, nil
	case tea.KeyRight:
		if m.fileEditCursor < len([]rune(m.fileContent)) {
			m.fileEditCursor++
		}
		return m, nil
	case tea.KeyUp:
		m.moveCursorVertical(-1)
		return m, nil
	case tea.KeyDown:
		m.moveCursorVertical(1)
		return m, nil
	}

	if msg.Type == tea.KeyRunes {
		m.insertStringAtCursor(msg.String())
		m.fileDirty = true
	}

	return m, nil
}

func (m Model) handleAuthInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if strings.TrimSpace(m.authURLInput) != "" {
			m.client.SetBaseURL(m.authURLInput)
		}
		m.client.SetToken(strings.TrimSpace(m.authTokenInput))
		_ = SaveConfig(AppConfig{
			BaseURL: m.client.BaseURL,
			Token:   m.client.Token,
		})
		m.authRequired = false
		m.err = ""
		return m, tea.Batch(
			fetchHealth(m.client),
			fetchSummary(m.client),
			fetchContainers(m.client),
			fetchDatabases(m.client),
			fetchFiles(m.client, m.filePath),
		)
	case tea.KeyUp:
		if m.authFieldIndex > 0 {
			m.authFieldIndex--
		}
		return m, nil
	case tea.KeyDown:
		if m.authFieldIndex < 1 {
			m.authFieldIndex++
		}
		return m, nil
	case tea.KeyBackspace:
		if m.authFieldIndex == 0 && len(m.authURLInput) > 0 {
			m.authURLInput = m.authURLInput[:len(m.authURLInput)-1]
		}
		if m.authFieldIndex == 1 && len(m.authTokenInput) > 0 {
			m.authTokenInput = m.authTokenInput[:len(m.authTokenInput)-1]
		}
		return m, nil
	}

	if msg.Type == tea.KeyRunes {
		if m.authFieldIndex == 0 {
			m.authURLInput += msg.String()
		} else {
			m.authTokenInput += msg.String()
		}
	}

	return m, nil
}

func (m *Model) updateFilteredContainers() {
	base := m.containers
	if !m.showAllContainers {
		filtered := make([]api.Container, 0, len(m.containers))
		for _, c := range m.containers {
			if c.Labels != nil && c.Labels["docklite.managed"] == "true" {
				filtered = append(filtered, c)
			}
		}
		base = filtered
	}

	// Which group each box sits in, and the order boxes appear in: a group's boxes together (in the group's own order),
	// then everything not in a group.
	m.groupOf = map[string]string{}
	rank := map[string]int{}
	for gi, g := range m.groups {
		for mi, name := range g.Members {
			m.groupOf[name] = g.Name
			rank[name] = gi*10000 + mi
		}
	}
	ordered := make([]api.Container, len(base))
	copy(ordered, base)
	sort.SliceStable(ordered, func(i, j int) bool {
		ri, iok := rank[ordered[i].Name]
		rj, jok := rank[ordered[j].Name]
		if iok != jok {
			return iok // grouped before ungrouped
		}
		return iok && ri < rj
	})

	if m.searchQuery == "" {
		m.filteredContainers = ordered
	} else {
		m.filteredContainers = []api.Container{}
		query := strings.ToLower(m.searchQuery)
		for _, c := range ordered {
			if strings.Contains(strings.ToLower(c.Name), query) ||
				strings.Contains(strings.ToLower(boxTitle(c)), query) ||
				strings.Contains(strings.ToLower(m.groupOf[c.Name]), query) ||
				strings.Contains(strings.ToLower(c.ID), query) ||
				strings.Contains(strings.ToLower(c.Image), query) {
				m.filteredContainers = append(m.filteredContainers, c)
			}
		}
	}

	// Reset cursor if out of bounds
	if m.containersCursor > len(m.filteredContainers) {
		m.containersCursor = 0
	}
}

// boxTitle is the name a person knows a box by: the site's address, or the container's name without DockLite's prefix.
func boxTitle(c api.Container) string {
	if d := c.Labels["docklite.domain"]; d != "" {
		return d
	}
	if d := c.Labels["docklite.database"]; d != "" {
		return d
	}
	for _, p := range []string{"docklite-site-", "docklite-db-", "docklite-"} {
		if strings.HasPrefix(c.Name, p) && len(c.Name) > len(p) {
			return c.Name[len(p):]
		}
	}
	return c.Name
}

// boxKind says what a box is in a word: its DockLite type, else the image's name without the registry and tag.
func boxKind(c api.Container) string {
	if t := c.Labels["docklite.type"]; t != "" {
		switch t {
		case "postgres", "mysql", "mariadb", "mongodb", "redis":
			return databaseLabel(t) + " DB"
		}
		return t
	}
	img := c.Image
	if i := strings.LastIndex(img, "/"); i >= 0 {
		img = img[i+1:]
	}
	if i := strings.Index(img, ":"); i >= 0 {
		img = img[:i]
	}
	return img
}

func (m *Model) updateFilteredDatabases() {
	if m.dbSearchQuery == "" {
		m.filteredDatabases = m.databases
		return
	}

	m.filteredDatabases = []api.Database{}
	query := strings.ToLower(m.dbSearchQuery)
	for _, db := range m.databases {
		if strings.Contains(strings.ToLower(db.Name), query) ||
			strings.Contains(strings.ToLower(db.Username), query) ||
			strings.Contains(strings.ToLower(db.ID), query) {
			m.filteredDatabases = append(m.filteredDatabases, db)
		}
	}

	if m.databasesCursor >= len(m.filteredDatabases) {
		m.databasesCursor = 0
	}
}

// Commands

func splashCmd() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
		return splashDoneMsg{}
	})
}

func tickCmd() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchUpdateStatus(client *api.Client, refresh bool) tea.Cmd {
	return func() tea.Msg {
		st, err := client.GetUpdateStatus(refresh)
		return updateStatusMsg{Status: st, Err: err}
	}
}

func runUpdateCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg { return updateStartedMsg{Err: client.RunUpdate()} }
}

func updatePollCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return updatePollMsg{} })
}

func fetchHealth(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		health, err := client.GetHealth()
		if err != nil {
			return errMsg(err)
		}
		return healthMsg(health)
	}
}

func fetchSummary(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		summary, err := client.GetSummary()
		if err != nil {
			return errMsg(err)
		}
		return summaryMsg(summary)
	}
}

func fetchContainers(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		containers, err := client.GetContainers()
		if err != nil {
			return errMsg(err)
		}
		return containersMsg(containers)
	}
}

func fetchContainerStats(client *api.Client, id string) tea.Cmd {
	return func() tea.Msg {
		stats, err := client.GetContainerStats(id)
		if err != nil {
			return errMsg(err)
		}
		return containerStatsMsg(stats)
	}
}

func fetchDatabases(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		databases, err := client.GetDatabases()
		if err != nil {
			return errMsg(err)
		}
		return databasesMsg(databases)
	}
}

func fetchFiles(client *api.Client, path string) tea.Cmd {
	return func() tea.Msg {
		files, err := client.ListFiles(path)
		if err != nil {
			return errMsg(err)
		}
		return filesMsg{Path: files.Path, Entries: files.Entries}
	}
}

func (m Model) executeContainerAction() tea.Cmd {
	if m.selectedContainer == nil {
		return nil
	}
	actions := m.detailActions()
	if m.actionCursor >= len(actions) {
		return nil
	}
	action := actions[m.actionCursor]

	return func() tea.Msg {
		var err error
		notice := ""
		returnToList := false
		switch action {
		case "Start":
			err = m.client.StartContainer(m.selectedContainer.ID)
			notice = "Container started: " + m.selectedContainer.Name
		case "Stop":
			err = m.client.StopContainer(m.selectedContainer.ID)
			notice = "Container stopped: " + m.selectedContainer.Name
		case "Restart":
			err = m.client.RestartContainer(m.selectedContainer.ID)
			notice = "Container restarted: " + m.selectedContainer.Name
		case "View Logs":
			logs, logsErr := m.client.GetContainerLogs(m.selectedContainer.ID)
			if logsErr != nil {
				return errMsg(logsErr)
			}
			return containerLogsMsg(logs)
		case "View Stats":
			stats, statsErr := m.client.GetContainerStats(m.selectedContainer.ID)
			if statsErr != nil {
				return errMsg(statsErr)
			}
			return containerStatsMsg(stats)
		case "Delete":
			err = m.client.DeleteContainer(m.selectedContainer.ID)
			notice = "Container deleted: " + m.selectedContainer.Name
			returnToList = true
		}

		if err != nil {
			return errMsg(err)
		}
		return containerActionSuccessMsg{Notice: notice, ReturnToList: returnToList}
	}
}

func (m Model) executeDatabaseAction() tea.Cmd {
	if m.selectedDatabase == nil {
		return nil
	}

	return func() tea.Msg {
		var err error
		switch m.dbActionCursor {
		case 0:
			err = m.client.StartContainer(m.selectedDatabase.ID)
		case 1:
			err = m.client.StopContainer(m.selectedDatabase.ID)
		case 2:
			err = m.client.RestartContainer(m.selectedDatabase.ID)
		case 3:
			err = m.client.DeleteContainer(m.selectedDatabase.ID)
		}

		if err != nil {
			return errMsg(err)
		}
		if m.dbActionCursor == 3 {
			return databaseActionSuccessMsg("DB deleted: " + m.selectedDatabase.Name)
		}
		return databaseActionSuccessMsg("DB action completed: " + m.selectedDatabase.Name)
	}
}

func (m Model) createSiteCmd(domain string, templateType string, includeWww bool, port int) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.CreateSiteContainer(domain, templateType, includeWww, port); err != nil {
			return errMsg(err)
		}
		return createSiteSuccessMsg(domain)
	}
}

func (m Model) createDatabaseCmd(name string, dbType string, username string, password string, port int) tea.Cmd {
	return func() tea.Msg {
		result, err := m.client.CreateDatabase(strings.TrimSpace(name), dbType, strings.TrimSpace(username), password, port)
		if err != nil {
			return errMsg(err)
		}
		return createDatabaseSuccessMsg{
			Name:     result.Connection.Database,
			Port:     result.Connection.Port,
			Username: result.Connection.Username,
			Password: result.Connection.Password,
		}
	}
}

func (m Model) openFileCmd(path string) tea.Cmd {
	return func() tea.Msg {
		result, err := m.client.ReadFile(path)
		if err != nil {
			return errMsg(err)
		}
		return fileContentMsg{Path: result.Path, Content: result.Content}
	}
}

func (m Model) saveFileCmd(path string, content string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.SaveFile(path, content); err != nil {
			return errMsg(err)
		}
		return fileActionSuccessMsg{Notice: "File saved: " + path, ClearDirty: true}
	}
}

func (m Model) createPathCmd(path string, kind string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.CreatePath(path, kind); err != nil {
			return errMsg(err)
		}
		return fileActionSuccessMsg{Notice: "Created " + kind + ": " + path}
	}
}

func (m Model) deletePathCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.DeletePath(path); err != nil {
			return errMsg(err)
		}
		return fileActionSuccessMsg{Notice: "Deleted: " + path}
	}
}

func (m Model) renamePathCmd(path string, newName string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.RenamePath(path, newName); err != nil {
			return errMsg(err)
		}
		return fileActionSuccessMsg{Notice: "Renamed: " + path}
	}
}

func (m *Model) insertStringAtCursor(value string) {
	for _, r := range value {
		m.insertRuneAtCursor(r)
	}
}

func (m *Model) insertRuneAtCursor(r rune) {
	runes := []rune(m.fileContent)
	if m.fileEditCursor < 0 {
		m.fileEditCursor = 0
	}
	if m.fileEditCursor > len(runes) {
		m.fileEditCursor = len(runes)
	}
	runes = append(runes[:m.fileEditCursor], append([]rune{r}, runes[m.fileEditCursor:]...)...)
	m.fileEditCursor++
	m.fileContent = string(runes)
}

func (m *Model) deleteRuneAtCursor() {
	runes := []rune(m.fileContent)
	if m.fileEditCursor <= 0 || len(runes) == 0 {
		return
	}
	if m.fileEditCursor > len(runes) {
		m.fileEditCursor = len(runes)
	}
	runes = append(runes[:m.fileEditCursor-1], runes[m.fileEditCursor:]...)
	m.fileEditCursor--
	m.fileContent = string(runes)
}

func (m *Model) moveCursorVertical(delta int) {
	line, col := cursorLineCol([]rune(m.fileContent), m.fileEditCursor)
	line += delta
	if line < 0 {
		line = 0
	}
	m.fileEditCursor = cursorIndexFromLineCol([]rune(m.fileContent), line, col)
}

func cursorLineCol(runes []rune, index int) (int, int) {
	if index < 0 {
		index = 0
	}
	if index > len(runes) {
		index = len(runes)
	}
	line := 0
	col := 0
	for i := 0; i < index; i++ {
		if runes[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return line, col
}

func cursorIndexFromLineCol(runes []rune, targetLine int, targetCol int) int {
	line := 0
	col := 0
	for i, r := range runes {
		if line == targetLine && col == targetCol {
			return i
		}
		if r == '\n' {
			line++
			col = 0
			if line > targetLine {
				return i
			}
		} else {
			col++
		}
	}
	if line == targetLine {
		return len(runes)
	}
	return len(runes)
}

func appendStatSample(series []float64, value float64) []float64 {
	const maxSamples = 30
	if len(series) >= maxSamples {
		copy(series, series[1:])
		series = series[:maxSamples-1]
	}
	return append(series, value)
}

// The kinds of database DockLite can create, as the API names them.
var databaseTypes = []string{"postgres", "mysql", "mariadb", "mongodb", "redis"}

// dbConnectURL is how a program on this server connects, in each engine's own form.
func dbConnectURL(db api.Database) string {
	host := fmt.Sprintf("localhost:%d", db.Port)
	switch db.Type {
	case "mysql", "mariadb":
		return fmt.Sprintf("mysql://%s:%s@%s/%s", db.Username, db.Password, host, db.Name)
	case "mongodb":
		return fmt.Sprintf("mongodb://%s:%s@%s/%s", db.Username, db.Password, host, db.Name)
	case "redis":
		return fmt.Sprintf("redis://:%s@%s", db.Password, host)
	case "sqlite":
		return ""
	}
	return fmt.Sprintf("postgres://%s:%s@%s/%s", db.Username, db.Password, host, db.Name)
}

// databaseLabel is the engine's everyday name.
func databaseLabel(t string) string {
	switch t {
	case "mysql":
		return "MySQL"
	case "mariadb":
		return "MariaDB"
	case "mongodb":
		return "MongoDB"
	case "redis":
		return "Redis"
	case "sqlite":
		return "SQLite"
	case "", "postgres":
		return "PostgreSQL"
	}
	return t
}

var siteTypes = []struct{ id, label, hint string }{
	{"static", "Static website", "plain HTML, CSS and JavaScript files (index.html)"},
	{"php", "PHP", "PHP pages (index.php)"},
	{"node", "Node.js app", "a JavaScript server that runs index.js"},
}

func siteTypeIndex(t string) int {
	for i, s := range siteTypes {
		if s.id == t {
			return i
		}
	}
	return 0
}

// siteTypeOf is the kind of website a box is, or "" if it isn't one of DockLite's websites.
func siteTypeOf(c api.Container) string {
	if c.Labels["docklite.domain"] == "" || c.Labels["docklite.managed"] != "true" {
		return ""
	}
	t := c.Labels["docklite.type"]
	if t == "static" || t == "php" || t == "node" {
		return t
	}
	return ""
}

// detailActions are the buttons on a box's details; websites also get "Change site type".
func (m Model) detailActions() []string {
	actions := []string{"Start", "Stop", "Restart", "View Logs", "View Stats"}
	if m.selectedContainer != nil && siteTypeOf(*m.selectedContainer) != "" {
		actions = append(actions, "Change site type")
	}
	return append(actions, "Delete")
}

func (m Model) ctPortValue() int {
	n := 0
	for _, r := range m.ctPort {
		n = n*10 + int(r-'0')
		if n > 65535 {
			return 65536
		}
	}
	return n
}

func (m Model) handleChangeTypeInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.selectedContainer
	if c == nil {
		m.ctMode = 0
		return m, nil
	}
	switch m.ctMode {
	case 1:
		switch msg.Type {
		case tea.KeyUp:
			if m.ctCursor > 0 {
				m.ctCursor--
			}
		case tea.KeyDown:
			if m.ctCursor < len(siteTypes)-1 {
				m.ctCursor++
			}
		case tea.KeyEnter:
			chosen := siteTypes[m.ctCursor].id
			if chosen == siteTypeOf(*c) && chosen != "node" {
				m.ctMode = 0
				m.notice = "It already runs as " + siteTypes[m.ctCursor].label + "."
				return m, nil
			}
			if chosen == "node" {
				m.ctMode = 2
				return m, nil
			}
			m.ctMode = 3
		}
	case 2:
		switch {
		case msg.Type == tea.KeyEnter:
			if p := m.ctPortValue(); m.ctPort != "" && (p < 1 || p > 65535) {
				m.err = "Port must be between 1 and 65535"
				return m, nil
			}
			m.err = ""
			m.ctMode = 3
		case msg.Type == tea.KeyBackspace:
			if len(m.ctPort) > 0 {
				m.ctPort = m.ctPort[:len(m.ctPort)-1]
			}
		case msg.Type == tea.KeyRunes:
			for _, r := range msg.Runes {
				if r >= '0' && r <= '9' && len(m.ctPort) < 5 {
					m.ctPort += string(r)
				}
			}
		}
	case 3:
		if msg.Type == tea.KeyEnter {
			id, chosen, port := c.ID, siteTypes[m.ctCursor], m.ctPortValue()
			label := boxTitle(*c)
			client := m.client
			m.ctMode = 0
			m.notice = "Restarting " + label + " as " + chosen.label + "..."
			return m, func() tea.Msg {
				warn, err := client.ChangeSiteType(id, chosen.id, port)
				return siteTypeDoneMsg{Notice: label + " now runs as " + chosen.label + ".", Warning: warn, Err: err}
			}
		}
		if msg.Type == tea.KeyUp || msg.Type == tea.KeyDown {
			m.ctMode = 1
		}
	}
	return m, nil
}

// apiMessage pulls the readable sentence out of an agent error body like {"error":"..."}.
func apiMessage(e *api.APIError) string {
	var b struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(e.Body), &b) == nil && b.Error != "" {
		return b.Error
	}
	return e.Error()
}
