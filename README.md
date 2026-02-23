# gha-tui

A terminal-first operations console for GitHub Actions. Monitor workflow runs, inspect jobs and logs, search across logs, manage runs, view metrics, manage cache, and monitor runners — all from your terminal.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss).

## Features

- **5-tab layout** — Runs, Workflows, Metrics, Cache, Runners
- **All runs at a glance** — runs from all workflows load immediately with server-side filtering
- **Server-side filtering** — filter runs by workflow, event, status, branch, or actor
- **Run management** — rerun, cancel, force-cancel, and delete workflow runs
- **Job inspection** — matrix-aware job grouping with reusable workflow nesting, step counts, duration tracking
- **Live step progress** — in-progress jobs show real-time step-by-step status with auto-loading logs on completion
- **Run info view** — full metadata overlay with real-time refresh for in-progress runs
- **Full-text log search** — regex support, case sensitivity, job filtering, context lines
- **In-log search** — find patterns within a single job log with match navigation
- **Enhanced metrics** — success/failure rates, duration percentiles, queue times, usage breakdowns by event/actor/branch, slowest workflows, job-level stats
- **Cache management** — browse, filter, sort, and delete GitHub Actions caches
- **Runners** — view self-hosted runners with status, labels, and OS info
- **Log caching** — disk-based cache with configurable TTL and size limits, metadata tracking
- **Bulk operations** — delete multiple runs by workflow with parallel deletion (3 at a time)
- **Context-aware footer** — key hints change based on focused pane/view with inline status icon legend
- **Pagination** — browse through all runs with automatic page loading
- **Filtering** — filter workflows and runs by typing

## Prerequisites

