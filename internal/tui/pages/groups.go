package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"flowt/internal/api"
	"flowt/internal/tui/components"
	"flowt/internal/tui/types"
)

// GroupsModel represents the groups list page
type GroupsModel struct {
	table    components.TableModel
	modeline components.ModeLineModel
	search   components.SearchModel
	spinner  components.SpinnerModel

	// Data
	allGroups []api.PipelineGroup
	groups    []api.PipelineGroup // Filtered list

	// State
	width        int
	height       int
	searchActive bool
	searchQuery  string
	loading      bool

	// Key bindings
	keys GroupsKeyMap
}

// GroupsKeyMap defines key bindings for the groups page
type GroupsKeyMap struct {
	Up           key.Binding
	Down         key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	CtrlPageUp   key.Binding
	CtrlPageDown key.Binding
	Home         key.Binding
	End          key.Binding
	Enter        key.Binding
	Search       key.Binding
	SearchNext   key.Binding
	SearchPrev   key.Binding
	Back         key.Binding
	Quit         key.Binding
}

// DefaultGroupsKeyMap returns default key bindings
func DefaultGroupsKeyMap() GroupsKeyMap {
	return GroupsKeyMap{
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
			key.WithHelp("enter", "select"),
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

// NewGroupsModel creates a new groups model
func NewGroupsModel() GroupsModel {
	columns := []table.Column{
		{Title: "ID", Width: 15},
		{Title: "Name", Width: 50},
	}

	t := components.NewTableModel(columns, "Pipeline Groups")
	ml := components.NewModeLineModel("Groups")
	search := components.NewSearchModel()
	spinner := components.NewSpinnerModel()

	return GroupsModel{
		table:    t,
		modeline: ml,
		search:   search,
		spinner:  spinner,
		keys:     DefaultGroupsKeyMap(),
	}
}

// SetSize sets the page size
func (m GroupsModel) SetSize(width, height int) GroupsModel {
	m.width = width
	m.height = height
	// Reserve space for modeline, search, and help line
	tableHeight := height - 4
	m.table = m.table.SetSize(width, tableHeight)
	m.modeline = m.modeline.SetWidth(width)
	m.search = m.search.SetWidth(width)
	return m
}

// SetGroups sets the groups data
func (m GroupsModel) SetGroups(groups []api.PipelineGroup) GroupsModel {
	m.allGroups = groups
	m.applyFilters()
	return m
}

// SetLoading sets the loading state
func (m GroupsModel) SetLoading(loading bool) GroupsModel {
	m.loading = loading
	m.spinner = m.spinner.SetActive(loading)
	if loading {
		m.spinner = m.spinner.SetMessage("Loading groups...")
	}
	return m
}

// applyFilters applies the current filters to the groups
func (m *GroupsModel) applyFilters() {
	filtered := make([]api.PipelineGroup, 0, len(m.allGroups))

	for _, g := range m.allGroups {
		// Apply search filter
		if m.searchQuery != "" && !types.FuzzyMatch(m.searchQuery, g.Name) {
			continue
		}
		filtered = append(filtered, g)
	}

	m.groups = filtered
	m.updateTable()
}

// updateTable updates the table rows from filtered groups
func (m *GroupsModel) updateTable() {
	rows := make([]table.Row, len(m.groups))
	rowData := make([]interface{}, len(m.groups))

	for i, g := range m.groups {
		rows[i] = table.Row{
			g.GroupID,
			types.TruncateString(g.Name, 50),
		}
		rowData[i] = g
	}

	m.table = m.table.SetRows(rows)
	m.table = m.table.SetRowData(rowData)
}

// updateModeline updates the mode line with current state
func (m *GroupsModel) updateModeline() {
	info := fmt.Sprintf("%d groups", len(m.groups))
	if len(m.groups) != len(m.allGroups) {
		info += fmt.Sprintf(" (of %d)", len(m.allGroups))
	}

	m.modeline = m.modeline.SetPage("Groups")
	m.modeline = m.modeline.SetInfo(info)
	m.modeline = m.modeline.SetHelp("")
}

// SelectedGroup returns the currently selected group
func (m GroupsModel) SelectedGroup() *api.PipelineGroup {
	data := m.table.SelectedRowData()
	if data == nil {
		return nil
	}
	g, ok := data.(api.PipelineGroup)
	if !ok {
		return nil
	}
	return &g
}

// Init implements tea.Model
func (m GroupsModel) Init() tea.Cmd {
	return m.spinner.Init()
}

// Update implements tea.Model
func (m GroupsModel) Update(msg tea.Msg) (GroupsModel, tea.Cmd) {
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
			m.applyFilters()
			m.updateModeline()
			return m, nil

		case components.SearchCancelMsg:
			m.searchActive = false
			m.search = m.search.Deactivate()
			m.searchQuery = "" // Clear search on cancel
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
			return m, func() tea.Msg {
				return types.GoBackMsg{}
			}

		case key.Matches(msg, m.keys.Enter):
			if group := m.SelectedGroup(); group != nil {
				return m, func() tea.Msg {
					return types.ViewModeChangedMsg{
						ViewMode:  types.ViewModePipelinesInGroup,
						GroupID:   group.GroupID,
						GroupName: group.Name,
					}
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

	case types.GroupsAPILoadedMsg:
		m.allGroups = msg.Groups
		m.loading = false
		m.spinner = m.spinner.SetActive(false)
		m.applyFilters()
		m.updateModeline()

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
func (m GroupsModel) View() string {
	var b strings.Builder

	// Search bar
	if m.searchActive {
		b.WriteString(m.search.View())
		b.WriteString("\n")
	}

	// Full screen loading indicator when initially loading data
	if m.loading && len(m.allGroups) == 0 {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)
		loadingText := loadingStyle.Render("Loading...")
		centered := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, loadingText)
		return centered
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
		{Key: "Enter", Desc: "select"},
		{Key: "/", Desc: "search"},
		{Key: "C-f/C-b", Desc: "page"},
		{Key: "q", Desc: "back"},
		{Key: "Q", Desc: "quit"},
	}
	b.WriteString(types.RenderHelpLine(helpItems))

	return b.String()
}

// Search searches for a query
func (m GroupsModel) Search(query string) GroupsModel {
	m.searchQuery = query
	m.applyFilters()
	m.table = m.table.Search(query)
	return m
}

// ClearSearch clears the search
func (m GroupsModel) ClearSearch() GroupsModel {
	m.searchQuery = ""
	m.applyFilters()
	m.table = m.table.ClearSearch()
	return m
}

