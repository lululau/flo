package pages

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"flowt/internal/config"
	"flowt/internal/tui/components"
	"flowt/internal/tui/types"
)

// LogsModel represents the logs view page
type LogsModel struct {
	viewport   components.ViewportModel
	statusLine components.StatusModeLineModel
	search     components.SearchModel
	spinner    components.SpinnerModel

	// Data
	pipelineID   string
	pipelineName string
	runID        string
	status       string
	content      string
	config       *config.Config

	// State
	width         int
	height        int
	searchActive  bool
	searchQuery   string
	loading       bool
	autoRefresh   bool
	isNewRun      bool
	currentJob    int
	totalJobs     int
	loadComplete  bool
	refreshTicker *time.Ticker
	stopRefresh   chan struct{}

	// Incremental refresh state
	streamState        *types.LogStreamState // State for incremental log fetching
	incrementalEnabled bool                  // Whether incremental refresh is active

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
	Refresh      key.Binding
	Stop         key.Binding
	OpenEditor   key.Binding
	OpenPager    key.Binding
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
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
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
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		SearchNext: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next"),
		),
		SearchPrev: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "prev"),
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

// NewLogsModel creates a new logs model
func NewLogsModel(cfg *config.Config) LogsModel {
	vp := components.NewViewportModel("Logs")
	sl := components.NewStatusModeLineModel()
	search := components.NewSearchModel()
	spinner := components.NewSpinnerModel()

	return LogsModel{
		viewport:   vp,
		statusLine: sl,
		search:     search,
		spinner:    spinner,
		config:     cfg,
		keys:       DefaultLogsKeyMap(),
	}
}

// SetConfig sets the configuration
func (m LogsModel) SetConfig(cfg *config.Config) LogsModel {
	m.config = cfg
	return m
}

// SetSize sets the page size
func (m LogsModel) SetSize(width, height int) LogsModel {
	m.width = width
	m.height = height
	// Reserve space for status line and search
	vpHeight := height - 2
	if m.searchActive {
		vpHeight--
	}
	m.viewport = m.viewport.SetSize(width, vpHeight)
	m.statusLine = m.statusLine.SetWidth(width)
	m.search = m.search.SetWidth(width)
	return m
}

// SetRun sets the run context
func (m LogsModel) SetRun(pipelineID, pipelineName, runID, status string, isNewRun bool) LogsModel {
	m.pipelineID = pipelineID
	m.pipelineName = pipelineName
	m.runID = runID
	m.status = status
	m.isNewRun = isNewRun
	m.viewport = m.viewport.SetTitle(fmt.Sprintf("Logs: %s (Run #%s)", pipelineName, runID))
	// Reset incremental state for new run context
	m.streamState = types.NewLogStreamState(pipelineID, runID)
	m.incrementalEnabled = false
	return m
}

// SetContent sets the log content
func (m LogsModel) SetContent(content string) LogsModel {
	m.content = content
	m.viewport = m.viewport.SetContent(content)
	return m
}

// AppendContent appends content to the log
func (m LogsModel) AppendContent(content string) LogsModel {
	m.content += content
	m.viewport = m.viewport.AppendContent(content)
	return m
}

// SetStatus sets the run status
func (m LogsModel) SetStatus(status string) LogsModel {
	m.status = status
	return m
}

// SetLoading sets the loading state
func (m LogsModel) SetLoading(loading bool) LogsModel {
	m.loading = loading
	m.spinner = m.spinner.SetActive(loading)
	if loading {
		m.spinner = m.spinner.SetMessage("Loading logs...")
	}
	return m
}

// SetLoadProgress sets the loading progress
func (m LogsModel) SetLoadProgress(currentJob, totalJobs int, complete bool) LogsModel {
	m.currentJob = currentJob
	m.totalJobs = totalJobs
	m.loadComplete = complete
	if !complete && totalJobs > 0 {
		m.spinner = m.spinner.SetMessage(fmt.Sprintf("Loading job %d/%d...", currentJob, totalJobs))
	}
	return m
}

