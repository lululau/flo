package components

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SearchModel represents a search input component
type SearchModel struct {
	input          textinput.Model
	active         bool
	committedQuery string // query to display when not actively editing
	width          int
	styles         SearchStyles
}

// SearchStyles defines styles for the search component
type SearchStyles struct {
	Container   lipgloss.Style
	Label       lipgloss.Style
	Input       lipgloss.Style
	Placeholder lipgloss.Style
}

// DefaultSearchStyles returns default search styles
func DefaultSearchStyles() SearchStyles {
	return SearchStyles{
		Container: lipgloss.NewStyle().
			Padding(0, 1),
		Label: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true),
		Input: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")),
		Placeholder: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")),
	}
}

// NewSearchModel creates a new search model
func NewSearchModel() SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Type to search..."
	ti.CharLimit = 100
	ti.Width = 30

	return SearchModel{
		input:  ti,
		styles: DefaultSearchStyles(),
	}
}

// SetWidth sets the search input width
func (m SearchModel) SetWidth(width int) SearchModel {
	m.width = width
	m.input.Width = width - 12 // Account for label and padding
	return m
}

// Activate activates the search input for editing.
// Returns the updated model and a Cmd for cursor blinking.
func (m SearchModel) Activate() (SearchModel, tea.Cmd) {
	m.active = true
	cmd := m.input.Focus()
	return m, cmd
}

// Deactivate deactivates the search input but preserves the committed query for display
func (m SearchModel) Deactivate() SearchModel {
	m.active = false
	m.committedQuery = m.input.Value()
	m.input.Blur()
	return m
}

// DeactivateAndClear deactivates the search input and clears the committed query
func (m SearchModel) DeactivateAndClear() SearchModel {
	m.active = false
	m.committedQuery = ""
	m.input.Blur()
	return m
}

// HasQuery returns true if there is a committed (non-empty) search query to display
func (m SearchModel) HasQuery() bool {
	return m.committedQuery != ""
}

// CommittedQuery returns the committed query for display
func (m SearchModel) CommittedQuery() string {
	return m.committedQuery
}

// ClearCommittedQuery clears the committed query
func (m SearchModel) ClearCommittedQuery() SearchModel {
	m.committedQuery = ""
	return m
}

// IsActive returns whether the search is active
func (m SearchModel) IsActive() bool {
	return m.active
}

// Query returns the current search query
func (m SearchModel) Query() string {
	return m.input.Value()
}

// SetQuery sets the search query
func (m SearchModel) SetQuery(query string) SearchModel {
	m.input.SetValue(query)
	return m
}

// Clear clears the search query and committed query
func (m SearchModel) Clear() SearchModel {
	m.input.SetValue("")
	m.committedQuery = ""
	return m
}

// Focus focuses the search input
func (m SearchModel) Focus() tea.Cmd {
	return m.input.Focus()
}

// Blur blurs the search input
func (m SearchModel) Blur() SearchModel {
	m.input.Blur()
	return m
}

// Init implements tea.Model
func (m SearchModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m SearchModel) Update(msg tea.Msg) (SearchModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			// Submit search
			query := m.input.Value()
			return m, func() tea.Msg {
				return SearchExecuteMsg{Query: query}
			}

		case tea.KeyEsc:
			// Cancel search
			m.active = false
			m.input.Blur()
			return m, func() tea.Msg {
				return SearchCancelMsg{}
			}
		}
	}

	// Get the previous value
	prevValue := m.input.Value()

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	// Check if the value changed - emit real-time search update
	newValue := m.input.Value()
	if newValue != prevValue {
		return m, tea.Batch(cmd, func() tea.Msg {
			return SearchQueryChangedMsg{Query: newValue}
		})
	}

	return m, cmd
}

// View implements tea.Model
func (m SearchModel) View() string {
	if m.active {
		label := m.styles.Label.Render("Search: ")
		input := m.input.View()
		return m.styles.Container.Width(m.width).Render(label + input)
	}

	if m.committedQuery != "" {
		label := m.styles.Label.Render("Search: ")
		queryText := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			Render(m.committedQuery)
		hint := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Render("  (/ to edit, Esc to clear)")
		return m.styles.Container.Width(m.width).Render(label + queryText + hint)
	}

	return ""
}

// SearchExecuteMsg is sent when the search is executed
type SearchExecuteMsg struct {
	Query string
}

// SearchCancelMsg is sent when the search is cancelled
type SearchCancelMsg struct{}

// SearchQueryChangedMsg is sent when the search query changes (for real-time filtering)
type SearchQueryChangedMsg struct {
	Query string
}

// SearchResultModel displays search results info
type SearchResultModel struct {
	query       string
	currentIdx  int
	totalCount  int
	hasResults  bool
	styles      SearchResultStyles
}

// SearchResultStyles defines styles for search results
type SearchResultStyles struct {
	Container lipgloss.Style
	Query     lipgloss.Style
	Count     lipgloss.Style
	NoResult  lipgloss.Style
}

// DefaultSearchResultStyles returns default search result styles
func DefaultSearchResultStyles() SearchResultStyles {
	return SearchResultStyles{
		Container: lipgloss.NewStyle().
			Padding(0, 1),
		Query: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true),
		Count: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#06B6D4")),
		NoResult: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Italic(true),
	}
}

// NewSearchResultModel creates a new search result model
func NewSearchResultModel() SearchResultModel {
	return SearchResultModel{
		styles: DefaultSearchResultStyles(),
	}
}

// SetResult sets the search result
func (m SearchResultModel) SetResult(query string, currentIdx, totalCount int) SearchResultModel {
	m.query = query
	m.currentIdx = currentIdx
	m.totalCount = totalCount
	m.hasResults = totalCount > 0
	return m
}

// Clear clears the search result
func (m SearchResultModel) Clear() SearchResultModel {
	m.query = ""
	m.currentIdx = 0
	m.totalCount = 0
	m.hasResults = false
	return m
}

// View implements the view for search results
func (m SearchResultModel) View() string {
	if m.query == "" {
		return ""
	}

	if !m.hasResults {
		return m.styles.NoResult.Render("No matches for '" + m.query + "'")
	}

	query := m.styles.Query.Render("'" + m.query + "'")
	count := m.styles.Count.Render(
		" (" + string(rune('0'+m.currentIdx+1)) + "/" + string(rune('0'+m.totalCount)) + ")",
	)

	return m.styles.Container.Render("Search: " + query + count + " | n: next, N: prev")
}

