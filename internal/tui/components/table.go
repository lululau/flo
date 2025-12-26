package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TableModel wraps bubbles/table with additional features
type TableModel struct {
	table       table.Model
	columns     []table.Column
	rows        []table.Row
	title       string
	width       int
	height      int
	focused     bool
	searchQuery string
	searchIndex int
	searchCount int
	matchRows   []int // Row indices that match search
	keys        TableKeyMap

	// Row data for additional operations
	rowData []interface{}

	// Styles
	styles TableStyles
}

// TableKeyMap defines key bindings for the table
type TableKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
	Enter    key.Binding
}

// DefaultTableKeyMap returns default key bindings
func DefaultTableKeyMap() TableKeyMap {
	return TableKeyMap{
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
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "first"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "last"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
	}
}

// TableStyles defines styles for the table
type TableStyles struct {
	Header      lipgloss.Style
	Cell        lipgloss.Style
	Selected    lipgloss.Style
	Border      lipgloss.Style
	Title       lipgloss.Style
	SearchMatch lipgloss.Style
}

// DefaultTableStyles returns default table styles
func DefaultTableStyles() TableStyles {
	return TableStyles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F59E0B")).
			Padding(0, 1),
		Cell: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			Padding(0, 1),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#374151")).
			Bold(true).
			Padding(0, 1),
		Border: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")),
		SearchMatch: lipgloss.NewStyle().
			Background(lipgloss.Color("#CA8A04")).
			Foreground(lipgloss.Color("#000000")),
	}
}

// NewTableModel creates a new table model
func NewTableModel(columns []table.Column, title string) TableModel {
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Set table styles
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("#F59E0B"))
	s.Selected = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#374151")).
		Bold(true)
	s.Cell = s.Cell.
		Foreground(lipgloss.Color("#E5E7EB"))

	t.SetStyles(s)
	t.Focus()

	return TableModel{
		table:   t,
		columns: columns,
		title:   title,
		keys:    DefaultTableKeyMap(),
		styles:  DefaultTableStyles(),
		focused: true,
	}
}

// SetRows sets the table rows
func (m TableModel) SetRows(rows []table.Row) TableModel {
	m.rows = rows
	m.table.SetRows(rows)
	// Clear search when data changes
	m.searchQuery = ""
	m.searchIndex = -1
	m.searchCount = 0
	m.matchRows = nil
	return m
}

// SetRowData sets the underlying data for each row
func (m TableModel) SetRowData(data []interface{}) TableModel {
	m.rowData = data
	return m
}

// SetTitle sets the table title
func (m TableModel) SetTitle(title string) TableModel {
	m.title = title
	return m
}

// SetSize sets the table size
func (m TableModel) SetSize(width, height int) TableModel {
	m.width = width
	m.height = height
	// Account for title and borders
	tableHeight := height - 4
	if tableHeight < 1 {
		tableHeight = 1
	}
	m.table.SetWidth(width - 2)
	m.table.SetHeight(tableHeight)
	return m
}

// SetFocused sets the focus state
func (m TableModel) SetFocused(focused bool) TableModel {
	m.focused = focused
	if focused {
		m.table.Focus()
	} else {
		m.table.Blur()
	}
	return m
}

// SetColumns sets the table columns
func (m TableModel) SetColumns(columns []table.Column) TableModel {
	m.columns = columns
	m.table.SetColumns(columns)
	return m
}

// SelectedRow returns the currently selected row index
func (m TableModel) SelectedRow() int {
	return m.table.Cursor()
}

// SelectedRowData returns the data for the selected row
func (m TableModel) SelectedRowData() interface{} {
	cursor := m.table.Cursor()
	if cursor >= 0 && cursor < len(m.rowData) {
		return m.rowData[cursor]
	}
	return nil
}

// RowCount returns the number of rows
func (m TableModel) RowCount() int {
	return len(m.rows)
}

// Cursor returns the current cursor position
func (m TableModel) Cursor() int {
	return m.table.Cursor()
}

// Init implements tea.Model
func (m TableModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m TableModel) Update(msg tea.Msg) (TableModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Enter):
			return m, func() tea.Msg {
				return TableSelectMsg{
					Index: m.table.Cursor(),
					Data:  m.SelectedRowData(),
				}
			}

		case key.Matches(msg, m.keys.Up):
			m.table.MoveUp(1)
			return m, nil

		case key.Matches(msg, m.keys.Down):
			m.table.MoveDown(1)
			return m, nil

		case key.Matches(msg, m.keys.PageUp):
			m.table.MoveUp(m.table.Height())
			return m, nil

		case key.Matches(msg, m.keys.PageDown):
			m.table.MoveDown(m.table.Height())
			return m, nil

		case key.Matches(msg, m.keys.Home):
			m.table.GotoTop()
			return m, nil

		case key.Matches(msg, m.keys.End):
			m.table.GotoBottom()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View implements tea.Model
