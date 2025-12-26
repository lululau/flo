package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"flowt/internal/api"
	"flowt/internal/config"
	"flowt/internal/tui/components"
	"flowt/internal/tui/types"
)

// PipelinesModel represents the pipelines list page
type PipelinesModel struct {
	table       components.TableModel
	modeline    components.ModeLineModel
	search      components.SearchModel
	modal       components.ModalModel
	spinner     components.SpinnerModel

	// Data
	allPipelines []api.Pipeline
	pipelines    []api.Pipeline // Filtered list
	config       *config.Config

	// State
	width             int
	height            int
	filterMode        types.FilterMode
	searchActive      bool
	searchQuery       string
	loading           bool
	viewMode          types.ViewMode
	groupID           string
	groupName         string
	currentPage       int
	totalPages        int
	loadingComplete   bool
	loadingBranchInfo bool                      // Loading branch info for run dialog
	repositoryURLs    map[string]string         // Repository URLs from latest run

	// Key bindings
	keys PipelinesKeyMap
}

// PipelinesKeyMap defines key bindings for the pipelines page
type PipelinesKeyMap struct {
	Up             key.Binding
	Down           key.Binding
	PageUp         key.Binding
	PageDown       key.Binding
	CtrlPageUp     key.Binding
	CtrlPageDown   key.Binding
	Home           key.Binding
	End            key.Binding
	Enter          key.Binding
	Run            key.Binding
	ToggleBookmark key.Binding
	FilterBookmark key.Binding
	FilterStatus   key.Binding
	SwitchToGroups key.Binding
	Search         key.Binding
	SearchNext     key.Binding
	SearchPrev     key.Binding
	Back           key.Binding
	Quit           key.Binding
}

// DefaultPipelinesKeyMap returns default key bindings
func DefaultPipelinesKeyMap() PipelinesKeyMap {
	return PipelinesKeyMap{
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
			key.WithHelp("enter", "history"),
		),
		Run: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "run"),
		),
		ToggleBookmark: key.NewBinding(
			key.WithKeys("B"),
			key.WithHelp("B", "bookmark"),
		),
		FilterBookmark: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "filter ★"),
		),
		FilterStatus: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "filter status"),
		),
		SwitchToGroups: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "groups"),
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

// NewPipelinesModel creates a new pipelines model
func NewPipelinesModel(cfg *config.Config) PipelinesModel {
	columns := []table.Column{
		{Title: "★", Width: 3},
		{Title: "Name", Width: 40},
		{Title: "Status", Width: 12},
	}

	t := components.NewTableModel(columns, "Pipelines")
	ml := components.NewModeLineModel("Pipelines")
	search := components.NewSearchModel()
	spinner := components.NewSpinnerModel()

	return PipelinesModel{
		table:    t,
		modeline: ml,
		search:   search,
		spinner:  spinner,
		config:   cfg,
		keys:     DefaultPipelinesKeyMap(),
		loading:  true, // Start with loading state
	}
}

// SetConfig sets the configuration
func (m PipelinesModel) SetConfig(cfg *config.Config) PipelinesModel {
	m.config = cfg
	return m
}

// SetSize sets the page size
func (m PipelinesModel) SetSize(width, height int) PipelinesModel {
	m.width = width
	m.height = height
	// Reserve space for modeline, search, and help line
	tableHeight := height - 4
	m.table = m.table.SetSize(width, tableHeight)
	m.modeline = m.modeline.SetWidth(width)
	m.search = m.search.SetWidth(width)
	return m
}

// SetPipelines sets the pipelines data
func (m PipelinesModel) SetPipelines(pipelines []api.Pipeline) PipelinesModel {
	m.allPipelines = pipelines
	m.applyFilters()
	return m
}

// SetViewMode sets the view mode (all pipelines or group pipelines)
func (m PipelinesModel) SetViewMode(mode types.ViewMode, groupID, groupName string) PipelinesModel {
	m.viewMode = mode
	m.groupID = groupID
	m.groupName = groupName
	return m
}

// SetLoading sets the loading state
func (m PipelinesModel) SetLoading(loading bool) PipelinesModel {
	m.loading = loading
	m.spinner = m.spinner.SetActive(loading)
	if loading {
		m.spinner = m.spinner.SetMessage("Loading pipelines...")
	}
	return m
}

// SetLoadingProgress sets the loading progress
func (m PipelinesModel) SetLoadingProgress(current, total int, complete bool) PipelinesModel {
	m.currentPage = current
	m.totalPages = total
	m.loadingComplete = complete
	if !complete {
		m.spinner = m.spinner.SetMessage(fmt.Sprintf("Loading page %d/%d...", current, total))
	}
	return m
}

