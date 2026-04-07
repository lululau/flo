# Pipeline Notification Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a configurable notification command that fires when a pipeline run (started from the current process) reaches a terminal state.

**Architecture:** A `Notifier` struct in `internal/notify/notifier.go` runs a single background goroutine that polls tracked runs every 5 seconds via the API client. When a run reaches a terminal state, it renders a Go template and executes the configured shell command. The `Config` struct gains a `NotifyCommand` field. The TUI `Model` holds the `Notifier` and calls `Track()` after `RunPipeline` succeeds.

**Tech Stack:** Go `text/template`, `os/exec`, `sync.Mutex`, `time.Ticker`, existing `api.Client`

---

### Task 1: Add `NotifyCommand` to Config

**Files:**
- Modify: `internal/config/config.go:12-32`

**Step 1: Add field to Config struct**

In `internal/config/config.go`, add a new field to the `Config` struct after `DefaultSort` (line 31):

```go
	// 通知命令 - 可选，流水线结束时执行
	// 支持 text/template 语法，可用占位符: .PipelineName, .Result, .Duration, .Branch
	NotifyCommand string `yaml:"notify_command,omitempty"`
```

**Step 2: Update config.yml.example**

In `config.yml.example`, add before the final notes section (before line 61):

```yaml

# ===== 通知配置 =====
# 流水线结束时执行的通知命令 - 可选
# 支持 text/template 语法，可用占位符:
#   {{.PipelineName}} - 流水线名称
#   {{.Result}}       - 结果 (success ✓ / failed ✗ / canceled ○)
#   {{.Duration}}     - 耗时 (如 2m 35s)
#   {{.Branch}}       - 触发分支
# 示例 (macOS): notify_command: "terminal-notifier -title 'flo' -message '{{.PipelineName}} {{.Result}}'"
# 示例 (Linux): notify_command: "notify-send 'flo' '{{.PipelineName}} {{.Result}}'"
# notify_command: "terminal-notifier -title 'flo' -message '{{.PipelineName}} {{.Result}}'"
```

**Step 3: Build to verify**

Run: `cd /Users/liuxiang/cascode/github.com/flowt && go build ./...`
Expected: no errors

**Step 4: Commit**

```bash
git add internal/config/config.go config.yml.example
git commit -m "feat(config): add notify_command field for pipeline completion notifications"
```

---

### Task 2: Create `internal/notify/notifier.go` — types and Notifier struct

**Files:**
- Create: `internal/notify/notifier.go`

**Step 1: Create the file with TrackedRun, NotifyData, and Notifier struct**

```go
package notify

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"text/template"
	"time"

	"flo/internal/api"
)

// TrackedRun holds the information needed to monitor a pipeline run.
type TrackedRun struct {
	OrganizationID string
	PipelineID     string
	PipelineName   string
	RunID          string
	Branch         string
	StartTime      time.Time
}

// NotifyData holds the template data for rendering the notification command.
type NotifyData struct {
	PipelineName string
	Result       string
	Duration     string
	Branch       string
}

// Notifier monitors pipeline runs and executes a notification command on terminal states.
type Notifier struct {
	client    *api.Client
	tmpl      *template.Template
	mu        sync.Mutex
	runs      []TrackedRun
	stopCh    chan struct{}
	stopped   chan struct{}
}

// New creates a Notifier. Returns nil if notifyCommand is empty (notifications disabled).
func New(client *api.Client, notifyCommand string) (*Notifier, error) {
	if strings.TrimSpace(notifyCommand) == "" {
		return nil, nil
	}

	tmpl, err := template.New("notify").Parse(notifyCommand)
	if err != nil {
		return nil, fmt.Errorf("failed to parse notify_command template: %w", err)
	}

	return &Notifier{
		client:  client,
		tmpl:    tmpl,
		stopCh:  make(chan struct{}),
		stopped: make(chan struct{}),
	}, nil
}

// Track adds a pipeline run to the monitor list and starts the background goroutine if needed.
func (n *Notifier) Track(run TrackedRun) {
	n.mu.Lock()
	n.runs = append(n.runs, run)
	needsStart := len(n.runs) == 1
	n.mu.Unlock()

	if needsStart {
		n.startLoop()
	}
}

// Stop gracefully shuts down the background goroutine.
func (n *Notifier) Stop() {
	select {
	case n.stopCh <- struct{}{}:
		<-n.stopped
	default:
		// already stopped or never started
	}
}

func (n *Notifier) startLoop() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				n.poll()
			case <-n.stopCh:
				n.stopped <- struct{}{}
				return
			}
		}
	}()
}

func (n *Notifier) poll() {
	n.mu.Lock()
	runs := make([]TrackedRun, len(n.runs))
	copy(runs, n.runs)
	n.mu.Unlock()

	var remaining []TrackedRun
	for _, run := range runs {
		terminal := n.checkAndNotify(run)
		if !terminal {
			remaining = append(remaining, run)
		}
	}

	n.mu.Lock()
	n.runs = remaining
	isEmpty := len(n.runs) == 0
	n.mu.Unlock()

	if isEmpty {
		// Drain stop channel to reset for future Track calls
		select {
		case <-n.stopCh:
		default:
		}
		n.stopCh = make(chan struct{})
		n.stopped = make(chan struct{})
	}
}

func (n *Notifier) checkAndNotify(run TrackedRun) bool {
	details, err := n.client.GetPipelineRunDetails(
		run.OrganizationID, run.PipelineID, run.RunID,
	)
	if err != nil {
		log.Printf("[notify] failed to check run %s: %v", run.RunID, err)
		return false // keep tracking on error
	}

	if !isTerminalStatus(details.Status) {
		return false
	}

	// Calculate duration
	var duration time.Duration
	if !run.StartTime.IsZero() {
		duration = time.Since(run.StartTime)
	}

	data := NotifyData{
		PipelineName: run.PipelineName,
		Result:       formatResult(details.Status),
		Duration:     formatDuration(duration),
		Branch:       run.Branch,
	}

	n.executeNotify(data)
	return true
}

func (n *Notifier) executeNotify(data NotifyData) {
	var buf strings.Builder
	if err := n.tmpl.Execute(&buf, data); err != nil {
		log.Printf("[notify] template execution failed: %v", err)
		return
	}

	cmd := buf.String()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	if output, err := c.CombinedOutput(); err != nil {
		log.Printf("[notify] command failed: %v, output: %s", err, string(output))
	}
}

func isTerminalStatus(status string) bool {
	s := strings.ToUpper(strings.TrimSpace(status))
	switch s {
	case "SUCCESS", "FAILED", "FAIL", "CANCELED", "CANCELLED":
		return true
	}
	return false
}

func formatResult(status string) string {
	s := strings.ToUpper(strings.TrimSpace(status))
	switch s {
	case "SUCCESS":
		return "success \u2713"
	case "FAILED", "FAIL":
		return "failed \u2717"
	case "CANCELED", "CANCELLED":
		return "canceled \u25CB"
	default:
		return strings.ToLower(s)
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	s := d % time.Minute
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s/time.Second)
	}
	return fmt.Sprintf("%ds", s/time.Second)
}
```

