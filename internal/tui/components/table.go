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
			Background(lipgloss.Color("#7C3AED")). // Purple background
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
	// Purple background with white text for selected row (entire row highlighted)
	s.Selected = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#7C3AED")).
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
	// Height here represents the total available lines for the table block (content area).
	// We render a header row ourselves, so dedicate one line for it and use the rest for data rows.
	headerLines := 1
	rowArea := height - headerLines
	if rowArea < 1 {
		rowArea = 1
	}

	m.table.SetWidth(width - 2) // allow for border padding
	m.table.SetHeight(rowArea)  // number of data rows to render
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

	// Custom table rendering to support full row highlighting
	tableContent := m.renderTable()

	// Add border
	bordered := m.styles.Border.
		Width(m.width - 2).
		Render(tableContent)

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

// renderTable renders the table with custom row highlighting
func (m TableModel) renderTable() string {
	var b strings.Builder

	tableWidth := m.width - 4 // Account for border padding
	if tableWidth < 10 {
		tableWidth = 10
	}

	// Calculate total columns width for proper cell sizing
	totalColWidth := 0
	for _, col := range m.columns {
		totalColWidth += col.Width
	}

	// Use the larger of totalColWidth or tableWidth
	rowWidth := tableWidth
	if totalColWidth > rowWidth {
		rowWidth = totalColWidth
	}

	// Render header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F59E0B")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		BorderBottom(true)

	var headerCells []string
	for _, col := range m.columns {
		cell := padRight(col.Title, col.Width)
		headerCells = append(headerCells, cell)
	}
	headerRow := strings.Join(headerCells, "")
	// Pad header to full row width
	headerRow = padRight(headerRow, rowWidth)
	b.WriteString(headerStyle.Render(headerRow))
	b.WriteString("\n")

	// Handle empty table
	if len(m.rows) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true)
		emptyMsg := padRight("  No data", rowWidth)
		b.WriteString(emptyStyle.Render(emptyMsg))
		return b.String()
	}

	// Render rows
	cursor := m.table.Cursor()
	visibleStart, visibleEnd := m.getVisibleRange()
	rowsCapacity := m.table.Height()

	cellStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#7C3AED")).
		Bold(true)

	renderedRows := 0
	for i := visibleStart; i < visibleEnd && i < len(m.rows); i++ {
		row := m.rows[i]
		var rowCells []string

		for j, col := range m.columns {
			cellContent := ""
			if j < len(row) {
				cellContent = row[j]
			}
			cell := padRight(cellContent, col.Width)
			rowCells = append(rowCells, cell)
		}

		rowContent := strings.Join(rowCells, "")
		// Pad row to full width for complete background highlighting
		rowContent = padRight(rowContent, rowWidth)

		if i == cursor {
			// Selected row - full row highlighted with purple background
			b.WriteString(selectedStyle.Width(rowWidth).Render(rowContent))
		} else {
			b.WriteString(cellStyle.Render(rowContent))
		}

		renderedRows++
		if i < visibleEnd-1 {
			b.WriteString("\n")
		}
	}

	// Pad remaining space with empty rows so the table block keeps a stable height
	for renderedRows < rowsCapacity {
		if renderedRows > 0 || visibleEnd > 0 {
			b.WriteString("\n")
		}
		emptyRow := padRight("", rowWidth)
		b.WriteString(cellStyle.Render(emptyRow))
		renderedRows++
	}

	return b.String()
}

// getVisibleRange returns the range of visible rows based on cursor position
func (m TableModel) getVisibleRange() (int, int) {
	cursor := m.table.Cursor()
	height := m.table.Height()
	totalRows := len(m.rows)

	if totalRows == 0 {
		return 0, 0
	}

	if height <= 0 {
		height = 10 // Default height
	}

	// Calculate visible start based on cursor position
	// The cursor should always be visible in the viewport
	start := 0

	// If we have more rows than can fit in the viewport
	if totalRows > height {
		// Position the viewport so cursor is visible
		// Try to keep cursor roughly centered when possible
		halfHeight := height / 2

		if cursor <= halfHeight {
			// Cursor near top - show from beginning
			start = 0
		} else if cursor >= totalRows-halfHeight {
			// Cursor near bottom - show the last 'height' rows
			start = totalRows - height
		} else {
			// Cursor in middle - center it
			start = cursor - halfHeight
		}

		// Ensure start is within bounds
		if start < 0 {
			start = 0
		}
		if start > totalRows-height {
			start = totalRows - height
		}
	}

	end := start + height
	if end > totalRows {
		end = totalRows
	}

	return start, end
}

// padRight pads a string to the specified width (handles unicode properly)
func padRight(s string, width int) string {
	// Use lipgloss width which handles unicode characters properly
	currentWidth := lipgloss.Width(s)
	if currentWidth >= width {
		// Truncate if too long
		return truncateToWidth(s, width)
	}
	return s + strings.Repeat(" ", width-currentWidth)
}

// truncateToWidth truncates a string to fit within the specified width
func truncateToWidth(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}

	// Truncate character by character
	result := ""
	for _, r := range s {
		newResult := result + string(r)
		if lipgloss.Width(newResult) > width {
			break
		}
		result = newResult
	}

	// If we have room, add ellipsis indicator
	if width > 3 && lipgloss.Width(result) > 0 {
		for lipgloss.Width(result+"…") > width && len(result) > 0 {
			result = result[:len(result)-1]
		}
		if len(result) > 0 {
			result += "…"
		}
	}

	return result
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
