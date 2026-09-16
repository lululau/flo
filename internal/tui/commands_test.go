package tui

import (
	"testing"

	"flo/internal/api"
)

func TestCountLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{"empty string", "", 0},
		{"single line with newline", "hello\n", 1},
		{"single line without newline", "hello", 1},
		{"multiple lines with newline", "line1\nline2\nline3\n", 3},
		{"multiple lines without trailing newline", "line1\nline2\nline3", 3},
		{"empty lines", "\n\n\n", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countLines(tt.input)
			if got != tt.expected {
				t.Errorf("countLines(%q) = %d; want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsYunxiaoRealtimeException(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"regular logs", "[INFO] build started\n[SUCCESS]", false},
		{"exception message", "[executionStep begins at ...]\n[ERROR] Log real-time query exception. Please try again later", true},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isYunxiaoRealtimeException(tt.input)
			if got != tt.expected {
				t.Errorf("isYunxiaoRealtimeException(%q) = %v; want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsMachineTerminal(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{"SUCCESS", true},
		{"success", true},
		{"FAILED", true},
		{"FAIL", true},
		{"CANCELED", true},
		{"TIMEOUT", true},
		{"RUNNING", false},
		{"running", false},
		{"INIT", false},
		{"QUEUED", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := isMachineTerminal(tt.status)
			if got != tt.expected {
				t.Errorf("isMachineTerminal(%q) = %v; want %v", tt.status, got, tt.expected)
			}
		})
	}
}

func TestExtractDeployOrderId(t *testing.T) {
	jobWithFloat := &api.Job{
		Actions: []api.JobAction{
			{
				Type: "GetVMDeployOrder",
				Params: map[string]interface{}{
					"deployOrderId": float64(69709667),
				},
			},
		},
	}
	if got := extractDeployOrderId(jobWithFloat); got != "69709667" {
		t.Errorf("extractDeployOrderId(float) = %q; want 69709667", got)
	}

	jobWithString := &api.Job{
		Actions: []api.JobAction{
			{
				Type: "VMDeploy",
				Params: map[string]interface{}{
					"deployOrderId": "69709667",
				},
			},
		},
	}
	if got := extractDeployOrderId(jobWithString); got != "69709667" {
		t.Errorf("extractDeployOrderId(string) = %q; want 69709667", got)
	}

	jobWithoutAction := &api.Job{
		Actions: []api.JobAction{
			{
				Type: "RegularBuild",
			},
		},
	}
	if got := extractDeployOrderId(jobWithoutAction); got != "" {
		t.Errorf("extractDeployOrderId(none) = %q; want empty", got)
	}
}
