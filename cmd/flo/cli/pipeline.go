package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"flo/internal/api"
	"flo/internal/config"
)

// newClient creates an API client from the given config.
func newClient(cfg *config.Config) (*api.Client, error) {
	if cfg.UsePersonalAccessToken() {
		return api.NewClientWithToken(cfg.GetEndpoint(), cfg.PersonalAccessToken)
	}
	return api.NewClient(cfg.AccessKeyID, cfg.AccessKeySecret, cfg.GetRegionID())
}

// getOrgID returns the effective organization ID (flag overrides config).
func getOrgID(cfg *config.Config) string {
	if orgID != "" {
		return orgID
	}
	return cfg.OrganizationID
}

// --- pipeline parent command ---

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Manage pipelines",
	Long:  "List, inspect, run, and manage Aliyun DevOps pipelines.",
}

func init() {
	pipelineCmd.AddCommand(pipelineListCmd)
	pipelineCmd.AddCommand(pipelineGroupsCmd)
	pipelineCmd.AddCommand(pipelineHistoryCmd)
	pipelineCmd.AddCommand(pipelineRunCmd)
	pipelineCmd.AddCommand(pipelineLogsCmd)
	pipelineCmd.AddCommand(pipelineStopCmd)
}

// =========================================================================
// Task 3: flo pipeline list
// =========================================================================

var (
	listSearch   string
	listStatus   string
	listSort     string
	listBookmark bool
)

var pipelineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pipelines",
	Long:  "List all pipelines with optional filtering and sorting.",
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

		// Build status list for the API call.
		var statusList []string
		if listStatus != "" && listStatus != "all" {
			statusList = strings.Split(listStatus, ",")
		}

		pipelines, err := client.ListPipelinesWithStatus(org, statusList)
		if err != nil {
			return fmt.Errorf("failed to list pipelines: %w", err)
		}

		// Filter by search text (case-insensitive substring match on name).
		if listSearch != "" {
			var filtered []api.Pipeline
			for _, p := range pipelines {
				if strings.Contains(strings.ToLower(p.Name), strings.ToLower(listSearch)) {
					filtered = append(filtered, p)
				}
			}
			pipelines = filtered
		}

		// Filter by bookmark.
		if listBookmark {
			var filtered []api.Pipeline
			for _, p := range pipelines {
				if cfg.IsBookmarked(p.Name) {
					filtered = append(filtered, p)
				}
			}
			pipelines = filtered
		}

		// Sort.
		switch listSort {
		case "time":
			sort.Slice(pipelines, func(i, j int) bool {
				return pipelines[i].LastRunTime.After(pipelines[j].LastRunTime)
			})
		default: // "name"
			sort.Slice(pipelines, func(i, j int) bool {
				return strings.ToLower(pipelines[i].Name) < strings.ToLower(pipelines[j].Name)
			})
		}

		// Build output.
		type pipelineRow struct {
			Name    string `json:"name"`
			ID      string `json:"id"`
			Status  string `json:"status"`
			LastRun string `json:"lastRun"`
			Creator string `json:"creator"`
		}
		rows := make([]pipelineRow, 0, len(pipelines))
		tableRows := make([][]string, 0, len(pipelines))
		for _, p := range pipelines {
			lastRun := p.LastRunTime.Format("2006-01-02 15:04")
			if p.LastRunTime.IsZero() {
				lastRun = "-"
			}
			status := p.LastRunStatus
			if status == "" {
				status = p.Status
			}
			creator := p.CreatorName
			if creator == "" {
				creator = p.Creator
			}
			rows = append(rows, pipelineRow{
				Name:    p.Name,
				ID:      p.PipelineID,
				Status:  status,
				LastRun: lastRun,
				Creator: creator,
			})
			tableRows = append(tableRows, []string{p.Name, status, lastRun, creator})
		}

		if outputFormat == "json" {
			return Output(map[string]interface{}{
				"pipelines": rows,
				"total":     len(rows),
			}, nil, nil)
		}
		return Output(nil, []string{"NAME", "STATUS", "LAST RUN", "CREATOR"}, tableRows)
	},
}

func init() {
	pipelineListCmd.Flags().StringVar(&listSearch, "search", "", "Filter by name (case-insensitive substring)")
	pipelineListCmd.Flags().StringVar(&listStatus, "status", "all", "Filter by status: all, running, success, failed")
	pipelineListCmd.Flags().StringVar(&listSort, "sort", "name", "Sort by: name, time")
	pipelineListCmd.Flags().BoolVar(&listBookmark, "bookmark", false, "Show bookmarked pipelines only")
}

