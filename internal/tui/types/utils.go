package types

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// FuzzyMatch performs fuzzy matching between query and text
// Returns true if all characters in query appear in text in order (case-insensitive)
func FuzzyMatch(query, text string) bool {
	if query == "" {
		return true
	}

	query = strings.ToLower(query)
	text = strings.ToLower(text)

	queryIndex := 0
	for _, char := range text {
		if queryIndex < len(query) && rune(query[queryIndex]) == char {
			queryIndex++
		}
	}

	return queryIndex == len(query)
}

// TruncateString truncates a string to fit within a specified width
func TruncateString(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}

// PadString pads a string to the specified width
func PadString(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// FormatDuration formats a duration for display
func FormatDuration(d time.Duration) string {
	if d > time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	} else if d > time.Minute {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}

// FormatTime formats a time for display
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

// FormatTimeWithSeconds formats a time with seconds for display
func FormatTimeWithSeconds(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}

// OpenInEditorCmd creates a command to open content in an external editor
func OpenInEditorCmd(content, editor string) tea.Cmd {
	return func() tea.Msg {
		if editor == "" {
			return ErrorMsg{Err: fmt.Errorf("no editor configured")}
		}

		// Create a temporary file
		tmpDir := os.TempDir()
		tmpFile := filepath.Join(tmpDir, fmt.Sprintf("flowt_logs_%d.txt", time.Now().Unix()))

		err := os.WriteFile(tmpFile, []byte(content), 0644)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to write temporary file: %w", err)}
		}

		// Parse editor command (might have arguments)
		cmdParts := strings.Fields(editor)
		if len(cmdParts) == 0 {
			return ErrorMsg{Err: fmt.Errorf("invalid editor command")}
		}

		// Add the temporary file as the last argument
		cmdParts = append(cmdParts, tmpFile)

		// Create the command
		c := exec.Command(cmdParts[0], cmdParts[1:]...)
		return tea.ExecProcess(c, func(err error) tea.Msg {
			// Clean up temp file after editor closes
			os.Remove(tmpFile)
			if err != nil {
				return ErrorMsg{Err: fmt.Errorf("editor command failed: %w", err)}
			}
			return EditorClosedMsg{}
		})
	}
}

// OpenInPagerCmd creates a command to open content in an external pager
func OpenInPagerCmd(content, pager string) tea.Cmd {
	return func() tea.Msg {
		if pager == "" {
			return ErrorMsg{Err: fmt.Errorf("no pager configured")}
		}

		// Create a temporary file
		tmpDir := os.TempDir()
		tmpFile := filepath.Join(tmpDir, fmt.Sprintf("flowt_logs_%d.txt", time.Now().Unix()))

		err := os.WriteFile(tmpFile, []byte(content), 0644)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to write temporary file: %w", err)}
		}

		// Parse pager command (might have arguments)
		cmdParts := strings.Fields(pager)
		if len(cmdParts) == 0 {
			return ErrorMsg{Err: fmt.Errorf("invalid pager command")}
		}

		// Add the temporary file as the last argument
		cmdParts = append(cmdParts, tmpFile)

		// Create the command
		c := exec.Command(cmdParts[0], cmdParts[1:]...)
		return tea.ExecProcess(c, func(err error) tea.Msg {
			// Clean up temp file after pager closes
			os.Remove(tmpFile)
			if err != nil {
				return ErrorMsg{Err: fmt.Errorf("pager command failed: %w", err)}
			}
			return PagerClosedMsg{}
		})
	}
}

// Clamp returns a value clamped between min and max
func Clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Max returns the maximum of two integers
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// HighlightMatches highlights search matches in a string
func HighlightMatches(text, query string, highlightStyle, normalStyle string) string {
	if query == "" {
		return text
	}

	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)

	var result strings.Builder
	lastEnd := 0

	for {
		idx := strings.Index(lowerText[lastEnd:], lowerQuery)
		if idx == -1 {
			break
		}

		actualIdx := lastEnd + idx

		// Add text before match
		result.WriteString(text[lastEnd:actualIdx])

		// Add highlighted match
		result.WriteString(highlightStyle)
		result.WriteString(text[actualIdx : actualIdx+len(query)])
		result.WriteString(normalStyle)

		lastEnd = actualIdx + len(query)
	}

	// Add remaining text
	result.WriteString(text[lastEnd:])

	return result.String()
}

// CountMatches counts the number of search matches in a string
func CountMatches(text, query string) int {
	if query == "" {
		return 0
	}
	return strings.Count(strings.ToLower(text), strings.ToLower(query))
}

// FindMatchPositions finds all positions of query matches in text
func FindMatchPositions(text, query string) []int {
	if query == "" {
		return nil
	}

	var positions []int
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)

	start := 0
	for {
		idx := strings.Index(lowerText[start:], lowerQuery)
		if idx == -1 {
			break
		}
		positions = append(positions, start+idx)
		start = start + idx + 1
	}

	return positions
}

// GetLineNumber returns the line number (0-based) for a position in text
func GetLineNumber(text string, position int) int {
	return strings.Count(text[:position], "\n")
}

