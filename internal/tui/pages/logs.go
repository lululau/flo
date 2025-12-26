package pages

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"flo/internal/config"
	"flo/internal/tui/components"
	"flo/internal/tui/types"
)

// LogsModel represents the logs view page with stage-tabs layout.
// The UI is a single log viewport with a tab bar for switching stages.
type LogsModel struct {
	// Components
	viewport   components.ViewportModel
	statusLine components.StatusModeLineModel
	search     components.SearchModel
	spinner    components.SpinnerModel

	// Data
	pipelineID   string
	pipelineName string
	runID        string
	status       string
	tabsData     *types.RunStageTabsData
	config       *config.Config

	// Layout
	width  int
	height int

	// State
	searchActive      bool
	searchQuery       string
	loading           bool
	autoRefresh       bool
	isNewRun          bool
	waitingForSecondY bool   // For yy copy sequence
	copyNotice        string // "Copied!" notice to show temporarily

	// Key bindings
	keys LogsKeyMap
}

// LogsKeyMap defines key bindings for the logs page
type LogsKeyMap struct {
	Up           key.Binding
	Down         key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	Home         key.Binding
	End          key.Binding
	NextTab      key.Binding
	PrevTab      key.Binding
	Refresh      key.Binding
	Stop         key.Binding
	OpenEditor   key.Binding
	OpenPager    key.Binding
	Yank         key.Binding
	Search       key.Binding
	SearchNext   key.Binding
	SearchPrev   key.Binding
	Back         key.Binding
	Quit         key.Binding
}

// DefaultLogsKeyMap returns default key bindings
func DefaultLogsKeyMap() LogsKeyMap {
	return LogsKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+b", "b"),
			key.WithHelp("pgup/b", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+f", "f"),
			key.WithHelp("pgdn/f", "page down"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("ctrl+u", "u"),
			key.WithHelp("u", "half page up"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("ctrl+d", "d"),
			key.WithHelp("d", "half page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g", "top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G", "bottom"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "next stage"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("S-Tab", "prev stage"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Stop: key.NewBinding(
			key.WithKeys("X"),
			key.WithHelp("X", "stop"),
		),
		OpenEditor: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "editor"),
		),
		OpenPager: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "pager"),
		),
		Yank: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("yy", "copy"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		SearchNext: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next match"),
		),
		SearchPrev: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "prev match"),
		),
		Back: key.NewBinding(
			key.WithKeys("q", "esc"),
			key.WithHelp("q", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("Q"),
			key.WithHelp("Q", "quit"),
		),
	}
}

// NewLogsModel creates a new logs model with stage-tabs layout
func NewLogsModel(cfg *config.Config) LogsModel {
	vp := components.NewViewportModel("")
	sl := components.NewStatusModeLineModel()
	search := components.NewSearchModel()
	spinner := components.NewSpinnerModel()

	vp = vp.SetFocused(true)

	return LogsModel{
		viewport:   vp,
		statusLine: sl,
		search:     search,
		spinner:    spinner,
		config:     cfg,
		keys:       DefaultLogsKeyMap(),
	}
}

