package pages

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"flo/internal/api"
	"flo/internal/config"
	"flo/internal/editutil"
	"flo/internal/tui/components"
	"flo/internal/tui/types"
)

// DetailModel is the pipeline-definition detail page: a read-only YAML
// viewport plus external view/edit actions. YAML-mode pipelines can be
// edited and written back; classic-mode pipelines are view-only.
type DetailModel struct {
	pipelineID   string
	pipelineName string
	def          *api.PipelineDefinition
	loading      bool

	viewport components.ViewportModel
	spinner  components.SpinnerModel
	modal    components.ModalModel
	keys     DetailKeyMap

	width, height int
	config        *config.Config

	// Edit-flow state
	pendingAction  types.DetailPendingAction // auto-start edit after load
	pendingConfirm string                    // "" | "write-back" | "overwrite" | "save-as"
	tempPath       string                    // current edit session temp file
	editedContent  string                    // content read back from the editor
	baseUpdateTime int64                     // server UpdateTime the edited content is based on
	checkPending   bool                      // re-GET in flight as the optimistic check
	queuedSaveName string                    // freshest known name for the PUT
}

// DetailReloadRequestMsg asks the app to re-fetch the definition.
type DetailReloadRequestMsg struct{ PipelineID string }

// DetailCheckRequestMsg asks the app to re-fetch for the optimistic check.
type DetailCheckRequestMsg struct{ PipelineID string }

// DetailSaveRequestMsg asks the app to PUT the definition.
type DetailSaveRequestMsg struct {
	PipelineID string
	Name       string
	Content    string
}

func reloadRequestCmd(pipelineID string) tea.Cmd {
	return func() tea.Msg { return DetailReloadRequestMsg{PipelineID: pipelineID} }
}

func checkRequestCmd(pipelineID string) tea.Cmd {
	return func() tea.Msg { return DetailCheckRequestMsg{PipelineID: pipelineID} }
}

func saveRequestCmd(pipelineID, name, content string) tea.Cmd {
	return func() tea.Msg {
		return DetailSaveRequestMsg{PipelineID: pipelineID, Name: name, Content: content}
	}
}

// DetailKeyMap defines key bindings for the detail page.
type DetailKeyMap struct {
	Up, Down, PageUp, PageDown key.Binding
	Edit                       key.Binding
	ViewExternal               key.Binding
	SaveAs                     key.Binding
	Yank                       key.Binding
	Reload                     key.Binding
	Back, Quit                 key.Binding
}

// DefaultDetailKeyMap returns default detail page key bindings.
func DefaultDetailKeyMap() DetailKeyMap {
	return DetailKeyMap{
		Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PageUp:      key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "page up")),
		PageDown:    key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "page down")),
		Edit:        key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		ViewExternal: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view ext")),
		SaveAs:      key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save as")),
		Yank:        key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy")),
		Reload:      key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "reload")),
		Back:        key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "back")),
		Quit:        key.NewBinding(key.WithKeys("Q"), key.WithHelp("Q", "quit")),
	}
}

// NewDetailModel creates a detail page model.
func NewDetailModel(cfg *config.Config) DetailModel {
	return DetailModel{
		viewport: components.NewViewportModel("Pipeline"),
		spinner:  components.NewSpinnerModel(),
		modal:    components.NewModalModel(),
		keys:     DefaultDetailKeyMap(),
		config:   cfg,
	}
}

// Init implements tea.Model
func (m DetailModel) Init() tea.Cmd {
	return m.spinner.Init()
}

// --- Setters used by app.go ---

// SetPipeline targets a pipeline, resetting any previously loaded definition
// when the target changes.
func (m DetailModel) SetPipeline(id, name string) DetailModel {
	if m.pipelineID != id {
		m.def = nil
	}
	m.pipelineID = id
	m.pipelineName = name
	return m
}

// SetLoading toggles the loading state.
func (m DetailModel) SetLoading(v bool) DetailModel { m.loading = v; return m }

