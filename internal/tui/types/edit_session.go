package types

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"flo/internal/editutil"
)

// ExecSessionFinishedMsg is emitted after an external editor/pager exits and
// its file has been read back. The temp file is NOT removed — the consumer
// owns its lifecycle.
type ExecSessionFinishedMsg struct {
	Program string // "editor" or "pager"
	Path    string
	Content string // read back after exit (empty if unreadable)
	Err     error
}

// execSessionCmd runs the program with stdio attached (suspending the TUI via
// tea.ExecProcess), then reads the file back. A non-zero exit is not fatal if
// the file is still readable — the user's edits must survive.
func execSessionCmd(program, path string, args []string) tea.Cmd {
	c := exec.Command(args[0], args[1:]...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			if err == nil {
				err = fmt.Errorf("failed to read back %s: %w", path, readErr)
			}
			content = nil
		}
		return ExecSessionFinishedMsg{Program: program, Path: path, Content: string(content), Err: err}
	})
}

// OpenEditorSessionCmd writes content to path, opens it in the user's editor,
// and emits ExecSessionFinishedMsg on exit. readOnly adds -R for vi-family
// editors.
func OpenEditorSessionCmd(content, editor, path string, readOnly bool) tea.Cmd {
	if editor == "" {
		return func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("no editor configured")}
		}
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to write temporary file: %w", err)}
		}
	}
	args := editutil.BuildEditorArgs(editor, path, readOnly)
	if args == nil {
		return func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("invalid editor command")}
		}
	}
	return execSessionCmd("editor", path, args)
}

// OpenPagerSessionCmd writes content to path, opens it in the user's pager,
// and emits ExecSessionFinishedMsg on exit.
func OpenPagerSessionCmd(content, pager, path string) tea.Cmd {
	if pager == "" {
		return func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("no pager configured")}
		}
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to write temporary file: %w", err)}
		}
	}
	return execSessionCmd("pager", path, append(strings.Fields(pager), path))
}

// LogsTempFilePath returns the temp path for logs viewer sessions
// (.txt: logs are not YAML).
func LogsTempFilePath() string {
	return fmt.Sprintf("%s/flo_logs_%d.txt", os.TempDir(), time.Now().Unix())
}
