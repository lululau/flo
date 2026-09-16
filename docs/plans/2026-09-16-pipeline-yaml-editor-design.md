# Pipeline Definition View & Edit Design

## Summary

Support viewing and editing Yunxiao pipeline definitions (YAML) in an external editor (NeoVim via the existing `$EDITOR` mechanism). YAML-mode pipelines (`type == "PIPELINEASCODE"`) support write-back to the server; classic form-mode pipelines are view-only (the update API is only documented for YAML pipelines). Dual entry points: a new TUI pipeline detail page and `flo pipeline view` / `flo pipeline edit` CLI subcommands.

## Background & API Constraints

Verified against the Yunxiao OpenAPI docs:

- `GET /oapi/v1/flow/organizations/{orgId}/pipelines/{pipelineId}` returns:
  - `pipelineConfig.flow` — YAML text describing stages/jobs/steps. **Populated for both YAML-mode and classic-mode pipelines** (classic mode returns a server-generated YAML representation, e.g. prefixed `schema: tb`, which may contain internal IDs).
  - `type` — `"PIPELINEASCODE"` for YAML-mode pipelines, `null` for classic UI-mode pipelines.
  - `name`, `updateTime`, etc.
- `PUT /oapi/v1/flow/organizations/{orgId}/pipelines/{pipelineId}` accepts `{"content": <yaml string>, "name": <string>}`. **Only documented for YAML pipelines**; behavior for classic-mode pipelines is unknown (rejected, or possibly destructive), so we never write back to them.

Consequence: viewing unifies both modes (both are YAML text); editing forks on a single boolean (`IsYAMLMode`). The interface shell (detail page, viewer, save-as, copy) is fully shared.

## Scope

**In scope (v1)**

- TUI pipeline detail page rendering `pipelineConfig.flow`, reachable from the pipelines list page
- External read-only view (`v`, vim/nvim automatically get `-R`)
- External edit with write-back for YAML-mode pipelines only, with confirm → local validation → optimistic concurrency check → PUT
- Save-as local `.yml` and copy-to-clipboard from the detail page
- `flo pipeline view` / `flo pipeline edit` CLI subcommands
- Shared pure-logic package `internal/editutil`

**Out of scope (v1)**

- Editing `settings` / `sources` (only `flow` YAML; leave extension room in the detail page)
- Syntax highlighting / line numbers in the TUI viewport (external editor has both)
- Writing back to classic-mode pipelines under any flag
- AccessKey (SDK) auth path for the two new API calls — token (personal access token, REST) only, with an explicit error otherwise
- `--force` / `--yes` flags to skip confirmations
- Detail-page entry from the history page (revisit after real usage)

## Architecture

```
┌─ CLI (cmd/flo/cli) ──────────────┐      ┌─ TUI (internal/tui) ─────────────────┐
│ flo pipeline view <name|id>      │      │ pipelines list page                  │
│   └→ fetch → stdout / editor -R  │      │   v/e ──→ navigateTo(PagePipelineDetail)
│ flo pipeline edit <name|id>      │      │              │                       │
│   └→ fetch → $EDITOR → read back │      │   detail page (new)                  │
│      → confirm → validate → PUT  │      │   viewport YAML + mode badge         │
└────────────┬─────────────────────┘      │   e → edit session → confirm → PUT   │
             │                            └────────────┬────────────────────────┘
             ▼                                         ▼
   ┌─ internal/api.Client (shared) ─────────────────────────────┐
   │ GetPipelineDefinition(orgId, pipelineId)   GET  .../pipelines/{id}
   │ UpdatePipelineYAML(orgId, id, name, yaml)  PUT  .../pipelines/{id}
   └─────────────────────────────────────────────────────────────┘

   ┌─ internal/editutil (new, shared pure logic) ───────────────┐
   │ BuildEditorArgs · ValidateYAML · LineDiffStats · TempFile   │
   └─────────────────────────────────────────────────────────────┘
```

CLI and TUI share the API methods and `editutil`; each implements its own interaction shell (sequential stdin/stdout vs bubbletea message flow).

### Module changes

| Location | Change |
|---|---|
| `internal/editutil` (new) | Editor arg construction (vim/nvim aware `-R`), YAML validation, line diff stats, temp file naming |
| `internal/api/client.go` | `GetPipelineDefinition` (replaces the `GetPipelineDetails` stub) / `UpdatePipelineYAML`, token path only |
| `internal/tui/types` | Upgrade `OpenInEditorCmd` into an editor-session model (read-back, no auto-delete, `.yml` suffix, read-only mode) |
| `internal/tui/pages/detail.go` (new) | Detail page + edit state machine |
| `internal/tui/app.go`, `keys.go`, `types.go` | `PagePipelineDetail` route, detail keymap, list-page `v`/`e` bindings |
| `cmd/flo/cli/pipeline.go` | `view` / `edit` subcommands |

