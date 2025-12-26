package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModalType represents different types of modals
type ModalType int

const (
	ModalTypeInfo ModalType = iota
	ModalTypeError
	ModalTypeSuccess
	ModalTypeConfirm
	ModalTypeInput
)

// ModalModel represents a modal dialog
type ModalModel struct {
	Visible     bool
	modalType   ModalType
	title       string
	content     string
	buttons     []string
	selected    int
	input       textinput.Model
	width       int
	height      int
	styles      ModalStyles
	keys        ModalKeyMap
}

// ModalStyles defines styles for the modal
type ModalStyles struct {
	Overlay     lipgloss.Style
	Container   lipgloss.Style
	Title       lipgloss.Style
	Content     lipgloss.Style
	Button      lipgloss.Style
	ButtonFocus lipgloss.Style
	Input       lipgloss.Style
	Error       lipgloss.Style
	Success     lipgloss.Style
}

// DefaultModalStyles returns default modal styles
func DefaultModalStyles() ModalStyles {
	return ModalStyles{
		Overlay: lipgloss.NewStyle(),
		Container: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7C3AED")).
			Padding(1, 2).
			Background(lipgloss.Color("#1F2937")),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1),
		Content: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			MarginBottom(1),
		Button: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			Background(lipgloss.Color("#374151")).
			Padding(0, 2).
			MarginRight(1),
		ButtonFocus: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7C3AED")).
			Padding(0, 2).
			MarginRight(1).
			Bold(true),
		Input: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")).
			Padding(0, 1).
			MarginBottom(1),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")),
	}
}

// ModalKeyMap defines key bindings for the modal
type ModalKeyMap struct {
	Left   key.Binding
	Right  key.Binding
	Select key.Binding
	Cancel key.Binding
}

// DefaultModalKeyMap returns default modal key bindings
func DefaultModalKeyMap() ModalKeyMap {
	return ModalKeyMap{
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l", "tab"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc", "q"),
		),
	}
}

// NewModalModel creates a new modal model
func NewModalModel() ModalModel {
	ti := textinput.New()
	ti.Placeholder = "Enter value..."
	ti.CharLimit = 100
	ti.Width = 40

	return ModalModel{
		styles: DefaultModalStyles(),
		keys:   DefaultModalKeyMap(),
		input:  ti,
	}
}

// NewInfoModal creates an info modal
func NewInfoModal(content string) ModalModel {
	m := NewModalModel()
	m.Visible = true
	m.modalType = ModalTypeInfo
	m.title = "Info"
	m.content = content
	m.buttons = []string{"OK"}
	return m
}

// NewErrorModal creates an error modal
func NewErrorModal(content string) ModalModel {
	m := NewModalModel()
	m.Visible = true
	m.modalType = ModalTypeError
	m.title = "Error"
	m.content = content
	m.buttons = []string{"OK"}
	return m
}

// NewSuccessModal creates a success modal
func NewSuccessModal(content string) ModalModel {
	m := NewModalModel()
	m.Visible = true
	m.modalType = ModalTypeSuccess
	m.title = "Success"
	m.content = content
	m.buttons = []string{"OK"}
	return m
}

// NewConfirmModal creates a confirmation modal
func NewConfirmModal(title, content string) ModalModel {
	m := NewModalModel()
	m.Visible = true
	m.modalType = ModalTypeConfirm
	m.title = title
	m.content = content
	m.buttons = []string{"Yes", "No"}
	m.selected = 1 // Default to "No"
	return m
}

// NewInputModal creates an input modal
func NewInputModal(title, placeholder string, defaultValue string) ModalModel {
	m := NewModalModel()
	m.Visible = true
	m.modalType = ModalTypeInput
	m.title = title
	m.buttons = []string{"OK", "Cancel"}
	m.input.Placeholder = placeholder
	m.input.SetValue(defaultValue)
	m.input.Focus()
	return m
}

// Hide hides the modal
func (m ModalModel) Hide() ModalModel {
	m.Visible = false
	return m
}

