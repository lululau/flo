# Pipeline Run Notification Design

## Summary

Add a configurable notification command that fires when a pipeline run (started from the current process) reaches a terminal state (success, failed, canceled). Uses a background goroutine to monitor all tracked runs independently of the TUI refresh cycle.

## Configuration

Add `notify_command` field to `~/.flo/config.yml`:

```yaml
notify_command: "terminal-notifier -title 'flo' -message '{{.PipelineName}} {{.Result}} ({{.Duration}}) [{{.Branch}}]'"
```

- Optional. When empty or absent, notifications are disabled.
- Uses Go `text/template` syntax for placeholders.

### Available Placeholders

| Placeholder | Type | Example |
|-------------|------|---------|
| `{{.PipelineName}}` | string | `my-pipeline` |
| `{{.Result}}` | string | `success ✓` / `failed ✗` / `canceled ○` |
| `{{.Duration}}` | string | `2m 35s` |
| `{{.Branch}}` | string | `main` |

### Result Value Mapping

| API Raw Value | Notification Value |
|---------------|--------------------|
| `SUCCESS` | `success ✓` |
| `FAILED` / `FAIL` | `failed ✗` |
| `CANCELED` / `CANCELLED` | `canceled ○` |

## Architecture

### Background Monitor (Approach A: Single goroutine)

A single background goroutine maintains a list of tracked runs. It polls all runs at a fixed interval and fires notifications when terminal states are detected.

### Data Structures

```go
type TrackedRun struct {
    OrganizationID string
    PipelineID     string
    PipelineName   string
    RunID          string
    Branch         string
    StartTime      time.Time
}

type NotifyData struct {
    PipelineName string
    Result       string
    Duration     string
    Branch       string
}
```

### Core Component: `Notifier`

Location: `internal/notify/notifier.go`

Responsibilities:
1. **Run tracking** — `Track(run TrackedRun)` adds a run to the monitor list.
2. **Polling & notification** — Background goroutine polls all tracked runs every 5 seconds.

### Lifecycle

1. On app startup, if `notify_command` is non-empty, create a `Notifier` instance.
2. After `RunPipeline` succeeds (returns `RunID`), call `notifier.Track(...)` to add the run.
3. Background goroutine calls API to query each tracked run's status every 5 seconds.
4. On terminal state detection: calculate duration, render template, execute notification command, remove from list.
5. When the list becomes empty, the goroutine exits automatically.
6. When the list goes from empty to non-empty, restart the goroutine.

### Polling Strategy

- Fixed 5-second interval, shared ticker for all runs.
- Batch query all tracked runs per tick.
- Sequential API calls (not concurrent) to avoid rate limits.
- On API failure, keep the run in the list and retry on next tick.

## Template Rendering & Command Execution

1. Extract data from the terminal run details, construct `NotifyData`.
2. Render `notify_command` config value with `NotifyData` using `text/template`.
3. On template render failure: log error, skip notification, do not crash.
4. Execute via `exec.Command("sh", "-c", renderedCommand)`.
5. Synchronous execution with 10-second timeout (`exec.CommandContext`).
6. On execution failure: log error, do not affect main program.

### Integration Point

`Notifier` holds a `*api.Client` reference to query run status. Called from `app.go` after `RunPipelineCmd` succeeds.

## Out of Scope

- Per-pipeline notification configuration (YAGNI — global config is sufficient).
- Stage-level notifications.
- CLI mode run monitoring (`flo run` does not currently follow runs; add later if needed).
