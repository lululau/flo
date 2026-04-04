# CLI Subcommands Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add non-interactive CLI subcommands for all existing TUI features using Cobra, with resource-oriented command structure and backward-compatible TUI entry.

**Architecture:** Extract TUI startup logic from `cmd/flo/main.go` into `cmd/flo/tui.go`. Add `cmd/flo/cli/` package with Cobra root command + pipeline subcommands. All CLI commands reuse the existing `internal/api` client directly — zero TUI code changes.

**Tech Stack:** Go 1.24, Cobra, existing `internal/api` and `internal/config` packages, `text/tabwriter` for table output.

---

### Task 1: Add Cobra dependency and extract TUI startup

**Files:**
- Modify: `cmd/flo/main.go`
- Create: `cmd/flo/tui.go`
- Modify: `go.mod` / `go.sum`

**Step 1: Add Cobra dependency**

Run: `cd /Users/liuxiang/cascode/github.com/flowt && go get github.com/spf13/cobra`

**Step 2: Create `cmd/flo/tui.go` — extract TUI startup logic**

Move lines 14-61 of `cmd/flo/main.go` into a new function:

```go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"flo/internal/api"
	"flo/internal/config"
	"flo/internal/tui"
)

// runTUI loads config, creates API client, and launches the TUI.
func runTUI(cfg *config.Config) error {
	var client *api.Client
	var err error
	if cfg.UsePersonalAccessToken() {
		client, err = api.NewClientWithToken(cfg.GetEndpoint(), cfg.PersonalAccessToken)
	} else {
		client, err = api.NewClient(cfg.AccessKeyID, cfg.AccessKeySecret, cfg.GetRegionID())
	}
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	model := tui.New(cfg, client)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
```

**Step 3: Rewrite `cmd/flo/main.go` to delegate to Cobra**

```go
package main

import (
	"fmt"
	"os"

	"flo/cmd/flo/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 4: Tidy and build**

Run: `go mod tidy && go build ./cmd/flo`
Expected: clean build

**Step 5: Commit**

```bash
git add cmd/flo/main.go cmd/flo/tui.go cmd/flo/cli/ go.mod go.sum
git commit -m "feat: add Cobra CLI framework and extract TUI startup"
```

---

### Task 2: Create Cobra root command and output utilities

**Files:**
- Create: `cmd/flo/cli/root.go`
- Create: `cmd/flo/cli/output.go`

**Step 1: Create `cmd/flo/cli/root.go`**

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"flo/internal/config"
)

var (
	outputFormat string
	configPath   string
	orgID        string
)

var rootCmd = &cobra.Command{
	Use:   "flo",
	Short: "Aliyun DevOps pipeline manager",
	Long:  "Flo is a TUI and CLI tool for managing Aliyun DevOps (云效) pipelines.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// No subcommand → launch TUI (backward compatible)
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return err
		}
		return runTUI(cfg)
	},
}

func loadConfig() (*config.Config, error) {
	if configPath != "" {
		return config.LoadConfigFrom(configPath)
	}
	return config.LoadConfig()
}

func getOrgID(cfg *config.Config) string {
	if orgID != "" {
		return orgID
	}
	return cfg.OrganizationID
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Config file path (default: ~/.flo/config.yml)")
	rootCmd.PersistentFlags().StringVar(&orgID, "org", "", "Organization ID (overrides config)")
}
```

Note: `runTUI` is defined in `cmd/flo/tui.go` (same `main` package), accessed from `cli` via a bridge. Since `cli` is a separate package, we need a different approach — see below.

**Actually**, because `cmd/flo/tui.go` is in `package main` and `cmd/flo/cli/` is a separate package, the CLI root cannot call `runTUI` directly. Solution: define a `RunTUI(cfg *config.Config) error` function in a shared location.

**Revised approach — create `cmd/flo/cli/tui_bridge.go`:**

Since Cobra root's `RunE` needs to launch TUI, and TUI code imports `bubbletea` (which we don't want as a dependency of the CLI package), the cleanest solution is to keep the TUI launch in `main.go` itself:

