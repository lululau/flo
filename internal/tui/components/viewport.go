package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ViewportModel wraps bubbles/viewport for log/content display
type ViewportModel struct {
	viewport    viewport.Model
	title       string
	content     string
	rawContent  string // Original content without search highlighting
	width       int
	height      int
	focused     bool
	searchQuery string
	searchIndex int
	searchCount int
	matchLines  []int // Line numbers that contain matches
	keys        ViewportKeyMap
	styles      ViewportStyles
}

// ViewportKeyMap defines key bindings for the viewport
type ViewportKeyMap struct {
	Up           key.Binding
	Down         key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	Home         key.Binding
	End          key.Binding
}

// DefaultViewportKeyMap returns default key bindings
func DefaultViewportKeyMap() ViewportKeyMap {
	return ViewportKeyMap{
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
			key.WithHelp("home/g", "top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "bottom"),
		),
	}
}

// ViewportStyles defines styles for the viewport
type ViewportStyles struct {
	Border      lipgloss.Style
	Title       lipgloss.Style
	Content     lipgloss.Style
	SearchMatch lipgloss.Style
	CurrentMatch lipgloss.Style
	JobHeader   lipgloss.Style
}

// DefaultViewportStyles returns default viewport styles
func DefaultViewportStyles() ViewportStyles {
	return ViewportStyles{
		Border: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")),
		Content: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")),
		SearchMatch: lipgloss.NewStyle().
			Background(lipgloss.Color("#854D0E")).
			Foreground(lipgloss.Color("#FFFFFF")),
		CurrentMatch: lipgloss.NewStyle().
			Background(lipgloss.Color("#CA8A04")).
			Foreground(lipgloss.Color("#000000")).
			Bold(true),
		JobHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true),
	}
}

// NewViewportModel creates a new viewport model
func NewViewportModel(title string) ViewportModel {
	vp := viewport.New(80, 20)

	return ViewportModel{
		viewport: vp,
		title:    title,
		keys:     DefaultViewportKeyMap(),
		styles:   DefaultViewportStyles(),
		focused:  true,
	}
}

// SetContent sets the viewport content
func (m ViewportModel) SetContent(content string) ViewportModel {
	m.rawContent = content
	m.content = content
	m.viewport.SetContent(content)
	return m
}

// AppendContent appends content to the viewport
func (m ViewportModel) AppendContent(content string) ViewportModel {
	m.rawContent += content
	m.content = m.rawContent
	m.viewport.SetContent(m.content)
	return m
}

// GetContent returns the raw content
func (m ViewportModel) GetContent() string {
	return m.rawContent
}

// SetTitle sets the viewport title
func (m ViewportModel) SetTitle(title string) ViewportModel {
	m.title = title
	return m
}

// SetSize sets the viewport size
func (m ViewportModel) SetSize(width, height int) ViewportModel {
	m.width = width
	m.height = height
	// Account for title and borders
	vpHeight := height - 4
	if vpHeight < 1 {
		vpHeight = 1
	}
	vpWidth := width - 4
	if vpWidth < 10 {
		vpWidth = 10
	}
	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight
	return m
}

// SetFocused sets the focus state
func (m ViewportModel) SetFocused(focused bool) ViewportModel {
	m.focused = focused
	return m
}

// ScrollToEnd scrolls to the end of the content
func (m ViewportModel) ScrollToEnd() ViewportModel {
	m.viewport.GotoBottom()
	return m
}

// ScrollToTop scrolls to the top of the content
func (m ViewportModel) ScrollToTop() ViewportModel {
	m.viewport.GotoTop()
	return m
}

// ScrollToLine scrolls to a specific line
func (m ViewportModel) ScrollToLine(line int) ViewportModel {
	m.viewport.SetYOffset(line)
	return m
}

// Init implements tea.Model
func (m ViewportModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m ViewportModel) Update(msg tea.Msg) (ViewportModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			m.viewport.LineUp(1)
			return m, nil

		case key.Matches(msg, m.keys.Down):
			m.viewport.LineDown(1)
			return m, nil

		case key.Matches(msg, m.keys.PageUp):
			m.viewport.ViewUp()
			return m, nil

		case key.Matches(msg, m.keys.PageDown):
			m.viewport.ViewDown()
			return m, nil

		case key.Matches(msg, m.keys.HalfPageUp):
			m.viewport.HalfViewUp()
			return m, nil

		case key.Matches(msg, m.keys.HalfPageDown):
			m.viewport.HalfViewDown()
			return m, nil

		case key.Matches(msg, m.keys.Home):
			m.viewport.GotoTop()
			return m, nil

		case key.Matches(msg, m.keys.End):
			m.viewport.GotoBottom()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View implements tea.Model
func (m ViewportModel) View() string {
	var b strings.Builder

	// Title
	if m.title != "" {
		title := m.styles.Title.Render(m.title)
		b.WriteString(title)
		b.WriteString("\n")
	}

	// Viewport content
	viewportContent := m.viewport.View()

	// Add border
	bordered := m.styles.Border.
		Width(m.width - 2).
		Render(viewportContent)

	b.WriteString(bordered)

	// Search info
	if m.searchQuery != "" {
		var searchInfo string
		if m.searchCount > 0 {
			searchInfo = fmt.Sprintf(" Search: '%s' (%d/%d) | n: next, N: prev ",
				m.searchQuery, m.searchIndex+1, m.searchCount)
		} else {
			searchInfo = fmt.Sprintf(" Search: '%s' (no matches) ", m.searchQuery)
		}
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Render(searchInfo))
	}

	return b.String()
}

