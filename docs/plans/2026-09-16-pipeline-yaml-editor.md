# Pipeline Definition View & Edit Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** View and edit Yunxiao pipeline definitions (YAML) in an external editor. YAML-mode pipelines support write-back; classic-mode pipelines are view-only. Dual entry: TUI detail page + `flo pipeline view` / `flo pipeline edit`.

**Architecture:** New shared pure-logic package `internal/editutil` (editor args, YAML validation, diff stats, temp paths). `internal/api.Client` gains `GetPipelineDefinition` / `UpdatePipelineYAML` (token path only, replacing the `GetPipelineDetails` stub). `internal/tui/types` gains a generic exec-session mechanism (write file → `tea.ExecProcess` → read back → message) replacing `OpenInEditorCmd`/`OpenInPagerCmd`; the logs page migrates onto it with unchanged behavior. A new TUI page `pages/detail.go` renders the definition and runs the edit state machine (confirm → validate → optimistic check → PUT). CLI subcommands reuse the API and editutil with a sequential stdin/stdout flow. Design doc: `docs/plans/2026-09-16-pipeline-yaml-editor-design.md`.

**Tech Stack:** Go, bubbletea `tea.ExecProcess`, yaml.v3 (existing dep), Cobra, httptest

---

### Task 1: Create `internal/editutil` package

**Files:**
- Create: `internal/editutil/editutil.go`
- Create: `internal/editutil/editutil_test.go`

**Step 1: Write editutil.go**

```go
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
```

**Step 2: Write editutil_test.go**

```go
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
		name            string
		old, new        string
		added, removed  int
	}{
		{"identical", "a\nb\n", "a\nb\n", 0, 0},
		{"append", "a\n", "a\nc\n", 1, 0},
		{"remove", "a\nb\n", "a\n", 0, 1},
		{"replace", "a\nb\nc\n", "a\nx\nc\n", 1, 1},
		{"empty old", "", "a\n", 1, 0},
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
	if !strings.HasPrefix(p, filepath.Join(tempDirForTest(), "flo_pipeline_12345_")) {
		t.Errorf("unexpected path: %s", p)
	}
	if !strings.HasSuffix(p, ".yml") {
		t.Errorf("expected .yml suffix for filetype detection: %s", p)
	}
}

func tempDirForTest() string { return filepath.Join(string(filepath.Separator), "var", "folders", "placeholder") }
```

Note: replace `tempDirForTest` with `os.TempDir()` in the assertion directly — keep the test simple:

```go
func TestTempFilePath(t *testing.T) {
	p := TempFilePath("12345")
	if !strings.HasPrefix(filepath.Base(p), "flo_pipeline_12345_") {
		t.Errorf("unexpected file name: %s", p)
	}
	if !strings.HasSuffix(p, ".yml") {
		t.Errorf("expected .yml suffix for filetype detection: %s", p)
	}
}
```

**Step 3: Run tests**

Run: `cd /Users/liuxiang/cascode/github.com/flowt && go test ./internal/editutil/ -v`
Expected: all tests PASS

**Step 4: Commit**

```bash
git add internal/editutil
git commit -m "feat(editutil): add shared editor-arg, YAML validation, and diff-stats helpers"
```

---

### Task 2: API layer — `GetPipelineDefinition` / `UpdatePipelineYAML`

**Files:**
- Modify: `internal/api/client.go` (replace the `GetPipelineDetails` stub at ~line 818)
- Create: `internal/api/client_definition_test.go`

**Step 1: Verify the stub has no callers, then replace it**

Run: `cd /Users/liuxiang/cascode/github.com/flowt && grep -rn "GetPipelineDetails" --include="*.go" .`
Expected: only the definition in client.go. If any caller exists, keep the stub name as a wrapper instead of deleting.

Replace the stub with:

```go
// PipelineDefinition is the viewable/editable definition of a pipeline.
// FlowYAML is populated for both YAML-mode and classic-mode pipelines;
// IsYAMLMode gates whether write-back is allowed.
type PipelineDefinition struct {
	PipelineID string
	Name       string
	IsYAMLMode bool   // type == "PIPELINEASCODE"
	FlowYAML   string // pipelineConfig.flow
	UpdateTime int64  // millis, for optimistic concurrency check before write-back
}

// GetPipelineDefinition fetches a pipeline's definition via the personal
// access token REST API.
// https://help.aliyun.com/zh/yunxiao/developer-reference/getpipeline-get-pipeline-details
func (c *Client) GetPipelineDefinition(organizationId string, pipelineId string) (*PipelineDefinition, error) {
	if !c.useToken {
		return nil, fmt.Errorf("viewing/editing pipeline definitions requires a personal access token (configure token in ~/.flo/config.yml)")
	}

	path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/%s", organizationId, pipelineId)
	result, err := c.makeTokenRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline definition: %w", err)
	}

	def := &PipelineDefinition{PipelineID: pipelineId}
	if v, ok := result["type"].(string); ok && v == "PIPELINEASCODE" {
		def.IsYAMLMode = true
	}
	def.Name = getStringField(result, "name")
	if v, ok := result["updateTime"].(float64); ok {
		def.UpdateTime = int64(v)
	}
	if cfg, ok := result["pipelineConfig"].(map[string]interface{}); ok {
		def.FlowYAML = getStringField(cfg, "flow")
	}
	if def.FlowYAML == "" {
		return nil, fmt.Errorf("pipeline %s has no flow configuration in the API response", pipelineId)
	}
	return def, nil
}

// UpdatePipelineYAML writes a YAML-mode pipeline's definition back to the server.
// https://help.aliyun.com/zh/yunxiao/developer-reference/updatepipeline-update-pipeline
func (c *Client) UpdatePipelineYAML(organizationId string, pipelineId string, name string, content string) error {
	if !c.useToken {
		return fmt.Errorf("viewing/editing pipeline definitions requires a personal access token (configure token in ~/.flo/config.yml)")
	}

	path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/%s", organizationId, pipelineId)
	body := map[string]string{"content": content, "name": name}
	if _, err := c.makeTokenRequest("PUT", path, body); err != nil {
		return fmt.Errorf("failed to update pipeline: %w", err)
	}
	return nil
}
```

Note: `getStringField` is the existing helper used by the list parsing. The updateTime from this API family is millis (`time.Unix(int64(ct)/1000, 0)` in the list parser) — for the optimistic check we only compare equality, so raw millis comparison is fine.