// QueueEditOnLoad makes the page auto-start an edit session once the
// definition loads (list-page 'e' entry).
func (m DetailModel) QueueEditOnLoad() DetailModel {
	m.pendingAction = types.DetailActionEdit
	return m
}

// GetPipelineID returns the targeted pipeline ID.
func (m DetailModel) GetPipelineID() string { return m.pipelineID }

// GetPipelineName returns the targeted pipeline name.
func (m DetailModel) GetPipelineName() string { return m.pipelineName }

// SetSize implements the shared page sizing convention.
func (m DetailModel) SetSize(width, height int) DetailModel {
	m.width, m.height = width, height
	m.viewport = m.viewport.SetSize(width, height-2) // header + footer
	m.modal = m.modal.SetSize(width, height)
	return m
}

// --- App-facing message entry points ---

// ApplyDefinition applies a fetched definition.
func (m DetailModel) ApplyDefinition(def *api.PipelineDefinition, err error) (DetailModel, tea.Cmd) {
	return m.applyLoaded(def, err)
}

// ApplySaveResult applies a write-back result.
func (m DetailModel) ApplySaveResult(err error) (DetailModel, tea.Cmd) {
	if err != nil {
		m.modal = components.NewErrorModal(err.Error() + "\n\nYour edits are kept at:\n" + m.tempPath).SetSize(m.width, m.height)
		return m, nil
	}
	os.Remove(m.tempPath)
	m.tempPath = ""
	m.modal = components.NewSuccessModal("Pipeline definition updated").SetSize(m.width, m.height)
	// Re-fetch: the server may normalize the YAML
	m.loading = true
	return m, reloadRequestCmd(m.pipelineID)
}

// Update implements tea.Model
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	// Modal messages arrive after the modal has closed itself
	switch msg := msg.(type) {
	case components.ModalConfirmMsg:
		return m.handleModalConfirm(msg)

	case components.ModalCancelMsg, components.ModalDismissMsg:
		m.modal = m.modal.Hide()
		if m.pendingConfirm == "write-back" || m.pendingConfirm == "overwrite" {
			// Keep the temp file; show where the edits live
			m.modal = components.NewInfoModal("Write-back cancelled.\nYour edits are kept at:\n" + m.tempPath).SetSize(m.width, m.height)
		}
		m.pendingConfirm = ""
		return m, nil
	}

	if m.modal.Visible {
		var cmd tea.Cmd
		m.modal, cmd = m.modal.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case types.PipelineDefinitionLoadedMsg:
		return m.applyLoaded(msg.Def, msg.Err)

	case types.PipelineSaveResultMsg:
		return m.ApplySaveResult(msg.Err)

	case types.ExecSessionFinishedMsg:
		return m.handleSessionFinished(msg)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return types.GoBackMsg{} }
		case key.Matches(msg, m.keys.Reload):
			if m.pipelineID != "" {
				m.loading = true
				return m, reloadRequestCmd(m.pipelineID)
			}
		case key.Matches(msg, m.keys.Yank):
			if m.def != nil {
				return m, types.CopyToClipboardCmd(m.def.FlowYAML)
			}
		case key.Matches(msg, m.keys.SaveAs):
			return m.saveAs()
		case key.Matches(msg, m.keys.ViewExternal):
			return m.viewExternal()
		case key.Matches(msg, m.keys.Edit):
			return m.startEdit()
		case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down),
			key.Matches(msg, m.keys.PageUp), key.Matches(msg, m.keys.PageDown):
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// applyLoaded applies a fetched definition. A fetch that was issued as the
// optimistic check (checkPending) is routed to the check completion instead.
func (m DetailModel) applyLoaded(def *api.PipelineDefinition, err error) (DetailModel, tea.Cmd) {
	m.loading = false

	if m.checkPending {
		m.checkPending = false
		return m.finishOptimisticCheck(def, err)
	}

	if err != nil {
		m.modal = components.NewErrorModal(err.Error()).SetSize(m.width, m.height)
		return m, nil
	}
	m.def = def
	m.baseUpdateTime = def.UpdateTime
	m.viewport = m.viewport.SetContent(def.FlowYAML)
	m.viewport = m.viewport.ScrollToTop()

	// Entry with pendingAction=edit: auto-start the edit session now
	if m.pendingAction == types.DetailActionEdit {
		m.pendingAction = types.DetailActionView
		return m.startEdit()
	}
	return m, nil
}

