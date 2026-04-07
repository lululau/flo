package tui

import (
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"flo/internal/api"
	"flo/internal/config"
	"flo/internal/notify"
	"flo/internal/tui/components"
	"flo/internal/tui/pages"
	"flo/internal/tui/types"
)

// Model is the root application model
type Model struct {
	// Page models
	pipelinesPage pages.PipelinesModel
	groupsPage    pages.GroupsModel
	historyPage   pages.HistoryModel
	logsPage      pages.LogsModel

	// Shared components
	modal   components.ModalModel
	spinner components.SpinnerModel

	// Application state
	currentPage    types.PageType
	previousPages  []pageState
	config         *config.Config
	client         *api.Client
	organizationID string

	// Window size
	width  int
	height int

	// Global state
	loading      bool
	errorMsg     string
	autoRefresh  bool
	refreshTimer *time.Ticker

	// Captures the branch used when starting a pipeline run
	lastRunBranch string

	// Notification
	notifier *notify.Notifier

	// Key bindings
	keys GlobalKeyMap
}

// pageState stores the state when navigating to a new page
type pageState struct {
	page types.PageType
	data interface{}
}

// GlobalKeyMap defines global key bindings
type GlobalKeyMap struct {
	Quit key.Binding
}

// DefaultGlobalKeyMap returns default global key bindings
func DefaultGlobalKeyMap() GlobalKeyMap {
	return GlobalKeyMap{
		Quit: key.NewBinding(
			key.WithKeys("Q"),
			key.WithHelp("Q", "quit"),
		),
	}
}