**Step 2: Build to verify**

Run: `cd /Users/liuxiang/cascode/github.com/flowt && go build ./...`
Expected: no errors

**Step 3: Commit**

```bash
git add internal/notify/notifier.go
git commit -m "feat(notify): add Notifier with background run monitoring and template rendering"
```

---

### Task 3: Write tests for `internal/notify/notifier.go`

**Files:**
- Create: `internal/notify/notifier_test.go`

**Step 1: Write tests**

```go
package notify

import (
	"strings"
	"testing"
	"time"
)

func TestFormatResult(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SUCCESS", "success \u2713"},
		{"FAILED", "failed \u2717"},
		{"FAIL", "failed \u2717"},
		{"CANCELED", "canceled \u25CB"},
		{"CANCELLED", "canceled \u25CB"},
		{"success", "success \u2713"},
		{"failed", "failed \u2717"},
		{"RUNNING", "running"},
		{"QUEUED", "queued"},
	}

	for _, tt := range tests {
		got := formatResult(tt.input)
		if got != tt.expected {
			t.Errorf("formatResult(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{0, "0s"},
		{1 * time.Second, "1s"},
		{59 * time.Second, "59s"},
		{1 * time.Minute, "1m 0s"},
		{1*time.Minute + 30*time.Second, "1m 30s"},
		{2*time.Minute + 35*time.Second, "2m 35s"},
		{10*time.Minute + 1*time.Second, "10m 1s"},
		{500 * time.Millisecond, "0s"}, // rounded
	}

	for _, tt := range tests {
		got := formatDuration(tt.input)
		if got != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsTerminalStatus(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{"SUCCESS", true},
		{"FAILED", true},
		{"FAIL", true},
		{"CANCELED", true},
		{"CANCELLED", true},
		{"RUNNING", false},
		{"QUEUED", false},
		{"INIT", false},
		{"", false},
		{"  SUCCESS  ", true},
		{"success", true},
	}

	for _, tt := range tests {
		got := isTerminalStatus(tt.status)
		if got != tt.expected {
			t.Errorf("isTerminalStatus(%q) = %v, want %v", tt.status, got, tt.expected)
		}
	}
}

func TestNewNilWhenEmpty(t *testing.T) {
	n, err := New(nil, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if n != nil {
		t.Fatal("expected nil notifier for empty command")
	}
}

func TestNewInvalidTemplate(t *testing.T) {
	_, err := New(nil, "{{.UnknownField}")
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
	if !strings.Contains(err.Error(), "template") {
		t.Errorf("error should mention template, got: %v", err)
	}
}

func TestNewValidTemplate(t *testing.T) {
	n, err := New(nil, "echo {{.PipelineName}} {{.Result}}")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if n == nil {
		t.Fatal("expected non-nil notifier")
	}
}

func TestTrackAndStop(t *testing.T) {
	n, err := New(nil, "echo {{.PipelineName}}")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	n.Track(TrackedRun{
		OrganizationID: "org1",
		PipelineID:     "p1",
		PipelineName:   "test-pipeline",
		RunID:          "r1",
		Branch:         "main",
		StartTime:      time.Now(),
	})

	// Give the goroutine time to start
	time.Sleep(100 * time.Millisecond)

	n.Stop()
}
```