// =========================================================================
// Task 4: flo pipeline groups
// =========================================================================

var groupsSearch string

var pipelineGroupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "List pipeline groups",
	Long:  "List all pipeline groups with optional filtering.",
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

		groups, err := client.ListPipelineGroups(org)
		if err != nil {
			return fmt.Errorf("failed to list pipeline groups: %w", err)
		}

		// Filter by search text.
		if groupsSearch != "" {
			var filtered []api.PipelineGroup
			for _, g := range groups {
				if strings.Contains(strings.ToLower(g.Name), strings.ToLower(groupsSearch)) {
					filtered = append(filtered, g)
				}
			}
			groups = filtered
		}

		type groupRow struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		}
		rows := make([]groupRow, 0, len(groups))
		tableRows := make([][]string, 0, len(groups))
		for _, g := range groups {
			rows = append(rows, groupRow{Name: g.Name, ID: g.GroupID})
			tableRows = append(tableRows, []string{g.Name, g.GroupID})
		}

		if outputFormat == "json" {
			return Output(map[string]interface{}{
				"groups": rows,
				"total":  len(rows),
			}, nil, nil)
		}
		return Output(nil, []string{"NAME", "ID"}, tableRows)
	},
}

func init() {
	pipelineGroupsCmd.Flags().StringVar(&groupsSearch, "search", "", "Filter by name (case-insensitive substring)")
}

// =========================================================================
// Task 5: flo pipeline history
// =========================================================================

var (
	historyPipeline string
	historyStatus   string
	historyLimit    int
	historyPage     int
)

var pipelineHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Show pipeline run history",
	Long:  "Show the run history for a specific pipeline.",
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

		// Resolve pipeline name to ID.
		pipelineID, err := resolvePipelineID(client, org, historyPipeline)
		if err != nil {
			return err
		}

		perPage := cfg.GetPerPage()
		if historyLimit > 0 {
			perPage = historyLimit
		}
		if historyPage < 1 {
			historyPage = 1
		}

		result, err := client.ListPipelineRunsPaginated(org, pipelineID, historyPage, perPage)
		if err != nil {
			return fmt.Errorf("failed to list pipeline runs: %w", err)
		}

		// Filter by status after fetching.
		runs := result.Runs
		if historyStatus != "" && historyStatus != "all" {
			statuses := strings.Split(historyStatus, ",")
			statusSet := make(map[string]bool, len(statuses))
			for _, s := range statuses {
				statusSet[strings.ToUpper(s)] = true
			}
			var filtered []api.PipelineRun
			for _, r := range runs {
				if statusSet[strings.ToUpper(r.Status)] {
					filtered = append(filtered, r)
				}
			}
			runs = filtered
		}

		type runRow struct {
			RunID     string `json:"runId"`
			Status    string `json:"status"`
			Trigger   string `json:"trigger"`
			StartTime string `json:"startTime"`
			Duration  string `json:"duration"`
		}
		rows := make([]runRow, 0, len(runs))
		tableRows := make([][]string, 0, len(runs))
		for _, r := range runs {
			startTime := r.StartTime.Format("2006-01-02 15:04:05")
			if r.StartTime.IsZero() {
				startTime = "-"
			}
			duration := formatDuration(r.StartTime, r.FinishTime)
			trigger := r.TriggerMode
			if trigger == "" {
				trigger = "-"
			}
			rows = append(rows, runRow{
				RunID:     r.RunID,
				Status:    r.Status,
				Trigger:   trigger,
				StartTime: startTime,
				Duration:  duration,
			})
			tableRows = append(tableRows, []string{r.RunID, r.Status, trigger, startTime, duration})
		}

		if outputFormat == "json" {
			return Output(map[string]interface{}{
				"runs":        rows,
				"currentPage": result.CurrentPage,
				"totalPages":  result.TotalPages,
				"total":       len(rows),
			}, nil, nil)
		}
		return Output(nil, []string{"RUN ID", "STATUS", "TRIGGER", "START TIME", "DURATION"}, tableRows)
	},
}

func init() {
	pipelineHistoryCmd.Flags().StringVar(&historyPipeline, "pipeline", "", "Pipeline name or ID (required)")
	pipelineHistoryCmd.MarkFlagRequired("pipeline")
	pipelineHistoryCmd.Flags().StringVar(&historyStatus, "status", "all", "Filter by status: all, running, success, failed")
	pipelineHistoryCmd.Flags().IntVar(&historyLimit, "limit", 0, "Limit number of results per page")
	pipelineHistoryCmd.Flags().IntVar(&historyPage, "page", 1, "Page number")
}