// SetAutoRefresh sets the auto-refresh state
func (m LogsModel) SetAutoRefresh(enabled bool) LogsModel {
	m.autoRefresh = enabled
	return m
}

// GetPipelineID returns the pipeline ID
func (m LogsModel) GetPipelineID() string {
	return m.pipelineID
}

// GetRunID returns the run ID
func (m LogsModel) GetRunID() string {
	return m.runID
}

// GetStatus returns the current status
func (m LogsModel) GetStatus() string {
	return m.status
}

// GetContent returns the log content
func (m LogsModel) GetContent() string {
	return m.content
}

// IsRunning returns whether the run is still active
func (m LogsModel) IsRunning() bool {
	status := strings.ToUpper(m.status)
	return status == "RUNNING" || status == "QUEUED" || status == "INIT"
}

// GetStreamState returns the current log stream state
func (m LogsModel) GetStreamState() *types.LogStreamState {
	return m.streamState
}

// SetStreamState sets the log stream state
func (m LogsModel) SetStreamState(state *types.LogStreamState) LogsModel {
	m.streamState = state
	return m
}

// IsIncrementalEnabled returns whether incremental refresh is enabled
func (m LogsModel) IsIncrementalEnabled() bool {
	return m.incrementalEnabled && m.streamState != nil && m.streamState.Initialized
}

// SetIncrementalEnabled enables or disables incremental refresh mode
func (m LogsModel) SetIncrementalEnabled(enabled bool) LogsModel {
	m.incrementalEnabled = enabled
	return m
}