// Search searches for a query in the content
func (m ViewportModel) Search(query string) ViewportModel {
	if query == "" {
		m.searchQuery = ""
		m.searchIndex = -1
		m.searchCount = 0
		m.matchLines = nil
		m.content = m.rawContent
		m.viewport.SetContent(m.content)
		return m
	}

	m.searchQuery = query
	lowerQuery := strings.ToLower(query)
	lowerContent := strings.ToLower(m.rawContent)

	// Count matches and find line numbers
	m.searchCount = strings.Count(lowerContent, lowerQuery)
	m.matchLines = nil

	if m.searchCount > 0 {
		// Find line numbers for each match
		lines := strings.Split(m.rawContent, "\n")
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), lowerQuery) {
				m.matchLines = append(m.matchLines, i)
			}
		}

		m.searchIndex = 0
		m.content = m.highlightSearchMatches()
		m.viewport.SetContent(m.content)
		m.scrollToMatch(0)
	} else {
		m.searchIndex = -1
	}

	return m
}

// highlightSearchMatches highlights search matches in the content
func (m ViewportModel) highlightSearchMatches() string {
	if m.searchQuery == "" {
		return m.rawContent
	}

	lowerContent := strings.ToLower(m.rawContent)
	lowerQuery := strings.ToLower(m.searchQuery)

	var result strings.Builder
	lastEnd := 0
	matchIdx := 0

	for {
		idx := strings.Index(lowerContent[lastEnd:], lowerQuery)
		if idx == -1 {
			break
		}

		actualIdx := lastEnd + idx

		// Add text before match
		result.WriteString(m.rawContent[lastEnd:actualIdx])

		// Get the match text (preserving case)
		match := m.rawContent[actualIdx : actualIdx+len(m.searchQuery)]

		// Apply highlighting based on whether this is the current match
		if matchIdx == m.searchIndex {
			result.WriteString(m.styles.CurrentMatch.Render(match))
		} else {
			result.WriteString(m.styles.SearchMatch.Render(match))
		}

		lastEnd = actualIdx + len(m.searchQuery)
		matchIdx++
	}

	// Add remaining text
	result.WriteString(m.rawContent[lastEnd:])

	return result.String()
}

// scrollToMatch scrolls to the nth match
func (m *ViewportModel) scrollToMatch(matchIndex int) {
	if matchIndex < 0 || len(m.matchLines) == 0 {
		return
	}

	// Find the line number for the match
	lineNum := 0
	if matchIndex < len(m.matchLines) {
		lineNum = m.matchLines[matchIndex]
	}

	m.viewport.SetYOffset(lineNum)
}

// NextSearchMatch moves to the next search match
func (m ViewportModel) NextSearchMatch() ViewportModel {
	if m.searchQuery == "" || m.searchCount == 0 {
		return m
	}

	m.searchIndex = (m.searchIndex + 1) % m.searchCount
	m.content = m.highlightSearchMatches()
	m.viewport.SetContent(m.content)
	m.scrollToMatch(m.searchIndex)

	return m
}

// PrevSearchMatch moves to the previous search match
func (m ViewportModel) PrevSearchMatch() ViewportModel {
	if m.searchQuery == "" || m.searchCount == 0 {
		return m
	}

	m.searchIndex = (m.searchIndex - 1 + m.searchCount) % m.searchCount
	m.content = m.highlightSearchMatches()
	m.viewport.SetContent(m.content)
	m.scrollToMatch(m.searchIndex)

	return m
}

// ClearSearch clears the search
func (m ViewportModel) ClearSearch() ViewportModel {
	m.searchQuery = ""
	m.searchIndex = -1
	m.searchCount = 0
	m.matchLines = nil
	m.content = m.rawContent
	m.viewport.SetContent(m.content)
	return m
}

// GetSearchQuery returns the current search query
func (m ViewportModel) GetSearchQuery() string {
	return m.searchQuery
}

// GetSearchInfo returns search information
func (m ViewportModel) GetSearchInfo() (query string, current, total int) {
	return m.searchQuery, m.searchIndex + 1, m.searchCount
}

// AtBottom returns whether the viewport is at the bottom
func (m ViewportModel) AtBottom() bool {
	return m.viewport.AtBottom()
}

// AtTop returns whether the viewport is at the top
func (m ViewportModel) AtTop() bool {
	return m.viewport.AtTop()
}

// ScrollPercent returns the scroll percentage
func (m ViewportModel) ScrollPercent() float64 {
	return m.viewport.ScrollPercent()
}