// Show shows the modal
func (m ModalModel) Show() ModalModel {
	m.Visible = true
	return m
}

// SetSize sets the modal container size for centering
func (m ModalModel) SetSize(width, height int) ModalModel {
	m.width = width
	m.height = height
	return m
}

// GetInputValue returns the input value for input modals
func (m ModalModel) GetInputValue() string {
	return m.input.Value()
}

// Init implements tea.Model
func (m ModalModel) Init() tea.Cmd {
	if m.modalType == ModalTypeInput {
		return m.input.Focus()
	}
	return nil
}

// Update implements tea.Model
func (m ModalModel) Update(msg tea.Msg) (ModalModel, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle input modal specially
		if m.modalType == ModalTypeInput {
			switch {
			case key.Matches(msg, m.keys.Select):
				if m.selected == 0 { // OK
					value := m.input.Value()
					m.Visible = false
					return m, func() tea.Msg {
						return ModalConfirmMsg{Data: value}
					}
				} else { // Cancel
					m.Visible = false
					return m, func() tea.Msg {
						return ModalCancelMsg{}
					}
				}

			case key.Matches(msg, m.keys.Cancel):
				m.Visible = false
				return m, func() tea.Msg {
					return ModalCancelMsg{}
				}

			case msg.Type == tea.KeyTab:
				m.selected = (m.selected + 1) % len(m.buttons)
				return m, nil
			}

			// Update text input
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		// Handle other modal types
		switch {
		case key.Matches(msg, m.keys.Left):
			if m.selected > 0 {
				m.selected--
			}
			return m, nil

		case key.Matches(msg, m.keys.Right):
			if m.selected < len(m.buttons)-1 {
				m.selected++
			}
			return m, nil

		case key.Matches(msg, m.keys.Select):
			m.Visible = false
			if m.modalType == ModalTypeConfirm && m.selected == 0 {
				return m, func() tea.Msg {
					return ModalConfirmMsg{}
				}
			}
			return m, func() tea.Msg {
				return ModalDismissMsg{}
			}

		case key.Matches(msg, m.keys.Cancel):
			m.Visible = false
			return m, func() tea.Msg {
				return ModalCancelMsg{}
			}
		}
	}

	return m, nil
}

// View implements tea.Model
func (m ModalModel) View() string {
	if !m.Visible {
		return ""
	}

	var b strings.Builder

	// Title with appropriate styling
	titleStyle := m.styles.Title
	switch m.modalType {
	case ModalTypeError:
		titleStyle = titleStyle.Foreground(lipgloss.Color("#EF4444"))
	case ModalTypeSuccess:
		titleStyle = titleStyle.Foreground(lipgloss.Color("#10B981"))
	}
	b.WriteString(titleStyle.Render(m.title))
	b.WriteString("\n")

	// Content
	if m.content != "" {
		contentStyle := m.styles.Content
		switch m.modalType {
		case ModalTypeError:
			contentStyle = m.styles.Error
		case ModalTypeSuccess:
			contentStyle = m.styles.Success
		}
		b.WriteString(contentStyle.Render(m.content))
		b.WriteString("\n")
	}

	// Input field for input modals
	if m.modalType == ModalTypeInput {
		b.WriteString(m.styles.Input.Render(m.input.View()))
		b.WriteString("\n")
	}

	// Buttons
	var buttons []string
	for i, btn := range m.buttons {
		if i == m.selected {
			buttons = append(buttons, m.styles.ButtonFocus.Render(btn))
		} else {
			buttons = append(buttons, m.styles.Button.Render(btn))
		}
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, buttons...))

	// Apply container style
	modal := m.styles.Container.Render(b.String())

	// Center the modal if we have dimensions
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
	}

	return modal
}

// ModalConfirmMsg is sent when the user confirms
type ModalConfirmMsg struct {
	Data interface{}
}

// ModalCancelMsg is sent when the user cancels
type ModalCancelMsg struct{}

// ModalDismissMsg is sent when the modal is dismissed
type ModalDismissMsg struct{}