**Revised `cmd/flo/cli/root.go`:**

```go
package cli

import (
	"github.com/spf13/cobra"

	"flo/internal/config"
)

var (
	outputFormat string
	configPath   string
	orgID        string
)

// RunTUI is set by main to allow the root command to launch the TUI.
var RunTUI func(cfg *config.Config) error

var rootCmd = &cobra.Command{
	Use:   "flo",
	Short: "Aliyun DevOps pipeline manager",
	Long:  "Flo is a TUI and CLI tool for managing Aliyun DevOps (云效) pipelines.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return err
		}
		return RunTUI(cfg)
	},
}

func loadConfig() (*config.Config, error) {
	if configPath != "" {
		return config.LoadConfigFrom(configPath)
	}
	return config.LoadConfig()
}

func GetOrgID(cfg *config.Config) string {
	if orgID != "" {
		return orgID
	}
	return cfg.OrganizationID
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Config file path (default: ~/.flo/config.yml)")
	rootCmd.PersistentFlags().StringVar(&orgID, "org", "", "Organization ID (overrides config)")
}
```

**Revised `cmd/flo/main.go`:**

```go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"flo/cmd/flo/cli"
	"flo/internal/api"
	"flo/internal/config"
	"flo/internal/tui"
)

func init() {
	cli.RunTUI = func(cfg *config.Config) error {
		var client *api.Client
		var err error
		if cfg.UsePersonalAccessToken() {
			client, err = api.NewClientWithToken(cfg.GetEndpoint(), cfg.PersonalAccessToken)
		} else {
			client, err = api.NewClient(cfg.AccessKeyID, cfg.AccessKeySecret, cfg.GetRegionID())
		}
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}
		model := tui.New(cfg, client)
		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err = p.Run()
		return err
	}
}

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 2: Add `LoadConfigFrom` to `internal/config/config.go`**

Add after `LoadConfig`:

```go
// LoadConfigFrom loads configuration from a specific file path.
func LoadConfigFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	return &cfg, nil
}
```

**Step 3: Create `cmd/flo/cli/output.go` — shared output formatting**

```go
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// Output writes data in the configured format (table or JSON).
func Output(data interface{}, headers []string, rows [][]string) error {
	if outputFormat == "json" {
		return outputJSON(data)
	}
	return outputTable(headers, rows)
}

func outputJSON(data interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func outputTable(headers []string, rows [][]string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	return w.Flush()
}

// PrintError prints an error message to stderr and exits.
func PrintError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

// MustLoadClient creates an API client from config, exiting on error.
func MustLoadClient(cfg *config.Config) *api.Client {
	// imported in pipeline.go
	return nil // placeholder
}
```

**Step 4: Build and verify**

Run: `go build ./cmd/flo && ./flo --help`
Expected: Shows root command help with global flags, no subcommands yet

**Step 5: Commit**

```bash
git add cmd/flo/main.go cmd/flo/cli/root.go cmd/flo/cli/output.go internal/config/config.go
git commit -m "feat: add Cobra root command with global flags and output utilities"
```

---

### Task 3: Add `flo pipeline list` subcommand

**Files:**
- Create: `cmd/flo/cli/pipeline.go`

**Step 1: Create `cmd/flo/cli/pipeline.go` with list subcommand**

```go
package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"flo/internal/api"
	"flo/internal/config"
)

func init() {
	pipelineCmd.AddCommand(pipelineListCmd)
}

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Manage pipelines",
}

var (
	listSearch    string
	listStatus    string
	listSort      string
	listBookmark  bool
)

var pipelineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all pipelines",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		client, err := newClient(cfg)
		if err != nil {
			return err
		}
		orgID := GetOrgID(cfg)

		var statusList []string
		if listStatus != "" && listStatus != "all" {
			statusList = strings.Split(listStatus, ",")
		}

		pipelines, err := client.ListPipelinesWithStatus(orgID, statusList)
		if err != nil {
			return fmt.Errorf("failed to list pipelines: %w", err)
		}

		// Filter by search
		if listSearch != "" {
			pipelines = filterPipelines(pipelines, listSearch)
		}

		// Filter by bookmark
		if listBookmark {
			pipelines = filterBookmarkPipelines(pipelines, cfg)
		}

		// Sort
		sortPipelines(pipelines, listSort)

		// Output
		type pipelineRow struct {
			Name        string `json:"name"`
			ID          string `json:"id"`
			Status      string `json:"status"`
			LastRun     string `json:"lastRun"`
			Creator     string `json:"creator"`
		}
		type pipelineListOutput struct {
			Pipelines []pipelineRow `json:"pipelines"`
			Total     int           `json:"total"`
		}

		output := pipelineListOutput{Total: len(pipelines)}
		headers := []string{"NAME", "STATUS", "LAST RUN", "CREATOR"}
		var rows [][]string

		for _, p := range pipelines {
			creator := p.CreatorName
			if creator == "" {
				creator = p.Creator
			}
			lastRun := p.LastRunTime.Format(time.RFC3339)
			if p.LastRunTime.IsZero() {
				lastRun = "-"
			}
			output.Pipelines = append(output.Pipelines, pipelineRow{
				Name:    p.Name,
				ID:      p.PipelineID,
				Status:  p.LastRunStatus,
				LastRun: lastRun,
				Creator: creator,
			})
			rows = append(rows, []string{p.Name, p.LastRunStatus, lastRun, creator})
		}

		return Output(output, headers, rows)
	},
}

func init() {
	pipelineListCmd.Flags().StringVar(&listSearch, "search", "", "Search pipelines by name")
	pipelineListCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status (running,success,failed,all)")
	pipelineListCmd.Flags().StringVar(&listSort, "sort", "name", "Sort by: name, time")
	pipelineListCmd.Flags().BoolVar(&listBookmark, "bookmark", false, "Show bookmarked pipelines only")
}

func filterPipelines(pipelines []api.Pipeline, search string) []api.Pipeline {
	s := strings.ToLower(search)
	var result []api.Pipeline
	for _, p := range pipelines {
		if strings.Contains(strings.ToLower(p.Name), s) {
			result = append(result, p)
		}
	}
	return result
}

func filterBookmarkPipelines(pipelines []api.Pipeline, cfg *config.Config) []api.Pipeline {
	var result []api.Pipeline
	for _, p := range pipelines {
		if cfg.IsBookmarked(p.Name) {
			result = append(result, p)
		}
	}
	return result
}

func sortPipelines(pipelines []api.Pipeline, sortBy string) {
	switch sortBy {
	case "time":
		sort.Slice(pipelines, func(i, j int) bool {
			return pipelines[i].LastRunTime.After(pipelines[j].LastRunTime)
		})
	default: // name
		sort.Slice(pipelines, func(i, j int) bool {
			return strings.ToLower(pipelines[i].Name) < strings.ToLower(pipelines[j].Name)
		})
	}
}
```

Also update `output.go` to add `newClient` helper (move the MustLoadClient placeholder):

```go
func newClient(cfg *config.Config) (*api.Client, error) {
	if cfg.UsePersonalAccessToken() {
		return api.NewClientWithToken(cfg.GetEndpoint(), cfg.PersonalAccessToken)
	}
	return api.NewClient(cfg.AccessKeyID, cfg.AccessKeySecret, cfg.GetRegionID())
}
```

**Step 2: Build and verify**

Run: `go build ./cmd/flo && ./flo pipeline list --help`
Expected: Shows `flo pipeline list` help with all flags

**Step 3: Commit**

```bash
git add cmd/flo/cli/pipeline.go cmd/flo/cli/output.go
git commit -m "feat: add pipeline list subcommand with search, filter, and sort"
```

---

### Task 4: Add `flo pipeline groups` subcommand

**Files:**
- Modify: `cmd/flo/cli/pipeline.go`

**Step 1: Add groups subcommand to `pipeline.go`**

```go
func init() {
	pipelineCmd.AddCommand(pipelineListCmd)
	pipelineCmd.AddCommand(pipelineGroupsCmd)
}

