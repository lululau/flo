package types

import (
	"strings"

	"flowt/internal/api"
	"github.com/charmbracelet/lipgloss"
)

// PageType represents different pages in the application
type PageType int

const (
	PagePipelinesList PageType = iota
	PageGroupsList
	PageHistory
	PageLogs
)

// String returns the string representation of PageType
func (p PageType) String() string {
	switch p {
	case PagePipelinesList:
		return "Pipelines"
	case PageGroupsList:
		return "Groups"
	case PageHistory:
		return "Run History"
	case PageLogs:
		return "Logs"
	default:
		return "Unknown"
	}
}

// ViewMode represents different view modes for the pipelines page
type ViewMode int

const (
	ViewModeAllPipelines ViewMode = iota
	ViewModePipelinesInGroup
)

// FilterMode represents different filter modes
type FilterMode int

const (
	FilterModeAll FilterMode = iota
	FilterModeRunningWaiting
	FilterModeBookmarked
)

// ModalType represents different modal types
type ModalType int

const (
	ModalTypeInfo ModalType = iota
	ModalTypeError
	ModalTypeConfirm
	ModalTypeInput
)

// --- Navigation Messages ---

// NavigateMsg requests navigation to a specific page
type NavigateMsg struct {
	Page PageType
	Data interface{} // Optional data to pass to the page
}

// GoBackMsg requests navigation to the previous page
type GoBackMsg struct{}

// --- Data Messages ---

// PipelinesLoadedMsg is sent when pipelines are loaded
type PipelinesLoadedMsg struct {
	Pipelines   []PipelineItem
	CurrentPage int
	TotalPages  int
	IsComplete  bool
}

// GroupsLoadedMsg is sent when groups are loaded
type GroupsLoadedMsg struct {
	Groups []GroupItem
}

// HistoryLoadedMsg is sent when run history is loaded
type HistoryLoadedMsg struct {
	Runs       []RunItem
	TotalRuns  int
	TotalPages int
}

// LogsLoadedMsg is sent when logs are loaded
type LogsLoadedMsg struct {
	Content     string
	Status      string
	IsComplete  bool
	CurrentJob  int
	TotalJobs   int
}

// RunStartedMsg is sent when a pipeline run is started
type RunStartedMsg struct {
	RunID      string
	PipelineID string
	Branch     string
}

// RunStoppedMsg is sent when a pipeline run is stopped
type RunStoppedMsg struct {
	RunID string
}

// --- Error Messages ---

// ErrorMsg represents an error that occurred
type ErrorMsg struct {
	Err error
}

// --- UI State Messages ---

// TickMsg is sent for timed updates (e.g., log refresh)
type TickMsg struct{}

// RefreshMsg requests a data refresh
type RefreshMsg struct{}

// --- Item Types ---

// PipelineItem represents a pipeline in the list
type PipelineItem struct {
	ID            string
	Name          string
	Status        string
	LastRunStatus string
	IsBookmarked  bool
}

// GroupItem represents a pipeline group
type GroupItem struct {
	ID   string
	Name string
}

// RunItem represents a pipeline run in the history
type RunItem struct {
	RunID       string
	PipelineID  string
	Status      string
	TriggerMode string
	StartTime   string
	FinishTime  string
	Duration    string
}

// --- Modal Messages ---

// ShowModalMsg requests showing a modal
type ShowModalMsg struct {
	Title   string
	Content string
	Type    ModalType
}

// HideModalMsg requests hiding the modal
type HideModalMsg struct{}

// ModalConfirmMsg is sent when the user confirms a modal action
type ModalConfirmMsg struct {
	Data interface{}
}

// ModalCancelMsg is sent when the user cancels a modal
type ModalCancelMsg struct{}

// --- Search Messages ---

// SearchQueryMsg is sent when a search query is submitted
type SearchQueryMsg struct {
	Query string
}

// SearchExitMsg is sent when search mode is exited
type SearchExitMsg struct{}

// SearchNextMsg requests moving to the next search match
type SearchNextMsg struct{}

// SearchPrevMsg requests moving to the previous search match
type SearchPrevMsg struct{}

// --- Clipboard Messages ---

// CopiedMsg is sent when something is copied to clipboard
type CopiedMsg struct {
	Content string
}

// --- External Editor/Pager Messages ---

// OpenEditorMsg requests opening content in an editor
type OpenEditorMsg struct {
	Content string
}

// OpenPagerMsg requests opening content in a pager
type OpenPagerMsg struct {
	Content string
}

// EditorClosedMsg is sent when the editor is closed
type EditorClosedMsg struct{}

// PagerClosedMsg is sent when the pager is closed
type PagerClosedMsg struct{}

// --- API Data Conversion Messages ---

// PipelinesAPILoadedMsg contains raw API response for pipelines
type PipelinesAPILoadedMsg struct {
	Pipelines   []api.Pipeline
	CurrentPage int
	TotalPages  int
	IsComplete  bool
}

// GroupsAPILoadedMsg contains raw API response for groups
type GroupsAPILoadedMsg struct {
	Groups []api.PipelineGroup
}

// HistoryAPILoadedMsg contains raw API response for run history
type HistoryAPILoadedMsg struct {
	Runs        []api.PipelineRun
	CurrentPage int
	TotalPages  int
	TotalRuns   int
	PerPage     int
}

