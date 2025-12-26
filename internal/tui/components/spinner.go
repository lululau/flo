package components

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SpinnerModel wraps bubbles/spinner
type SpinnerModel struct {
	spinner spinner.Model
	active  bool
	message string
	styles  SpinnerStyles
}

// SpinnerStyles defines styles for the spinner
type SpinnerStyles struct {
	Spinner lipgloss.Style
	Message lipgloss.Style
}

// DefaultSpinnerStyles returns default spinner styles
func DefaultSpinnerStyles() SpinnerStyles {
	return SpinnerStyles{
		Spinner: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")),
		Message: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")),
	}
}

// NewSpinnerModel creates a new spinner model
func NewSpinnerModel() SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))

	return SpinnerModel{
		spinner: s,
		styles:  DefaultSpinnerStyles(),
	}
}

// SetActive sets the spinner active state
func (m SpinnerModel) SetActive(active bool) SpinnerModel {
	m.active = active
	return m
}

// SetMessage sets the spinner message
func (m SpinnerModel) SetMessage(message string) SpinnerModel {
	m.message = message
	return m
}

// IsActive returns whether the spinner is active
func (m SpinnerModel) IsActive() bool {
	return m.active
}

// Init implements tea.Model
func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update implements tea.Model
func (m SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// View implements tea.Model
func (m SpinnerModel) View() string {
	if !m.active {
		return ""
	}

	view := m.spinner.View()
	if m.message != "" {
		view += " " + m.styles.Message.Render(m.message)
	}

	return view
}

// Tick returns the spinner tick command
func (m SpinnerModel) Tick() tea.Cmd {
	return m.spinner.Tick
}