var groupsSearch string

var pipelineGroupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "List pipeline groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		client, err := newClient(cfg)
		if err != nil {
			return err
		}
		orgID := GetOrgID(cfg)

		groups, err := client.ListPipelineGroups(orgID)
		if err != nil {
			return fmt.Errorf("failed to list groups: %w", err)
		}

		if groupsSearch != "" {
			s := strings.ToLower(groupsSearch)
			var filtered []api.PipelineGroup
			for _, g := range groups {
				if strings.Contains(strings.ToLower(g.Name), s) {
					filtered = append(filtered, g)
				}
			}
			groups = filtered
		}

		type groupOutput struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		type groupsListOutput struct {
			Groups []groupOutput `json:"groups"`
			Total  int           `json:"total"`
		}

		output := groupsListOutput{Total: len(groups)}
		headers := []string{"NAME", "ID"}
		var rows [][]string

		for _, g := range groups {
			output.Groups = append(output.Groups, groupOutput{ID: g.GroupID, Name: g.Name})
			rows = append(rows, []string{g.Name, g.GroupID})
		}

		return Output(output, headers, rows)
	},
}

func init() {
	pipelineGroupsCmd.Flags().StringVar(&groupsSearch, "search", "", "Search groups by name")
}
```

**Step 2: Build and verify**

Run: `go build ./cmd/flo && ./flo pipeline groups --help`
Expected: Shows groups help

**Step 3: Commit**

```bash
git add cmd/flo/cli/pipeline.go
git commit -m "feat: add pipeline groups subcommand"
```

---

### Task 5: Add `flo pipeline history` subcommand

**Files:**
- Modify: `cmd/flo/cli/pipeline.go`

**Step 1: Add history subcommand**

```go
func init() {
	pipelineCmd.AddCommand(pipelineListCmd)
	pipelineCmd.AddCommand(pipelineGroupsCmd)
	pipelineCmd.AddCommand(pipelineHistoryCmd)
}

var (
	historyPipeline string
	historyStatus   string
	historyLimit    int
	historyPage     int
)

var pipelineHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "View pipeline run history",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		client, err := newClient(cfg)
		if err != nil {
			return err
		}
		orgID := GetOrgID(cfg)

		if historyPipeline == "" {
			return fmt.Errorf("--pipeline is required (pipeline name or ID)")
		}

		// Resolve pipeline name to ID
		pipelineID := historyPipeline
		pipelines, err := client.ListPipelinesWithStatus(orgID, nil)
		if err != nil {
			return fmt.Errorf("failed to list pipelines: %w", err)
		}
		for _, p := range pipelines {
			if p.Name == historyPipeline {
				pipelineID = p.PipelineID
				break
			}
		}

		if historyPage <= 0 {
			historyPage = 1
		}
		if historyLimit <= 0 {
			historyLimit = 30
		}

		result, err := client.ListPipelineRunsPaginated(orgID, pipelineID, historyPage, historyLimit)
		if err != nil {
			return fmt.Errorf("failed to load history: %w", err)
		}

		runs := result.Runs
		if historyStatus != "" && historyStatus != "all" {
			statuses := strings.Split(historyStatus, ",")
			var filtered []api.PipelineRun
			for _, r := range runs {
				for _, s := range statuses {
					if strings.EqualFold(r.Status, s) {
						filtered = append(filtered, r)
						break
					}
				}
			}
			runs = filtered
		}

		type runOutput struct {
			RunID       string `json:"runId"`
			Status      string `json:"status"`
			Trigger     string `json:"trigger"`
			StartTime   string `json:"startTime"`
			Duration    string `json:"duration"`
		}
		type historyOutput struct {
			Runs        []runOutput `json:"runs"`
			CurrentPage int         `json:"currentPage"`
			TotalPages  int         `json:"totalPages"`
			Total       int         `json:"total"`
		}

		output := historyOutput{
			CurrentPage: result.CurrentPage,
			TotalPages:  result.TotalPages,
			Total:       result.TotalRuns,
		}
		headers := []string{"RUN ID", "STATUS", "TRIGGER", "START TIME", "DURATION"}
		var rows [][]string

		for _, r := range runs {
			duration := "-"
			if !r.FinishTime.IsZero() && !r.StartTime.IsZero() {
				d := r.FinishTime.Sub(r.StartTime)
				if d < time.Minute {
					duration = d.Round(time.Second).String()
				} else if d < time.Hour {
					duration = fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
				} else {
					duration = fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
				}
			}
			startTime := r.StartTime.Format("2006-01-02 15:04:05")
			if r.StartTime.IsZero() {
				startTime = "-"
			}
			output.Runs = append(output.Runs, runOutput{
				RunID:     r.RunID,
				Status:    r.Status,
				Trigger:   r.TriggerMode,
				StartTime: startTime,
				Duration:  duration,
			})
			rows = append(rows, []string{r.RunID, r.Status, r.TriggerMode, startTime, duration})
		}

		return Output(output, headers, rows)
	},
}