// applyFilters applies the current filters to the pipelines
func (m *PipelinesModel) applyFilters() {
	filtered := make([]api.Pipeline, 0, len(m.allPipelines))

	for _, p := range m.allPipelines {
		// Apply bookmark filter
		if m.filterMode == types.FilterModeBookmarked && !m.config.IsBookmarked(p.Name) {
			continue
		}

		// Apply status filter (running/waiting)
		if m.filterMode == types.FilterModeRunningWaiting {
			status := strings.ToUpper(p.LastRunStatus)
			if status != "RUNNING" && status != "QUEUED" && status != "INIT" {
				continue
			}
		}

		// Apply search filter
		if m.searchQuery != "" && !types.FuzzyMatch(m.searchQuery, p.Name) {
			continue
		}

		filtered = append(filtered, p)
	}

	m.pipelines = filtered
	m.updateTable()
}

// updateTable updates the table rows from filtered pipelines
func (m *PipelinesModel) updateTable() {
	rows := make([]table.Row, len(m.pipelines))
	rowData := make([]interface{}, len(m.pipelines))

	for i, p := range m.pipelines {
		bookmark := " "
		if m.config.IsBookmarked(p.Name) {
			bookmark = "★"
		}

		rows[i] = table.Row{
			bookmark,
			types.TruncateString(p.Name, 40),
			p.LastRunStatus,
		}
		rowData[i] = p
	}

	m.table = m.table.SetRows(rows)
	m.table = m.table.SetRowData(rowData)
}

// updateModeline updates the mode line with current state
func (m *PipelinesModel) updateModeline() {
	title := "Pipelines"
	if m.viewMode == types.ViewModePipelinesInGroup {
		title = fmt.Sprintf("Group: %s", m.groupName)
	}

	var status string
	switch m.filterMode {
	case types.FilterModeBookmarked:
		status = "Filter: ★ Bookmarked"
	case types.FilterModeRunningWaiting:
		status = "Filter: Running/Waiting"
	default:
		status = ""
	}

	info := fmt.Sprintf("%d pipelines", len(m.pipelines))
	if len(m.pipelines) != len(m.allPipelines) {
		info += fmt.Sprintf(" (of %d)", len(m.allPipelines))
	}

	m.modeline = m.modeline.SetPage(title)
	m.modeline = m.modeline.SetStatus(status)
	m.modeline = m.modeline.SetInfo(info)
	m.modeline = m.modeline.SetHelp("")
}

// SelectedPipeline returns the currently selected pipeline
func (m PipelinesModel) SelectedPipeline() *api.Pipeline {
	data := m.table.SelectedRowData()
	if data == nil {
		return nil
	}
	p, ok := data.(api.Pipeline)
	if !ok {
		return nil
	}
	return &p
}

// Init implements tea.Model
func (m PipelinesModel) Init() tea.Cmd {
	return m.spinner.Init()
}

