package pages

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flo/internal/api"
	"flo/internal/config"
	"flo/internal/tui/components"
	"flo/internal/tui/types"
)

func newTestDetailModel() DetailModel {
	return NewDetailModel(&config.Config{}).
		SetPipeline("123", "deploy-service").
		SetSize(100, 30)
}

func yamlDef() *api.PipelineDefinition {
	return &api.PipelineDefinition{
		PipelineID: "123", Name: "deploy-service",
		IsYAMLMode: true, FlowYAML: "stages:\n  build:\n", UpdateTime: 1000,
	}
}

func classicDef() *api.PipelineDefinition {
	d := yamlDef()
	d.IsYAMLMode = false
	d.Name = "classic-pipe"
	return d
}

func TestApplyLoadedSetsViewportAndBaseTime(t *testing.T) {
	m := newTestDetailModel()
	m, cmd := m.applyLoaded(yamlDef(), nil)
	if m.loading {
		t.Error("loading should be cleared")
	}
	if m.def == nil || m.baseUpdateTime != 1000 {
		t.Errorf("def/baseUpdateTime not applied: %+v", m.def)
	}
	if got := m.viewport.GetContent(); got != "stages:\n  build:\n" {
		t.Errorf("viewport content = %q", got)
	}
	if cmd != nil {
		t.Error("no command expected for plain load")
	}
}

func TestApplyLoadedErrorShowsErrorModal(t *testing.T) {
	m := newTestDetailModel()
	m, _ = m.applyLoaded(nil, fmt.Errorf("boom"))
	if !m.modal.Visible {
		t.Error("error modal should be visible")
	}
	if m.def != nil {
		t.Error("def should stay nil on error")
	}
}

func TestClassicModeEditRefused(t *testing.T) {
	m := newTestDetailModel()
	m, cmd := m.applyLoaded(classicDef(), nil)
	m, cmd = m.startEdit()
	if !m.modal.Visible {
		t.Fatal("expected info modal for classic pipeline")
	}
	if v := m.View(); !strings.Contains(v, "classic") {
		t.Errorf("modal should explain classic mode: %q", v)
	}
	if cmd != nil {
		t.Error("no edit session should start for classic mode")
	}
}

func TestQueueEditOnLoadStartsSession(t *testing.T) {
	m := newTestDetailModel().QueueEditOnLoad()
	m, cmd := m.applyLoaded(yamlDef(), nil)
	if cmd == nil {
		t.Fatal("edit session command expected after load")
	}
	if m.pendingAction != types.DetailActionView {
		t.Error("pendingAction should be consumed")
	}
	os.Remove(m.tempPath)
}

func TestSessionUnchangedRemovesFile(t *testing.T) {
	m := newTestDetailModel()
	m, _ = m.applyLoaded(yamlDef(), nil)
	m.tempPath = filepath.Join(t.TempDir(), "e.yml")
	os.WriteFile(m.tempPath, []byte(m.def.FlowYAML), 0644)

	m, cmd := m.handleSessionFinished(types.ExecSessionFinishedMsg{
		Program: "editor", Path: m.tempPath, Content: m.def.FlowYAML,
	})
	if _, err := os.Stat(m.tempPath); !os.IsNotExist(err) {
		t.Error("temp file should be removed when content unchanged")
	}
	if m.pendingConfirm != "" {
		t.Errorf("expected no confirm, got %q", m.pendingConfirm)
	}
	if cmd != nil {
		t.Error("no command expected")
	}
}

func TestSessionChangedOpensWriteBackConfirm(t *testing.T) {
	m := newTestDetailModel()
	m, _ = m.applyLoaded(yamlDef(), nil)
	m.tempPath = "/tmp/flo_pipeline_123_keep.yml"

	m, _ = m.handleSessionFinished(types.ExecSessionFinishedMsg{
		Program: "editor", Path: m.tempPath, Content: "stages: []\n",
	})
	if m.pendingConfirm != "write-back" {
		t.Errorf("pendingConfirm = %q, want write-back", m.pendingConfirm)
	}
	if !m.modal.Visible {
		t.Error("confirm modal should be visible")
	}
	if v := m.View(); !strings.Contains(v, "+1") {
		t.Errorf("diff stats should be shown: %q", v)
	}
}

func TestConfirmInvalidYAMLErrorsBeforeCheck(t *testing.T) {
	m := newTestDetailModel()
	m, _ = m.applyLoaded(yamlDef(), nil)
	m.editedContent = "stages: [\n"
	m.tempPath = "/tmp/flo_pipeline_123_keep.yml"
	m.pendingConfirm = "write-back"

	m, cmd := m.handleModalConfirm(components.ModalConfirmMsg{})
	if !m.modal.Visible || !strings.Contains(m.View(), "Invalid YAML") {
		t.Error("expected invalid-YAML error modal")
	}
	if m.checkPending {
		t.Error("check must not be requested for invalid YAML")
	}
	if cmd != nil {
		t.Error("no command expected for invalid YAML")
	}
}

