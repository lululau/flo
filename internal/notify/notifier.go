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
				n.mu.Lock()
				empty := len(n.runs) == 0
				n.mu.Unlock()
				if empty {
					return
				}
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
	n.mu.Unlock()
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