**Step 2: Write client_definition_test.go**

The client needs a way to point at a test server. Check how `Client` is constructed: `NewClientWithToken(endpoint, token)` uses the endpoint directly. If `makeTokenRequest` hardcodes `https://` + endpoint, an httptest server URL (`http://127.0.0.1:port`) will not work as-is. Inspect `NewClientWithToken` and the `endpoint` handling first; if the scheme is hardcoded, add an unexported test hook in `client.go`:

```go
// httpClient baseURL override for tests (empty means https://<endpoint>)
func (c *Client) withBaseURL(u string) *Client {
	c2 := *c
	c2.baseURLOverride = u
	return c2
}
```

and use it in `makeTokenRequest` (`url := c.baseURL(c.endpoint) + path`). Keep the change minimal — an unexported field defaulting to `""`, and:

```go
func (c *Client) baseURL() string {
	if c.baseURLOverride != "" {
		return c.baseURLOverride
	}
	return "https://" + c.endpoint
}
```

Then the test:

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func definitionTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClientWithToken("devops.cn-hangzhou.aliyuncs.com", "test-token")
	if err != nil {
		t.Fatalf("NewClientWithToken: %v", err)
	}
	return c.withBaseURL(srv.URL)
}

func TestGetPipelineDefinition(t *testing.T) {
	c := definitionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/pipelines/123") {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-yunxiao-token") != "test-token" {
			t.Errorf("missing token header")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     123,
			"name":   "deploy-service",
			"type":   "PIPELINEASCODE",
			"updateTime": 1758000000000,
			"pipelineConfig": map[string]interface{}{
				"flow":     "stages:\n  build:\n",
				"settings": "{}",
			},
		})
	})

	def, err := c.GetPipelineDefinition("org1", "123")
	if err != nil {
		t.Fatalf("GetPipelineDefinition: %v", err)
	}
	if !def.IsYAMLMode {
		t.Error("expected IsYAMLMode=true for PIPELINEASCODE")
	}
	if def.Name != "deploy-service" || def.FlowYAML != "stages:\n  build:\n" {
		t.Errorf("unexpected definition: %+v", def)
	}
	if def.UpdateTime != 1758000000000 {
		t.Errorf("UpdateTime = %d", def.UpdateTime)
	}
}

func TestGetPipelineDefinitionClassicMode(t *testing.T) {
	c := definitionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "classic-pipe", "type": nil,
			"pipelineConfig": map[string]interface{}{"flow": "schema: tb\npipeline:\n"},
		})
	})

	def, err := c.GetPipelineDefinition("org1", "123")
	if err != nil {
		t.Fatalf("GetPipelineDefinition: %v", err)
	}
	if def.IsYAMLMode {
		t.Error("expected IsYAMLMode=false when type is null")
	}
}

func TestUpdatePipelineYAML(t *testing.T) {
	c := definitionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "deploy-service" || body["content"] != "stages: []\n" {
			t.Errorf("unexpected body: %+v", body)
		}
		json.NewEncoder(w).Encode(true)
	})

	if err := c.UpdatePipelineYAML("org1", "123", "deploy-service", "stages: []\n"); err != nil {
		t.Fatalf("UpdatePipelineYAML: %v", err)
	}
}
```

**Step 3: Build and test**

Run: `cd /Users/liuxiang/cascode/github.com/flowt && go build ./... && go test ./internal/api/ -run 'Definition|UpdatePipeline' -v`
Expected: no build errors, tests PASS

**Step 4: Commit**

```bash
git add internal/api
git commit -m "feat(api): add GetPipelineDefinition and UpdatePipelineYAML (token path)"
```

---

### Task 3: Generic exec session in `internal/tui/types` + logs page migration

**Files:**
- Create: `internal/tui/types/edit_session.go`
- Modify: `internal/tui/types/utils.go` (delete `OpenInEditorCmd` / `OpenInPagerCmd`)
- Modify: `internal/tui/types/types.go` (delete dead `EditorClosedMsg` / `PagerClosedMsg`)
- Modify: `internal/tui/pages/logs.go:410-422` (migrate `e`/`v` handlers)

**Step 1: Create edit_session.go**

```go
package types

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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

// execSessionCmd writes content to path, runs the program with stdio
// attached (suspending the TUI via tea.ExecProcess), then reads the file
// back. A non-zero exit is not fatal if the file is still readable —
// the user's edits must survive.
func execSessionCmd(program, path string, args []string) tea.Cmd {
	return tea.ExecProcess(exec.Command(args[0], args[1:]...), func(err error) tea.Msg {
		content, readErr := os.ReadFile(path)
		if readErr != nil && err == nil {
			err = fmt.Errorf("failed to read back %s: %w", path, readErr)
		}
		if readErr != nil {
			content = nil
		}
		return ExecSessionFinishedMsg{Program: program, Path: path, Content: string(content), Err: err}
	})
}

// OpenEditorSessionCmd opens content in the user's editor and emits
// ExecSessionFinishedMsg on exit. readOnly adds -R for vi-family editors.
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

// OpenPagerSessionCmd opens content in the user's pager and emits
// ExecSessionFinishedMsg on exit.
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
	parts := append(strings.Fields(pager), path)
	return execSessionCmd("pager", path, parts)
}

// LogsTempFilePath returns the temp path for logs viewer sessions
// (kept .txt: logs are not YAML).
func LogsTempFilePath() string {
	return fmt.Sprintf("%s/flo_logs_%d.txt", os.TempDir(), time.Now().Unix())
}
```

Add `"time"` to imports.

**Step 2: Delete the old functions and dead messages**

- Delete `OpenInEditorCmd` and `OpenInPagerCmd` from `utils.go`.
- Delete `EditorClosedMsg` / `PagerClosedMsg` (and the now-unused `OpenEditorMsg` / `OpenPagerMsg` if nothing else references them — grep first) from `types.go`.

**Step 3: Migrate logs.go e/v handlers**

Replace logs.go:410-422 with:

```go
		case key.Matches(msg, m.keys.OpenEditor):
			if m.viewport.GetContent() != "" {
				return m, types.OpenEditorSessionCmd(m.viewport.GetContent(), m.config.GetEditor(), types.LogsTempFilePath(), true)
			}
			return m, nil

		case key.Matches(msg, m.keys.OpenPager):
			if m.viewport.GetContent() != "" {
				return m, types.OpenPagerSessionCmd(m.viewport.GetContent(), m.config.GetPager(), types.LogsTempFilePath())
			}
			return m, nil