// viewExternal opens the YAML in the editor read-only (vi-family gets -R).
func (m DetailModel) viewExternal() (DetailModel, tea.Cmd) {
	if m.def == nil {
		return m, nil
	}
	path := editutil.TempFilePath(m.pipelineID)
	return m, types.OpenEditorSessionCmd(m.def.FlowYAML, m.config.GetEditor(), path, true)
}

// startEdit opens the YAML in the editor for writing. Classic-mode pipelines
// are refused: the update API only supports YAML-mode pipelines.
func (m DetailModel) startEdit() (DetailModel, tea.Cmd) {
	if m.def == nil {
		return m, nil
	}
	if !m.def.IsYAMLMode {
		m.modal = components.NewInfoModal(
			"This is a classic (form-mode) pipeline.\n\n"+
				"The update API only supports YAML-mode pipelines.\n"+
				"Use v (external view), s (save as), or y (copy) instead.").SetSize(m.width, m.height)
		return m, nil
	}
	m.tempPath = editutil.TempFilePath(m.pipelineID)
	return m, types.OpenEditorSessionCmd(m.def.FlowYAML, m.config.GetEditor(), m.tempPath, false)
}

// handleSessionFinished processes the editor/pager exit. Read-only viewer
// sessions are cleaned up; edit sessions enter the write-back flow.
func (m DetailModel) handleSessionFinished(msg types.ExecSessionFinishedMsg) (DetailModel, tea.Cmd) {
	// Not the running edit session (read-only viewer, or pager): clean up.
	if m.tempPath == "" || msg.Path != m.tempPath {
		os.Remove(msg.Path)
		return m, nil
	}

	if msg.Err != nil && msg.Content == "" {
		os.Remove(msg.Path)
		m.tempPath = ""
		m.modal = components.NewErrorModal(msg.Err.Error()).SetSize(m.width, m.height)
		return m, nil
	}

	if msg.Content == m.def.FlowYAML {
		os.Remove(msg.Path) // unchanged
		m.tempPath = ""
		return m, nil
	}

	m.editedContent = msg.Content
	body := "Content modified."
	if added, removed, ok := editutil.LineDiffStats(m.def.FlowYAML, msg.Content); ok {
		body = fmt.Sprintf("Content modified (+%d / -%d lines).", added, removed)
	}
	m.pendingConfirm = "write-back"
	m.modal = components.NewConfirmModal("Write back?", body+"\nWrite back to Yunxiao?")
	m.modal = m.modal.SetSize(m.width, m.height)
	return m, nil
}

// handleModalConfirm dispatches modal confirmations by pendingConfirm.
func (m DetailModel) handleModalConfirm(msg components.ModalConfirmMsg) (DetailModel, tea.Cmd) {
	switch m.pendingConfirm {
	case "write-back":
		m.pendingConfirm = ""
		m.modal = m.modal.Hide()
		if err := editutil.ValidateYAML(m.editedContent); err != nil {
			m.modal = components.NewErrorModal("Invalid YAML:\n" + err.Error() +
				"\n\nYour edits are kept at:\n" + m.tempPath).SetSize(m.width, m.height)
			return m, nil
		}
		m.queuedSaveName = m.def.Name
		m.checkPending = true
		return m, checkRequestCmd(m.pipelineID)

	case "overwrite":
		// Optimistic check saw a server change; user confirmed anyway.
		m.pendingConfirm = ""
		m.modal = m.modal.Hide()
		return m, saveRequestCmd(m.pipelineID, m.queuedSaveName, m.editedContent)

	case "save-as":
		m.pendingConfirm = ""
		m.modal = m.modal.Hide()
		return m.writeSaveAs(saveAsPath(m.def.Name, m.pipelineID))
	}
	m.modal = m.modal.Hide()
	return m, nil
}