// formatDuration formats the duration between start and finish.
// < 1min -> "Xs", < 1hr -> "XmYs", >= 1hr -> "XhYm"
func formatDuration(start, finish time.Time) string {
	if start.IsZero() {
		return "-"
	}
	end := finish
	if end.IsZero() {
		end = time.Now()
	}
	d := end.Sub(start)
	if d < 0 {
		return "-"
	}
	totalSec := int(d.Seconds())
	if totalSec < 60 {
		return fmt.Sprintf("%ds", totalSec)
	}
	totalMin := totalSec / 60
	sec := totalSec % 60
	if totalMin < 60 {
		return fmt.Sprintf("%dm%ds", totalMin, sec)
	}
	hours := totalMin / 60
	min := totalMin % 60
	return fmt.Sprintf("%dh%dm", hours, min)
}

// resolvePipelineID resolves a pipeline name or numeric ID to a pipeline ID string.
func resolvePipelineID(client *api.Client, orgID, nameOrID string) (string, error) {
	// If it looks like a numeric ID, use it directly.
	if _, err := strconv.Atoi(nameOrID); err == nil {
		return nameOrID, nil
	}

	// Otherwise, list all pipelines and find by name.
	pipelines, err := client.ListPipelinesWithStatus(orgID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to resolve pipeline name %q: %w", nameOrID, err)
	}
	for _, p := range pipelines {
		if p.Name == nameOrID {
			return p.PipelineID, nil
		}
	}
	return "", fmt.Errorf("pipeline %q not found", nameOrID)
}

// =========================================================================
// Task 6: flo pipeline run
// =========================================================================

var (
	runPipeline string
	runBranch   string
	runFollow   bool
)

var pipelineRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a pipeline",
	Long:  "Trigger a pipeline run with optional branch specification and follow mode.",
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

		// Resolve pipeline name to ID.
		pipelineID, err := resolvePipelineID(client, org, runPipeline)
		if err != nil {
			return err
		}

		// Get repository URLs from the latest run info.
		params := make(map[string]string)
		if runBranch != "" {
			runInfo, err := client.GetLatestPipelineRunInfo(org, pipelineID)
			if err != nil {
				return fmt.Errorf("failed to get pipeline repository info: %w", err)
			}

			runningBranchs := buildRunningBranches(runBranch, runInfo.RepositoryURLs)
			branchJSON, err := json.Marshal(runningBranchs)
			if err != nil {
				return fmt.Errorf("failed to marshal runningBranchs: %w", err)
			}
			params["runningBranchs"] = string(branchJSON)
		}

		// Trigger the pipeline.
		run, err := client.RunPipeline(org, pipelineID, params)
		if err != nil {
			return fmt.Errorf("failed to run pipeline: %w", err)
		}

		if outputFormat == "json" {
			return Output(map[string]interface{}{
				"runId":      run.RunID,
				"pipelineId": pipelineID,
				"status":     run.Status,
			}, nil, nil)
		}

		fmt.Fprintf(os.Stdout, "Pipeline run started: %s (run ID: %s)\n", pipelineID, run.RunID)

		// Follow mode: poll until terminal status.
		if runFollow {
			return followRun(client, org, pipelineID, run.RunID)
		}
		return nil
	},
}

func init() {
	pipelineRunCmd.Flags().StringVar(&runPipeline, "pipeline", "", "Pipeline name or ID (required)")
	pipelineRunCmd.MarkFlagRequired("pipeline")
	pipelineRunCmd.Flags().StringVar(&runBranch, "branch", "", "Branch spec: 'main' or 'repo1:main,repo2:develop'")
	pipelineRunCmd.Flags().BoolVarP(&runFollow, "follow", "f", false, "Follow the pipeline run until completion")
}

// buildRunningBranches builds the runningBranchs map from a branch spec and repository URLs.
//   - "main" -> all repos use "main"
//   - "repo1:main,repo2:develop" -> match repo URLs by substring
func buildRunningBranches(branchSpec string, repoURLs map[string]string) map[string]string {
	result := make(map[string]string)

	// Check if the spec contains colons (multi-repo format).
	if strings.Contains(branchSpec, ":") {
		// Parse "repo1:branch1,repo2:branch2" format.
		pairs := strings.Split(branchSpec, ",")
		for _, pair := range pairs {
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) != 2 {
				continue
			}
			repoSubstr := strings.TrimSpace(parts[0])
			branch := strings.TrimSpace(parts[1])
			// Match against repository URLs by substring.
			for url := range repoURLs {
				if strings.Contains(url, repoSubstr) {
					result[url] = branch
				}
			}
		}
	} else {
		// Single branch: apply to all repos.
		for url := range repoURLs {
			result[url] = branchSpec
		}
	}

	return result
}