```

Note: logs `e` becomes read-only (`-R`) — a small deliberate improvement: the current "edit" discards changes anyway, so marking it read-only removes the trap of editing then losing work.

Add a handler in LogsModel.Update for cleanup:

```go
	case types.ExecSessionFinishedMsg:
		os.Remove(msg.Path)
		return m, nil
```

(add `"os"` import to logs.go).

**Step 4: Build and verify no stale references**

Run: `cd /Users/liuxiang/cascode/github.com/flowt && go build ./... && go test ./...`
Expected: no errors

**Step 5: Manual smoke check**

Run: `go run ./cmd/flo`, open a run's logs, press `e` then `v`.
Expected: nvim opens read-only (`-R`), pager opens as before, TUI restores cleanly.

**Step 6: Commit**

```bash
git add internal/tui
git commit -m "refactor(tui): replace OpenInEditorCmd/OpenInPagerCmd with exec-session mechanism"
```

---

### Task 4: TUI plumbing — page type, messages, commands, routing

**Files:**
- Modify: `internal/tui/types/types.go`
- Modify: `internal/tui/commands.go`
- Modify: `internal/tui/app.go`

**Step 1: types.go additions**

```go
// PageType constants: add after PageLogs
	PagePipelineDetail

// String(): add case
	case PagePipelineDetail:
		return "Pipeline Detail"

// --- Pipeline Detail Messages ---

// DetailPendingAction expresses the entry intent for the detail page.
type DetailPendingAction int

const (
	DetailActionView DetailPendingAction = iota
	DetailActionEdit
)

// PipelineDetailContext is the navigation data for the detail page.
type PipelineDetailContext struct {
	PipelineID     string
	PipelineName   string
	PendingAction  DetailPendingAction
}

// PipelineDefinitionLoadedMsg is sent when a pipeline definition is fetched.
type PipelineDefinitionLoadedMsg struct {
	Def *api.PipelineDefinition
	Err error
}

// PipelineSaveResultMsg is sent after a write-back attempt.
type PipelineSaveResultMsg struct {
	Err error
}
```

**Step 2: commands.go additions**

```go
// LoadPipelineDefinitionCmd fetches a pipeline definition asynchronously.
func LoadPipelineDefinitionCmd(client *api.Client, organizationID, pipelineID string) tea.Cmd {
	return func() tea.Msg {
		def, err := client.GetPipelineDefinition(organizationID, pipelineID)
		return types.PipelineDefinitionLoadedMsg{Def: def, Err: err}
	}
}

// SavePipelineDefinitionCmd writes a definition back asynchronously.
func SavePipelineDefinitionCmd(client *api.Client, organizationID, pipelineID, name, content string) tea.Cmd {
	return func() tea.Msg {
		err := client.UpdatePipelineYAML(organizationID, pipelineID, name, content)
		return types.PipelineSaveResultMsg{Err: err}
	}
}
```

**Step 3: app.go wiring**

1. Add `detailPage pages.DetailModel` to `Model`; initialize in `New()` with `pages.NewDetailModel(cfg)`.
2. `navigateTo`: add case:

```go
	case types.PagePipelineDetail:
		if ctx, ok := data.(types.PipelineDetailContext); ok {
			m.detailPage = m.detailPage.SetPipeline(ctx.PipelineID, ctx.PipelineName)
			m.detailPage = m.detailPage.SetLoading(true)
			cmd = LoadPipelineDefinitionCmd(m.client, m.organizationID, ctx.PipelineID)
			if ctx.PendingAction == types.DetailActionEdit {
				m.detailPage = m.detailPage.QueueEditOnLoad()
			}
		}
```

3. `Update`: add cases (before the per-page dispatch):

```go
	case types.PipelineDefinitionLoadedMsg:
		m.loading = false
		m.detailPage, cmd = m.detailPage.ApplyDefinition(msg.Def, msg.Err)
		cmds = append(cmds, cmd)

	case types.PipelineSaveResultMsg:
		m.detailPage, cmd = m.detailPage.ApplySaveResult(msg.Err)
		cmds = append(cmds, cmd)

	case pages.DetailReloadRequestMsg:
		m.detailPage = m.detailPage.SetLoading(true)
		cmds = append(cmds, LoadPipelineDefinitionCmd(m.client, m.organizationID, msg.PipelineID))

	case pages.DetailCheckRequestMsg:
		// Optimistic-concurrency check: re-fetch before writing.
		cmds = append(cmds, LoadPipelineDefinitionCmd(m.client, m.organizationID, msg.PipelineID))

	case pages.DetailSaveRequestMsg:
		cmds = append(cmds, SavePipelineDefinitionCmd(m.client, m.organizationID, msg.PipelineID, msg.Name, msg.Content))
```

Note: the check re-uses `PipelineDefinitionLoadedMsg` to return; the detail page distinguishes "load for display" from "check result" with an internal flag (`checkPending`), see Task 6.

4. Add `case types.PagePipelineDetail:` to the page-dispatch switch in `Update`, to `View`, and to `updatePageSizes`.
5. `getPageData`: return `types.PipelineDetailContext{PipelineID: m.detailPage.GetPipelineID(), PipelineName: m.detailPage.GetPipelineName()}` for the detail page (so navigating deeper and back restores context).

**Step 4: Build**

Run: `go build ./...` — will fail until Task 5 creates the page. Write a minimal `pages/detail.go` stub first (struct + the referenced methods returning `m`), or do Tasks 4+5 in one working session; either way, commit only when the build is green.

**Step 5: Commit**

```bash
git add internal/tui
git commit -m "feat(tui): add PagePipelineDetail routing and definition commands"
```

---

### Task 5: Detail page — model, rendering, view actions

**Files:**
- Create: `internal/tui/pages/detail.go`

**Step 1: Write the page model**

```go
package pages

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"flo/internal/api"
	"flo/internal/config"
	"flo/internal/editutil"
	"flo/internal/tui/components"
	"flo/internal/tui/types"
)