func init() {
	pipelineHistoryCmd.Flags().StringVar(&historyPipeline, "pipeline", "", "Pipeline name or ID (required)")
	pipelineHistoryCmd.Flags().StringVar(&historyStatus, "status", "", "Filter by status (running,success,failed,all)")
	pipelineHistoryCmd.Flags().IntVar(&historyLimit, "limit", 30, "Number of results per page")
	pipelineHistoryCmd.Flags().IntVar(&historyPage, "page", 1, "Page number")
	pipelineHistoryCmd.MarkFlagRequired("pipeline")
}
```

**Step 2: Build and verify**

Run: `go build ./cmd/flo && ./flo pipeline history --help`
Expected: Shows history help with `--pipeline` marked required

**Step 3: Commit**

```bash
git add cmd/flo/cli/pipeline.go
git commit -m "feat: add pipeline history subcommand with pagination"
```

---

### Task 6: Add `flo pipeline run` subcommand

**Files:**
- Modify: `cmd/flo/cli/pipeline.go`

**Step 1: Add run subcommand with multi-repo branch support**

```go
func init() {
	pipelineCmd.AddCommand(pipelineListCmd)
	pipelineCmd.AddCommand(pipelineGroupsCmd)
	pipelineCmd.AddCommand(pipelineHistoryCmd)
	pipelineCmd.AddCommand(pipelineRunCmd)
}

var (
	runPipeline string
	runBranch   string
	runFollow   bool
)

var pipelineRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a pipeline",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		client, err := newClient(cfg)
		if err != nil {
			return err
		}
		orgID := GetOrgID(cfg)

		// Resolve pipeline name to ID
		pipelineID := runPipeline
		pipelines, err := client.ListPipelinesWithStatus(orgID, nil)
		if err != nil {
			return fmt.Errorf("failed to list pipelines: %w", err)
		}
		for _, p := range pipelines {
			if p.Name == runPipeline {
				pipelineID = p.PipelineID
				break
			}
		}

		// Get repo URLs for multi-repo branch support
		repoURLs := make(map[string]string)
		runInfo, err := client.GetLatestPipelineRunInfo(orgID, pipelineID)
		if err == nil && runInfo != nil {
			repoURLs = runInfo.RepositoryURLs
		}

		// Build runningBranchs param
		params := make(map[string]string)
		if runBranch != "" && len(repoURLs) > 0 {
			branchMap := make(map[string]string)
			if strings.Contains(runBranch, ":") {
				// Multi-repo format: repo1:branch1,repo2:branch2
				// Try to match repo URLs or repo names
				entries := strings.Split(runBranch, ",")
				for _, entry := range entries {
					parts := strings.SplitN(entry, ":", 2)
					if len(parts) == 2 {
						repoKey, branch := parts[0], parts[1]
						// Match by URL suffix
						matched := false
						for url := range repoURLs {
							if strings.Contains(url, repoKey) {
								branchMap[url] = branch
								matched = true
								break
							}
						}
						if !matched {
							// Store as-is, will be matched later
							branchMap[repoKey] = branch
						}
					}
				}
			} else {
				// Single branch for all repos
				for url := range repoURLs {
					branchMap[url] = runBranch
				}
			}
			if len(branchMap) > 0 {
				b, _ := json.Marshal(branchMap)
				params["runningBranchs"] = string(b)
			}
		}

		run, err := client.RunPipeline(orgID, pipelineID, params)
		if err != nil {
			return fmt.Errorf("failed to run pipeline: %w", err)
		}

		type runOutput struct {
			RunID      string `json:"runId"`
			PipelineID string `json:"pipelineId"`
			Status     string `json:"status"`
		}
		output := runOutput{
			RunID:      run.RunID,
			PipelineID: pipelineID,
			Status:     run.Status,
		}

		if outputFormat == "json" {
			return outputJSON(output)
		}
		fmt.Printf("Pipeline run started: %s (run ID: %s)\n", pipelineID, run.RunID)

		if runFollow {
			// Poll until terminal
			return followRun(client, orgID, pipelineID, run.RunID)
		}
		return nil
	},
}