// isTerminalStatus checks if a run status is terminal.
func isTerminalStatus(status string) bool {
	s := strings.ToUpper(strings.TrimSpace(status))
	switch s {
	case "SUCCESS", "FAILED", "FAIL", "CANCELED", "CANCELLED":
		return true
	default:
		return false
	}
}

// followRun polls the pipeline run every 5 seconds until it reaches a terminal status.
func followRun(client *api.Client, orgID, pipelineID, runID string) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			run, err := client.GetPipelineRun(orgID, pipelineID, runID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error polling run status: %v\n", err)
				return err
			}
			fmt.Fprintf(os.Stdout, "Status: %s\n", run.Status)
			if isTerminalStatus(run.Status) {
				return nil
			}
		}
	}
}

// =========================================================================
// Task 7: flo pipeline logs
// =========================================================================

var (
	logsRunID  string
	logsStage  string
	logsFollow bool
)

var pipelineLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show pipeline run logs",
	Long:  "Show logs for a pipeline run, optionally filtered by stage.",
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

		// Parse run ID format: pipelineId/runId.
		parts := strings.SplitN(logsRunID, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --run-id format, expected pipelineId/runId")
		}
		pipelineID := parts[0]
		runID := parts[1]

		if logsFollow {
			return streamLogs(client, org, pipelineID, runID, logsStage)
		}
		return showLogs(client, org, pipelineID, runID, logsStage)
	},
}

func init() {
	pipelineLogsCmd.Flags().StringVar(&logsRunID, "run-id", "", "Pipeline run ID in format pipelineId/runId (required)")
	pipelineLogsCmd.MarkFlagRequired("run-id")
	pipelineLogsCmd.Flags().StringVar(&logsStage, "stage", "", "Filter by stage name")
	pipelineLogsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Stream logs until run completes")
}

// stageLog represents a stage with its jobs and logs for JSON output.
type stageLog struct {
	Name   string    `json:"name"`
	Status string    `json:"status"`
	Jobs   []jobLog  `json:"jobs"`
}

// jobLog represents a job with its log content.
type jobLog struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Logs   string `json:"logs"`
}

// showLogs fetches and displays logs for a pipeline run (non-streaming).
func showLogs(client *api.Client, orgID, pipelineID, runID, stageFilter string) error {
	details, err := client.GetPipelineRunDetails(orgID, pipelineID, runID)
	if err != nil {
		return fmt.Errorf("failed to get run details: %w", err)
	}

	stages := details.Stages
	// Filter by stage name if specified.
	if stageFilter != "" {
		var filtered []api.Stage
		for _, s := range stages {
			if s.Name == stageFilter {
				filtered = append(filtered, s)
			}
		}
		stages = filtered
	}

	if outputFormat == "json" {
		stageLogs := make([]stageLog, 0, len(stages))
		for _, stage := range stages {
			sl := stageLog{
				Name:   stage.Name,
				Status: computeStageStatus(stage),
			}
			for _, job := range stage.Jobs {
				jobIDStr := fmt.Sprintf("%d", job.ID)
				logContent, logErr := client.GetPipelineJobRunLog(orgID, pipelineID, runID, jobIDStr)
				if logErr != nil {
					logContent = fmt.Sprintf("Failed to get job log: %v", logErr)
				}
				sl.Jobs = append(sl.Jobs, jobLog{
					Name:   job.Name,
					Status: job.Status,
					Logs:   logContent,
				})
			}
			stageLogs = append(stageLogs, sl)
		}
		return Output(map[string]interface{}{
			"runId":  runID,
			"status": details.Status,
			"stages": stageLogs,
		}, nil, nil)
	}

	// Table format: print headers and logs.
	for _, stage := range stages {
		fmt.Fprintf(os.Stdout, "\n=== Stage: %s (%s) ===\n", stage.Name, computeStageStatus(stage))
		for _, job := range stage.Jobs {
			fmt.Fprintf(os.Stdout, "--- Job: %s (%s) ---\n", job.Name, job.Status)
			jobIDStr := fmt.Sprintf("%d", job.ID)
			logContent, logErr := client.GetPipelineJobRunLog(orgID, pipelineID, runID, jobIDStr)
			if logErr != nil {
				fmt.Fprintf(os.Stdout, "Failed to get job log: %v\n", logErr)
			} else {
				if logContent != "" {
					fmt.Fprintln(os.Stdout, logContent)
				}
			}
		}
	}
	return nil
}