// DetailModel is the pipeline-definition detail page.
type DetailModel struct {
	pipelineID   string
	pipelineName string
	def          *api.PipelineDefinition
	loading      bool

	viewport components.ViewportModel
	spinner  components.SpinnerModel
	modal    components.ModalModel
	keys     DetailKeyMap

	width, height int
	config        *config.Config

	// Edit-flow state (Task 6)
	pendingAction  types.DetailPendingAction
	pendingConfirm string          // "" | "write-back" | "overwrite" | "save-as"
	tempPath       string          // current edit session temp file
	editedContent  string          // content read back from the editor
	baseUpdateTime int64           // server UpdateTime when the content was loaded
	checkPending   bool            // true while a re-GET is in flight as the optimistic check
	queuedSaveName string          // freshest known name for the PUT
}

type DetailKeyMap struct {
	Up, Down, PageUp, PageDown key.Binding
	Edit                       key.Binding // e
	ViewExternal               key.Binding // v
	SaveAs                     key.Binding // s
	Yank                       key.Binding // y
	Reload                     key.Binding // r
	Back, Quit                 key.Binding
}

func DefaultDetailKeyMap() DetailKeyMap {
	return DetailKeyMap{
		Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PageUp: key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "page down")),
		Edit:         key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		ViewExternal: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view ext")),
		SaveAs:       key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save as")),
		Yank:         key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy")),
		Reload:       key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "reload")),
		Back:         key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "back")),
		Quit:         key.NewBinding(key.WithKeys("Q"), key.WithHelp("Q", "quit")),
	}
}

func NewDetailModel(cfg *config.Config) DetailModel {
	return DetailModel{
		viewport: components.NewViewportModel("Pipeline"),
		spinner:  components.NewSpinnerModel(),
		modal:    components.NewModalModel(),
		keys:     DefaultDetailKeyMap(),
		config:   cfg,
	}
}

// DetailReloadRequestMsg asks the app to re-fetch the definition.
type DetailReloadRequestMsg struct{ PipelineID string }

// DetailCheckRequestMsg asks the app to re-fetch for the optimistic check.
type DetailCheckRequestMsg struct{ PipelineID string }

// DetailSaveRequestMsg asks the app to PUT the definition.
type DetailSaveRequestMsg struct {
	PipelineID string
	Name       string
	Content    string
}

// --- Setters used by app.go ---

func (m DetailModel) SetPipeline(id, name string) DetailModel {
	if m.pipelineID != id {
		m.def = nil
	}
	m.pipelineID = id
	m.pipelineName = name
	return m
}

func (m DetailModel) SetLoading(v bool) DetailModel   { m.loading = v; return m }
func (m DetailModel) QueueEditOnLoad() DetailModel    { m.pendingAction = types.DetailActionEdit; return m }
func (m DetailModel) GetPipelineID() string           { return m.pipelineID }
func (m DetailModel) GetPipelineName() string         { return m.pipelineName }

func (m DetailModel) SetSize(width, height int) DetailModel {
	m.width, m.height = width, height
	m.viewport = m.viewport.SetSize(width, height-2) // header + footer
	m.modal = m.modal.SetSize(width, height)
	return m
}

func (m DetailModel) Init() tea.Cmd { return m.spinner.Init() }
```

**Step 2: Rendering**

```go
func (m DetailModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	if m.modal.Visible {
		return m.modal.View()
	}
	if m.loading {
		return m.spinner.View() + " loading pipeline definition..."
	}
	if m.def == nil {
		return "No pipeline definition loaded"
	}

	header := m.renderHeader()
	body := m.viewport.View()
	footer := types.RenderHelpLine([]types.HelpItem{
		{Key: "e", Desc: "edit"}, {Key: "v", Desc: "view ext"},
		{Key: "s", Desc: "save as"}, {Key: "y", Desc: "copy"},
		{Key: "R", Desc: "reload"}, {Key: "q", Desc: "back"},
	})
	return header + "\n" + body + "\n" + footer
}

func (m DetailModel) renderHeader() string {
	badge := "YAML"
	badgeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	hint := ""
	if !m.def.IsYAMLMode {
		badge = "Classic"
		badgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
		hint = " · server-generated YAML, reference only"
	}
	name := m.def.Name
	if name == "" {
		name = m.pipelineName
	}
	headerStyle := lipgloss.NewStyle().Bold(true)
	return headerStyle.Render(name) + "  " + badgeStyle.Render("["+badge+"]") +
		fmt.Sprintf("  updated %s  ID %s%s", formatMillis(m.def.UpdateTime), m.def.PipelineID, hint)
}

func formatMillis(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return time.Unix(ms/1000, 0).Format("01-02 15:04")
}
```

Add `lipgloss` and `time` imports.

**Step 3: Update — definition loading + view actions**

```go
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	var cmds []tea.Cmd

	// Modal messages arrive after the modal has closed itself
	switch msg := msg.(type) {
	case components.ModalConfirmMsg:
		return m.handleModalConfirm(msg)

	case components.ModalCancelMsg, components.ModalDismissMsg:
		m.modal = m.modal.Hide()
		if m.pendingConfirm == "write-back" || m.pendingConfirm == "overwrite" {
			// Keep the temp file; show where the edits live
			m.modal = components.NewInfoModal("Write-back cancelled.\nYour edits are kept at:\n" + m.tempPath)
			m.modal = m.modal.SetSize(m.width, m.height)
		}
		m.pendingConfirm = ""
		return m, nil
	}

	if m.modal.Visible {
		var cmd tea.Cmd
		m.modal, cmd = m.modal.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case types.PipelineDefinitionLoadedMsg:
		return m.applyLoaded(msg.Def, msg.Err)

	case types.ExecSessionFinishedMsg:
		return m.handleSessionFinished(msg)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return types.GoBackMsg{} }
		case key.Matches(msg, m.keys.Reload):
			m.loading = true
			return m, DetailReloadRequestMsg{PipelineID: m.pipelineID}
		case key.Matches(msg, m.keys.Yank):
			if m.def != nil {
				return m, types.CopyToClipboardCmd(m.def.FlowYAML)
			}
		case key.Matches(msg, m.keys.SaveAs):
			return m.saveAs()
		case key.Matches(msg, m.keys.ViewExternal):
			return m.viewExternal()
		case key.Matches(msg, m.keys.Edit):
			return m.startEdit()
		case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down),
			key.Matches(msg, m.keys.PageUp), key.Matches(msg, m.keys.PageDown):
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	return m, tea.Batch(cmds...)
}

