package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"flowt/internal/api"
	"flowt/internal/tui/types"
)

// LoadPipelinesCmd loads pipelines from the API
func LoadPipelinesCmd(client *api.Client, organizationID string) tea.Cmd {
	return func() tea.Msg {
		pipelines, err := client.ListPipelinesWithStatus(organizationID, nil)
		if err != nil {
			return types.ErrorMsg{Err: fmt.Errorf("failed to load pipelines: %w", err)}
		}

		return types.PipelinesAPILoadedMsg{
			Pipelines:   pipelines,
			CurrentPage: 1,
			TotalPages:  1,
			IsComplete:  true,
		}
	}
}

// LoadPipelinesProgressiveCmd loads pipelines progressively with callbacks
func LoadPipelinesProgressiveCmd(client *api.Client, organizationID string) tea.Cmd {
	return func() tea.Msg {
		var allPipelines []api.Pipeline
		var lastCurrentPage, lastTotalPages int

		err := client.ListPipelinesWithStatusAndCallback(organizationID, nil, func(pipelines []api.Pipeline, currentPage, totalPages int, isComplete bool) error {
			allPipelines = append(allPipelines, pipelines...)
			lastCurrentPage = currentPage
			lastTotalPages = totalPages
			return nil
		})

		if err != nil {
			return types.ErrorMsg{Err: fmt.Errorf("failed to load pipelines: %w", err)}
		}

		return types.PipelinesAPILoadedMsg{
			Pipelines:   allPipelines,
			CurrentPage: lastCurrentPage,
			TotalPages:  lastTotalPages,
			IsComplete:  true,
		}
	}
}

// LoadGroupsCmd loads pipeline groups from the API
func LoadGroupsCmd(client *api.Client, organizationID string) tea.Cmd {
	return func() tea.Msg {
		groups, err := client.ListPipelineGroups(organizationID)
		if err != nil {
			return types.ErrorMsg{Err: fmt.Errorf("failed to load groups: %w", err)}
		}

		return types.GroupsAPILoadedMsg{
			Groups: groups,
		}
	}
}

// LoadGroupPipelinesCmd loads pipelines for a specific group
func LoadGroupPipelinesCmd(client *api.Client, organizationID, groupID string) tea.Cmd {
	return func() tea.Msg {
		groupIDInt, err := strconv.Atoi(groupID)
		if err != nil {
			return types.ErrorMsg{Err: fmt.Errorf("invalid group ID: %w", err)}
		}

		pipelines, err := client.ListPipelineGroupPipelines(organizationID, groupIDInt, nil)
		if err != nil {
			return types.ErrorMsg{Err: fmt.Errorf("failed to load group pipelines: %w", err)}
		}

		return types.PipelinesAPILoadedMsg{
			Pipelines:   pipelines,
			CurrentPage: 1,
			TotalPages:  1,
			IsComplete:  true,
		}
	}
}

// LoadHistoryCmd loads run history for a pipeline with pagination
func LoadHistoryCmd(client *api.Client, organizationID, pipelineID string, page, perPage int) tea.Cmd {
	return func() tea.Msg {
		result, err := client.ListPipelineRunsPaginated(organizationID, pipelineID, page, perPage)
		if err != nil {
			return types.ErrorMsg{Err: fmt.Errorf("failed to load history: %w", err)}
		}

		return types.HistoryAPILoadedMsg{
			Runs:        result.Runs,
			CurrentPage: result.CurrentPage,
			TotalPages:  result.TotalPages,
			TotalRuns:   result.TotalRuns,
			PerPage:     result.PerPage,
		}
	}
}

// LoadLogsCmd loads logs for a pipeline run
func LoadLogsCmd(client *api.Client, organizationID, pipelineID, runID string) tea.Cmd {
	return func() tea.Msg {
		// Get run details
		details, err := client.GetPipelineRunDetails(organizationID, pipelineID, runID)
		if err != nil {
			return types.ErrorMsg{Err: fmt.Errorf("failed to get run details: %w", err)}
		}

		// Format logs
		var logContent strings.Builder
		logContent.WriteString(formatRunOverview(details))

		totalJobs := countTotalJobs(details)
		currentJob := 0

		for _, stage := range details.Stages {
			logContent.WriteString(fmt.Sprintf("\n=== Stage: %s ===\n", stage.Name))

			for _, job := range stage.Jobs {
				currentJob++
				logContent.WriteString(fmt.Sprintf("\n--- Job: %s (Status: %s) ---\n", job.Name, job.Status))

				// Check if this is a VM deployment job
				if isVMDeploymentJob(&job) {
					vmLogs, err := getVMDeploymentLogs(client, organizationID, pipelineID, &job)
					if err != nil {
						logContent.WriteString(fmt.Sprintf("Failed to get VM deployment logs: %s\n", err))
					} else {
						logContent.WriteString(vmLogs)
					}
				} else {
					// Regular job log
					jobIDStr := fmt.Sprintf("%d", job.ID)
					jobLog, err := client.GetPipelineJobRunLog(organizationID, pipelineID, runID, jobIDStr)
					if err != nil {
						logContent.WriteString(fmt.Sprintf("Failed to get job log: %s\n", err))
					} else {
						logContent.WriteString(jobLog + "\n")
					}
				}
			}
		}

		return types.LogsAPILoadedMsg{
			Details:    details,
			LogContent: logContent.String(),
			Status:     details.Status,
			CurrentJob: currentJob,
			TotalJobs:  totalJobs,
			IsComplete: true,
		}
	}
}