## API Layer

```go
type PipelineDefinition struct {
    PipelineID string
    Name       string
    IsYAMLMode bool   // type == "PIPELINEASCODE"
    FlowYAML   string // pipelineConfig.flow (populated for both modes)
    UpdateTime int64  // for optimistic concurrency check before write-back
}
```

- `GetPipelineDefinition(organizationId, pipelineId) (*PipelineDefinition, error)` — token path via the existing `makeTokenRequest`; parses `type`, `pipelineConfig.flow`, `name`, `updateTime` from the dynamic-map response (follow the existing field-extraction style in `listPipelinesWithTokenAndStatus`).
- `UpdatePipelineYAML(organizationId, pipelineId, name, content string) error` — PUT, body `{"content": <yaml>, "name": <name>}`.
- Both return an explicit error under AccessKey auth: "viewing/editing pipeline definitions requires a personal access token".

## internal/editutil

- `BuildEditorArgs(editor string, readOnly bool) []string` — splits editor (supports `code --wait` style args); when `readOnly` and the base name of the executable is `vim`/`nvim` (path forms like `/usr/local/bin/nvim` included), inserts `-R`; unknown editors get no extra flags.
- `ValidateYAML(content string) error` — `yaml.Unmarshal` (yaml.v3 already a dependency); surfaces `yaml.TypeError` line numbers.
- `LineDiffStats(old, new string) (added, removed int, ok bool)` — line-level LCS; returns `ok=false` (skip stats) above 5000 lines to avoid O(n²) stalls on large files.
- `TempFilePath(pipelineID string) string` — `<tmpdir>/flo_pipeline_<id>_<unix-ts>.yml`. The `.yml` suffix makes nvim set `filetype=yaml` automatically.

## Editor Session (TUI infrastructure upgrade)

Upgrade `types.OpenInEditorCmd` (currently: write temp file → `tea.ExecProcess` → delete file, discard edits, emit an unconsumed `EditorClosedMsg`):

```go
type EditorSessionOpts struct {
    FileSuffix string                       // ".yml" or ".txt"
    ReadOnly   bool                          // vim/nvim get -R
    OnExit     func(path string, err error)  // called after editor exits, file NOT removed
}
func OpenEditorSessionCmd(content, editor string, opts EditorSessionOpts) tea.Cmd
```

