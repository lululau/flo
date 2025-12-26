package pages

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"flowt/internal/api"
	"flowt/internal/tui/components"
	"flowt/internal/tui/types"
)

// HistoryModel represents the run history page
type HistoryModel struct {
	table    components.TableModel
	modeline components.ModeLineModel
	search   components.SearchModel
	modal    components.ModalModel
	spinner  components.SpinnerModel

	// Data
	runs         []api.PipelineRun
	pipelineID   string
	pipelineName string
	groupID      string
	groupName    string

	// State
	width             int
	height            int
	searchActive      bool
	searchQuery       string
	loading           bool
	currentPage       int
	totalPages        int
	totalRuns         int
	loadingBranchInfo bool              // Loading branch info for run dialog
	repositoryURLs    map[string]string // Repository URLs from latest run

	// Key bindings
	keys HistoryKeyMap
}

// HistoryKeyMap defines key bindings for the history page
type HistoryKeyMap struct {
	Up           key.Binding
	Down         key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	CtrlPageUp   key.Binding
	CtrlPageDown key.Binding
	Home         key.Binding
	End          key.Binding
	Enter        key.Binding
	Run          key.Binding
	Stop         key.Binding
	NextPage     key.Binding
	PrevPage     key.Binding
	FirstPage    key.Binding
	Search       key.Binding
	SearchNext   key.Binding
	SearchPrev   key.Binding
	Back         key.Binding
	Quit         key.Binding
}