// SetSize sets the page size
func (m LogsModel) SetSize(width, height int) LogsModel {
	m.width = width
	m.height = height

	// Layout lines:
	// 1 title + 1 tabs + (optional 1 search) + 1 status + 1 help
	contentHeight := height - 4
	if m.searchActive {
		contentHeight--
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	m.viewport = m.viewport.SetSize(width, contentHeight)
	m.statusLine = m.statusLine.SetWidth(width)
	m.search = m.search.SetWidth(width)

	return m
}

// SetRun sets the run context.
func (m LogsModel) SetRun(pipelineID, pipelineName, runID, status string, isNewRun bool) LogsModel {
	m.pipelineID = pipelineID
	m.pipelineName = pipelineName
	m.runID = runID
	m.status = status
	m.isNewRun = isNewRun
	m.tabsData = nil
	m.viewport = m.viewport.SetTitle("")
	m.viewport = m.viewport.SetContent("")
	return m
}

// SetStarting enters a starting/loading state (used for immediate navigation after triggering a run).
func (m LogsModel) SetStarting(pipelineID, pipelineName string) LogsModel {
	m = m.SetRun(pipelineID, pipelineName, "", "STARTING", true)
	m = m.SetLoading(true)
	return m
}

// SetTabsData sets the stage-tabs data model and updates the viewport.
func (m LogsModel) SetTabsData(data *types.RunStageTabsData) LogsModel {
	m.tabsData = data
	if data != nil {
		m.pipelineID = data.PipelineID
		m.pipelineName = data.PipelineName
		m.runID = data.RunID
		m.status = data.RunStatus
	}
	m.updateViewportForSelectedTab(true)
	return m
}

// SetLoading sets the loading state
func (m LogsModel) SetLoading(loading bool) LogsModel {
	m.loading = loading
	m.spinner = m.spinner.SetActive(loading)
	if loading {
		if m.runID == "" {
			m.spinner = m.spinner.SetMessage("Starting pipeline...")
		} else {
			m.spinner = m.spinner.SetMessage("Loading logs...")
		}
	}
	return m
}

// SetAutoRefresh sets auto-refresh on/off
func (m LogsModel) SetAutoRefresh(enabled bool) LogsModel {
	m.autoRefresh = enabled
	return m
}

func (m LogsModel) GetPipelineID() string   { return m.pipelineID }
func (m LogsModel) GetPipelineName() string { return m.pipelineName }
func (m LogsModel) GetRunID() string        { return m.runID }
func (m LogsModel) GetStatus() string       { return m.status }
func (m LogsModel) GetTabsData() *types.RunStageTabsData {
	return m.tabsData
}

// IsRunning returns whether the run is still active.
func (m LogsModel) IsRunning() bool {
	status := strings.ToUpper(strings.TrimSpace(m.status))
	switch status {
	case "", "SUCCESS", "FAILED", "FAIL", "CANCELED", "CANCELLED":
		return false
	default:
		return true
	}
}

// updateStatusLine updates the status line content.
func (m *LogsModel) updateStatusLine() {
	autoRefreshStatus := "Off"
	if m.autoRefresh {
		autoRefreshStatus = "On"
	}

	searchInfo := ""
	if m.searchQuery != "" {
		query, current, total := m.viewport.GetSearchInfo()
		if total > 0 {
			searchInfo = fmt.Sprintf("'%s' (%d/%d)", query, current, total)
		} else {
			searchInfo = fmt.Sprintf("'%s' (no matches)", query)
		}
	}

	m.statusLine = m.statusLine.SetRunStatus(m.status)
	m.statusLine = m.statusLine.SetAutoRefresh(autoRefreshStatus)
	m.statusLine = m.statusLine.SetSearchInfo(searchInfo)
}

// Init implements tea.Model
func (m LogsModel) Init() tea.Cmd {
	return m.spinner.Init()
}

// Update implements tea.Model
func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	var cmds []tea.Cmd

	// Search mode
	if m.searchActive {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyEsc {
				m.searchActive = false
				m.search = m.search.Deactivate()
				return m, nil
			}
		case components.SearchExecuteMsg:
			m.searchQuery = msg.Query
			m.searchActive = false
			m.search = m.search.Deactivate()
			m.viewport = m.viewport.Search(msg.Query)
			m.updateStatusLine()
			return m, nil
		case components.SearchCancelMsg:
			m.searchActive = false
			m.search = m.search.Deactivate()
			return m, nil
		}

		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle yy sequence for copying
		if m.waitingForSecondY {
			m.waitingForSecondY = false
			if key.Matches(msg, m.keys.Yank) {
				// Second 'y' pressed - execute copy
				content := m.viewport.GetContent()
				if content != "" {
					return m, types.CopyToClipboardCmd(content)
				}
				return m, nil
			}
			// Other key pressed - fall through to normal handling
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return types.GoBackMsg{} }

		case key.Matches(msg, m.keys.NextTab):
			if m.tabsData != nil && len(m.tabsData.Stages) > 0 {
				m.tabsData.SelectedIndex++
				if m.tabsData.SelectedIndex >= len(m.tabsData.Stages) {
					m.tabsData.SelectedIndex = 0
				}
				m.tabsData.FollowActive = false
				m.updateViewportForSelectedTab(false)
				return m, func() tea.Msg {
					return LogsTabLoadMsg{TabsData: m.tabsData, TabIndex: m.tabsData.SelectedIndex}
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.PrevTab):
			if m.tabsData != nil && len(m.tabsData.Stages) > 0 {
				m.tabsData.SelectedIndex--
				if m.tabsData.SelectedIndex < 0 {
					m.tabsData.SelectedIndex = len(m.tabsData.Stages) - 1
				}
				m.tabsData.FollowActive = false
				m.updateViewportForSelectedTab(false)
				return m, func() tea.Msg {
					return LogsTabLoadMsg{TabsData: m.tabsData, TabIndex: m.tabsData.SelectedIndex}
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.Refresh):
			// Refresh the currently selected tab immediately.
			if m.tabsData != nil {
				return m, func() tea.Msg {
					return LogsTabLoadMsg{TabsData: m.tabsData, TabIndex: m.tabsData.SelectedIndex}
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.Stop):
			if m.IsRunning() {
				return m, func() tea.Msg {
					return StopRunRequestMsg{PipelineID: m.pipelineID, RunID: m.runID}
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.OpenEditor):
			if m.viewport.GetContent() != "" {
				editor := m.config.GetEditor()
				return m, types.OpenInEditorCmd(m.viewport.GetContent(), editor)
			}
			return m, nil

		case key.Matches(msg, m.keys.OpenPager):
			if m.viewport.GetContent() != "" {
				pager := m.config.GetPager()
				return m, types.OpenInPagerCmd(m.viewport.GetContent(), pager)
			}
			return m, nil

		case key.Matches(msg, m.keys.Search):
			m.searchActive = true
			m.search = m.search.Activate()
			return m, m.search.Focus()

		case key.Matches(msg, m.keys.SearchNext):
			m.viewport = m.viewport.NextSearchMatch()
			m.updateStatusLine()
			return m, nil

		case key.Matches(msg, m.keys.SearchPrev):
			m.viewport = m.viewport.PrevSearchMatch()
			m.updateStatusLine()
			return m, nil

		case key.Matches(msg, m.keys.Yank):
			// First 'y' pressed - wait for second 'y'
			m.waitingForSecondY = true
			return m, nil

		case key.Matches(msg, m.keys.Home), key.Matches(msg, m.keys.End),
			key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down),
			key.Matches(msg, m.keys.PageUp), key.Matches(msg, m.keys.PageDown),
			key.Matches(msg, m.keys.HalfPageUp), key.Matches(msg, m.keys.HalfPageDown):
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case types.RunStageTabsLoadedMsg:
		m.tabsData = msg.Data
		if msg.Data != nil {
			m.status = msg.Data.RunStatus
			m.pipelineID = msg.Data.PipelineID
			m.pipelineName = msg.Data.PipelineName
			m.runID = msg.Data.RunID
		}
		m.loading = false
		m.spinner = m.spinner.SetActive(false)
		m.updateViewportForSelectedTab(true)
		m.updateStatusLine()
		return m, nil

	case types.RunStageTabsUpdatedMsg:
		oldSelected := -1
		atBottom := m.viewport.AtBottom()
		if m.tabsData != nil {
			oldSelected = m.tabsData.SelectedIndex
		}

		m.tabsData = msg.Data
		if msg.Data != nil {
			m.status = msg.Data.RunStatus
		}

		// If selection changed (auto-advance), jump to end of new stage logs.
		if m.tabsData != nil && m.tabsData.SelectedIndex != oldSelected {
			m.updateViewportForSelectedTab(true)
		} else if msg.HasNewContent && m.tabsData != nil {
			// Only update viewport if we're viewing the active stage.
			if m.tabsData.SelectedIndex == m.tabsData.ActiveIndex {
				m.updateViewportForSelectedTab(atBottom)
				if atBottom {
					m.viewport = m.viewport.ScrollToEnd()
				}
			}
		}

		m.updateStatusLine()
		return m, nil

	case types.TickMsg:
		// Auto-refresh tick (running runs only)
		if m.autoRefresh && m.IsRunning() && m.tabsData != nil {
			cmds = append(cmds, func() tea.Msg {
				return LogsTabsRefreshMsg{TabsData: m.tabsData}
			})
		}

	case types.CopyToClipboardMsg:
		if msg.Success {
			m.copyNotice = "Copied!"
			// Clear the notice after 2 seconds
			cmds = append(cmds, clearCopyNoticeCmd())
		} else if msg.Error != nil {
			m.copyNotice = fmt.Sprintf("Copy failed: %s", msg.Error.Error())
			cmds = append(cmds, clearCopyNoticeCmd())
		}
		return m, tea.Batch(cmds...)

	case clearCopyNoticeMsg:
		m.copyNotice = ""
		return m, nil

	case tea.WindowSizeMsg:
		m = m.SetSize(msg.Width, msg.Height)
	}

	// Spinner updates
	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m LogsModel) View() string {
	var b strings.Builder

	// Title line
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	title := m.pipelineName
	if title == "" {
		title = m.pipelineID
	}
	if m.runID != "" {
		title = fmt.Sprintf("%s (Run #%s)", title, m.runID)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")

	// Tabs line
	b.WriteString(m.renderTabsLine())
	b.WriteString("\n")

	// Search bar
	if m.searchActive {
		b.WriteString(m.search.View())
		b.WriteString("\n")
	}

	// Main logs area - always render the viewport to maintain consistent layout
	// When loading, show spinner content inside the viewport
	if m.loading && (m.tabsData == nil || len(m.tabsData.Stages) == 0) {
		// Create a temporary viewport with spinner content to maintain layout
		spinnerContent := m.spinner.View()
		tempViewport := m.viewport.SetContent(spinnerContent)
		b.WriteString(tempViewport.View())
	} else {
		b.WriteString(m.viewport.View())
	}

	b.WriteString("\n")

	// Status line
	m.updateStatusLine()
	b.WriteString(m.statusLine.View())
	b.WriteString("\n")

	// Help line
	helpItems := []types.HelpItem{
		{Key: "Tab/S-Tab", Desc: "switch stage"},
		{Key: "j/k", Desc: "scroll"},
		{Key: "r", Desc: "refresh"},
	}
	if m.IsRunning() {
		helpItems = append(helpItems, types.HelpItem{Key: "X", Desc: "stop"})
	}
	helpItems = append(helpItems,
		types.HelpItem{Key: "/", Desc: "search"},
		types.HelpItem{Key: "e", Desc: "editor"},
		types.HelpItem{Key: "yy", Desc: "copy"},
		types.HelpItem{Key: "q", Desc: "back"},
	)

	// Show copy notice if active
	if m.copyNotice != "" {
		noticeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E")).Bold(true)
		b.WriteString(noticeStyle.Render(m.copyNotice))
		b.WriteString(" | ")
	}
	b.WriteString(types.RenderHelpLine(helpItems))

	return b.String()
}

func (m LogsModel) renderTabsLine() string {
	if m.tabsData == nil || len(m.tabsData.Stages) == 0 {
		// Show a placeholder tab when loading to maintain consistent layout
		if m.loading {
			placeholderStyle := lipgloss.NewStyle().
				Background(lipgloss.Color("#374151")).
				Foreground(lipgloss.Color("#9CA3AF")).
				Italic(true).
				Padding(0, 1)
			return placeholderStyle.Render("○ Loading stages...")
		}
		empty := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
		return empty.Render("No stages")
	}

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 1)
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB")).
		Padding(0, 1)
	activeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22C55E")).
		Bold(true)

	var parts []string
	for i, tab := range m.tabsData.Stages {
		icon := stageStatusIcon(tab.Status)
		label := fmt.Sprintf("%s %s", icon, tab.Name)

		style := normalStyle
		if i == m.tabsData.SelectedIndex {
			style = selectedStyle
		}
		rendered := style.Render(label)

		// Mark active stage (only if different from selected)
		if i == m.tabsData.ActiveIndex && i != m.tabsData.SelectedIndex && m.IsRunning() {
			rendered = activeStyle.Render("●") + " " + rendered
		}

		parts = append(parts, rendered)
	}

	return strings.Join(parts, " ")
}

func stageStatusIcon(status types.StageTabStatus) string {
	switch status {
	case types.StageTabStatusSuccess:
		return "✔"
	case types.StageTabStatusFailed:
		return "✘"
	case types.StageTabStatusRunning:
		return "●"
	case types.StageTabStatusCanceled:
		return "⊘"
	default:
		return "○"
	}
}

func (m *LogsModel) updateViewportForSelectedTab(scrollToEnd bool) {
	if m.tabsData == nil || len(m.tabsData.Stages) == 0 {
		m.viewport = m.viewport.SetTitle("")
		m.viewport = m.viewport.SetContent("")
		return
	}

	idx := m.tabsData.SelectedIndex
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.tabsData.Stages) {
		idx = len(m.tabsData.Stages) - 1
	}

	tab := m.tabsData.Stages[idx]
	title := tab.Name
	m.viewport = m.viewport.SetTitle(title)

	content := buildStageTabLogText(tab)
	m.viewport = m.viewport.SetContent(content)
	if scrollToEnd && m.IsRunning() && idx == m.tabsData.ActiveIndex {
		m.viewport = m.viewport.ScrollToEnd()
	}
}

func buildStageTabLogText(tab types.StageTab) string {
	if !tab.Loaded {
		return "Loading logs for this stage...\n"
	}
	if len(tab.Entries) == 0 {
		return "No logs yet.\n"
	}

	var b strings.Builder
	var lastJobID int64 = -1

	for _, e := range tab.Entries {
		if e.JobID != lastJobID {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(fmt.Sprintf("=== Job: %s (Status: %s) ===\n", e.JobName, e.Status))
			lastJobID = e.JobID
		}

		// Step header (skip for pure job-level log entries where StepName == JobName)
		if e.StepName != "" && e.StepName != e.JobName {
			b.WriteString(fmt.Sprintf("--- Step: %s ---\n", e.StepName))
		}

		if e.Logs != "" {
			b.WriteString(e.Logs)
			if !strings.HasSuffix(e.Logs, "\n") {
				b.WriteString("\n")
			}
		}
	}

	if b.Len() == 0 {
		return "No logs yet.\n"
	}
	return b.String()
}

// LogsTabsRefreshMsg requests refreshing the stage-tabs data (used on TickMsg).
type LogsTabsRefreshMsg struct {
	TabsData *types.RunStageTabsData
}

// LogsTabLoadMsg requests loading/refreshing logs for a specific selected tab.
type LogsTabLoadMsg struct {
	TabsData *types.RunStageTabsData
	TabIndex int
}

// clearCopyNoticeMsg is sent to clear the copy notice after a delay
type clearCopyNoticeMsg struct{}

// clearCopyNoticeCmd returns a command that clears the copy notice after 2 seconds
func clearCopyNoticeCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return clearCopyNoticeMsg{}
	})
}


