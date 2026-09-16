package editutil

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildEditorArgs(t *testing.T) {
	tests := []struct {
		name     string
		editor   string
		file     string
		readOnly bool
		want     []string
	}{
		{"nvim readonly", "nvim", "/tmp/f.yml", true, []string{"nvim", "-R", "/tmp/f.yml"}},
		{"vim readonly", "vim", "/tmp/f.yml", true, []string{"vim", "-R", "/tmp/f.yml"}},
		{"nvim path readonly", "/usr/local/bin/nvim", "/tmp/f.yml", true, []string{"/usr/local/bin/nvim", "-R", "/tmp/f.yml"}},
		{"mvim readonly", "mvim", "/tmp/f.yml", true, []string{"mvim", "-R", "/tmp/f.yml"}},
		{"nvim editable", "nvim", "/tmp/f.yml", false, []string{"nvim", "/tmp/f.yml"}},
		{"code with args editable", "code --wait", "/tmp/f.yml", false, []string{"code", "--wait", "/tmp/f.yml"}},
		{"code with args readonly gets no flag", "code --wait", "/tmp/f.yml", true, []string{"code", "--wait", "/tmp/f.yml"}},
		{"less readonly no flag", "less", "/tmp/f.yml", true, []string{"less", "/tmp/f.yml"}},
		{"empty editor", "", "/tmp/f.yml", false, nil},
	}
	for _, tt := range tests {
		got := BuildEditorArgs(tt.editor, tt.file, tt.readOnly)
		if len(got) != len(tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
				break
			}
		}
	}
}

func TestValidateYAML(t *testing.T) {
	if err := ValidateYAML("stages:\n  build:\n    jobs: []\n"); err != nil {
		t.Errorf("valid yaml should pass: %v", err)
	}
	if err := ValidateYAML(""); err != nil {
		t.Errorf("empty content should pass: %v", err)
	}
	err := ValidateYAML("stages:\n  build:\n jobs: [\n")
	if err == nil {
		t.Fatal("invalid yaml should fail")
	}
	// yaml.TypeError carries line numbers for syntax errors
	if !strings.Contains(err.Error(), "line") {
		t.Logf("note: error lacks line info: %v", err)
	}
}

func TestLineDiffStats(t *testing.T) {
	tests := []struct {
		name           string
		old, new       string
		added, removed int
	}{
		{"identical", "a\nb\n", "a\nb\n", 0, 0},
		{"append", "a\n", "a\nc\n", 1, 0},
		{"remove", "a\nb\n", "a\n", 0, 1},
		{"replace", "a\nb\nc\n", "a\nx\nc\n", 1, 1},
		{"empty old", "", "a\n", 1, 0},
		{"reorder", "a\nb\n", "b\na\n", 1, 1},
	}
	for _, tt := range tests {
		added, removed, ok := LineDiffStats(tt.old, tt.new)
		if !ok || added != tt.added || removed != tt.removed {
			t.Errorf("%s: got +%d/-%d ok=%v, want +%d/-%d ok=true", tt.name, added, removed, ok, tt.added, tt.removed)
		}
	}
}

func TestLineDiffStatsLargeFileSkips(t *testing.T) {
	line := strings.Repeat("x\n", 5001)
	_, _, ok := LineDiffStats(line, line)
	if ok {
		t.Error("expected ok=false for >5000 lines")
	}
}

func TestTempFilePath(t *testing.T) {
	p := TempFilePath("12345")
	if !strings.HasPrefix(filepath.Base(p), "flo_pipeline_12345_") {
		t.Errorf("unexpected file name: %s", p)
	}
	if !strings.HasSuffix(p, ".yml") {
		t.Errorf("expected .yml suffix for filetype detection: %s", p)
	}
	if got := TempFilePath("a/b"); !strings.Contains(filepath.Base(got), "a_b") {
		t.Errorf("path separators should be sanitized: %s", got)
	}
}