func followRun(client *api.Client, orgID, pipelineID, runID string) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		run, err := client.GetPipelineRun(orgID, pipelineID, runID)
		if err != nil {
			return fmt.Errorf("failed to check run status: %w", err)
		}
		fmt.Printf("Status: %s\n", run.Status)
		switch strings.ToUpper(run.Status) {
		case "SUCCESS", "FAILED", "FAIL", "CANCELED", "CANCELLED":
			fmt.Printf("Pipeline finished: %s\n", run.Status)
			return nil
		}
		<-ticker.C
	}
}

func init() {
	pipelineRunCmd.Flags().StringVar(&runPipeline, "pipeline", "", "Pipeline name or ID (required)")
	pipelineRunCmd.Flags().StringVar(&runBranch, "branch", "", "Branch (e.g., main) or repo:branch map (e.g., repo1:main,repo2:develop)")
	pipelineRunCmd.Flags().BoolVarP(&runFollow, "follow", "f", false, "Follow run until completion")
	pipelineRunCmd.MarkFlagRequired("pipeline")
}
```

Note: need to add `"encoding/json"` to imports in `pipeline.go`.

**Step 2: Build and verify**

Run: `go build ./cmd/flo && ./flo pipeline run --help`
Expected: Shows run help

**Step 3: Commit**

```bash
git add cmd/flo/cli/pipeline.go
git commit -m "feat: add pipeline run subcommand with multi-repo branch support"
```

---

### Task 7: Add `flo pipeline logs` subcommand

**Files:**
- Modify: `cmd/flo/cli/pipeline.go`

**Step 1: Add logs subcommand with follow mode**

```go
func init() {
	pipelineCmd.AddCommand(pipelineListCmd)
	pipelineCmd.AddCommand(pipelineGroupsCmd)
	pipelineCmd.AddCommand(pipelineHistoryCmd)
	pipelineCmd.AddCommand(pipelineRunCmd)
	pipelineCmd.AddCommand(pipelineLogsCmd)
}

var (
	logsPipeline string
	logsRunID    string
	logsStage    string
	logsFollow   bool
)

var pipelineLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show pipeline run logs",
	Long:  "Show logs for a pipeline run. Without --stage, lists all stages with their status. Use --stage to show logs for a specific stage.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		client, err := newClient(cfg)
		if err != nil {
			return err
		}
		orgID := GetOrgID(cfg)

		pipelineID, err := resolvePipelineID(client, orgID, logsPipeline)
		if err != nil {
			return err
		}

		if logsFollow {
			return streamLogs(client, orgID, pipelineID, logsRunID, logsStage)
		}
		return showLogs(client, orgID, pipelineID, logsRunID, logsStage)
	},
}

