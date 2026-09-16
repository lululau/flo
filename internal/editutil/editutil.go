// Package editutil provides shared pure-logic helpers for viewing and
// editing pipeline definitions in an external editor.
package editutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// BuildEditorArgs splits an editor command (which may carry arguments,
// e.g. "code --wait") and appends the file path. For read-only sessions,
// vi-family editors get -R so content is highlighted but protected.
// Unknown editors get no extra flags.
func BuildEditorArgs(editor, filePath string, readOnly bool) []string {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return nil
	}
	if readOnly {
		// Covers vim, nvim, mvim, gvim, and path forms like /usr/local/bin/nvim
		base := strings.ToLower(filepath.Base(parts[0]))
		if base == "vim" || base == "nvim" || strings.HasSuffix(base, "vim") {
			rest := append([]string{"-R"}, parts[1:]...)
			parts = append(parts[:1:1], rest...)
		}
	}
	return append(parts, filePath)
}

// ValidateYAML checks that content parses as YAML.
// yaml.TypeError errors carry line numbers; callers surface them to the user.
func ValidateYAML(content string) error {
	var v interface{}
	if err := yaml.Unmarshal([]byte(content), &v); err != nil {
		return err
	}
	return nil
}

// maxDiffLines caps LineDiffStats input size; above it, callers skip stats.
const maxDiffLines = 5000

// LineDiffStats returns added/removed line counts between old and new content.
// ok is false when either side exceeds maxDiffLines (O(n*m) LCS would stall).
func LineDiffStats(oldContent, newContent string) (added, removed int, ok bool) {
	a := strings.Split(oldContent, "\n")
	b := strings.Split(newContent, "\n")
	if len(a) > maxDiffLines || len(b) > maxDiffLines {
		return 0, 0, false
	}

	// LCS via rolling table, computed bottom-up
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				curr[j] = prev[j+1] + 1
			} else if prev[j] >= curr[j+1] {
				curr[j] = prev[j]
			} else {
				curr[j] = curr[j+1]
			}
		}
		prev, curr = curr, prev
	}
	common := prev[0]

	return len(b) - common, len(a) - common, true
}

// TempFilePath returns a temp file path for editing a pipeline definition.
// The .yml suffix makes editors (e.g. nvim) set filetype=yaml automatically.
func TempFilePath(pipelineID string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_").Replace(pipelineID)
	return filepath.Join(os.TempDir(), fmt.Sprintf("flo_pipeline_%s_%d.yml", safe, time.Now().Unix()))
}