// updateStatusLine updates the status line
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

	// Handle search input if active
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

	// Handle key messages
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg {
				return types.GoBackMsg{}
			}

		case key.Matches(msg, m.keys.Refresh):
			return m, func() tea.Msg {
				return LogsRefreshMsg{
					PipelineID: m.pipelineID,
					RunID:      m.runID,
				}
			}

		case key.Matches(msg, m.keys.Stop):
			if m.IsRunning() {
				return m, func() tea.Msg {
					return StopRunRequestMsg{
						PipelineID: m.pipelineID,
						RunID:      m.runID,
					}
				}
			}

		case key.Matches(msg, m.keys.OpenEditor):
			if m.content != "" {
				editor := m.config.GetEditor()
				return m, types.OpenInEditorCmd(m.content, editor)
			}

		case key.Matches(msg, m.keys.OpenPager):
			if m.content != "" {
				pager := m.config.GetPager()
				return m, types.OpenInPagerCmd(m.content, pager)
			}

		case key.Matches(msg, m.keys.Search):
			m.searchActive = true
			m.search = m.search.Activate()
			return m, m.search.Focus()

		case key.Matches(msg, m.keys.SearchNext):
			m.viewport = m.viewport.NextSearchMatch()
			m.updateStatusLine()

		case key.Matches(msg, m.keys.SearchPrev):
			m.viewport = m.viewport.PrevSearchMatch()
			m.updateStatusLine()

		case key.Matches(msg, m.keys.Home):
			m.viewport = m.viewport.ScrollToTop()

		case key.Matches(msg, m.keys.End):
			m.viewport = m.viewport.ScrollToEnd()
		}

	case types.LogsAPILoadedMsg:
		m.content = msg.LogContent
		m.status = msg.Status
		m.currentJob = msg.CurrentJob
		m.totalJobs = msg.TotalJobs
		m.loadComplete = msg.IsComplete
		m.viewport = m.viewport.SetContent(m.content)
		m.loading = false
		m.spinner = m.spinner.SetActive(false)
		
		// Update stream state if provided (for incremental loading)
		if msg.StreamState != nil {
			m.streamState = msg.StreamState
			m.incrementalEnabled = true
		}
		
		m.updateStatusLine()

		// Scroll to bottom for new runs or running pipelines
		if m.isNewRun || m.IsRunning() {
			m.viewport = m.viewport.ScrollToEnd()
		}

	case types.LogsProgressMsg:
		if msg.AppendMode {
			m.content += msg.Content
			m.viewport = m.viewport.AppendContent(msg.Content)
		} else {
			m.content = msg.Content
			m.viewport = m.viewport.SetContent(msg.Content)
		}
		m.status = msg.Status
		m.currentJob = msg.CurrentJob
		m.totalJobs = msg.TotalJobs
		m.loadComplete = msg.IsComplete

		if msg.IsComplete {
			m.loading = false
			m.spinner = m.spinner.SetActive(false)
		} else {
			m.spinner = m.spinner.SetMessage(fmt.Sprintf("Loading job %d/%d...", msg.CurrentJob, msg.TotalJobs))
		}

		m.updateStatusLine()

		// Auto scroll for running pipelines
		if m.IsRunning() && m.viewport.AtBottom() {
			m.viewport = m.viewport.ScrollToEnd()
		}

	case types.LogsIncrementalLoadedMsg:
		// Handle incremental log update
		m.status = msg.Status
		if msg.StreamState != nil {
			m.streamState = msg.StreamState
		}
		
		if msg.HasNewContent && msg.IncrementalContent != "" {
			m.content += msg.IncrementalContent
			m.viewport = m.viewport.AppendContent(msg.IncrementalContent)
			
			// Auto scroll if at bottom
			if m.viewport.AtBottom() {
				m.viewport = m.viewport.ScrollToEnd()
			}
		}
		m.updateStatusLine()

	case types.TickMsg:
		// Handle auto-refresh tick
		if m.autoRefresh && m.IsRunning() {
			cmds = append(cmds, func() tea.Msg {
				return LogsIncrementalRefreshMsg{
					PipelineID:  m.pipelineID,
					RunID:       m.runID,
					StreamState: m.streamState,
				}
			})
		}

	case types.EditorClosedMsg, types.PagerClosedMsg:
		// Nothing special needed after editor/pager closes

	case types.WindowSizeMsg:
		m = m.SetSize(msg.Width, msg.Height)
	}

	// Update spinner
	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m LogsModel) View() string {
	var b strings.Builder

	// Search bar
	if m.searchActive {
		b.WriteString(m.search.View())
		b.WriteString("\n")
	}

	// Loading spinner
	if m.loading && m.content == "" {
		b.WriteString(m.spinner.View())
		b.WriteString("\n")
	}

	// Viewport
	vpView := m.viewport.View()
	b.WriteString(vpView)
	b.WriteString("\n")

	// Status line
	m.updateStatusLine()
	b.WriteString(m.statusLine.View())

	return b.String()
}

// Helper to get status style
func logsStatusStyle(status string) lipgloss.Style {
	style := lipgloss.NewStyle()
	switch strings.ToUpper(status) {
	case "SUCCESS":
		return style.Foreground(lipgloss.Color("#10B981"))
	case "RUNNING", "QUEUED", "INIT":
		return style.Foreground(lipgloss.Color("#22C55E"))
	case "FAILED", "FAIL":
		return style.Foreground(lipgloss.Color("#EF4444"))
	case "CANCELED":
		return style.Foreground(lipgloss.Color("#9CA3AF"))
	default:
		return style.Foreground(lipgloss.Color("#6B7280"))
	}
}

// LogsRefreshMsg requests refreshing the logs (full refresh)
type LogsRefreshMsg struct {
	PipelineID string
	RunID      string
}

// LogsIncrementalRefreshMsg requests incremental log refresh
type LogsIncrementalRefreshMsg struct {
	PipelineID  string
	RunID       string
	StreamState *types.LogStreamState
}

// AutoRefreshTickCmd returns a command for auto-refresh ticking
func AutoRefreshTickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return types.TickMsg{}
	})
}

