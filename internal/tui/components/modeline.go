package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModeLineModel represents the bottom status bar
type ModeLineModel struct {
	width   int
	page    string
	status  string
	info    string
	help    string
	loading bool
	styles  ModeLineStyles
}

// ModeLineStyles defines styles for the mode line
type ModeLineStyles struct {
	Container lipgloss.Style
	Page      lipgloss.Style
	Status    lipgloss.Style
	Info      lipgloss.Style
	Help      lipgloss.Style
	Loading   lipgloss.Style
	Separator lipgloss.Style
}

// DefaultModeLineStyles returns default mode line styles
func DefaultModeLineStyles() ModeLineStyles {
	return ModeLineStyles{
		Container: lipgloss.NewStyle().
			Background(lipgloss.Color("#1F2937")).
			Foreground(lipgloss.Color("#E5E7EB")),
		Page: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true).
			Padding(0, 1),
		Status: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22C55E")).
			Padding(0, 1),
		Info: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#06B6D4")).
			Padding(0, 1),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Padding(0, 1),
		Loading: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Padding(0, 1),
		Separator: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			SetString(" | "),
	}
}

// NewModeLineModel creates a new mode line model
func NewModeLineModel(page string) ModeLineModel {
	return ModeLineModel{
		page:   page,
		styles: DefaultModeLineStyles(),
	}
}

// SetWidth sets the mode line width
func (m ModeLineModel) SetWidth(width int) ModeLineModel {
	m.width = width
	return m
}

// SetPage sets the current page name
func (m ModeLineModel) SetPage(page string) ModeLineModel {
	m.page = page
	return m
}

// SetStatus sets the status text
func (m ModeLineModel) SetStatus(status string) ModeLineModel {
	m.status = status
	return m
}

// SetInfo sets the info text
func (m ModeLineModel) SetInfo(info string) ModeLineModel {
	m.info = info
	return m
}

// SetHelp sets the help text
func (m ModeLineModel) SetHelp(help string) ModeLineModel {
	m.help = help
	return m
}

// SetLoading sets the loading state
func (m ModeLineModel) SetLoading(loading bool) ModeLineModel {
	m.loading = loading
	return m
}

// Init implements tea.Model
func (m ModeLineModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m ModeLineModel) Update(msg tea.Msg) (ModeLineModel, tea.Cmd) {
	return m, nil
}

// View implements tea.Model
func (m ModeLineModel) View() string {
	var sections []string

	// Page name (left)
	if m.page != "" {
		sections = append(sections, m.styles.Page.Render(m.page))
	}

	// Loading indicator
	if m.loading {
		sections = append(sections, m.styles.Loading.Render("Loading..."))
	}

	// Status
	if m.status != "" {
		sections = append(sections, m.styles.Status.Render(m.status))
	}

	// Info
	if m.info != "" {
		sections = append(sections, m.styles.Info.Render(m.info))
	}

	// Join left sections
	left := strings.Join(sections, m.styles.Separator.String())

	// Help text (right)
	right := ""
	if m.help != "" {
		right = m.styles.Help.Render(m.help)
	}

	// Calculate spacing
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	spacing := m.width - leftWidth - rightWidth
	if spacing < 0 {
		spacing = 0
	}

	// Build the final line
	content := left + strings.Repeat(" ", spacing) + right

	// Apply container style
	return m.styles.Container.Width(m.width).Render(content)
}

// StatusModeLineModel is a specialized mode line for status display (logs page)
type StatusModeLineModel struct {
	ModeLineModel
	runStatus     string
	autoRefresh   string
	searchInfo    string
}

// NewStatusModeLineModel creates a new status mode line model
func NewStatusModeLineModel() StatusModeLineModel {
	return StatusModeLineModel{
		ModeLineModel: NewModeLineModel("Logs"),
	}
}

// SetWidth sets the mode line width
func (m StatusModeLineModel) SetWidth(width int) StatusModeLineModel {
	m.ModeLineModel = m.ModeLineModel.SetWidth(width)
	return m
}

// SetRunStatus sets the pipeline run status
func (m StatusModeLineModel) SetRunStatus(status string) StatusModeLineModel {
	m.runStatus = status
	return m
}

// SetAutoRefresh sets the auto-refresh status
func (m StatusModeLineModel) SetAutoRefresh(status string) StatusModeLineModel {
	m.autoRefresh = status
	return m
}

// SetSearchInfo sets the search information
func (m StatusModeLineModel) SetSearchInfo(info string) StatusModeLineModel {
	m.searchInfo = info
	return m
}

// View implements the specialized view for status mode line
func (m StatusModeLineModel) View() string {
	var parts []string

	// Status part with color
	if m.runStatus != "" {
		statusStyle := m.styles.Status
		switch strings.ToUpper(m.runStatus) {
		case "RUNNING", "QUEUED", "INIT":
			statusStyle = statusStyle.Foreground(lipgloss.Color("#22C55E")) // Green
		case "SUCCESS":
			statusStyle = statusStyle.Foreground(lipgloss.Color("#FFFFFF")) // White
		case "FAILED", "FAIL":
			statusStyle = statusStyle.Foreground(lipgloss.Color("#EF4444")) // Red
		case "CANCELED":
			statusStyle = statusStyle.Foreground(lipgloss.Color("#9CA3AF")) // Gray
		}
		parts = append(parts, "Status: "+statusStyle.Render(m.runStatus))
	}

	// Auto-refresh
	if m.autoRefresh != "" {
		parts = append(parts, "Auto-refresh: "+m.autoRefresh)
	}

	// Search info
	if m.searchInfo != "" {
		parts = append(parts, m.searchInfo)
	}

	// Help
	help := "Press '/' search, 'r' refresh, 'X' stop, 'q' return, 'e' edit, 'v' pager"

	left := strings.Join(parts, " | ")
	right := m.styles.Help.Render(help)

	// Calculate spacing
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	spacing := m.width - leftWidth - rightWidth
	if spacing < 0 {
		spacing = 0
	}

	content := left + strings.Repeat(" ", spacing) + right

	return m.styles.Container.Width(m.width).Render(content)
}