// RunPipelineCmd runs a pipeline
func RunPipelineCmd(client *api.Client, organizationID, pipelineID, branch string) tea.Cmd {
	return func() tea.Msg {
		params := map[string]string{
			"branch": branch,
		}

		run, err := client.RunPipeline(organizationID, pipelineID, params)
		if err != nil {
			return types.RunAPIStartedMsg{
				RunID: "",
				Error: fmt.Errorf("failed to run pipeline: %w", err),
			}
		}

		return types.RunAPIStartedMsg{
			RunID: run.RunID,
			Error: nil,
		}
	}
}

// StopPipelineRunCmd stops a pipeline run
func StopPipelineRunCmd(client *api.Client, organizationID, pipelineID, runID string) tea.Cmd {
	return func() tea.Msg {
		err := client.StopPipelineRun(organizationID, pipelineID, runID)
		if err != nil {
			return types.RunAPIStoppedMsg{
				Error: fmt.Errorf("failed to stop pipeline: %w", err),
			}
		}

		return types.RunAPIStoppedMsg{
			Error: nil,
		}
	}
}

// AutoRefreshTickCmd returns a tick command for auto-refresh
func AutoRefreshTickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return types.TickMsg{}
	})
}

// LoadBranchInfoCmd loads the default branch info for a pipeline
func LoadBranchInfoCmd(client *api.Client, organizationID, pipelineID string) tea.Cmd {
	return func() tea.Msg {
		defaultBranch := "master" // Fallback default
		repositoryURLs := make(map[string]string)

		// Try to get latest run information to extract branch and repository information
		latestRunInfo, err := client.GetLatestPipelineRunInfo(organizationID, pipelineID)
		if err == nil && latestRunInfo != nil && len(latestRunInfo.RepositoryURLs) > 0 {
			repositoryURLs = latestRunInfo.RepositoryURLs
			// Use the first repository's branch as default
			for _, branch := range latestRunInfo.RepositoryURLs {
				defaultBranch = branch
				break
			}
		}

		return types.BranchInfoLoadedMsg{
			DefaultBranch:  defaultBranch,
			RepositoryURLs: repositoryURLs,
		}
	}
}

// Helper functions

func formatRunOverview(details *api.PipelineRunDetails) string {
	var sb strings.Builder

	sb.WriteString("╔══════════════════════════════════════════════════════════════╗\n")
	sb.WriteString(fmt.Sprintf("║  Pipeline Run #%d\n", details.PipelineRunID))
	sb.WriteString(fmt.Sprintf("║  Status: %s\n", details.Status))

	if details.CreateTime > 0 {
		t := time.Unix(details.CreateTime/1000, 0)
		sb.WriteString(fmt.Sprintf("║  Created: %s\n", t.Local().Format("2006-01-02 15:04:05")))
	}

	if details.UpdateTime > 0 && details.CreateTime > 0 {
		duration := time.Duration(details.UpdateTime-details.CreateTime) * time.Millisecond
		sb.WriteString(fmt.Sprintf("║  Duration: %s\n", formatDurationPretty(duration)))
	}

	sb.WriteString("╚══════════════════════════════════════════════════════════════╝\n")

	return sb.String()
}

func formatDurationPretty(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func countTotalJobs(details *api.PipelineRunDetails) int {
	total := 0
	for _, stage := range details.Stages {
		total += len(stage.Jobs)
	}
	return total
}

func isVMDeploymentJob(job *api.Job) bool {
	for _, action := range job.Actions {
		if action.Type == "vm-deploy-build" || action.Type == "VMDeploy" {
			return true
		}
	}
	return false
}

func getVMDeploymentLogs(client *api.Client, organizationID, pipelineID string, job *api.Job) (string, error) {
	var deployOrderIDStr string

	// Find deploy order ID from actions
	for _, action := range job.Actions {
		if action.Type == "vm-deploy-build" || action.Type == "VMDeploy" {
			// Try to extract deployOrderId from action params
			if id, ok := action.Params["deployOrderId"]; ok {
				deployOrderIDStr = fmt.Sprintf("%v", id)
				break
			}
		}
	}

	if deployOrderIDStr == "" {
		return "", fmt.Errorf("could not find deploy order ID")
	}

	// Get deploy order details
	order, err := client.GetVMDeployOrder(organizationID, pipelineID, deployOrderIDStr)
	if err != nil {
		return "", fmt.Errorf("failed to get deploy order: %w", err)
	}

	var logs strings.Builder
	logs.WriteString(fmt.Sprintf("VM Deployment Order: %d (Status: %s)\n", order.DeployOrderId, order.Status))

	// Get logs for each machine
	for _, machine := range order.DeployMachineInfo.DeployMachines {
		logs.WriteString(fmt.Sprintf("\n>> Machine: %s (Status: %s)\n", machine.IP, machine.Status))

		machineLog, err := client.GetVMDeployMachineLog(organizationID, pipelineID, deployOrderIDStr, machine.MachineSn)
		if err != nil {
			logs.WriteString(fmt.Sprintf("Failed to get machine log: %s\n", err))
		} else {
			logs.WriteString(machineLog.DeployLog + "\n")
		}
	}

	return logs.String(), nil
}