func TestConfirmValidEmitsCheckRequest(t *testing.T) {
	m := newTestDetailModel()
	m, _ = m.applyLoaded(yamlDef(), nil)
	m.editedContent = "stages: []\n"
	m.tempPath = "/tmp/flo_pipeline_123_keep.yml"
	m.pendingConfirm = "write-back"

	m, cmd := m.handleModalConfirm(components.ModalConfirmMsg{})
	if !m.checkPending {
		t.Error("checkPending should be set")
	}
	msg, ok := cmd().(DetailCheckRequestMsg)
	if !ok || msg.PipelineID != "123" {
		t.Errorf("expected DetailCheckRequestMsg, got %#v", msg)
	}
}

func TestOptimisticCheckUnchangedEmitsSave(t *testing.T) {
	m := newTestDetailModel()
	m, _ = m.applyLoaded(yamlDef(), nil) // baseUpdateTime = 1000
	m.editedContent = "stages: []\n"
	m.tempPath = "/tmp/flo_pipeline_123_keep.yml"

	m, cmd := m.finishOptimisticCheck(yamlDef(), nil)
	msg, ok := cmd().(DetailSaveRequestMsg)
	if !ok {
		t.Fatalf("expected DetailSaveRequestMsg, got %#v", msg)
	}
	if msg.Name != "deploy-service" || msg.Content != "stages: []\n" {
		t.Errorf("unexpected save request: %+v", msg)
	}
}

func TestOptimisticCheckChangedOpensOverwriteConfirm(t *testing.T) {
	m := newTestDetailModel()
	m, _ = m.applyLoaded(yamlDef(), nil) // baseUpdateTime = 1000
	m.tempPath = "/tmp/flo_pipeline_123_keep.yml"

	changed := yamlDef()
	changed.UpdateTime = 2000
	changed.Name = "renamed-elsewhere"
	m, cmd := m.finishOptimisticCheck(changed, nil)
	if m.pendingConfirm != "overwrite" {
		t.Fatalf("pendingConfirm = %q, want overwrite", m.pendingConfirm)
	}
	if cmd != nil {
		t.Error("no save should be emitted before the overwrite confirm")
	}
	if !strings.Contains(m.View(), "Overwrite") {
		t.Error("overwrite confirm modal expected")
	}
}

func TestOverwriteConfirmUsesFreshName(t *testing.T) {
	m := newTestDetailModel()
	m, _ = m.applyLoaded(yamlDef(), nil)
	m.editedContent = "stages: []\n"
	m.tempPath = "/tmp/flo_pipeline_123_keep.yml"
	m.queuedSaveName = "renamed-by-someone-else" // from the re-GET
	m.pendingConfirm = "overwrite"

	m, cmd := m.handleModalConfirm(components.ModalConfirmMsg{})
	msg, ok := cmd().(DetailSaveRequestMsg)
	if !ok || msg.Name != "renamed-by-someone-else" {
		t.Errorf("save request must carry the fresh name, got %+v", msg)
	}
}

func TestOptimisticCheckErrorFailsClosed(t *testing.T) {
	m := newTestDetailModel()
	m.tempPath = "/tmp/flo_pipeline_123_keep.yml"

	m, cmd := m.finishOptimisticCheck(nil, fmt.Errorf("network down"))
	if cmd != nil {
		t.Error("no save should be emitted when the check fails")
	}
	if !strings.Contains(m.View(), "Pre-write check failed") {
		t.Error("fail-closed error modal expected")
	}
}

func TestApplySaveResultFailureKeepsFile(t *testing.T) {
	m := newTestDetailModel()
	m.tempPath = filepath.Join(t.TempDir(), "keep.yml")
	os.WriteFile(m.tempPath, []byte("x"), 0644)

	m, _ = m.ApplySaveResult(fmt.Errorf("server rejected"))
	if _, err := os.Stat(m.tempPath); err != nil {
		t.Error("temp file must be kept on save failure")
	}
}

func TestApplySaveResultSuccessRemovesFileAndReloads(t *testing.T) {
	m := newTestDetailModel()
	m.tempPath = filepath.Join(t.TempDir(), "gone.yml")
	os.WriteFile(m.tempPath, []byte("x"), 0644)

	m, cmd := m.ApplySaveResult(nil)
	if _, err := os.Stat(m.tempPath); !os.IsNotExist(err) {
		t.Error("temp file should be removed on success")
	}
	if !m.loading {
		t.Error("page should reload after save")
	}
	msg, ok := cmd().(DetailReloadRequestMsg)
	if !ok || msg.PipelineID != "123" {
		t.Errorf("expected reload request, got %#v", msg)
	}
}

func TestSaveAsPathSanitizes(t *testing.T) {
	if got := saveAsPath("deploy/service: v2", "9"); got != "deploy-service--v2-9.yml" {
		t.Errorf("saveAsPath = %q", got)
	}
	if got := saveAsPath("", "9"); got != "pipeline-9.yml" {
		t.Errorf("saveAsPath fallback = %q", got)
	}
}
