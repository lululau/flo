# Flo CLI Subcommands Design

## Summary

Add non-interactive CLI interfaces for all existing TUI features, using Cobra framework with resource-oriented command structure. Backward compatible: `flo` without arguments still launches TUI.

## Command Structure

```
flo                          → Show help
flo tui                       → Launch interactive TUI
flo pipeline list             → List pipelines
flo pipeline groups           → List pipeline groups
flo pipeline history          → View run history
flo pipeline run              → Run a pipeline
flo pipeline logs             → View logs
flo pipeline stop             → Stop a running pipeline
```

## Subcommand Details

### `flo pipeline list`

```
flo pipeline list [--search TEXT] [--status all|running|success|failed] [--sort name|time] [--bookmark] [-o json]
```

- Default output: table with Name | Status | Last Run | Creator
- `--bookmark` filters to bookmarked pipelines only
- All TUI filter/sort capabilities exposed as flags

### `flo pipeline groups`

```
flo pipeline groups [--search TEXT] [-o json]
```

- Output: group name + pipeline count

### `flo pipeline history`

```
flo pipeline history [--pipeline NAME|ID] [--status all|running|success|failed] [--limit N] [--page N] [-o json]
```

- Without `--pipeline`: show history for all pipelines
- With `--pipeline`: show history for a specific pipeline

### `flo pipeline run`

```
flo pipeline run --pipeline NAME|ID --branch REPO:BRANCH,... [-o json]
```

- `--branch main` → all repos use main
- `--branch repo1:main,repo2:develop` → per-repo branch mapping
- Output: run ID + status
- Optional `-f` flag to follow/wait for completion

### `flo pipeline logs`

```
flo pipeline logs --pipeline NAME|ID --run-id RUN_ID [--stage NAME] [-f] [-o json]
```

- Without `--stage`: lists all stages with their status and job count (table), so user can see available stage names
- With `--stage NAME`: shows full logs for that specific stage
- With invalid `--stage`: prints error with list of available stage names
- `-f` for streaming/follow mode (like TUI auto-refresh)
- JSON mode outputs structured log data

### `flo pipeline stop`

```
flo pipeline stop --pipeline NAME|ID --run-id RUN_ID [-o json]
```

- Output: operation result + current status

## Global Flags

All subcommands share:
- `-o, --output` → `table` (default) | `json`
- `--config` → config file path (default: `~/.flo/config.yml`)
- `--org` → organization ID (overrides config)

## Architecture

```
cmd/flo/
├── main.go              # Entry: delegate to Cobra root command
├── tui.go               # TUI startup logic (extracted from main.go)
└── cli/
    ├── root.go          # Cobra root command, Run launches TUI
    ├── pipeline.go      # pipeline subcommand group
    └── output.go        # table/JSON formatting utilities
```

Key principle: **reuse existing API client** (`internal/api/client.go`), bypass TUI layer entirely. No changes to existing TUI code. Bare `flo` shows help; TUI is launched via `flo tui`.

## New Dependencies

- `github.com/spf13/cobra` — CLI framework
- All existing dependencies unchanged, zero TUI code modifications

## Output Format

- Default: human-readable table (using `text/tabwriter` or similar)
- `-o json`: structured JSON output for scripting/integration