// applyLoaded applies a fetched definition (initial load, reload, or the
// optimistic-check re-GET — distinguished by checkPending).
func (m DetailModel) applyLoaded(def *api.PipelineDefinition, err error) (DetailModel, tea.Cmd) {
	m.loading = false

	if m.checkPending {
		m.checkPending = false
		return m.finishOptimisticCheck(def, err)
	}

	if err != nil {
		m.modal = components.NewErrorModal(err.Error()).SetSize(m.width, m.height)
		return m, nil
	}
	m.def = def
	m.baseUpdateTime = def.UpdateTime
	m.viewport = m.viewport.SetContent(def.FlowYAML)
	m.viewport = m.viewport.ScrollToTop()

	// Entry with pendingAction=edit: auto-start the edit session now
	if m.pendingAction == types.DetailActionEdit {
		m.pendingAction = types.DetailActionView
		return m.startEdit()
	}
	return m, nil
}

func (m DetailModel) viewExternal() (DetailModel, tea.Cmd) {
	if m.def == nil {
		return m, nil
	}
	path := editutil.TempFilePath(m.pipelineID)
	return m, types.OpenEditorSessionCmd(m.def.FlowYAML, m.config.GetEditor(), path, true)
}

func (m DetailModel) saveAs() (DetailModel, tea.Cmd) {
	if m.def == nil {
		return m, nil
	}
	path := saveAsPath(m.def.Name, m.pipelineID)
	if _, err := os.Stat(path); err == nil {
		m.pendingConfirm = "save-as"
		m.modal = components.NewConfirmModal("Overwrite?", path+" already exists. Overwrite?")
		m.modal = m.modal.SetSize(m.width, m.height)
		return m, nil
	}
	return m.writeSaveAs(path)
}

func (m DetailModel) writeSaveAs(path string) (DetailModel, tea.Cmd) {
	if err := os.WriteFile(path, []byte(m.def.FlowYAML), 0644); err != nil {
		m.modal = components.NewErrorModal(err.Error()).SetSize(m.width, m.height)
		return m, nil
	}
	m.modal = components.NewSuccessModal("Saved to " + path).SetSize(m.width, m.height)
	return m, nil
}

func saveAsPath(name, id string) string {
	safe := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			return '-'
		}
		return r
	}, name)
	if safe == "" {
		safe = "pipeline"
	}
	return fmt.Sprintf("%s-%s.yml", safe, id)
}
```

Note: `DetailReloadRequestMsg` / `DetailCheckRequestMsg` / `DetailSaveRequestMsg` are returned directly as `tea.Msg` values; app.go (Task 4) maps them to commands. Also note the design doc's `OnExit` callback is refined into the `ExecSessionFinishedMsg` message — more idiomatic in bubbletea; behavior is identical (caller owns file lifecycle).

**Step 4: Build**

Run: `go build ./...`
Expected: compiles (edit-flow methods from Task 6 can be added as `startEdit`/`handleSessionFinished`/`handleModalConfirm`/`finishOptimisticCheck`/`ApplyDefinition`/`ApplySaveResult` stubs that just return `m, nil` if Task 5 lands alone — but prefer implementing Tasks 5+6 together).

**Step 5: Commit**

```bash
git add internal/tui/pages/detail.go
git commit -m "feat(tui): add pipeline detail page with YAML viewport and view actions"
```

---

### Task 6: Detail page — edit state machine

**Files:**
- Modify: `internal/tui/pages/detail.go`

**Step 1: Implement the edit flow**

```go
// ApplyDefinition / ApplySaveResult are called by app.go when the shared
// messages arrive (they simply re-dispatch into the page Update path).
func (m DetailModel) ApplyDefinition(def *api.PipelineDefinition, err error) (DetailModel, tea.Cmd) {
	return m.applyLoaded(def, err)
}

func (m DetailModel) ApplySaveResult(err error) (DetailModel, tea.Cmd) {
	if err != nil {
		m.modal = components.NewErrorModal(err.Error() + "\nYour edits are kept at:\n" + m.tempPath).SetSize(m.width, m.height)
		return m, nil
	}
	os.Remove(m.tempPath)
	m.tempPath = ""
	m.modal = components.NewSuccessModal("Pipeline definition updated").SetSize(m.width, m.height)
	// Re-fetch: the server may normalize the YAML
	m.loading = true
	return m, DetailReloadRequestMsg{PipelineID: m.pipelineID}
}

func (m DetailModel) startEdit() (DetailModel, tea.Cmd) {
	if m.def == nil {
		return m, nil
	}
	if !m.def.IsYAMLMode {
		m.modal = components.NewInfoModal(
			"This is a classic (form-mode) pipeline.\n"+
				"The update API only supports YAML-mode pipelines.\n"+
				"Use v (external view), s (save as), or y (copy) instead.").SetSize(m.width, m.height)
		return m, nil
	}
	m.tempPath = editutil.TempFilePath(m.pipelineID)
	return m, types.OpenEditorSessionCmd(m.def.FlowYAML, m.config.GetEditor(), m.tempPath, false)
}

func (m DetailModel) handleSessionFinished(msg types.ExecSessionFinishedMsg) (DetailModel, tea.Cmd) {
	// Read-only viewer session: clean up and done.
	if msg.Program != "editor" || m.tempPath == "" || msg.Path != m.tempPath {
		if msg.Program == "pager" || m.pendingConfirm == "" {
			os.Remove(msg.Path)
		}
		return m, nil
	}

	if msg.Err != nil && msg.Content == "" {
		m.modal = components.NewErrorModal(msg.Err.Error()).SetSize(m.width, m.height)
		os.Remove(msg.Path)
		m.tempPath = ""
		return m, nil
	}

	if msg.Content == m.def.FlowYAML {
		os.Remove(msg.Path) // unchanged
		m.tempPath = ""
		return m, nil
	}

	m.editedContent = msg.Content
	body := "Content modified."
	if added, removed, ok := editutil.LineDiffStats(m.def.FlowYAML, msg.Content); ok {
		body = fmt.Sprintf("Content modified (+%d / -%d lines).", added, removed)
	}
	m.pendingConfirm = "write-back"
	m.modal = components.NewConfirmModal("Write back?", body+"\nWrite back to Yunxiao?")
	m.modal = m.modal.SetSize(m.width, m.height)
	return m, nil
}