// LogsAPILoadedMsg contains raw API response for logs
type LogsAPILoadedMsg struct {
	Details     *api.PipelineRunDetails
	LogContent  string
	Status      string
	CurrentJob  int
	TotalJobs   int
	IsComplete  bool
	StreamState *LogStreamState // Optional stream state for incremental loading
}

// RunAPIStartedMsg contains response from starting a pipeline run
type RunAPIStartedMsg struct {
	RunID string
	Error error
}

// RunAPIStoppedMsg contains response from stopping a pipeline run
type RunAPIStoppedMsg struct {
	Error error
}

// --- Branch Selection Messages ---

// BranchSelectedMsg is sent when the user selects a branch
type BranchSelectedMsg struct {
	Branch        string
	RepositoryURL string
}

// LoadBranchInfoMsg requests loading branch info for a pipeline
type LoadBranchInfoMsg struct {
	PipelineID string
}

// BranchInfoLoadedMsg is sent when branch info is loaded
type BranchInfoLoadedMsg struct {
	DefaultBranch  string
	RepositoryURLs map[string]string
}

// --- Progressive Loading Messages ---

// LogsProgressMsg is sent during progressive log loading
type LogsProgressMsg struct {
	Content    string
	Status     string
	CurrentJob int
	TotalJobs  int
	IsComplete bool
	AppendMode bool // If true, append to existing content
}

// --- Incremental Log Loading ---

// StepLogState tracks the log position for a single step within a job
type StepLogState struct {
	StepIndex int   // Step index from API
	BuildId   int64 // Build ID from API
	LastPos   int64 // Last position of log content (for incremental fetching)
	HasMore   bool  // Whether there are more logs available
}

// JobLogState tracks all step states for a single job
type JobLogState struct {
	JobId       int64                   // Job ID
	JobName     string                  // Job name for display
	StageIndex  string                  // Parent stage index
	StageName   string                  // Parent stage name
	Steps       map[int]*StepLogState   // Map of stepIndex -> StepLogState
	IsComplete  bool                    // Whether job has finished
}

// LogStreamState manages incremental log fetching state for a pipeline run
type LogStreamState struct {
	PipelineID   string
	RunID        string
	Jobs         map[int64]*JobLogState // Map of jobId -> JobLogState
	Initialized  bool                   // Whether initial state has been fetched
}

// NewLogStreamState creates a new LogStreamState
func NewLogStreamState(pipelineID, runID string) *LogStreamState {
	return &LogStreamState{
		PipelineID:  pipelineID,
		RunID:       runID,
		Jobs:        make(map[int64]*JobLogState),
		Initialized: false,
	}
}

// LogsIncrementalLoadedMsg is sent when incremental logs are loaded
type LogsIncrementalLoadedMsg struct {
	IncrementalContent string           // Only the new log content (to append)
	Status             string           // Current pipeline status
	StreamState        *LogStreamState  // Updated stream state
	HasNewContent      bool             // Whether there's new content to display
}

// PipelinesProgressMsg is sent during progressive pipeline loading
type PipelinesProgressMsg struct {
	Pipelines   []api.Pipeline
	CurrentPage int
	TotalPages  int
	IsComplete  bool
}

// --- Context/State for Navigation ---

// PipelineContext contains context data for pipeline-related navigation
type PipelineContext struct {
	PipelineID   string
	PipelineName string
	GroupID      string
	GroupName    string
}

// RunContext contains context data for run-related navigation
type RunContext struct {
	PipelineID   string
	PipelineName string
	RunID        string
	Status       string
	IsNewRun     bool
}

// --- Filter State Messages ---

// FilterChangedMsg is sent when filter state changes
type FilterChangedMsg struct {
	FilterMode FilterMode
}

// BookmarkToggledMsg is sent when a bookmark is toggled
type BookmarkToggledMsg struct {
	PipelineName string
	IsBookmarked bool
}

// --- View Mode Messages ---

// ViewModeChangedMsg is sent when view mode changes
type ViewModeChangedMsg struct {
	ViewMode  ViewMode
	GroupID   string
	GroupName string
}

// --- Window Size ---

// WindowSizeMsg is sent when the window size changes
type WindowSizeMsg struct {
	Width  int
	Height int
}

// --- Focus Messages ---

// FocusSearchMsg requests focus on the search input
type FocusSearchMsg struct{}

// BlurSearchMsg requests blurring the search input
type BlurSearchMsg struct{}

// --- Loading State ---

// LoadingMsg indicates loading state
type LoadingMsg struct {
	Message string
}

// LoadingCompleteMsg indicates loading is complete
type LoadingCompleteMsg struct{}

// --- Helper Functions ---

// HelpItem represents a single help item with key and description
type HelpItem struct {
	Key  string
	Desc string
}

// RenderHelpLine renders a help line with styled keys
// Keys are displayed in orange, descriptions in gray, separated by " | "
func RenderHelpLine(items []HelpItem) string {
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")) // Orange
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")) // Gray
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))  // Gray

	var parts []string
	for _, item := range items {
		part := keyStyle.Render(item.Key) + descStyle.Render(": "+item.Desc)
		parts = append(parts, part)
	}

	return strings.Join(parts, sepStyle.Render(" | "))
}