**Step 2: Run tests**

Run: `cd /Users/liuxiang/cascode/github.com/flowt && go test ./internal/notify/ -v`
Expected: all tests PASS

**Step 3: Commit**

```bash
git add internal/notify/notifier_test.go
git commit -m "test(notify): add unit tests for format helpers and Notifier lifecycle"
```

---

### Task 4: Integrate Notifier into TUI

**Files:**
- Modify: `internal/tui/app.go:19-49` (Model struct and New function)
- Modify: `cmd/flo/tui.go:14-30` (RunTUIFunc)

**Step 1: Add notifier field to Model struct**

In `internal/tui/app.go`, add import and field. Add `"flo/internal/notify"` to imports, then add a field to the `Model` struct (after line 48, before `keys`):

```go
	// Notification
	notifier *notify.Notifier
```

**Step 2: Create Notifier in `New()`**

In `internal/tui/app.go`, modify the `New` function to create a Notifier. After the `Model` initialization (before `return m`):

```go
	// Initialize notifier if configured
	if cfg.NotifyCommand != "" {
		notifier, err := notify.New(client, cfg.NotifyCommand)
		if err != nil {
			// Log but don't fail — notification is best-effort
			log.Printf("failed to create notifier: %v", err)
		} else if notifier != nil {
			m.notifier = notifier
		}
	}
```

Also add `"log"` to imports.

**Step 3: Track runs after pipeline starts**

In `internal/tui/app.go`, in the `RunAPIStartedMsg` case (around line 194), after the run succeeds and we set up the logs page, add tracking. After the line `m.logsPage = m.logsPage.SetAutoRefresh(true)` (line 228), add:

```go
			// Track this run for background notification
			if m.notifier != nil {
				m.notifier.Track(notify.TrackedRun{
					OrganizationID: m.organizationID,
					PipelineID:     pipelineID,
					PipelineName:   pipelineName,
					RunID:          msg.RunID,
					Branch:         branch, // captured from BranchSelectedMsg
					StartTime:      time.Now(),
				})
			}
```

**Important:** We need the branch name at this point. The branch comes from `BranchSelectedMsg`, but by the time `RunAPIStartedMsg` arrives, we don't have it directly. We need to capture it. Add a `lastRunBranch` field to the Model struct to pass the branch from `BranchSelectedMsg` to `RunAPIStartedMsg`:

Add to Model struct:
```go
	// Captures the branch used when starting a pipeline run
	lastRunBranch string
```

In the `BranchSelectedMsg` handler (around line 160), after `if pipelineID != "" {`, add:
```go
				m.lastRunBranch = msg.Branch
```

Then in `RunAPIStartedMsg` handler, use `m.lastRunBranch` instead of `branch`:
```go
			if m.notifier != nil {
				m.notifier.Track(notify.TrackedRun{
					OrganizationID: m.organizationID,
					PipelineID:     pipelineID,
					PipelineName:   pipelineName,
					RunID:          msg.RunID,
					Branch:         m.lastRunBranch,
					StartTime:      time.Now(),
				})
			}
```

**Step 4: Build to verify**

Run: `cd /Users/liuxiang/cascode/github.com/flowt && go build ./...`
Expected: no errors

**Step 5: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(notify): integrate Notifier into TUI to track pipeline runs"
```

---

### Task 5: Manual verification

**Step 1: Configure notification**

Add to `~/.flo/config.yml`:

```yaml
notify_command: "echo 'NOTIFY: {{.PipelineName}} {{.Result}} ({{.Duration}}) [{{.Branch}}]'"
```

**Step 2: Build and run**

Run: `cd /Users/liuxiang/cascode/github.com/flowt && go run ./cmd/flo`

**Step 3: Start a pipeline run**

1. Select a pipeline
2. Press `r` to run
3. Navigate away from the logs page (go back to pipelines list)
4. Wait for the pipeline to finish
5. Verify the echo output appears in the terminal where flo was launched

**Step 4: Switch to terminal-notifier (optional)**

If on macOS, change to:
```yaml
notify_command: "terminal-notifier -title 'flo' -message '{{.PipelineName}} {{.Result}} ({{.Duration}}) [{{.Branch}}]'"
```

Verify the macOS notification appears.

**Step 5: Commit any fixes if needed**