- [GitHub CLI](https://cli.github.com/) (`gh`) — authenticated via `gh auth login`
- [Go](https://go.dev/) 1.25+ (only needed to build from source)

## Installation

### Download a binary

Grab the latest release from the [Releases](https://github.com/altinukshini/gh-actions-tui/releases) page. Binaries are available for Linux, macOS, and Windows (amd64 and arm64).

```bash
# Example: macOS arm64
curl -sL https://github.com/altinukshini/gh-actions-tui/releases/latest/download/gha-tui-v0.1.0-darwin-arm64.tar.gz | tar xz
sudo mv gha-tui /usr/local/bin/
```

### Go install

```bash
go install github.com/altinukshini/gh-actions-tui/cmd/gha-tui@latest
```

### Build from source

```bash
git clone https://github.com/altinukshini/gh-actions-tui.git
cd gh-actions-tui
make build
# Binary: ./gha-tui
```

## Authentication

`gha-tui` authenticates using the GitHub CLI. It reads the token from `gh auth`, so you must be logged in:

```bash
gh auth login
```

The token needs the `repo` scope (or `actions:read` for read-only usage, `actions:write` for run management).

## Usage

```bash
gha-tui -R owner/repo
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-R` | *(required)* | Repository in `owner/repo` format |
| `-cache-size` | `500` | Max log cache size in MB |
| `-cache-ttl` | `24h` | Log cache TTL |
| `-version` | | Print version and exit |

### Examples

```bash
# Monitor runs for a specific repo
gha-tui -R octocat/hello-world

# With custom cache settings
gha-tui -R octocat/hello-world -cache-size 1000 -cache-ttl 48h
```

## Layout

```
+------------------+--------------------+
| Runs / Jobs      | Job Details        |
| (left pane)      | (middle pane)      |
+------------------+--------------------+
```

The focused pane has a purple border. Switch panes with `Tab` / `Shift+Tab`.

Full-screen overlays open on top of the layout:
- **Log View** — `Enter` on a job
- **Info View** — `i` on a run
- **Search View** — `/` from runs
- **Filter Overlay** — `S` from runs

## Key Bindings

### Global

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` | Quit |
| `?` | Toggle help |
| `1` / `2` / `3` / `4` / `5` | Switch tab: Runs, Workflows, Metrics, Cache, Runners |
| `Tab` / `Shift+Tab` | Next / previous pane |
| `Esc` | Back / close overlay |
| `j` / `k` / `Up` / `Down` | Move up / down |
| `PgUp` / `PgDn` / `Ctrl+U` / `Ctrl+D` | Page up / down |
| `Enter` | Select item |

### Runs

| Key | Action |
|-----|--------|
| `Enter` | View job logs |
| `f` | Filter runs (client-side) |
| `S` | Server-side filter (workflow, event, status, branch, actor) |
| `r` | Refresh |
| `i` | Run info overlay |
| `/` | Search across logs |
| `Space` | Toggle select run |
| `R` | Rerun all jobs |
| `F` | Rerun failed jobs |
| `C` | Cancel run |
| `X` | Force cancel run |
| `d` | Delete run (or all selected) |
| `h` / `l` / `←` / `→` | Previous / next page |

### Workflows

| Key | Action |
|-----|--------|
| `Enter` | View runs for workflow |
| `f` | Filter workflows |
| `e` | Enable workflow |
| `D` | Disable workflow |
| `d` / `x` | Bulk delete all runs |

### Cache (Actions Caches)

| Key | Action |
|-----|--------|
| `Space` | Toggle select cache |
| `d` | Delete cache (or all selected) |
| `x` | Clear all caches |
| `s` | Cycle sort mode (last used / created / size) |
| `r` | Refresh caches |
| `f` | Filter caches |

### Runners

| Key | Action |
|-----|--------|
| `f` | Filter runners |
| `r` | Refresh runners |

### Log View

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll line by line |
| `g` / `G` | Jump to top / bottom |
| `PgUp` / `PgDn` | Page scroll |
| `/` | Search within log |
| `n` / `N` | Next / previous match |
| `Esc` | Close log view |

### Info View

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll |
| `g` / `G` | Top / bottom |
| `PgUp` / `PgDn` | Page scroll |
| `Esc` | Close info view |

## Status Icons

| Icon | Color | Meaning |
|------|-------|---------|
| `V` | Green | Success |
| `X` | Red | Failure |
| `!` | Amber | Cancelled |
| `-` | Gray | Skipped |
| `*` | Blue | In progress |
| `o` | Gray | Queued |

## Runs

Runs from all workflows are displayed immediately on startup. Each run shows status icon, run number, branch, relative time, and workflow name. Runs are loaded 30 per page with automatic pagination.

### Server-Side Filtering

Press `S` to open the filter overlay. Filter by:
- **Workflow** — cycle through available workflows
- **Event** — push, pull_request, schedule, workflow_dispatch, etc.
- **Status** — completed, in_progress, queued, waiting
- **Branch** — text input
- **Actor** — text input

The active filter is shown in the tab label: `[1] Runs (branch:main event:push)`.

### Run Operations

All destructive operations show a confirmation dialog.

| Operation | Key | Description |
|-----------|-----|-------------|
| Rerun all | `R` | Re-execute all jobs, creating a new attempt |
| Rerun failed | `F` | Re-execute only the failed jobs |
| Cancel | `C` | Graceful cancel — stops after the current step |
| Force cancel | `X` | Force terminate — use when regular cancel is stuck |
| Delete | `d` | Permanently remove the run |

## Workflows

Each workflow shows a state badge (`[active]`, `[disabled]`, `[inactive]`), total run count with recent success/failure breakdown, and the workflow file path. You can enable/disable workflows with `e` / `D`.

Selecting a workflow with `Enter` switches to the Runs tab filtered to that workflow.

## Job Details

The middle pane shows jobs for the selected run. Jobs are automatically grouped:

- **Matrix jobs** — variants like `build (ubuntu, 18)` and `build (macos, 20)` group under `build`
- **Reusable workflows** — jobs using reusable workflows (e.g., `deploy-ecs-service (common) / Deploy ECS-Service`) are nested hierarchically: top-level group → caller variant → called jobs

Each group and sub-group shows an aggregate status icon (in-progress > failure > cancelled > queued > success). Each job shows: status icon, name, duration, and step count.

### Live Step Progress

Opening an in-progress job shows a real-time step-by-step progress view instead of logs (GitHub's API does not expose logs for running jobs). Each step displays its status icon, name, and elapsed time. The view polls every 3 seconds and automatically fetches the full log once the job completes.

## Run Info View

Press `i` to open a full-screen overlay showing all run metadata:

- Run number, attempt, status, conclusion
- Actor, branch, commit SHA, event type
- Created/started/updated timestamps and duration
- GitHub URL
- Job summary (pass/fail/running counts) and individual job list

For in-progress runs, the view auto-refreshes every 3 seconds.

## Search

### Cross-Log Search (`/` from runs)

Search across all job logs for the selected run:

| Option | Description |
|--------|-------------|
| Pattern | Text to search for |
| Regex | Prefix with `/` (e.g., `/error.*timeout`) |
| Case sensitive | Toggle with `Tab` |
| Failed only | Only search failed job logs |
| Job filter | Filter by job name pattern |

Results are grouped by job name with line numbers and context lines. Press `Enter` on a match to jump directly to that line in the job's log view with the matching line highlighted. Press `Esc` from the log view to return to search results.

### In-Log Search (`/` from log view)

Search within the currently displayed log. Matches are highlighted. Navigate with `n` / `N`.

## Metrics

Press `3` to switch to the Metrics tab. Cycle time windows with `[` and `]` — data is re-fetched for each window. Available windows are derived from the repository's artifact and log retention setting (e.g., 90-day retention yields: 24h, 7d, 30d, 90d). Falls back to 24h/7d/30d if the retention API is unavailable.

### Overview

| Metric | Description |
|--------|-------------|
| Total runs | Runs in the time window (with sample size note) |
| Success / Failure rate | Count and percentage |
| Cancel count | Cancelled runs |
| Retry rate | Runs with attempt > 1 |

### Performance

| Metric | Description |
|--------|-------------|
| Mean / Median / P95 / P99 duration | Run duration percentiles |
| Mean / Median / P95 queue time | Time from creation to start |

### Slowest Workflows

Top 5 workflows by P95 duration, showing median, P95, and run count.

### Usage Breakdowns

- **Runs by Event** — all trigger events sorted by count
- **Top Actors** — top 10 actors by run count
- **Top Branches** — top 10 branches by run count

### Top Failing Workflows

Workflows ranked by failure rate (up to 5), displayed as:

```
  1.  30.0%  ████████████████████  3/10   CI Pipeline
  2.  16.7%  ███████████░░░░░░░░░  2/12   Deploy
  3.   6.2%  ████░░░░░░░░░░░░░░░░  1/16   Lint
```

### Top Failing Jobs

Individual jobs ranked by failure count (up to 5).

### Job Performance

Total jobs, success/fail counts, mean/median/P95 duration.

### Metrics Keys

| Key | Action |
|-----|--------|
| `[` / `]` | Cycle time window |

## Cache Management

Press `4` to switch to the Cache tab. Browse GitHub Actions caches for the repository (the same caches shown at `repo/actions/caches` on GitHub).

Each entry shows: cache key, size, branch, creation date, and last used time.

- **Filter** — press `f` to filter by cache key or branch ref
- **Sort** — press `s` to cycle sort modes: last used, created, or size
- **Refresh** — press `r` to reload caches from the API
- **Select** — press `Space` to multi-select caches, then `d` to bulk delete selected
- **Delete** — press `d` to delete the focused cache (or all selected), `x` to clear all caches

The header shows cache count, total size, and current sort mode.

## Runners

Press `5` to switch to the Runners tab. View self-hosted runners for the repository.

Each runner shows:
- Status indicator (green dot = online, red dot = offline)
- Runner name and busy state
- OS and labels
- Ephemeral flag

Press `r` to refresh, `f` to filter by name, OS, or labels.

## Bulk Delete

From the workflows view, press `d` or `x` to bulk delete all runs for a workflow. Runs are deleted 3 at a time in parallel, fetching all pages (not just the first 100).

## Log Cache

Logs are cached to disk at `$TMPDIR/gha-tui/logs/`. The cache extracts GitHub's zip archives and serves subsequent requests from disk. Each entry stores metadata (workflow name, branch, actor, event, timestamps).

- **TTL eviction** — entries older than `-cache-ttl` (default 24h) are removed
- **Size eviction** — oldest entries removed when total exceeds `-cache-size` (default 500 MB)

```bash
# Clean cache
rm -rf /tmp/gha-tui/
# or
make clean
```

## Auto-Refresh

When viewing an in-progress run, jobs and run metadata are polled every 3 seconds. Opening an in-progress job shows live step progress (also polled every 3 seconds) and automatically loads the full log when the job completes. Polling stops when the run completes or you navigate away.

## Context-Aware Footer

The status bar at the bottom of the screen shows key hints that change based on your current view and focused pane. The runs and jobs panes include an inline status icon legend so you can identify what each icon means at a glance. Overlay views (log, search, info, filter) show their own relevant shortcuts.

## API Rate Limiting

The header displays your GitHub API rate limit (`remaining/limit`). Bulk deletes are throttled to respect limits.

## Architecture

```
cmd/gha-tui/         CLI entry point
internal/
  api/               GitHub REST API client (runs, jobs, workflows, runners)
  cache/             Disk-based log cache with TTL/size eviction + metadata
  config/            Repository configuration
  model/             Domain types (Run, Job, Workflow, Runner, SearchQuery)
  ops/               Bulk operations
  search/            Full-text search engine with regex
  tui/               Bubble Tea components
    app.go           Root model and routing
    runs/            Runs list with pagination
    workflows/       Workflow selector with inline stats
    details/         Job details with matrix + reusable workflow grouping
    logview/         Log viewer with in-log search
    infoview/        Run info overlay
    searchview/      Cross-log search
    filteroverlay/   Server-side filter overlay
    dashboard/       Enhanced metrics and analytics
    cacheview/       Cache management view
    runnersview/     Runners list view
    confirm/         Confirmation dialog
  ui/                Styles, key bindings, messages
```

## Development

```bash
make build              # Build binary
make run REPO=owner/repo # Build and run
make test               # Unit tests
make test-integration   # Integration tests (requires gh auth)
make lint               # Run go vet
make clean              # Remove binary and cache
```

## Troubleshooting

| Problem | Solution |
|---------|----------|
| "Auth error" on startup | Run `gh auth login` and `gh auth status` |
| No workflows appear | Check `.github/workflows/` exists in the repo |
| Logs stuck on "Loading..." | Check rate limit in header; wait if `0/5000` |
| Force cancel fails | Only works on `in_progress` runs |
| Cache too large | Use `-cache-size 100 -cache-ttl 1h` or `rm -rf /tmp/gha-tui/` |
| Filter won't activate | Press `f` with the correct pane focused (purple border) |
| No runners shown | Repository may not have self-hosted runners configured |

## License

See [LICENSE](LICENSE) for details.