// finishOptimisticCheck compares the server's current UpdateTime with the
// one the edited content is based on. Fails closed on fetch errors.
func (m DetailModel) finishOptimisticCheck(def *api.PipelineDefinition, err error) (DetailModel, tea.Cmd) {
	if err != nil {
		m.modal = components.NewErrorModal("Pre-write check failed (nothing was written):\n" + err.Error() +
			"\n\nYour edits are kept at:\n" + m.tempPath).SetSize(m.width, m.height)
		return m, nil
	}
	m.queuedSaveName = def.Name // never revert a concurrent rename

	if def.UpdateTime != m.baseUpdateTime {
		m.pendingConfirm = "overwrite"
		m.modal = components.NewConfirmModal("Server changed",
			"The pipeline was updated on the server after you started editing.\n"+
				"Overwrite the server version anyway?")
		m.modal = m.modal.SetSize(m.width, m.height)
		return m, nil
	}
	return m, saveRequestCmd(m.pipelineID, m.queuedSaveName, m.editedContent)
}

// saveAs writes the YAML to a local file, confirming on overwrite.
func (m DetailModel) saveAs() (DetailModel, tea.Cmd) {
	if m.def == nil {
		return m, nil
	}
	path := saveAsPath(m.def.Name, m.pipelineID)
	if _, err := os.Stat(path); err == nil {
		m.pendingConfirm = "save-as"
		m.modal = components.NewConfirmModal("Overwrite?", path+" already exists.\nOverwrite?")
		m.modal = m.modal.SetSize(m.width, m.height)
		return m, nil
	}
	return m.writeSaveAs(path)
}

func (m DetailModel) writeSaveAs(path string) (DetailModel, tea.Cmd) {
	if err := os.WriteFile(path, []byte(m.def.FlowYAML), 0644); err != nil {
		m.modal = components.NewErrorModal(err.Error()).SetSize(m.width, m.height)
		return m, nil
	}
	m.modal = components.NewSuccessModal("Saved to " + path).SetSize(m.width, m.height)
	return m, nil
}

// saveAsPath builds a cwd-relative file name from name and id.
func saveAsPath(name, id string) string {
	safe := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			return '-'
		}
		return r
	}, name)
	if safe == "" {
		safe = "pipeline"
	}
	return fmt.Sprintf("%s-%s.yml", safe, id)
}

// View implements tea.Model
func (m DetailModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	if m.modal.Visible {
		return m.modal.View()
	}
	if m.loading {
		return m.spinner.View() + " loading pipeline definition..."
	}
	if m.def == nil {
		return "No pipeline definition loaded"
	}

	header := m.renderHeader()
	body := m.viewport.View()
	footer := types.RenderHelpLine([]types.HelpItem{
		{Key: "e", Desc: "edit"}, {Key: "v", Desc: "view ext"},
		{Key: "s", Desc: "save as"}, {Key: "y", Desc: "copy"},
		{Key: "R", Desc: "reload"}, {Key: "q", Desc: "back"},
	})
	return header + "\n" + body + "\n" + footer
}

func (m DetailModel) renderHeader() string {
	badge := "YAML"
	badgeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981"))
	hint := ""
	if !m.def.IsYAMLMode {
		badge = "Classic"
		badgeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6B7280"))
		hint = " · server-generated YAML, reference only"
	}
	name := m.def.Name
	if name == "" {
		name = m.pipelineName
	}
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	return nameStyle.Render(name) + "  " + badgeStyle.Render("["+badge+"]") +
		metaStyle.Render(fmt.Sprintf("  updated %s  ID %s%s", formatMillis(m.def.UpdateTime), m.def.PipelineID, hint))
}

func formatMillis(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return time.Unix(ms/1000, 0).Format("01-02 15:04")
}
