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
		{500 * time.Millisecond, "1s"}, // rounded
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