// Update implements tea.Model
func (m PipelinesModel) Update(msg tea.Msg) (PipelinesModel, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle modal messages (these arrive AFTER the modal has closed itself)
	switch msg := msg.(type) {
	case components.ModalConfirmMsg:
		// Handle branch input confirmation
		if branchInput, ok := msg.Data.(string); ok && branchInput != "" {
			pipeline := m.SelectedPipeline()
			if pipeline != nil {
				cmds = append(cmds, func() tea.Msg {
					return types.BranchSelectedMsg{Branch: branchInput}
				})
			}
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
			m.applyFilters()
			m.updateModeline()
			return m, nil

		case components.SearchCancelMsg:
			m.searchActive = false
			m.search = m.search.Deactivate()
			m.searchQuery = "" // Clear the search on cancel
			m.applyFilters()
			m.updateModeline()
			return m, nil

		case components.SearchQueryChangedMsg:
			// Real-time filtering as user types
			m.searchQuery = msg.Query
			m.applyFilters()
			m.updateModeline()
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
			if m.viewMode == types.ViewModePipelinesInGroup {
				// Return to groups view
				return m, func() tea.Msg {
					return types.GoBackMsg{}
				}
			}
			return m, tea.Quit

		case key.Matches(msg, m.keys.Enter):
			if pipeline := m.SelectedPipeline(); pipeline != nil {
				return m, func() tea.Msg {
					return types.NavigateMsg{
						Page: types.PageHistory,
						Data: types.PipelineContext{
							PipelineID:   pipeline.PipelineID,
							PipelineName: pipeline.Name,
							GroupID:      m.groupID,
							GroupName:    m.groupName,
						},
					}
				}
			}

		case key.Matches(msg, m.keys.Run):
			if pipeline := m.SelectedPipeline(); pipeline != nil {
				m.loadingBranchInfo = true
				return m, func() tea.Msg {
					return types.LoadBranchInfoMsg{PipelineID: pipeline.PipelineID}
				}
			}

		case key.Matches(msg, m.keys.ToggleBookmark):
			if pipeline := m.SelectedPipeline(); pipeline != nil {
				isAdded := m.config.ToggleBookmark(pipeline.Name)
				_ = config.SaveConfig(m.config)
				m.applyFilters()
				m.updateModeline()
				return m, func() tea.Msg {
					return types.BookmarkToggledMsg{
						PipelineName: pipeline.Name,
						IsBookmarked: isAdded,
					}
				}
			}

		case key.Matches(msg, m.keys.FilterBookmark):
			if m.filterMode == types.FilterModeBookmarked {
				m.filterMode = types.FilterModeAll
			} else {
				m.filterMode = types.FilterModeBookmarked
			}
			m.applyFilters()
			m.updateModeline()

		case key.Matches(msg, m.keys.FilterStatus):
			if m.filterMode == types.FilterModeRunningWaiting {
				m.filterMode = types.FilterModeAll
			} else {
				m.filterMode = types.FilterModeRunningWaiting
			}
			m.applyFilters()
			m.updateModeline()

		case key.Matches(msg, m.keys.SwitchToGroups):
			return m, func() tea.Msg {
				return types.NavigateMsg{Page: types.PageGroupsList}
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
			// Ctrl+F for page down
			m.table = m.table.PageDown()

		case key.Matches(msg, m.keys.CtrlPageUp):
			// Ctrl+B for page up
			m.table = m.table.PageUp()
		}

	case types.PipelinesAPILoadedMsg:
		m.allPipelines = msg.Pipelines
		m.loading = false
		m.spinner = m.spinner.SetActive(false)
		m.loadingComplete = msg.IsComplete
		m.currentPage = msg.CurrentPage
		m.totalPages = msg.TotalPages
		m.applyFilters()
		m.updateModeline()

	case types.PipelinesProgressMsg:
		m.allPipelines = append(m.allPipelines, msg.Pipelines...)
		m.currentPage = msg.CurrentPage
		m.totalPages = msg.TotalPages
		m.loadingComplete = msg.IsComplete
		if msg.IsComplete {
			m.loading = false
			m.spinner = m.spinner.SetActive(false)
		} else {
			m.spinner = m.spinner.SetMessage(fmt.Sprintf("Loading page %d/%d...", msg.CurrentPage, msg.TotalPages))
		}
		m.applyFilters()
		m.updateModeline()

	case types.WindowSizeMsg:
		m = m.SetSize(msg.Width, msg.Height)

	case types.BranchInfoLoadedMsg:
		m.loadingBranchInfo = false
		m.repositoryURLs = msg.RepositoryURLs
		m.modal = components.NewInputModal(
			"Run Pipeline",
			fmt.Sprintf("Branch (default: %s)", msg.DefaultBranch),
			msg.DefaultBranch,
		)
		m.modal = m.modal.SetSize(m.width, m.height)
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
func (m PipelinesModel) View() string {
	var b strings.Builder

	// Modal overlay
	if m.modal.Visible {
		return m.modal.View()
	}

	// Full screen loading indicator when initially loading data
	if m.loading && len(m.allPipelines) == 0 {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)
		loadingText := loadingStyle.Render("Loading...")
		centered := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, loadingText)
		return centered
	}

	// Search bar
	if m.searchActive {
		b.WriteString(m.search.View())
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
	helpItems := []types.HelpItem{
		{Key: "Enter", Desc: "history"},
		{Key: "r", Desc: "run"},
		{Key: "a", Desc: "running/all"},
		{Key: "b", Desc: "bookmarks"},
		{Key: "B", Desc: "bookmark"},
		{Key: "C-g", Desc: "groups"},
		{Key: "/", Desc: "search"},
		{Key: "C-f/C-b", Desc: "page"},
		{Key: "q", Desc: "back"},
		{Key: "Q", Desc: "quit"},
	}
	b.WriteString(types.RenderHelpLine(helpItems))

	return b.String()
}

// Search searches for a query
func (m PipelinesModel) Search(query string) PipelinesModel {
	m.searchQuery = query
	m.applyFilters()
	m.table = m.table.Search(query)
	return m
}

// ClearSearch clears the search
func (m PipelinesModel) ClearSearch() PipelinesModel {
	m.searchQuery = ""
	m.applyFilters()
	m.table = m.table.ClearSearch()
	return m
}

// Helper to render status with color
func renderStatus(status string) string {
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