func (m TableModel) View() string {
	var b strings.Builder

	// Title
	if m.title != "" {
		title := m.styles.Title.Render(m.title)
		b.WriteString(title)
		b.WriteString("\n")
	}

	// Table content
	tableView := m.table.View()

	// Add border
	bordered := m.styles.Border.
		Width(m.width - 2).
		Render(tableView)

	b.WriteString(bordered)

	// Search info
	if m.searchQuery != "" {
		searchInfo := fmt.Sprintf(" Search: %s (%d/%d) ", m.searchQuery, m.searchIndex+1, m.searchCount)
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Render(searchInfo))
	}

	return b.String()
}

// Search searches for a query in the table
func (m TableModel) Search(query string) TableModel {
	if query == "" {
		m.searchQuery = ""
		m.searchIndex = -1
		m.searchCount = 0
		m.matchRows = nil
		return m
	}

	m.searchQuery = query
	lowerQuery := strings.ToLower(query)

	// Find matching rows
	m.matchRows = nil
	for i, row := range m.rows {
		for _, cell := range row {
			if strings.Contains(strings.ToLower(cell), lowerQuery) {
				m.matchRows = append(m.matchRows, i)
				break
			}
		}
	}

	m.searchCount = len(m.matchRows)
	if m.searchCount > 0 {
		m.searchIndex = 0
		m.table.SetCursor(m.matchRows[0])
	} else {
		m.searchIndex = -1
	}

	return m
}

// NextSearchMatch moves to the next search match
func (m TableModel) NextSearchMatch() TableModel {
	if m.searchQuery == "" || m.searchCount == 0 {
		return m
	}

	m.searchIndex = (m.searchIndex + 1) % m.searchCount
	m.table.SetCursor(m.matchRows[m.searchIndex])

	return m
}

// PrevSearchMatch moves to the previous search match
func (m TableModel) PrevSearchMatch() TableModel {
	if m.searchQuery == "" || m.searchCount == 0 {
		return m
	}

	m.searchIndex = (m.searchIndex - 1 + m.searchCount) % m.searchCount
	m.table.SetCursor(m.matchRows[m.searchIndex])

	return m
}

// ClearSearch clears the search
func (m TableModel) ClearSearch() TableModel {
	m.searchQuery = ""
	m.searchIndex = -1
	m.searchCount = 0
	m.matchRows = nil
	return m
}

// GetSearchQuery returns the current search query
func (m TableModel) GetSearchQuery() string {
	return m.searchQuery
}

// PageUp moves up by a page
func (m TableModel) PageUp() TableModel {
	m.table.MoveUp(m.table.Height())
	return m
}

// PageDown moves down by a page
func (m TableModel) PageDown() TableModel {
	m.table.MoveDown(m.table.Height())
	return m
}

// TableSelectMsg is sent when a row is selected
type TableSelectMsg struct {
	Index int
	Data  interface{}
}

// Helper function to create columns from headers and widths
func CreateColumns(headers []string, widths []int) []table.Column {
	cols := make([]table.Column, len(headers))
	for i, h := range headers {
		width := 20 // default width
		if i < len(widths) && widths[i] > 0 {
			width = widths[i]
		}
		cols[i] = table.Column{
			Title: h,
			Width: width,
		}
	}
	return cols
}

// AutoColumnWidths calculates column widths automatically
func AutoColumnWidths(headers []string, rows []table.Row, maxWidth int) []int {
	widths := make([]int, len(headers))

	// Start with header widths
	for i, h := range headers {
		widths[i] = len(h)
	}

	// Check row contents
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Apply max width constraint and add padding
	totalWidth := 0
	for i := range widths {
		widths[i] += 2 // padding
		if widths[i] > 50 {
			widths[i] = 50
		}
		totalWidth += widths[i]
	}

	// Scale down if too wide
	if totalWidth > maxWidth && maxWidth > 0 {
		scale := float64(maxWidth) / float64(totalWidth)
		for i := range widths {
			widths[i] = int(float64(widths[i]) * scale)
			if widths[i] < 5 {
				widths[i] = 5
			}
		}
	}

	return widths
}