- Writes the temp file synchronously, suspends via `tea.ExecProcess`, on exit **reads the file back** and hands `(path, err)` to `OnExit` — the caller owns file lifecycle.
- The logs page migrates onto this mechanism with behavior unchanged: `e` (editor) uses the session with read-back discarded and the file deleted on exit; `v` keeps launching the **pager** (`config.GetPager()`) through the same session mechanism (file deleted on exit, no read-back). This retires the dead `EditorClosedMsg`/`PagerClosedMsg` handling.
- If the editor exits non-zero, the file is still read back (preserve the user's edits); only a read failure is an error.

## TUI: Pipeline Detail Page

### Layout

```
┌ Pipeline ─────────────────────────────────────────────────────────┐
│ deploy-service  [YAML]  updated 09-15 14:32  ID 12345             │ ← meta bar
├───────────────────────────────────────────────────────────────────┤
│ sources:                                                          │
│   repo:                                                           │
│     type: codeup                                                  │ ← viewport
│ stages:                                                           │   (plain YAML,
│   build:                                                          │    logs-page style)
├───────────────────────────────────────────────────────────────────┤
│ e edit · v view ext · s save-as · y copy · r reload · q back      │ ← footer
└───────────────────────────────────────────────────────────────────┘
```

- Meta bar: name + mode badge (`[YAML]` green / `[Classic]` gray) + update time + ID. Classic mode adds the hint "server-generated YAML representation, for reference only".
- Plain-text viewport (no highlighting, no line numbers in v1). Loading uses the existing spinner + loaded-message pattern; load failure shows an ErrorModal and navigates back.

### Keys (detail-page local keymap)

| Key | Action |
|---|---|
| `e` | YAML mode: start edit session. Classic mode: InfoModal explaining read-only and why |
| `v` | External read-only view (vim/nvim get `-R`, unknown editors open as-is); temp file deleted on exit |
| `s` | Save as `<name>-<id>.yml` in cwd; ConfirmModal on overwrite |
| `y` | Copy YAML to clipboard (reuse `CopyToClipboardCmd`) |
| `r` | Reload definition |
| `q`/`esc` | `navigateBack` |

### Navigation

```
pipelines list ──Enter──→ history ──Enter──→ logs
      │
      ├── v ──→ detail (view intent)
      └── e ──→ detail (pendingAction=edit: auto-start edit session after load)
```

`v` and `e` are unused on the pipelines list page today (verified against the current keymap). History page gets no detail-page entry in v1.

## Edit Flow State Machine (detail page)

```
[e] ──IsYAMLMode?──no──→ InfoModal (classic read-only)
 │yes
 ▼
write temp .yml → suspend TUI → $EDITOR (nvim)
 ▼ editor exits
EditorSessionFinished (path, content)
 ├─ unchanged ─────────→ remove temp file, return silently
 └─ changed (+N/-M) ───→ ConfirmModal "write back to Yunxiao?"
      ├─ cancel ───────→ keep file, InfoModal shows path
      └─ confirm ──────→ ValidateYAML
           ├─ syntax error → ErrorModal (with line numbers), keep file
           └─ valid ──────→ optimistic check: re-GET, compare UpdateTime
                ├─ server changed → ConfirmModal "someone may have updated; overwrite?"
                └─ unchanged ─────→ UpdatePipelineYAML (async PUT)
                     ├─ success → remove file, SuccessModal, re-fetch to refresh viewport
                     └─ failure → ErrorModal (server error + kept file path)
```

- **Optimistic check fails closed**: if the pre-write GET errors, abort the write-back with an error (retryable) rather than blindly overwriting.
- **`name` sent in the PUT always comes from the freshest fetch**: normally the definition loaded by the detail page; when the optimistic check re-GETs and the user confirms an overwrite, use that re-GET's `name` (never the stale one — a concurrent rename must not be reverted). The CLI edit flow follows the same rule.
- Diff stats are skipped (message says only "modified") when the content exceeds 5000 lines.
- On write success the definition is re-fetched because the server may normalize the YAML.

## CLI Subcommands

### `flo pipeline view --pipeline <name|id>`

- Uses the required `--pipeline` flag (name or ID), matching every existing subcommand (`history`/`run`/`status`/`logs`/`stop`); resolves via the existing `resolvePipelineID`.
- Default: pure YAML to **stdout**, meta info (name/mode/ID/update time) to stderr — pipe-friendly (`flo pipeline view --pipeline my-pipe | bat -lyaml`).
- `--editor` flag: external read-only view, same experience as the TUI `v` key.
- `-o/--output json`: full `PipelineDefinition` object (name, mode, updateTime, flow) as JSON, consistent with the shared `Output` helper used by existing subcommands.
- Works for both modes (viewing is universal).

### `flo pipeline edit --pipeline <name|id>`

- Classic mode: refuses with an explanation and points to `view`.
- Sequential flow (plain Go, no bubbletea):

```
fetch → write temp .yml → exec $EDITOR (stdin/stdout attached)
→ read back → unchanged: print "not modified", exit
→ changed: print +N/-M lines → "write back? [y/N]" on stdin
   → N: keep file, print path, exit
   → y: ValidateYAML → optimistic check (re-GET, compare UpdateTime)
      → server changed: second confirm "overwrite? [y/N]"
      → PUT: success → remove file, print "updated"
              failure → keep file, print error + path
```

## Error Handling

| Scenario | Behavior |
|---|---|
| AccessKey (SDK) auth calling the new APIs | Explicit error: "viewing/editing pipeline definitions requires a personal access token" |
| Editor exits non-zero | Still read the file back (preserve edits); only a read failure errors |
| YAML syntax error | CLI prints / TUI modal, both with line numbers (`yaml.TypeError`); temp file kept |
| Server rejects (403/404/…) | Surface error code and message as-is; temp file kept + path shown |
| Optimistic check detects server-side update | Second confirmation required before overwrite |
| Temp file kept on | cancel / validation failure / write failure; removed on unchanged / success |

## Testing

| Layer | Coverage |
|---|---|
| `internal/editutil` unit tests (core) | `BuildEditorArgs`: vim/nvim get `-R`, `code --wait` with args, path form `/usr/local/bin/nvim`, unknown editors untouched. `ValidateYAML`: valid / invalid / line numbers. `LineDiffStats`: add/remove/replace/large-file skip |
| API client | `httptest` fake server: GET parsing (`type` mode detection, `pipelineConfig.flow` extraction) and PUT body serialization (`{"content","name"}`) |
| TUI | Detail page model logic tests (follow `commands_test.go` style): load messages, every state-machine branch (unchanged / confirm / cancel / validation failure), classic-mode `e` shows InfoModal. No automation of the real suspend |
| CLI | Fake editor script (`EDITOR` pointing at a script that modifies the file) drives `edit` end-to-end: unchanged exit / change + confirm + write / classic-mode refusal |

Real nvim suspend/resume in AltScreen stays manual verification — the logs page already exercises the same `tea.ExecProcess` mechanism.
