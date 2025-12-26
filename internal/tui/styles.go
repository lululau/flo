package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette
var (
	// Primary colors
	PrimaryColor   = lipgloss.Color("#7C3AED") // Purple
	SecondaryColor = lipgloss.Color("#06B6D4") // Cyan
	AccentColor    = lipgloss.Color("#F59E0B") // Amber

	// Status colors
	SuccessColor = lipgloss.Color("#10B981") // Green
	WarningColor = lipgloss.Color("#F59E0B") // Amber
	ErrorColor   = lipgloss.Color("#EF4444") // Red
	InfoColor    = lipgloss.Color("#3B82F6") // Blue
	RunningColor = lipgloss.Color("#22C55E") // Bright Green

	// Neutral colors
	TextColor       = lipgloss.Color("#E5E7EB") // Light gray
	SubtleTextColor = lipgloss.Color("#9CA3AF") // Gray
	MutedTextColor  = lipgloss.Color("#6B7280") // Dark gray
	BorderColor     = lipgloss.Color("#374151") // Dark border
	HighlightBg     = lipgloss.Color("#1F2937") // Highlight background

	// Special colors
	SelectedBg     = lipgloss.Color("#374151")
	BookmarkColor  = lipgloss.Color("#FBBF24") // Yellow for bookmarks
	SearchMatchBg  = lipgloss.Color("#854D0E") // Dark yellow
	CurrentMatchBg = lipgloss.Color("#CA8A04") // Bright yellow
)

// Styles contains all application styles
type Styles struct {
	// App container
	App lipgloss.Style

	// Header and title
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Description lipgloss.Style

	// Table styles
	TableHeader      lipgloss.Style
	TableCell        lipgloss.Style
	TableSelectedRow lipgloss.Style
	TableBorder      lipgloss.Style

	// List styles
	ListItem         lipgloss.Style
	ListItemSelected lipgloss.Style

	// Status styles
	StatusSuccess lipgloss.Style
	StatusWarning lipgloss.Style
	StatusError   lipgloss.Style
	StatusInfo    lipgloss.Style
	StatusRunning lipgloss.Style

	// Search styles
	SearchBar      lipgloss.Style
	SearchMatch    lipgloss.Style
	SearchCurrent  lipgloss.Style
	SearchLabel    lipgloss.Style
	SearchNoResult lipgloss.Style

	// Mode line styles
	ModeLine        lipgloss.Style
	ModeLineSection lipgloss.Style
	ModeLineHelp    lipgloss.Style
	ModeLineInfo    lipgloss.Style

	// Modal styles
	ModalOverlay lipgloss.Style
	ModalContent lipgloss.Style
	ModalTitle   lipgloss.Style
	ModalButton  lipgloss.Style

	// Bookmark styles
	Bookmark lipgloss.Style

	// Log view styles
	LogHeader  lipgloss.Style
	LogContent lipgloss.Style
	LogJob     lipgloss.Style

	// Help
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
	HelpSep  lipgloss.Style

	// Border
	Border        lipgloss.Style
	FocusedBorder lipgloss.Style
}