// DefaultHistoryKeyMap returns default key bindings
func DefaultHistoryKeyMap() HistoryKeyMap {
	return HistoryKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down"),
		),
		CtrlPageUp: key.NewBinding(
			key.WithKeys("ctrl+b"),
			key.WithHelp("C-b", "page up"),
		),
		CtrlPageDown: key.NewBinding(
			key.WithKeys("ctrl+f"),
			key.WithHelp("C-f", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g", "first"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G", "last"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "logs"),
		),
		Run: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "run"),
		),
		Stop: key.NewBinding(
			key.WithKeys("X"),
			key.WithHelp("X", "stop"),
		),
		NextPage: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "next page"),
		),
		PrevPage: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "prev page"),
		),
		FirstPage: key.NewBinding(
			key.WithKeys("0"),
			key.WithHelp("0", "first page"),
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

// NewHistoryModel creates a new history model
func NewHistoryModel() HistoryModel {
	columns := []table.Column{
		{Title: "Run ID", Width: 15},
		{Title: "Status", Width: 12},
		{Title: "Trigger", Width: 10},
		{Title: "Start Time", Width: 18},
		{Title: "Duration", Width: 10},
	}

	t := components.NewTableModel(columns, "Run History")
	ml := components.NewModeLineModel("History")
	search := components.NewSearchModel()
	spinner := components.NewSpinnerModel()

	return HistoryModel{
		table:       t,
		modeline:    ml,
		search:      search,
		spinner:     spinner,
		keys:        DefaultHistoryKeyMap(),
		currentPage: 1,
		totalPages:  1,
	}
}

// SetSize sets the page size
func (m HistoryModel) SetSize(width, height int) HistoryModel {
	m.width = width
	m.height = height
	// Reserve space for modeline, search, and help line
	tableHeight := height - 4
	m.table = m.table.SetSize(width, tableHeight)
	m.modeline = m.modeline.SetWidth(width)
	m.search = m.search.SetWidth(width)
	m.modal = m.modal.SetSize(width, height)
	return m
}

// SetPipeline sets the pipeline context
func (m HistoryModel) SetPipeline(id, name, groupID, groupName string) HistoryModel {
	m.pipelineID = id
	m.pipelineName = name
	m.groupID = groupID
	m.groupName = groupName
	m.table = m.table.SetTitle(fmt.Sprintf("Run History: %s", name))
	return m
}

// SetRuns sets the run history data
func (m HistoryModel) SetRuns(runs []api.PipelineRun) HistoryModel {
	m.runs = runs
	m.updateTable()
	return m
}

// SetPagination sets pagination info
func (m HistoryModel) SetPagination(currentPage, totalPages, totalRuns int) HistoryModel {
	m.currentPage = currentPage
	m.totalPages = totalPages
	m.totalRuns = totalRuns
	return m
}

// SetLoading sets the loading state
func (m HistoryModel) SetLoading(loading bool) HistoryModel {
	m.loading = loading
	m.spinner = m.spinner.SetActive(loading)
	if loading {
		m.spinner = m.spinner.SetMessage("Loading history...")
	}
	return m
}

// updateTable updates the table rows from runs
func (m *HistoryModel) updateTable() {
	rows := make([]table.Row, len(m.runs))
	rowData := make([]interface{}, len(m.runs))

	for i, r := range m.runs {
		// Format start time
		startTime := "-"
		if !r.StartTime.IsZero() {
			startTime = r.StartTime.Local().Format("2006-01-02 15:04")
		}

		// Calculate duration
		duration := "-"
		if !r.StartTime.IsZero() && !r.FinishTime.IsZero() {
			d := r.FinishTime.Sub(r.StartTime)
			duration = formatDuration(d)
		} else if !r.StartTime.IsZero() {
			// Still running
			d := time.Since(r.StartTime)
			duration = formatDuration(d) + "+"
		}

		rows[i] = table.Row{
			r.RunID,
			r.Status,
			r.TriggerMode,
			startTime,
			duration,
		}
		rowData[i] = r
	}

	m.table = m.table.SetRows(rows)
	m.table = m.table.SetRowData(rowData)
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// updateModeline updates the mode line with current state
func (m *HistoryModel) updateModeline() {
	info := fmt.Sprintf("Page %d/%d | %d runs", m.currentPage, m.totalPages, m.totalRuns)

	m.modeline = m.modeline.SetPage(fmt.Sprintf("History: %s", types.TruncateString(m.pipelineName, 30)))
	m.modeline = m.modeline.SetInfo(info)
	m.modeline = m.modeline.SetHelp("")
}

// SelectedRun returns the currently selected run
func (m HistoryModel) SelectedRun() *api.PipelineRun {
	data := m.table.SelectedRowData()
	if data == nil {
		return nil
	}
	r, ok := data.(api.PipelineRun)
	if !ok {
		return nil
	}
	return &r
}

// GetPipelineID returns the pipeline ID
func (m HistoryModel) GetPipelineID() string {
	return m.pipelineID
}

// GetPipelineName returns the pipeline name
func (m HistoryModel) GetPipelineName() string {
	return m.pipelineName
}

// GetCurrentPage returns the current page
func (m HistoryModel) GetCurrentPage() int {
	return m.currentPage
}

// Init implements tea.Model
func (m HistoryModel) Init() tea.Cmd {
	return m.spinner.Init()
}

// Update implements tea.Model
func (m HistoryModel) Update(msg tea.Msg) (HistoryModel, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle modal messages (these arrive AFTER the modal has closed itself)
	switch msg := msg.(type) {
	case components.ModalConfirmMsg:
		// Handle branch input confirmation
		if branchInput, ok := msg.Data.(string); ok && branchInput != "" {
			cmds = append(cmds, func() tea.Msg {
				return types.BranchSelectedMsg{Branch: branchInput}
			})
		}
		m.modal = m.modal.Hide()
		return m, tea.Batch(cmds...)

	case components.ModalCancelMsg, components.ModalDismissMsg:
		m.modal = m.modal.Hide()
		return m, nil
	}

	// Handle modal input if visible
	if m.modal.Visible {
		var cmd tea.Cmd
		m.modal, cmd = m.modal.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

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
			m.table = m.table.Search(msg.Query)
			return m, nil

		case components.SearchCancelMsg:
			m.searchActive = false
			m.search = m.search.Deactivate()
			m.searchQuery = ""
			m.table = m.table.ClearSearch()
			return m, nil

		case components.SearchQueryChangedMsg:
			// Real-time filtering as user types
			m.searchQuery = msg.Query
			m.table = m.table.Search(msg.Query)
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			return m, cmd
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

		case key.Matches(msg, m.keys.Enter):
			if run := m.SelectedRun(); run != nil {
				return m, func() tea.Msg {
					return types.NavigateMsg{
						Page: types.PageLogs,
						Data: types.RunContext{
							PipelineID:   m.pipelineID,
							PipelineName: m.pipelineName,
							RunID:        run.RunID,
							Status:       run.Status,
							IsNewRun:     false,
						},
					}
				}
			}

		case key.Matches(msg, m.keys.Run):
			// #region agent log
			if f, err := os.OpenFile("/Users/liuxiang/cascode/github.com/flowt/.cursor/debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil { f.WriteString(fmt.Sprintf(`{"hypothesisId":"C","location":"history.go:Run","message":"r key pressed in history, requesting branch info","data":{"pipelineID":"%s"},"timestamp":%d}`+"\n", m.pipelineID, time.Now().UnixMilli())); f.Close() }
			// #endregion
			m.loadingBranchInfo = true
			return m, func() tea.Msg {
				return types.LoadBranchInfoMsg{PipelineID: m.pipelineID}
			}

		case key.Matches(msg, m.keys.Stop):
			if run := m.SelectedRun(); run != nil {
				status := strings.ToUpper(run.Status)
				if status == "RUNNING" || status == "QUEUED" || status == "INIT" {
					return m, func() tea.Msg {
						return StopRunRequestMsg{
							PipelineID: m.pipelineID,
							RunID:      run.RunID,
						}
					}
				}
			}

		case key.Matches(msg, m.keys.NextPage):
			if m.currentPage < m.totalPages {
				return m, func() tea.Msg {
					return HistoryPageChangeMsg{Page: m.currentPage + 1}
				}
			}

		case key.Matches(msg, m.keys.PrevPage):
			if m.currentPage > 1 {
				return m, func() tea.Msg {
					return HistoryPageChangeMsg{Page: m.currentPage - 1}
				}
			}

		case key.Matches(msg, m.keys.FirstPage):
			if m.currentPage != 1 {
				return m, func() tea.Msg {
					return HistoryPageChangeMsg{Page: 1}
				}
			}

		case key.Matches(msg, m.keys.Search):
			m.searchActive = true
			m.search = m.search.Activate()
			return m, m.search.Focus()

		case key.Matches(msg, m.keys.SearchNext):
			m.table = m.table.NextSearchMatch()

		case key.Matches(msg, m.keys.SearchPrev):
			m.table = m.table.PrevSearchMatch()

		case key.Matches(msg, m.keys.CtrlPageDown):
			m.table = m.table.PageDown()

		case key.Matches(msg, m.keys.CtrlPageUp):
			m.table = m.table.PageUp()
		}

	case types.HistoryAPILoadedMsg:
		m.runs = msg.Runs
		m.currentPage = msg.CurrentPage
		m.totalPages = msg.TotalPages
		m.totalRuns = msg.TotalRuns
		m.loading = false
		m.spinner = m.spinner.SetActive(false)
		m.updateTable()
		m.updateModeline()

	case types.BranchInfoLoadedMsg:
		// #region agent log
		if f, err := os.OpenFile("/Users/liuxiang/cascode/github.com/flowt/.cursor/debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil { f.WriteString(fmt.Sprintf(`{"hypothesisId":"C","location":"history.go:BranchInfoLoaded","message":"branch info loaded in history, showing modal","data":{"defaultBranch":"%s","repoCount":%d},"timestamp":%d}`+"\n", msg.DefaultBranch, len(msg.RepositoryURLs), time.Now().UnixMilli())); f.Close() }
		// #endregion
		m.loadingBranchInfo = false
		m.repositoryURLs = msg.RepositoryURLs
		m.modal = components.NewInputModal(
			"Run Pipeline",
			fmt.Sprintf("Branch (default: %s)", msg.DefaultBranch),
			msg.DefaultBranch,
		)
		m.modal = m.modal.SetSize(m.width, m.height)

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

	// Update table
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m HistoryModel) View() string {
	var b strings.Builder

	// Modal overlay
	if m.modal.Visible {
		return m.modal.View()
	}

	// Search bar
	if m.searchActive {
		b.WriteString(m.search.View())
		b.WriteString("\n")
	}

	// Loading spinner
	if m.loading {
		b.WriteString(m.spinner.View())
		b.WriteString("\n")
	}

	// Table
	tableView := m.table.View()
	b.WriteString(tableView)
	b.WriteString("\n")

	// Mode line
	m.updateModeline()
	b.WriteString(m.modeline.View())
	b.WriteString("\n")

	// Help line
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	help := "Enter=logs r=run X=stop [/]=prev/next page /=search C-f/C-b=scroll q=back Q=quit"
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

// Helper to render status with color
func historyRenderStatus(status string) string {
	style := lipgloss.NewStyle()
	switch strings.ToUpper(status) {
	case "SUCCESS":
		style = style.Foreground(lipgloss.Color("#10B981"))
	case "RUNNING", "QUEUED", "INIT":
		style = style.Foreground(lipgloss.Color("#22C55E"))
	case "FAILED", "FAIL":
		style = style.Foreground(lipgloss.Color("#EF4444"))
	case "CANCELED":
		style = style.Foreground(lipgloss.Color("#9CA3AF"))
	default:
		style = style.Foreground(lipgloss.Color("#6B7280"))
	}
	return style.Render(status)
}

// HistoryPageChangeMsg requests changing the history page
type HistoryPageChangeMsg struct {
	Page int
}

// StopRunRequestMsg requests stopping a run
type StopRunRequestMsg struct {
	PipelineID string
	RunID      string
}