func (m DetailModel) handleModalConfirm(msg components.ModalConfirmMsg) (DetailModel, tea.Cmd) {
	switch m.pendingConfirm {
	case "write-back":
		m.pendingConfirm = ""
		m.modal = m.modal.Hide()
		if err := editutil.ValidateYAML(m.editedContent); err != nil {
			m.modal = components.NewErrorModal("Invalid YAML:\n" + err.Error() +
				"\nYour edits are kept at:\n" + m.tempPath).SetSize(m.width, m.height)
			return m, nil
		}
		m.queuedSaveName = m.def.Name
		m.checkPending = true
		return m, DetailCheckRequestMsg{PipelineID: m.pipelineID}

	case "overwrite":
		// Optimistic check saw a server change; user confirmed anyway.
		m.pendingConfirm = ""
		m.modal = m.modal.Hide()
		return m, DetailSaveRequestMsg{
			PipelineID: m.pipelineID,
			Name:       m.queuedSaveName, // freshest name, from the re-GET
			Content:    m.editedContent,
		}

	case "save-as":
		m.pendingConfirm = ""
		m.modal = m.modal.Hide()
		return m.writeSaveAs(saveAsPath(m.def.Name, m.pipelineID))
	}
	m.modal = m.modal.Hide()
	return m, nil
}

// finishOptimisticCheck compares the server's current UpdateTime with the
// one the edited content was based on. Fails closed on fetch errors.
func (m DetailModel) finishOptimisticCheck(def *api.PipelineDefinition, err error) (DetailModel, tea.Cmd) {
	if err != nil {
		m.modal = components.NewErrorModal("Pre-write check failed (nothing was written):\n" + err.Error() +
			"\nYour edits are kept at:\n" + m.tempPath).SetSize(m.width, m.height)
		return m, nil
	}
	m.queuedSaveName = def.Name // never revert a concurrent rename

	if def.UpdateTime != m.baseUpdateTime {
		m.pendingConfirm = "overwrite"
		m.modal = components.NewConfirmModal("Server changed",
			"The pipeline was updated on the server after you started editing.\n"+
				"Overwrite the server version anyway?")
		m.modal = m.modal.SetSize(m.width, m.height)
		return m, nil
	}
	return m, DetailSaveRequestMsg{
		PipelineID: m.pipelineID,
		Name:       m.queuedSaveName,
		Content:    m.editedContent,
	}
}
```

Add `"os"` import. Also handle `types.CopiedMsg` minimally (optional toast) — or ignore; `CopyToClipboardCmd` already emits it and nothing breaks if unhandled.

**Step 2: Build**

Run: `go build ./... && go vet ./...`
Expected: clean

**Step 3: Commit**

```bash
git add internal/tui/pages/detail.go
git commit -m "feat(tui): pipeline detail page edit flow with confirm, validation, and optimistic check"
```

---

### Task 7: Pipelines list page — `v`/`e` entry keys

**Files:**
- Modify: `internal/tui/pages/pipelines.go` (keymap + handlers + help line)

**Step 1: Add key bindings**

Add to `PipelinesKeyMap` struct:

```go
	ViewDetail key.Binding
	EditDetail key.Binding
```

In `DefaultPipelinesKeyMap()`:

```go
		ViewDetail: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "detail"),
		),
		EditDetail: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
```

**Step 2: Add handlers in PipelinesModel.Update's tea.KeyMsg switch**

```go
		case key.Matches(msg, m.keys.ViewDetail):
			if pipeline := m.SelectedPipeline(); pipeline != nil {
				return m, func() tea.Msg {
					return types.NavigateMsg{
						Page: types.PagePipelineDetail,
						Data: types.PipelineDetailContext{
							PipelineID:    pipeline.PipelineID,
							PipelineName:  pipeline.Name,
							PendingAction: types.DetailActionView,
						},
					}
				}
			}

		case key.Matches(msg, m.keys.EditDetail):
			if pipeline := m.SelectedPipeline(); pipeline != nil {
				return m, func() tea.Msg {
					return types.NavigateMsg{
						Page: types.PagePipelineDetail,
						Data: types.PipelineDetailContext{
							PipelineID:    pipeline.PipelineID,
							PipelineName:  pipeline.Name,
							PendingAction: types.DetailActionEdit,
						},
					}
				}
			}
```

**Step 3: Update the footer help line** (the `updateModeline`/footer render that lists keys) to include `v:detail · e:edit`.

**Step 4: Build + manual smoke**

Run: `go build ./... && go run ./cmd/flo`
Expected: list page `v` opens detail (loading → YAML), `q` returns; `e` on a classic pipeline shows the read-only InfoModal after load.

**Step 5: Commit**

```bash
git add internal/tui/pages/pipelines.go
git commit -m "feat(tui): add v/e detail entry keys on the pipelines list page"
```

---

### Task 8: CLI — `flo pipeline view`

**Files:**
- Modify: `cmd/flo/cli/pipeline.go`

**Step 1: Add the subcommand**

```go
// =========================================================================
// flo pipeline view
// =========================================================================

var (
	viewPipeline   string
	viewInEditor   bool
)

var pipelineViewCmd = &cobra.Command{
	Use:   "view",
	Short: "View a pipeline's YAML definition",
	Long:  "Fetch and print a pipeline's YAML definition. Works for both YAML-mode and classic-mode pipelines (classic mode prints the server-generated representation). Meta info goes to stderr so stdout stays pipe-friendly.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		client, err := newClient(cfg)
		if err != nil {
			return err
		}
		org := getOrgID(cfg)

		pipelineID, err := resolvePipelineID(client, org, viewPipeline)
		if err != nil {
			return err
		}

		def, err := client.GetPipelineDefinition(org, pipelineID)
		if err != nil {
			return fmt.Errorf("failed to get pipeline definition: %w", err)
		}

		mode := "classic"
		if def.IsYAMLMode {
			mode = "yaml"
		}
		if outputFormat == "json" {
			return Output(map[string]interface{}{
				"pipelineId": def.PipelineID,
				"name":       def.Name,
				"mode":       mode,
				"updateTime": def.UpdateTime,
				"flow":       def.FlowYAML,
			}, nil, nil)
		}

		fmt.Fprintf(os.Stderr, "pipeline: %s (id %s, mode: %s, updated %s)\n",
			def.Name, def.PipelineID, mode, time.Unix(def.UpdateTime/1000, 0).Format("2006-01-02 15:04"))

		if viewInEditor {
			path := editutil.TempFilePath(def.PipelineID)
			if err := os.WriteFile(path, []byte(def.FlowYAML), 0644); err != nil {
				return fmt.Errorf("failed to write temporary file: %w", err)
			}
			args := editutil.BuildEditorArgs(cfg.GetEditor(), path, true)
			c := exec.Command(args[0], args[1:]...)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			runErr := c.Run()
			os.Remove(path)
			return runErr
		}

		fmt.Print(def.FlowYAML)
		return nil
	},
}