// DefaultStyles returns the default style set
func DefaultStyles() *Styles {
	s := &Styles{}

	// App container
	s.App = lipgloss.NewStyle()

	// Header and title
	s.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		MarginBottom(1)

	s.Subtitle = lipgloss.NewStyle().
		Foreground(SecondaryColor)

	s.Description = lipgloss.NewStyle().
		Foreground(SubtleTextColor)

	// Table styles
	s.TableHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(AccentColor).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(BorderColor)

	s.TableCell = lipgloss.NewStyle().
		Foreground(TextColor).
		Padding(0, 1)

	s.TableSelectedRow = lipgloss.NewStyle().
		Background(SelectedBg).
		Foreground(TextColor).
		Bold(true)

	s.TableBorder = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor)

	// List styles
	s.ListItem = lipgloss.NewStyle().
		Foreground(TextColor).
		PaddingLeft(2)

	s.ListItemSelected = lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true).
		PaddingLeft(2)

	// Status styles
	s.StatusSuccess = lipgloss.NewStyle().
		Foreground(SuccessColor)

	s.StatusWarning = lipgloss.NewStyle().
		Foreground(WarningColor)

	s.StatusError = lipgloss.NewStyle().
		Foreground(ErrorColor)

	s.StatusInfo = lipgloss.NewStyle().
		Foreground(InfoColor)

	s.StatusRunning = lipgloss.NewStyle().
		Foreground(RunningColor)

	// Search styles
	s.SearchBar = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(0, 1)

	s.SearchMatch = lipgloss.NewStyle().
		Background(SearchMatchBg).
		Foreground(TextColor)

	s.SearchCurrent = lipgloss.NewStyle().
		Background(CurrentMatchBg).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	s.SearchLabel = lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true)

	s.SearchNoResult = lipgloss.NewStyle().
		Foreground(ErrorColor).
		Italic(true)

	// Mode line styles
	s.ModeLine = lipgloss.NewStyle().
		Background(HighlightBg).
		Foreground(TextColor).
		Padding(0, 1)

	s.ModeLineSection = lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true)

	s.ModeLineHelp = lipgloss.NewStyle().
		Foreground(SubtleTextColor)

	s.ModeLineInfo = lipgloss.NewStyle().
		Foreground(SecondaryColor)

	// Modal styles
	s.ModalOverlay = lipgloss.NewStyle().
		Background(lipgloss.Color("#000000"))

	s.ModalContent = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(1, 2).
		Background(HighlightBg)

	s.ModalTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		MarginBottom(1)

	s.ModalButton = lipgloss.NewStyle().
		Foreground(TextColor).
		Background(BorderColor).
		Padding(0, 2).
		MarginRight(1)

	// Bookmark styles
	s.Bookmark = lipgloss.NewStyle().
		Foreground(BookmarkColor)

	// Log view styles
	s.LogHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor)

	s.LogContent = lipgloss.NewStyle().
		Foreground(TextColor)

	s.LogJob = lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true)

	// Help
	s.HelpKey = lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true)

	s.HelpDesc = lipgloss.NewStyle().
		Foreground(SubtleTextColor)

	s.HelpSep = lipgloss.NewStyle().
		Foreground(MutedTextColor)

	// Border
	s.Border = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor)

	s.FocusedBorder = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor)

	return s
}

// GlobalStyles is the default style instance
var GlobalStyles = DefaultStyles()

// Helper functions for common styling operations

// RenderTitle renders a title with the default style
func RenderTitle(title string) string {
	return GlobalStyles.Title.Render(title)
}

// RenderError renders an error message
func RenderError(msg string) string {
	return GlobalStyles.StatusError.Render("Error: " + msg)
}

// RenderSuccess renders a success message
func RenderSuccess(msg string) string {
	return GlobalStyles.StatusSuccess.Render(msg)
}

// RenderInfo renders an info message
func RenderInfo(msg string) string {
	return GlobalStyles.StatusInfo.Render(msg)
}

// GetStatusStyle returns the appropriate style for a status string
func GetStatusStyle(status string) lipgloss.Style {
	switch status {
	case "SUCCESS":
		return GlobalStyles.StatusSuccess
	case "RUNNING", "QUEUED", "INIT":
		return GlobalStyles.StatusRunning
	case "FAILED", "FAIL":
		return GlobalStyles.StatusError
	case "CANCELED":
		return GlobalStyles.StatusWarning
	default:
		return GlobalStyles.StatusInfo
	}
}

// RenderStatus renders a status with the appropriate color
func RenderStatus(status string) string {
	return GetStatusStyle(status).Render(status)
}

// RenderBookmark renders a bookmark indicator
func RenderBookmark(isBookmarked bool) string {
	if isBookmarked {
		return GlobalStyles.Bookmark.Render("★")
	}
	return " "
}

// CenterHorizontally centers content horizontally within the given width
func CenterHorizontally(content string, width int) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}

// CenterVertically centers content vertically within the given height
func CenterVertically(content string, height int) string {
	return lipgloss.PlaceVertical(height, lipgloss.Center, content)
}

// Center centers content both horizontally and vertically
func Center(content string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

