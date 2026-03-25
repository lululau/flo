package tui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"flo/internal/api"
	"flo/internal/tui/types"
)

// LoadPipelinesCmd loads pipelines from the API
func LoadPipelinesCmd(client *api.Client, organizationID string) tea.Cmd {
	return LoadPipelinesWithStatusCmd(client, organizationID, nil)
}

// LoadPipelinesWithStatusCmd loads pipelines from the API with optional status filter
func LoadPipelinesWithStatusCmd(client *api.Client, organizationID string, statusList []string) tea.Cmd {
	return func() tea.Msg {
		pipelines, err := client.ListPipelinesWithStatus(organizationID, statusList)
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
	return LoadGroupPipelinesWithStatusCmd(client, organizationID, groupID, nil)
}

// LoadGroupPipelinesWithStatusCmd loads pipelines for a specific group with optional status filter
func LoadGroupPipelinesWithStatusCmd(client *api.Client, organizationID, groupID string, statusList []string) tea.Cmd {
	return func() tea.Msg {
		groupIDInt, err := strconv.Atoi(groupID)
		if err != nil {
			return types.ErrorMsg{Err: fmt.Errorf("invalid group ID: %w", err)}
		}

		// Build options map for the API call
		var options map[string]interface{}
		if len(statusList) > 0 {
			options = map[string]interface{}{
				"statusList": strings.Join(statusList, ","),
			}
		}

		pipelines, err := client.ListPipelineGroupPipelines(organizationID, groupIDInt, options)
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

// RunPipelineCmd runs a pipeline
func RunPipelineCmd(client *api.Client, organizationID, pipelineID, branch string, repositoryURLs map[string]string) tea.Cmd {
	return func() tea.Msg {
		params := make(map[string]string)

		if len(repositoryURLs) > 0 {
			// Build runningBranchs: map each repo URL to the selected branch
			runningBranchs := make(map[string]string)
			for repoURL := range repositoryURLs {
				runningBranchs[repoURL] = branch
			}
			branchJSON, err := json.Marshal(runningBranchs)
			if err != nil {
				return types.RunAPIStartedMsg{
					RunID: "",
					Error: fmt.Errorf("failed to marshal runningBranchs: %w", err),
				}
			}
			params["runningBranchs"] = string(branchJSON)
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

func isVMDeploymentJob(job *api.Job) bool {
	for _, action := range job.Actions {
		if action.Type == "vm-deploy-build" || action.Type == "VMDeploy" || action.Type == "GetVMDeployOrder" {
			return true
		}
	}
	return false
}

func getVMDeploymentLogs(client *api.Client, organizationID, pipelineID string, job *api.Job) (string, error) {
	var deployOrderIDStr string

	// Find deploy order ID from actions
	for _, action := range job.Actions {
		if action.Type == "vm-deploy-build" || action.Type == "VMDeploy" || action.Type == "GetVMDeployOrder" {
			// Try to extract deployOrderId from action params
			if id, ok := action.Params["deployOrderId"]; ok {
				// Properly convert to integer string to avoid scientific notation
				// JSON unmarshals numbers as float64, which fmt.Sprintf("%v") may format
				// in scientific notation for large numbers (e.g., 5.4882198e+07)
				// The API expects a plain integer string (e.g., "54882198")
				switch v := id.(type) {
				case float64:
					deployOrderIDStr = strconv.FormatInt(int64(v), 10)
				case string:
					deployOrderIDStr = v
				case int64:
					deployOrderIDStr = strconv.FormatInt(v, 10)
				case int:
					deployOrderIDStr = strconv.Itoa(v)
				default:
					deployOrderIDStr = fmt.Sprintf("%v", id)
				}
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

// --- Stage Tabs Logs (new) ---

// LoadRunStageTabsCmd loads the stage tabs model and (by default) the selected tab's logs.
func LoadRunStageTabsCmd(client *api.Client, organizationID, pipelineID, pipelineName, runID string) tea.Cmd {
	return func() tea.Msg {
		// Get run details
		details, err := client.GetPipelineRunDetails(organizationID, pipelineID, runID)
		if err != nil {
			return types.ErrorMsg{Err: fmt.Errorf("failed to get run details: %w", err)}
		}

		data := &types.RunStageTabsData{
			PipelineID:     pipelineID,
			PipelineName:   pipelineName,
			RunID:          runID,
			RunStatus:      details.Status,
			Stages:         make([]types.StageTab, 0, len(details.Stages)),
			SelectedIndex:  0,
			ActiveIndex:    -1,
			LastActiveIndex: -1,
			FollowActive:   true,
		}

		for _, st := range details.Stages {
			status, complete := computeStageTabStatus(st)
			data.Stages = append(data.Stages, types.StageTab{
				StageIndex: st.Index,
				Name:       st.Name,
				Status:     status,
				Complete:   complete,
				Loaded:     false,
				Entries:    nil,
			})
		}

		data.ActiveIndex = computeActiveStageIndex(data.Stages)
		data.LastActiveIndex = data.ActiveIndex

		// Default selection: non-running -> first tab, running -> active tab
		if isRunTerminal(details.Status) {
			data.SelectedIndex = 0
			data.FollowActive = false
		} else if data.ActiveIndex >= 0 {
			data.SelectedIndex = data.ActiveIndex
			data.FollowActive = true
		}

		// Load selected tab logs immediately (so the page has content on entry)
		if len(details.Stages) > 0 {
			sel := clampIndex(data.SelectedIndex, len(details.Stages))
			data.SelectedIndex = sel
			_, _ = loadStageTabFull(client, organizationID, pipelineID, runID, details.Stages[sel], &data.Stages[sel])
		}

		return types.RunStageTabsLoadedMsg{Data: data}
	}
}

// RefreshRunStageTabsCmd refreshes run details and incrementally updates the active stage logs.
// This is intended to be called on TickMsg for running pipelines.
func RefreshRunStageTabsCmd(client *api.Client, organizationID string, existing *types.RunStageTabsData) tea.Cmd {
	return func() tea.Msg {
		if existing == nil {
			return types.ErrorMsg{Err: fmt.Errorf("no existing stage tabs data")}
		}

		updated := deepCopyRunStageTabsData(existing)
		if updated == nil {
			return types.ErrorMsg{Err: fmt.Errorf("failed to copy stage tabs data")}
		}

		pipelineID := updated.PipelineID
		runID := updated.RunID

		// Get current run details
		details, err := client.GetPipelineRunDetails(organizationID, pipelineID, runID)
		if err != nil {
			return types.ErrorMsg{Err: fmt.Errorf("failed to get run details: %w", err)}
		}
		updated.RunStatus = details.Status

		// Rebuild stages while preserving any loaded logs by stage index
		prevByStageIndex := make(map[string]types.StageTab, len(updated.Stages))
		for _, st := range updated.Stages {
			prevByStageIndex[st.StageIndex] = st
		}

		newStages := make([]types.StageTab, 0, len(details.Stages))
		for _, st := range details.Stages {
			status, complete := computeStageTabStatus(st)
			tab := types.StageTab{
				StageIndex: st.Index,
				Name:       st.Name,
				Status:     status,
				Complete:   complete,
				Loaded:     false,
				Entries:    nil,
			}
			if prev, ok := prevByStageIndex[st.Index]; ok {
				tab.Loaded = prev.Loaded
				tab.Entries = append([]types.StageLogEntry(nil), prev.Entries...)
			}
			newStages = append(newStages, tab)
		}
		updated.Stages = newStages

		updated.ActiveIndex = computeActiveStageIndex(updated.Stages)
		if updated.ActiveIndex < 0 && len(updated.Stages) > 0 {
			updated.ActiveIndex = len(updated.Stages) - 1
		}

		// Auto-advance selection only forward, and only if user hasn't manually moved away.
		if !isRunTerminal(updated.RunStatus) && updated.FollowActive {
			if updated.LastActiveIndex >= 0 &&
				updated.ActiveIndex > updated.LastActiveIndex &&
				updated.SelectedIndex == updated.LastActiveIndex {
				updated.SelectedIndex = updated.ActiveIndex
			}
			updated.LastActiveIndex = updated.ActiveIndex
		}

		// Keep selection in range
		if len(updated.Stages) > 0 {
			updated.SelectedIndex = clampIndex(updated.SelectedIndex, len(updated.Stages))
		} else {
			updated.SelectedIndex = 0
		}

		hasNew := false

		// Refresh logs for the active stage (current stage) to achieve real-time updates.
		if updated.ActiveIndex >= 0 && updated.ActiveIndex < len(details.Stages) {
			tab := &updated.Stages[updated.ActiveIndex]
			stage := details.Stages[updated.ActiveIndex]

			if !tab.Loaded {
				changed, _ := loadStageTabFull(client, organizationID, pipelineID, runID, stage, tab)
				hasNew = hasNew || changed
			} else {
				changed, _ := refreshStageTabIncremental(client, organizationID, pipelineID, runID, stage, tab)
				hasNew = hasNew || changed
			}
		}

		return types.RunStageTabsUpdatedMsg{Data: updated, HasNewContent: hasNew}
	}
}

// LoadRunStageTabCmd ensures a specific tab's logs are loaded (or refreshed) when the user switches tabs.
func LoadRunStageTabCmd(client *api.Client, organizationID string, existing *types.RunStageTabsData, tabIndex int) tea.Cmd {
	return func() tea.Msg {
		if existing == nil {
			return types.ErrorMsg{Err: fmt.Errorf("no existing stage tabs data")}
		}

		updated := deepCopyRunStageTabsData(existing)
		if updated == nil {
			return types.ErrorMsg{Err: fmt.Errorf("failed to copy stage tabs data")}
		}

		pipelineID := updated.PipelineID
		runID := updated.RunID

		details, err := client.GetPipelineRunDetails(organizationID, pipelineID, runID)
		if err != nil {
			return types.ErrorMsg{Err: fmt.Errorf("failed to get run details: %w", err)}
		}
		updated.RunStatus = details.Status

		// Rebuild stages while preserving previously loaded logs
		prevByStageIndex := make(map[string]types.StageTab, len(updated.Stages))
		for _, st := range updated.Stages {
			prevByStageIndex[st.StageIndex] = st
		}
		newStages := make([]types.StageTab, 0, len(details.Stages))
		for _, st := range details.Stages {
			status, complete := computeStageTabStatus(st)
			tab := types.StageTab{
				StageIndex: st.Index,
				Name:       st.Name,
				Status:     status,
				Complete:   complete,
				Loaded:     false,
				Entries:    nil,
			}
			if prev, ok := prevByStageIndex[st.Index]; ok {
				tab.Loaded = prev.Loaded
				tab.Entries = append([]types.StageLogEntry(nil), prev.Entries...)
			}
			newStages = append(newStages, tab)
		}
		updated.Stages = newStages

		updated.ActiveIndex = computeActiveStageIndex(updated.Stages)
		if updated.ActiveIndex < 0 && len(updated.Stages) > 0 {
			updated.ActiveIndex = len(updated.Stages) - 1
		}

		if len(updated.Stages) == 0 {
			updated.SelectedIndex = 0
			return types.RunStageTabsUpdatedMsg{Data: updated, HasNewContent: false}
		}

		tabIndex = clampIndex(tabIndex, len(updated.Stages))
		updated.SelectedIndex = tabIndex

		hasNew := false
		tab := &updated.Stages[tabIndex]
		stage := details.Stages[tabIndex]

		if !tab.Loaded {
			changed, _ := loadStageTabFull(client, organizationID, pipelineID, runID, stage, tab)
			hasNew = hasNew || changed
		} else {
			changed, _ := refreshStageTabIncremental(client, organizationID, pipelineID, runID, stage, tab)
			hasNew = hasNew || changed
		}

		return types.RunStageTabsUpdatedMsg{Data: updated, HasNewContent: hasNew}
	}
}

func isRunTerminal(status string) bool {
	s := strings.ToUpper(strings.TrimSpace(status))
	switch s {
	case "SUCCESS", "FAILED", "FAIL", "CANCELED", "CANCELLED":
		return true
	default:
		return false
	}
}

func clampIndex(idx, length int) int {
	if length <= 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= length {
		return length - 1
	}
	return idx
}

func computeActiveStageIndex(stages []types.StageTab) int {
	for i := range stages {
		if !stages[i].Complete {
			return i
		}
	}
	if len(stages) == 0 {
		return -1
	}
	return len(stages) - 1
}

func computeStageTabStatus(stage api.Stage) (types.StageTabStatus, bool) {
	anyRunning := false
	anyFailed := false
	anyCanceled := false
	allTerminal := true
	allSuccessOrSkipped := true

	for _, job := range stage.Jobs {
		st := strings.ToUpper(strings.TrimSpace(job.Status))
		switch st {
		case "RUNNING":
			anyRunning = true
			allTerminal = false
			allSuccessOrSkipped = false
		case "FAILED", "FAIL":
			anyFailed = true
		case "CANCELED", "CANCELLED":
			anyCanceled = true
		case "SUCCESS":
			// terminal success
		case "SKIPPED":
			// terminal skipped
		case "QUEUED", "INIT":
			allTerminal = false
			allSuccessOrSkipped = false
		default:
			// unknown -> treat as waiting
			allTerminal = false
			allSuccessOrSkipped = false
		}
		if st != "SUCCESS" && st != "SKIPPED" {
			// Keep allSuccessOrSkipped false for any other status.
		}
	}

	complete := allTerminal

	if anyFailed {
		return types.StageTabStatusFailed, complete
	}
	if anyCanceled && complete {
		return types.StageTabStatusCanceled, true
	}
	if anyRunning {
		return types.StageTabStatusRunning, false
	}
	if complete {
		if allSuccessOrSkipped {
			return types.StageTabStatusSuccess, true
		}
		return types.StageTabStatusSuccess, true
	}
	return types.StageTabStatusWaiting, false
}

func deepCopyRunStageTabsData(src *types.RunStageTabsData) *types.RunStageTabsData {
	if src == nil {
		return nil
	}
	dst := *src
	dst.Stages = make([]types.StageTab, len(src.Stages))
	for i := range src.Stages {
		dst.Stages[i] = src.Stages[i]
		dst.Stages[i].Entries = append([]types.StageLogEntry(nil), src.Stages[i].Entries...)
	}
	return &dst
}

func loadStageTabFull(client *api.Client, organizationID, pipelineID, runID string, stage api.Stage, tab *types.StageTab) (bool, error) {
	if tab == nil {
		return false, nil
	}

	entries := make([]types.StageLogEntry, 0)

	for _, job := range stage.Jobs {
		jobIDStr := fmt.Sprintf("%d", job.ID)

		// VM deploy jobs: fetch via dedicated APIs (full text each time).
		if isVMDeploymentJob(&job) {
			logs, err := getVMDeploymentLogs(client, organizationID, pipelineID, &job)
			if err != nil {
				logs = fmt.Sprintf("Failed to get VM deployment logs: %s\n", err)
			}
			entries = append(entries, types.StageLogEntry{
				Key:        types.StageLogEntryKey{JobID: job.ID, StepIndex: 0},
				JobID:      job.ID,
				JobName:    job.Name,
				StepIndex:  0,
				StepName:   job.Name,
				IsVMDeploy: true,
				Status:     job.Status,
				Logs:       logs,
			})
			continue
		}

		// Prefer step logs API (supports incremental offsets).
		jobSteps, err := client.GetPipelineJobSteps(organizationID, pipelineID, runID, jobIDStr)
		if err != nil || jobSteps == nil || len(jobSteps.BuildProcessNodes) == 0 {
			// Fallback to job run log
			jobLog, jerr := client.GetPipelineJobRunLog(organizationID, pipelineID, runID, jobIDStr)
			if jerr != nil {
				jobLog = fmt.Sprintf("Failed to get job log: %s\n", jerr)
			}
			entries = append(entries, types.StageLogEntry{
				Key:       types.StageLogEntryKey{JobID: job.ID, StepIndex: 0},
				JobID:     job.ID,
				JobName:   job.Name,
				StepIndex: 0,
				StepName:  job.Name,
				Status:    job.Status,
				Logs:      jobLog,
			})
			continue
		}

		for _, node := range jobSteps.BuildProcessNodes {
			entry := types.StageLogEntry{
				Key:       types.StageLogEntryKey{JobID: job.ID, StepIndex: node.StepIndex},
				JobID:     job.ID,
				JobName:   job.Name,
				StepIndex: node.StepIndex,
				StepName:  node.StepName,
				IsVMDeploy: false,
				BuildId:   jobSteps.BuildId,
				Offset:    0,
				HasMore:   true,
				Status:    node.Status,
				Logs:      "",
			}

			stepLog, serr := client.GetPipelineJobStepLog(
				organizationID, pipelineID, runID, jobIDStr,
				node.StepIndex, jobSteps.BuildId, 0, 100000,
			)
			if serr == nil && stepLog != nil {
				entry.Logs = stepLog.Logs
				entry.Offset = stepLog.Last
				entry.HasMore = stepLog.More
			}

			entries = append(entries, entry)
		}
	}

	tab.Entries = entries
	tab.Loaded = true
	return true, nil
}

func refreshStageTabIncremental(client *api.Client, organizationID, pipelineID, runID string, stage api.Stage, tab *types.StageTab) (bool, error) {
	if tab == nil {
		return false, nil
	}

	hasNew := false

	indexByKey := make(map[types.StageLogEntryKey]int, len(tab.Entries))
	for i := range tab.Entries {
		indexByKey[tab.Entries[i].Key] = i
	}

	ensureEntry := func(e types.StageLogEntry) int {
		if idx, ok := indexByKey[e.Key]; ok {
			return idx
		}
		tab.Entries = append(tab.Entries, e)
		idx := len(tab.Entries) - 1
		indexByKey[e.Key] = idx
		return idx
	}

	jobHasMore := func(jobID int64) bool {
		for i := range tab.Entries {
			if tab.Entries[i].JobID == jobID && tab.Entries[i].HasMore {
				return true
			}
		}
		return false
	}

	for _, job := range stage.Jobs {
		jobIDStr := fmt.Sprintf("%d", job.ID)

		if isVMDeploymentJob(&job) {
			key := types.StageLogEntryKey{JobID: job.ID, StepIndex: 0}
			idx := ensureEntry(types.StageLogEntry{
				Key:        key,
				JobID:      job.ID,
				JobName:    job.Name,
				StepIndex:  0,
				StepName:   job.Name,
				IsVMDeploy: true,
			})
			tab.Entries[idx].JobName = job.Name
			tab.Entries[idx].StepName = job.Name
			tab.Entries[idx].Status = job.Status

			// VM deploy logs are fetched as a whole; refresh while running (and when empty).
			if strings.ToUpper(job.Status) == "RUNNING" || tab.Entries[idx].Logs == "" {
				logs, err := getVMDeploymentLogs(client, organizationID, pipelineID, &job)
				if err != nil {
					// Only write error if we have no logs yet.
					if tab.Entries[idx].Logs == "" {
						tab.Entries[idx].Logs = fmt.Sprintf("Failed to get VM deployment logs: %s\n", err)
						hasNew = true
					}
				} else if logs != "" && logs != tab.Entries[idx].Logs {
					tab.Entries[idx].Logs = logs
					hasNew = true
				}
			}
			continue
		}

		needSteps := strings.ToUpper(job.Status) == "RUNNING" || jobHasMore(job.ID)
		if !needSteps {
			continue
		}

		jobSteps, err := client.GetPipelineJobSteps(organizationID, pipelineID, runID, jobIDStr)
		if err != nil || jobSteps == nil || len(jobSteps.BuildProcessNodes) == 0 {
			// Fallback: refresh job-level log while running.
			key := types.StageLogEntryKey{JobID: job.ID, StepIndex: 0}
			idx := ensureEntry(types.StageLogEntry{
				Key:       key,
				JobID:     job.ID,
				JobName:   job.Name,
				StepIndex: 0,
				StepName:  job.Name,
			})
			tab.Entries[idx].JobName = job.Name
			tab.Entries[idx].StepName = job.Name
			tab.Entries[idx].Status = job.Status

			if strings.ToUpper(job.Status) == "RUNNING" {
				jobLog, jerr := client.GetPipelineJobRunLog(organizationID, pipelineID, runID, jobIDStr)
				if jerr == nil && jobLog != "" && jobLog != tab.Entries[idx].Logs {
					tab.Entries[idx].Logs = jobLog
					hasNew = true
				}
			}
			continue
		}

		for _, node := range jobSteps.BuildProcessNodes {
			key := types.StageLogEntryKey{JobID: job.ID, StepIndex: node.StepIndex}
			idx := ensureEntry(types.StageLogEntry{
				Key:       key,
				JobID:     job.ID,
				JobName:   job.Name,
				StepIndex: node.StepIndex,
				StepName:  node.StepName,
				IsVMDeploy: false,
				BuildId:   jobSteps.BuildId,
				Offset:    0,
				HasMore:   true,
				Status:    node.Status,
				Logs:      "",
			})

			// Keep names/buildId/status up to date.
			tab.Entries[idx].JobName = job.Name
			tab.Entries[idx].StepName = node.StepName
			tab.Entries[idx].BuildId = jobSteps.BuildId
			tab.Entries[idx].Status = node.Status

			// Fetch logs for running steps, and drain remaining logs for finished steps.
			shouldFetch := node.Running || (node.Finish && tab.Entries[idx].HasMore)
			if !shouldFetch {
				continue
			}

			const logChunkLimit = 5000
			const maxChunksPerTick = 3

			offset := tab.Entries[idx].Offset
			for c := 0; c < maxChunksPerTick; c++ {
				prevOffset := offset
				stepLog, serr := client.GetPipelineJobStepLog(
					organizationID, pipelineID, runID, jobIDStr,
					node.StepIndex, jobSteps.BuildId, offset, logChunkLimit,
				)
				if serr != nil || stepLog == nil {
					break
				}

				if stepLog.Logs != "" {
					tab.Entries[idx].Logs += stepLog.Logs
					hasNew = true
				}

				tab.Entries[idx].Offset = stepLog.Last
				tab.Entries[idx].HasMore = stepLog.More
				offset = stepLog.Last

				if offset == prevOffset {
					break
				}
				if !stepLog.More {
					break
				}
			}
		}
	}

	tab.Loaded = true
	return hasNew, nil
}