func init() {
	pipelineViewCmd.Flags().StringVar(&viewPipeline, "pipeline", "", "Pipeline name or ID (required)")
	pipelineViewCmd.MarkFlagRequired("pipeline")
	pipelineViewCmd.Flags().BoolVar(&viewInEditor, "editor", false, "Open in $EDITOR read-only instead of printing")
}
```

Register in `init()`: `pipelineCmd.AddCommand(pipelineViewCmd)`. Add imports `"os/exec"`, `"flo/internal/editutil"`.

**Step 2: Build + manual check**

Run: `go build ./... && ./flo-equivalent pipeline view --pipeline <name>`
Expected: YAML on stdout, meta on stderr; `--editor` opens nvim read-only; `-o json` prints the object.

**Step 3: Commit**

```bash
git add cmd/flo/cli/pipeline.go
git commit -m "feat(cli): add 'flo pipeline view' subcommand"
```

---

### Task 9: CLI — `flo pipeline edit`

**Files:**
- Modify: `cmd/flo/cli/pipeline.go`

**Step 1: Add the subcommand**

```go
// =========================================================================
// flo pipeline edit
// =========================================================================

var editPipeline string

var pipelineEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit a YAML-mode pipeline's definition in $EDITOR and write it back",
	Long:  "Fetch the definition, open it in $EDITOR, and on save confirm, validate, re-check the server version, and PUT it back. Classic (form-mode) pipelines are refused: the update API only supports YAML-mode pipelines.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		client, err := newClient(cfg)
		if err != nil {
			return err
		}
		org := getOrgID(cfg)

		pipelineID, err := resolvePipelineID(client, org, editPipeline)
		if err != nil {
			return err
		}

		def, err := client.GetPipelineDefinition(org, pipelineID)
		if err != nil {
			return fmt.Errorf("failed to get pipeline definition: %w", err)
		}
		if !def.IsYAMLMode {
			return fmt.Errorf("pipeline %q is a classic (form-mode) pipeline; the update API only supports YAML-mode pipelines (use 'flo pipeline view')", def.Name)
		}

		path := editutil.TempFilePath(pipelineID)
		if err := os.WriteFile(path, []byte(def.FlowYAML), 0644); err != nil {
			return fmt.Errorf("failed to write temporary file: %w", err)
		}

		editorArgs := editutil.BuildEditorArgs(cfg.GetEditor(), path, false)
		c := exec.Command(editorArgs[0], editorArgs[1:]...)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := c.Run(); err != nil {
			// The editor may still have written the file; read it back
			fmt.Fprintf(os.Stderr, "editor exited with error: %v\n", err)
		}

		edited, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read back edited file: %w", err)
		}
		if string(edited) == def.FlowYAML {
			os.Remove(path)
			fmt.Println("Not modified.")
			return nil
		}

		if added, removed, ok := editutil.LineDiffStats(def.FlowYAML, string(edited)); ok {
			fmt.Printf("Modified: +%d / -%d lines\n", added, removed)
		} else {
			fmt.Println("Modified.")
		}
		if !confirm("Write back to Yunxiao?") {
			fmt.Printf("Cancelled. Your edits are kept at:\n%s\n", path)
			return nil
		}

		if err := editutil.ValidateYAML(string(edited)); err != nil {
			return fmt.Errorf("invalid YAML (nothing was written; edits kept at %s):\n%w", path, err)
		}

		// Optimistic check: fail closed, use the freshest name
		fresh, err := client.GetPipelineDefinition(org, pipelineID)
		if err != nil {
			return fmt.Errorf("pre-write check failed (nothing was written; edits kept at %s): %w", path, err)
		}
		if fresh.UpdateTime != def.UpdateTime && !confirm("The pipeline changed on the server after you started editing. Overwrite anyway?") {
			fmt.Printf("Cancelled. Your edits are kept at:\n%s\n", path)
			return nil
		}

		if err := client.UpdatePipelineYAML(org, pipelineID, fresh.Name, string(edited)); err != nil {
			return fmt.Errorf("failed to update pipeline (edits kept at %s): %w", path, err)
		}
		os.Remove(path)
		fmt.Println("Pipeline definition updated.")
		return nil
	},
}