// streamLogs polls for new logs every 3 seconds until the run is terminal.
func streamLogs(client *api.Client, orgID, pipelineID, runID, stageFilter string) error {
	// Track which jobs we've already printed logs for to avoid duplication.
	printedLogs := make(map[string]bool) // key: "jobID"

	for {
		details, err := client.GetPipelineRunDetails(orgID, pipelineID, runID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting run details: %v\n", err)
			return err
		}

		stages := details.Stages
		if stageFilter != "" {
			var filtered []api.Stage
			for _, s := range stages {
				if s.Name == stageFilter {
					filtered = append(filtered, s)
				}
			}
			stages = filtered
		}

		// Print logs for RUNNING jobs that haven't been printed yet.
		for _, stage := range stages {
			for _, job := range stage.Jobs {
				key := fmt.Sprintf("%d", job.ID)
				if strings.ToUpper(job.Status) == "RUNNING" && !printedLogs[key] {
					fmt.Fprintf(os.Stdout, "\n=== Stage: %s (%s) ===\n", stage.Name, computeStageStatus(stage))
					fmt.Fprintf(os.Stdout, "--- Job: %s (%s) ---\n", job.Name, job.Status)
					jobIDStr := fmt.Sprintf("%d", job.ID)
					logContent, logErr := client.GetPipelineJobRunLog(orgID, pipelineID, runID, jobIDStr)
					if logErr != nil {
						fmt.Fprintf(os.Stdout, "Failed to get job log: %v\n", logErr)
					} else if logContent != "" {
						fmt.Fprintln(os.Stdout, logContent)
					}
					printedLogs[key] = true
				}
			}
		}

		// Check if run is terminal.
		if isTerminalStatus(details.Status) {
			// Print any remaining unprinted job logs.
			for _, stage := range stages {
				for _, job := range stage.Jobs {
					key := fmt.Sprintf("%d", job.ID)
					if !printedLogs[key] {
						fmt.Fprintf(os.Stdout, "\n=== Stage: %s (%s) ===\n", stage.Name, computeStageStatus(stage))
						fmt.Fprintf(os.Stdout, "--- Job: %s (%s) ---\n", job.Name, job.Status)
						jobIDStr := fmt.Sprintf("%d", job.ID)
						logContent, logErr := client.GetPipelineJobRunLog(orgID, pipelineID, runID, jobIDStr)
						if logErr != nil {
							fmt.Fprintf(os.Stdout, "Failed to get job log: %v\n", logErr)
						} else if logContent != "" {
							fmt.Fprintln(os.Stdout, logContent)
						}
						printedLogs[key] = true
					}
				}
			}
			fmt.Fprintf(os.Stdout, "\nRun finished with status: %s\n", details.Status)
			return nil
		}

		time.Sleep(3 * time.Second)
	}
}

// computeStageStatus computes a summary status for a stage based on its jobs.
func computeStageStatus(stage api.Stage) string {
	hasRunning := false
	hasFailed := false
	hasSuccess := false
	for _, job := range stage.Jobs {
		s := strings.ToUpper(strings.TrimSpace(job.Status))
		switch s {
		case "RUNNING":
			hasRunning = true
		case "FAILED", "FAIL":
			hasFailed = true
		case "SUCCESS":
			hasSuccess = true
		}
	}
	if hasFailed {
		return "FAILED"
	}
	if hasRunning {
		return "RUNNING"
	}
	if hasSuccess {
		return "SUCCESS"
	}
	if len(stage.Jobs) > 0 {
		return stage.Jobs[0].Status
	}
	return "UNKNOWN"
}

// =========================================================================
// Task 8: flo pipeline stop
// =========================================================================

var stopRunID string

var pipelineStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a pipeline run",
	Long:  "Stop a running pipeline run.",
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

		// Parse run ID format: pipelineId/runId.
		parts := strings.SplitN(stopRunID, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --run-id format, expected pipelineId/runId")
		}
		pipelineID := parts[0]
		runID := parts[1]

		if err := client.StopPipelineRun(org, pipelineID, runID); err != nil {
			return fmt.Errorf("failed to stop pipeline run: %w", err)
		}

		if outputFormat == "json" {
			return Output(map[string]interface{}{
				"runId":      runID,
				"pipelineId": pipelineID,
				"status":     "STOPPED",
				"message":    "Pipeline run stopped successfully",
			}, nil, nil)
		}

		fmt.Fprintf(os.Stdout, "Pipeline run %s stopped successfully\n", runID)
		return nil
	},
}

func init() {
	pipelineStopCmd.Flags().StringVar(&stopRunID, "run-id", "", "Pipeline run ID in format pipelineId/runId (required)")
	pipelineStopCmd.MarkFlagRequired("run-id")
}