// New creates a new application model
func New(cfg *config.Config, client *api.Client) Model {
	m := Model{
		pipelinesPage:  pages.NewPipelinesModel(cfg),
		groupsPage:     pages.NewGroupsModel(),
		historyPage:    pages.NewHistoryModel(),
		logsPage:       pages.NewLogsModel(cfg),
		modal:          components.NewModalModel(),
		spinner:        components.NewSpinnerModel(),
		currentPage:    types.PagePipelinesList,
		config:         cfg,
		client:         client,
		organizationID: cfg.OrganizationID,
		keys:           DefaultGlobalKeyMap(),
	}

	// Initialize notifier if configured
	if cfg.NotifyCommand != "" {
		notifier, err := notify.New(client, cfg.NotifyCommand)
		if err != nil {
			log.Printf("failed to create notifier: %v", err)
		} else if notifier != nil {
			m.notifier = notifier
		}
	}

	return m
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.spinner.Init(),
		LoadPipelinesCmd(m.client, m.organizationID),
	)
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.updatePageSizes()

	case tea.KeyMsg:
		// Handle global quit
		if key.Matches(msg, m.keys.Quit) && !m.modal.Visible {
			return m, tea.Quit
		}

	case types.ErrorMsg:
		m.errorMsg = msg.Err.Error()
		m.loading = false
		m.modal = components.NewErrorModal(msg.Err.Error())
		m.modal = m.modal.SetSize(m.width, m.height)

	case components.ModalDismissMsg, components.ModalCancelMsg:
		m.modal = m.modal.Hide()
		m.errorMsg = ""

	case types.NavigateMsg:
		return m.navigateTo(msg.Page, msg.Data)

	case types.GoBackMsg:
		return m.navigateBack()

	case types.ViewModeChangedMsg:
		// Handle group selection - save current page to previousPages before navigating
		m.previousPages = append(m.previousPages, pageState{
			page: m.currentPage,
			data: m.getPageData(),
		})
		m.pipelinesPage = m.pipelinesPage.SetViewMode(msg.ViewMode, msg.GroupID, msg.GroupName)
		m.pipelinesPage = m.pipelinesPage.SetLoading(true)
		m.currentPage = types.PagePipelinesList

		return m, LoadGroupPipelinesCmd(m.client, m.organizationID, msg.GroupID)

	case types.ReloadPipelinesWithFilterMsg:
		// Handle filter change - reload from server with status filter
		var statusList []string
		if msg.FilterMode == types.FilterModeRunningWaiting {
			statusList = []string{"RUNNING", "WAITING"}
		}

		if msg.ViewMode == types.ViewModePipelinesInGroup && msg.GroupID != "" {
			return m, LoadGroupPipelinesWithStatusCmd(m.client, m.organizationID, msg.GroupID, statusList)
		}
		return m, LoadPipelinesWithStatusCmd(m.client, m.organizationID, statusList)

	case types.LoadBranchInfoMsg:
		// Load branch info for the run dialog
		return m, LoadBranchInfoCmd(m.client, m.organizationID, msg.PipelineID)

	case types.BranchSelectedMsg:
		// Handle branch selection for running pipeline.
		// UX requirement: immediately enter logs page (Starting...) after the user confirms,
		// then load real logs once the runID is returned.
		var pipelineID, pipelineName string
		switch m.currentPage {
		case types.PagePipelinesList:
			if p := m.pipelinesPage.SelectedPipeline(); p != nil {
				pipelineID = p.PipelineID
				pipelineName = p.Name
			}
		case types.PageHistory:
			pipelineID = m.historyPage.GetPipelineID()
			pipelineName = m.historyPage.GetPipelineName()
		case types.PageLogs:
			pipelineID = m.logsPage.GetPipelineID()
			pipelineName = m.logsPage.GetPipelineName()
		}

		if pipelineID != "" {
			m.lastRunBranch = msg.Branch

			// Save current page to previousPages before navigating to logs
			if m.currentPage != types.PageLogs {
				m.previousPages = append(m.previousPages, pageState{
					page: m.currentPage,
					data: m.getPageData(),
				})
			}
			m.logsPage = m.logsPage.SetStarting(pipelineID, pipelineName)
			m.logsPage = m.logsPage.SetAutoRefresh(false) // enable once runID is known
			m.currentPage = types.PageLogs
			m = m.updatePageSizes()
			return m, RunPipelineCmd(m.client, m.organizationID, pipelineID, msg.Branch, msg.RepositoryURLs)
		}

	case types.RunAPIStartedMsg:
		m.loading = false
		if msg.Error != nil {
			m.modal = components.NewErrorModal(msg.Error.Error())
			m.modal = m.modal.SetSize(m.width, m.height)
		} else {
			// Enter logs for the new run (tabs UI).
			pipelineID := m.logsPage.GetPipelineID()
			pipelineName := m.logsPage.GetPipelineName()
			if pipelineID == "" {
				// Fallback if we weren't already on logs page
				switch m.currentPage {
				case types.PagePipelinesList:
					if p := m.pipelinesPage.SelectedPipeline(); p != nil {
						pipelineID = p.PipelineID
						pipelineName = p.Name
					}
				case types.PageHistory:
					pipelineID = m.historyPage.GetPipelineID()
					pipelineName = m.historyPage.GetPipelineName()
				}
			}

			// Save current page to previousPages if not already on logs page
			// (BranchSelectedMsg already saves the state when navigating to logs)
			if m.currentPage != types.PageLogs {
				m.previousPages = append(m.previousPages, pageState{
					page: m.currentPage,
					data: m.getPageData(),
				})
			}

			m.logsPage = m.logsPage.SetRun(pipelineID, pipelineName, msg.RunID, "RUNNING", true)
			m.logsPage = m.logsPage.SetLoading(true)
			m.logsPage = m.logsPage.SetAutoRefresh(true)

			// Track this run for background notification
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

			m.currentPage = types.PageLogs
			m = m.updatePageSizes()

			return m, tea.Batch(
				LoadRunStageTabsCmd(m.client, m.organizationID, pipelineID, pipelineName, msg.RunID),
				AutoRefreshTickCmd(3*time.Second),
			)
		}

	case types.RunAPIStoppedMsg:
		if msg.Error != nil {
			m.modal = components.NewErrorModal(msg.Error.Error())
			m.modal = m.modal.SetSize(m.width, m.height)
		} else {
			// Refresh current view
			switch m.currentPage {
			case types.PageHistory:
				perPage := m.config.GetPerPage()
				cmds = append(cmds, LoadHistoryCmd(m.client, m.organizationID,
					m.historyPage.GetPipelineID(), m.historyPage.GetCurrentPage(), perPage))
			case types.PageLogs:
				cmds = append(cmds, LoadRunStageTabsCmd(m.client, m.organizationID,
					m.logsPage.GetPipelineID(), m.logsPage.GetPipelineName(), m.logsPage.GetRunID()))
			}
		}

	case pages.StopRunRequestMsg:
		return m, StopPipelineRunCmd(m.client, m.organizationID, msg.PipelineID, msg.RunID)

	case pages.HistoryPageChangeMsg:
		m.historyPage = m.historyPage.SetLoading(true)
		perPage := m.config.GetPerPage()
		return m, LoadHistoryCmd(m.client, m.organizationID,
			m.historyPage.GetPipelineID(), msg.Page, perPage)

	case pages.LogsTabsRefreshMsg:
		// Running pipelines: periodic refresh (active stage + auto-advance).
		if msg.TabsData != nil {
			return m, RefreshRunStageTabsCmd(m.client, m.organizationID, msg.TabsData)
		}

	case pages.LogsTabLoadMsg:
		// User switched tabs or pressed manual refresh: load/refresh that tab.
		if msg.TabsData != nil {
			return m, LoadRunStageTabCmd(m.client, m.organizationID, msg.TabsData, msg.TabIndex)
		}

	case types.TickMsg:
		// Schedule next tick if logs page is running
		// The actual refresh is triggered by LogsStagesRefreshMsg from logs page
		if m.currentPage == types.PageLogs && m.logsPage.IsRunning() && m.logsPage.GetRunID() != "" {
			cmds = append(cmds, AutoRefreshTickCmd(3*time.Second))
		}
	}

	// Handle modal updates
	if m.modal.Visible {
		var cmd tea.Cmd
		m.modal, cmd = m.modal.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// Update current page
	switch m.currentPage {
	case types.PagePipelinesList:
		var cmd tea.Cmd
		m.pipelinesPage, cmd = m.pipelinesPage.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case types.PageGroupsList:
		var cmd tea.Cmd
		m.groupsPage, cmd = m.groupsPage.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case types.PageHistory:
		var cmd tea.Cmd
		m.historyPage, cmd = m.historyPage.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case types.PageLogs:
		var cmd tea.Cmd
		m.logsPage, cmd = m.logsPage.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var view string

	// Render current page
	switch m.currentPage {
	case types.PagePipelinesList:
		view = m.pipelinesPage.View()
	case types.PageGroupsList:
		view = m.groupsPage.View()
	case types.PageHistory:
		view = m.historyPage.View()
	case types.PageLogs:
		view = m.logsPage.View()
	default:
		view = "Unknown page"
	}

	// Overlay modal if visible
	if m.modal.Visible {
		view = m.modal.View()
	}

	return view
}

// navigateTo navigates to a specific page
func (m Model) navigateTo(page types.PageType, data interface{}) (Model, tea.Cmd) {
	// Save current state
	m.previousPages = append(m.previousPages, pageState{
		page: m.currentPage,
		data: m.getPageData(),
	})

	m.currentPage = page
	var cmd tea.Cmd

	switch page {
	case types.PagePipelinesList:
		m.pipelinesPage = m.pipelinesPage.SetLoading(true)
		cmd = LoadPipelinesCmd(m.client, m.organizationID)

	case types.PageGroupsList:
		m.groupsPage = m.groupsPage.SetLoading(true)
		cmd = LoadGroupsCmd(m.client, m.organizationID)

	case types.PageHistory:
		if ctx, ok := data.(types.PipelineContext); ok {
			m.historyPage = m.historyPage.SetPipeline(ctx.PipelineID, ctx.PipelineName, ctx.GroupID, ctx.GroupName)
			m.historyPage = m.historyPage.SetLoading(true)
			perPage := m.config.GetPerPage()
			cmd = LoadHistoryCmd(m.client, m.organizationID, ctx.PipelineID, 1, perPage)
		}

	case types.PageLogs:
		if ctx, ok := data.(types.RunContext); ok {
			m.logsPage = m.logsPage.SetRun(ctx.PipelineID, ctx.PipelineName, ctx.RunID, ctx.Status, ctx.IsNewRun)
			m.logsPage = m.logsPage.SetLoading(true)

			// Enable auto-refresh for running pipelines
			status := strings.ToUpper(ctx.Status)
			if status == "RUNNING" || status == "QUEUED" || status == "INIT" {
				m.logsPage = m.logsPage.SetAutoRefresh(true)
				// Use stage-tabs data loading with incremental refresh
				cmd = tea.Batch(
					LoadRunStageTabsCmd(m.client, m.organizationID, ctx.PipelineID, ctx.PipelineName, ctx.RunID),
					AutoRefreshTickCmd(3*time.Second),
				)
			} else {
				// For completed pipelines, load stage-tabs data once.
				cmd = LoadRunStageTabsCmd(m.client, m.organizationID, ctx.PipelineID, ctx.PipelineName, ctx.RunID)
			}
		}
	}

	m = m.updatePageSizes()
	return m, cmd
}

// navigateBack navigates to the previous page
func (m Model) navigateBack() (Model, tea.Cmd) {
	if len(m.previousPages) == 0 {
		// If on groups page and no previous, go to pipelines
		if m.currentPage == types.PageGroupsList {
			m.currentPage = types.PagePipelinesList
			m.pipelinesPage = m.pipelinesPage.SetViewMode(types.ViewModeAllPipelines, "", "")
			m.pipelinesPage = m.pipelinesPage.SetLoading(true)
			return m, LoadPipelinesCmd(m.client, m.organizationID)
		}
		// If on pipelines in group mode, go back to all pipelines
		return m, tea.Quit
	}

	// Pop the previous state
	lastIdx := len(m.previousPages) - 1
	prev := m.previousPages[lastIdx]
	m.previousPages = m.previousPages[:lastIdx]

	m.currentPage = prev.page

	// Restore page state if needed
	m = m.updatePageSizes()

	return m, nil
}

// getPageData returns the current page's state data
func (m Model) getPageData() interface{} {
	switch m.currentPage {
	case types.PageHistory:
		return types.PipelineContext{
			PipelineID:   m.historyPage.GetPipelineID(),
			PipelineName: m.historyPage.GetPipelineName(),
		}
	case types.PageLogs:
		return types.RunContext{
			PipelineID:   m.logsPage.GetPipelineID(),
			PipelineName: "",
			RunID:        m.logsPage.GetRunID(),
			Status:       m.logsPage.GetStatus(),
		}
	}
	return nil
}

// updatePageSizes updates all page sizes
func (m Model) updatePageSizes() Model {
	m.pipelinesPage = m.pipelinesPage.SetSize(m.width, m.height)
	m.groupsPage = m.groupsPage.SetSize(m.width, m.height)
	m.historyPage = m.historyPage.SetSize(m.width, m.height)
	m.logsPage = m.logsPage.SetSize(m.width, m.height)
	m.modal = m.modal.SetSize(m.width, m.height)
	return m
}

// Styling helper for the app
func (m Model) titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))
}