func init() {
	pipelineLogsCmd.Flags().StringVar(&logsPipeline, "pipeline", "", "Pipeline name or ID (required)")
	pipelineLogsCmd.Flags().StringVar(&logsRunID, "run-id", "", "Pipeline run ID (required)")
	pipelineLogsCmd.Flags().StringVar(&logsStage, "stage", "", "Show logs for a specific stage")
	pipelineLogsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Stream logs until run completes")
	pipelineLogsCmd.MarkFlagRequired("pipeline")
	pipelineLogsCmd.MarkFlagRequired("run-id")
}
```

Key behavior of `showLogs`:
- Without `--stage`: prints a stage summary table (STAGE / STATUS / JOBS) so user can see available stage names
- With `--stage NAME`: prints full logs for that stage
- With invalid `--stage`: prints error with list of available stages

**Step 2: Build and verify**

Run: `go build ./cmd/flo && ./flo pipeline logs --help`
Expected: Shows logs help

**Step 3: Commit**

```bash
git add cmd/flo/cli/pipeline.go
git commit -m "feat: add pipeline logs subcommand with follow mode"
```

---

### Task 8: Add `flo pipeline stop` subcommand

**Files:**
- Modify: `cmd/flo/cli/pipeline.go`

**Step 1: Add stop subcommand**

```go
func init() {
	pipelineCmd.AddCommand(pipelineListCmd)
	pipelineCmd.AddCommand(pipelineGroupsCmd)
	pipelineCmd.AddCommand(pipelineHistoryCmd)
	pipelineCmd.AddCommand(pipelineRunCmd)
	pipelineCmd.AddCommand(pipelineLogsCmd)
	pipelineCmd.AddCommand(pipelineStopCmd)
}

var stopPipeline string
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
		orgID := GetOrgID(cfg)

		pipelineID, err := resolvePipelineID(client, orgID, stopPipeline)
		if err != nil {
			return err
		}

		if err := client.StopPipelineRun(orgID, pipelineID, stopRunID); err != nil {
			return fmt.Errorf("failed to stop pipeline run: %w", err)
		}

		if outputFormat == "json" {
			return Output(map[string]interface{}{
				"runId":      stopRunID,
				"pipelineId": pipelineID,
				"status":     "STOPPED",
				"message":    "Pipeline run stopped successfully",
			}, nil, nil)
		}

		fmt.Fprintf(os.Stdout, "Pipeline run %s stopped successfully\n", stopRunID)
		return nil
	},
}

func init() {
	pipelineStopCmd.Flags().StringVar(&stopPipeline, "pipeline", "", "Pipeline name or ID (required)")
	pipelineStopCmd.Flags().StringVar(&stopRunID, "run-id", "", "Pipeline run ID (required)")
	pipelineStopCmd.MarkFlagRequired("pipeline")
	pipelineStopCmd.MarkFlagRequired("run-id")
}
```

**Step 2: Build and verify**

Run: `go build ./cmd/flo && ./flo pipeline stop --help`
Expected: Shows stop help

**Step 3: Commit**

```bash
git add cmd/flo/cli/pipeline.go
git commit -m "feat: add pipeline stop subcommand"
```

---

### Task 9: End-to-end verification and cleanup

**Files:**
- Modify: `cmd/flo/cli/pipeline.go` (if needed for cleanup)

**Step 1: Full build**

Run: `go build ./cmd/flo`
Expected: Clean build

**Step 2: Verify all help outputs**

```bash
./flo --help
./flo pipeline --help
./flo pipeline list --help
./flo pipeline groups --help
./flo pipeline history --help
./flo pipeline run --help
./flo pipeline logs --help
./flo pipeline stop --help
```

**Step 3: Verify bare flo shows help**

Run: `./flo` (no args)
Expected: Shows help, not TUI

Run: `./flo tui`
Expected: TUI launches

**Step 4: Verify JSON output format**

```bash
./flo pipeline list -o json | head -20
```

**Step 5: Final commit**

```bash
git add -A
git commit -m "feat: complete CLI subcommands for all TUI features"
```