// confirm asks a yes/no question on stdin. Default is No.
func confirm(question string) bool {
	fmt.Printf("%s [y/N]: ", question)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func init() {
	pipelineEditCmd.Flags().StringVar(&editPipeline, "pipeline", "", "Pipeline name or ID (required)")
	pipelineEditCmd.MarkFlagRequired("pipeline")
}
```

Register in `init()`. Add `"bufio"` import.

**Step 2: Manual check with a throwaway pipeline**

Run against a test pipeline: `EDITOR=nvim` happy path (edit → y → updated), cancel path (file kept, path printed), syntax-error path (edits kept), classic-mode refusal.

**Step 3: Commit**

```bash
git add cmd/flo/cli/pipeline.go
git commit -m "feat(cli): add 'flo pipeline edit' subcommand with confirm-validate-check flow"
```

---

### Task 10: Detail page unit tests + final verification

**Files:**
- Create: `internal/tui/pages/detail_test.go`

**Step 1: Write state-machine tests**

Cover (model-level, no real suspend):

1. `applyLoaded` with error → ErrorModal visible, def stays nil
2. `applyLoaded` success → viewport content set, `baseUpdateTime` recorded; classic + `QueueEditOnLoad` → InfoModal, no session
3. YAML mode + `QueueEditOnLoad` → returns an `OpenEditorSession` command (assert via returned `tea.Cmd` being non-nil)
4. `handleSessionFinished` unchanged content → file removed (use a real temp file), no modal
5. `handleSessionFinished` changed content → `pendingConfirm == "write-back"`, ConfirmModal visible
6. `handleModalConfirm("write-back")` with invalid YAML → ErrorModal, `DetailCheckRequestMsg` NOT emitted
7. `handleModalConfirm("write-back")` with valid YAML → returns `DetailCheckRequestMsg`, `checkPending=true`
8. `finishOptimisticCheck` same UpdateTime → `DetailSaveRequestMsg` with original name; different UpdateTime → "overwrite" ConfirmModal; fetch error → ErrorModal (fail closed)
9. `finishOptimisticCheck` different UpdateTime + confirm → `DetailSaveRequestMsg` carries the re-GET's name (rename-safe)
10. `ApplySaveResult` error → temp file kept; success → temp file removed + `DetailReloadRequestMsg`

```go
package pages

import (
	"os"
	"path/filepath"
	"testing"

	"flo/internal/api"
	"flo/internal/config"
	"flo/internal/tui/components"
	"flo/internal/tui/types"
)

func newTestDetailModel() DetailModel {
	return NewDetailModel(config.Default()).
		SetPipeline("123", "deploy-service").
		SetSize(100, 30)
}

func yamlDef() *api.PipelineDefinition {
	return &api.PipelineDefinition{
		PipelineID: "123", Name: "deploy-service",
		IsYAMLMode: true, FlowYAML: "stages:\n  build:\n", UpdateTime: 1000,
	}
}

func TestApplyLoadedSetsViewportAndBaseTime(t *testing.T) { /* case 2 */ }
func TestClassicEditShowsInfoModal(t *testing.T) { /* case 2b: def.IsYAMLMode=false + startEdit */ }
func TestSessionUnchangedRemovesFile(t *testing.T) {
	m := newTestDetailModel()
	m, _ = m.applyLoaded(yamlDef(), nil)
	m.tempPath = filepath.Join(t.TempDir(), "e.yml")
	os.WriteFile(m.tempPath, []byte(m.def.FlowYAML), 0644)
	m, _ = m.handleSessionFinished(types.ExecSessionFinishedMsg{
		Program: "editor", Path: m.tempPath, Content: m.def.FlowYAML,
	})
	if _, err := os.Stat(m.tempPath); !os.Is(err, os.ErrNotExist) {
		t.Error("temp file should be removed when content unchanged")
	}
	if m.pendingConfirm != "" {
		t.Errorf("expected no confirm, got %q", m.pendingConfirm)
	}
}
func TestSessionChangedOpensWriteBackConfirm(t *testing.T) { /* case 5 */ }
func TestConfirmInvalidYAMLErrorsBeforeCheck(t *testing.T) { /* case 6: m.editedContent = "a: [\n" */ }
func TestConfirmValidEmitsCheckRequest(t *testing.T) { /* case 7: msg must be DetailCheckRequestMsg */ }
func TestOptimisticCheckUnchangedEmitsSave(t *testing.T) { /* case 8a */ }
func TestOptimisticCheckChangedOpensOverwriteConfirm(t *testing.T) { /* case 8b */ }
func TestOverwriteConfirmUsesFreshName(t *testing.T) {
	m := newTestDetailModel()
	m, _ = m.applyLoaded(yamlDef(), nil)
	m.editedContent = "stages: []\n"
	m.tempPath = "/tmp/keep"
	m.queuedSaveName = "renamed-by-someone-else" // from the re-GET
	m.pendingConfirm = "overwrite"
	m, cmd := m.handleModalConfirm(components.ModalConfirmMsg{})
	msg := cmd() // DetailSaveRequestMsg must carry the fresh name
	if save, ok := msg.(DetailSaveRequestMsg); !ok || save.Name != "renamed-by-someone-else" {
		t.Errorf("save request should use fresh name, got %+v", msg)
	}
}
func TestApplySaveResultFailureKeepsFile(t *testing.T) {
	m := newTestDetailModel()
	m.tempPath = filepath.Join(t.TempDir(), "keep.yml")
	os.WriteFile(m.tempPath, []byte("x"), 0644)
	m, _ = m.ApplySaveResult(assertErr("server rejected"))
	if _, err := os.Stat(m.tempPath); err != nil {
		t.Error("temp file must be kept on save failure")
	}
}
```

Fill in the remaining cases following the same pattern; add a tiny `assertErr` helper (`fmt.Errorf`). Check `config.Default()` exists (see `internal/config/config.go`; if the constructor is named differently, e.g. `config.New()`, use that).

**Step 2: Run all tests**

Run: `cd /Users/liuxiang/cascode/github.com/flowt && go test ./... -v`
Expected: all PASS

**Step 3: Full manual verification checklist**

1. TUI: list → `v` → detail renders YAML (YAML pipeline shows green `[YAML]` badge; classic shows gray `[Classic]` + hint)
2. TUI: detail → `v` → nvim opens read-only with filetype=yaml, `:q` returns to intact TUI
3. TUI: detail → `s` → file saved to cwd; overwrite confirm works
4. TUI: detail → `y` → clipboard has the YAML
5. TUI: list → `e` (YAML pipeline) → detail loads then nvim opens editable; quit without change → silent return
6. TUI: edit + change + quit → confirm modal shows +N/-M; cancel → info modal with kept file path
7. TUI: edit + invalid YAML + confirm → error modal with line numbers, file kept
8. TUI: edit + valid change + confirm → success modal, viewport refreshed with server-normalized YAML
9. TUI: classic pipeline `e` → read-only InfoModal
10. CLI: `flo pipeline view --pipeline X | head`, `--editor`, `-o json`
11. CLI: `flo pipeline edit --pipeline X` happy path / cancel / syntax error / classic refusal
12. Regression: logs page `e`/`v` still work (read-only editor + pager)

**Step 4: Commit**

```bash
git add internal/tui/pages/detail_test.go
git commit -m "test(tui): cover pipeline detail page edit state machine"
```

---

## Verification summary

- Every task ends with `go build ./...` green and its tests passing before commit
- `go test ./...` must pass at Tasks 3, 6, and 10
- Real-nvim suspend/resume verified manually at Tasks 3 and 10 (checklist above)
- Non-goals (design doc): settings/sources editing, viewport highlighting, AccessKey path, history-page entry, confirmation-skip flags
