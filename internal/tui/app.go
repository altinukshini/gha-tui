package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/altinukshini/gha-tui/internal/api"
	"github.com/altinukshini/gha-tui/internal/cache"
	"github.com/altinukshini/gha-tui/internal/config"
	"github.com/altinukshini/gha-tui/internal/model"
	"github.com/altinukshini/gha-tui/internal/search"
	"github.com/altinukshini/gha-tui/internal/tui/cacheview"
	"github.com/altinukshini/gha-tui/internal/tui/confirm"
	"github.com/altinukshini/gha-tui/internal/tui/dashboard"
	"github.com/altinukshini/gha-tui/internal/tui/details"
	"github.com/altinukshini/gha-tui/internal/tui/filteroverlay"
	"github.com/altinukshini/gha-tui/internal/tui/infoview"
	"github.com/altinukshini/gha-tui/internal/tui/logview"
	"github.com/altinukshini/gha-tui/internal/tui/runs"
	"github.com/altinukshini/gha-tui/internal/tui/runnersview"
	"github.com/altinukshini/gha-tui/internal/tui/searchview"
	"github.com/altinukshini/gha-tui/internal/tui/workflows"
	"github.com/altinukshini/gha-tui/internal/ui"
)

type View int

const (
	ViewRuns View = iota
	ViewWorkflows
	ViewMetrics
	ViewCache
	ViewRunners
)

type Pane int

const (
	PaneLeft Pane = iota
	PaneMiddle
	PaneRight
)

type App struct {
	cfg      config.Config
	client   *api.Client
	logCache *cache.LogCache
	search   *search.Engine

	// Views
	runsView      runs.Model
	detailsView   details.Model
	logView       logview.Model
	infoView      infoview.Model
	searchView    searchview.Model
	workflowsView workflows.Model
	dashboardView dashboard.Model
	confirmDialog confirm.Model

	// Server-side filter for Runs tab
	runsFilter    filteroverlay.FilterResult
	filterOverlay filteroverlay.Model
	workflows     []model.Workflow // cached for filter picker

	// New views
	cacheView   cacheview.Model
	runnersView runnersview.Model

	// State
	currentView   View
	focusedPane   Pane
	width         int
	height        int
	status        string
	rateRemaining int
	rateLimit     int

	// Cached log data for search
	currentRunLogs map[string]string
	currentRunID   int64

	// Pagination
	runsPage       int
	runsTotalCount int
	runsHasMore    bool
	runsLoading    bool

	// Auto-refresh for in-progress runs
	autoRefreshRunID int64

	// Live log tailing for in-progress jobs
	tailingJobID   int64
	tailingJobName string

	// Track the currently viewed job for failed-step jump
	viewingJob *model.Job

	// Attempt cycling: 0 = latest/merged, 1+ = specific attempt
	viewingAttempt int

	showHelp     bool
	helpViewport viewport.Model
	helpReady    bool

	logFullScreen bool
	infoFullScreen     bool
	infoStartedRefresh bool
	cameFromSearch     bool
}

func NewApp(cfg config.Config, client *api.Client, logCache *cache.LogCache) App {
	return App{
		cfg:            cfg,
		client:         client,
		logCache:       logCache,
		search:         search.New(),
		runsView:       runs.New(),
		detailsView:    details.New(),
		logView:        logview.New(),
		infoView:       infoview.New(),
		searchView:     searchview.New(),
		workflowsView:  workflows.NewWithStats(),
		dashboardView:  dashboard.New(),
		cacheView:      cacheview.New(),
		runnersView:    runnersview.New(),
		currentView:    ViewRuns,
		focusedPane:    PaneLeft,
		status:         "Loading runs...",
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(a.fetchWorkflows(), a.fetchRuns(), a.fetchRetention())
}

func (a App) fetchRetention() tea.Cmd {
	return func() tea.Msg {
		settings, err := a.client.GetRetentionSettings()
		if err != nil {
			return ui.RetentionLoadedMsg{Err: err}
		}
		return ui.RetentionLoadedMsg{RetentionDays: settings.ArtifactAndLogRetentionDays}
	}
}

// --- Data fetching commands ---

const runsPerPage = 30

func (a App) fetchRuns() tea.Cmd {
	filter := api.RunsFilter{PerPage: runsPerPage, Page: 1}
	if a.runsFilter.WorkflowID != 0 {
		filter.WorkflowID = a.runsFilter.WorkflowID
	}
	if a.runsFilter.Event != "" {
		filter.Event = a.runsFilter.Event
	}
	if a.runsFilter.Status != "" {
		filter.Status = a.runsFilter.Status
	}
	if a.runsFilter.Branch != "" {
		filter.Branch = a.runsFilter.Branch
	}
	if a.runsFilter.Actor != "" {
		filter.Actor = a.runsFilter.Actor
	}
	return func() tea.Msg {
		resp, err := a.client.ListRuns(filter)
		if err != nil {
			return ui.RunsLoadedMsg{Err: err}
		}
		return ui.RunsLoadedMsg{Runs: resp.Runs, TotalCount: resp.TotalCount}
	}
}

func (a App) refreshCurrentRuns() tea.Cmd {
	page := a.runsPage
	filter := api.RunsFilter{PerPage: runsPerPage, Page: page}
	if a.runsFilter.WorkflowID != 0 {
		filter.WorkflowID = a.runsFilter.WorkflowID
	}
	if a.runsFilter.Event != "" {
		filter.Event = a.runsFilter.Event
	}
	if a.runsFilter.Status != "" {
		filter.Status = a.runsFilter.Status
	}
	if a.runsFilter.Branch != "" {
		filter.Branch = a.runsFilter.Branch
	}
	if a.runsFilter.Actor != "" {
		filter.Actor = a.runsFilter.Actor
	}
	return func() tea.Msg {
		resp, err := a.client.ListRuns(filter)
		if err != nil {
			return ui.RunsRefreshedMsg{Err: err}
		}
		return ui.RunsRefreshedMsg{Runs: resp.Runs, TotalCount: resp.TotalCount}
	}
}

func (a App) fetchRunsPage(page int) tea.Cmd {
	filter := api.RunsFilter{PerPage: runsPerPage, Page: page}
	if a.runsFilter.WorkflowID != 0 {
		filter.WorkflowID = a.runsFilter.WorkflowID
	}
	if a.runsFilter.Event != "" {
		filter.Event = a.runsFilter.Event
	}
	if a.runsFilter.Status != "" {
		filter.Status = a.runsFilter.Status
	}
	if a.runsFilter.Branch != "" {
		filter.Branch = a.runsFilter.Branch
	}
	if a.runsFilter.Actor != "" {
		filter.Actor = a.runsFilter.Actor
	}
	return func() tea.Msg {
		resp, err := a.client.ListRuns(filter)
		if err != nil {
			return ui.RunsPageMsg{Page: page, Err: err}
		}
		return ui.RunsPageMsg{Runs: resp.Runs, TotalCount: resp.TotalCount, Page: page}
	}
}

func (a App) fetchJobs(runID int64) tea.Cmd {
	return func() tea.Msg {
		resp, err := a.client.ListJobs(runID, api.JobsFilter{Filter: "latest", PerPage: 100})
		if err != nil {
			return ui.JobsLoadedMsg{RunID: runID, Err: err}
		}
		return ui.JobsLoadedMsg{RunID: runID, Jobs: resp.Jobs}
	}
}

func (a App) fetchJobsForAttempt(runID int64, attempt int) tea.Cmd {
	return func() tea.Msg {
		resp, err := a.client.ListJobsForAttempt(runID, attempt, api.JobsFilter{PerPage: 100})
		if err != nil {
			return ui.JobsLoadedMsg{RunID: runID, Err: err}
		}
		return ui.JobsLoadedMsg{RunID: runID, Jobs: resp.Jobs}
	}
}

func (a App) fetchLogsForAttempt(run *model.Run, attempt int) tea.Cmd {
	return func() tea.Msg {
		runID := run.ID
		// Check cache first
		if a.logCache.HasRun(runID, attempt) {
			if logs, err := a.logCache.GetAllJobLogs(runID, attempt); err == nil && len(logs) > 0 {
				return ui.LogsLoadedMsg{RunID: runID, Attempt: attempt, Logs: logs}
			}
		}

		// Download from API
		var body io.ReadCloser
		var err error
		if attempt == run.RunAttempt {
			body, err = a.client.DownloadRunLogs(context.Background(), runID)
		} else {
			body, err = a.client.DownloadRunAttemptLogs(context.Background(), runID, attempt)
		}
		if err != nil {
			return ui.LogsLoadedMsg{RunID: runID, Attempt: attempt, Err: err}
		}

		_, storeErr := a.logCache.StoreRunLogs(runID, attempt, body)
		body.Close()
		if storeErr != nil {
			return ui.LogsLoadedMsg{RunID: runID, Attempt: attempt, Err: storeErr}
		}

		a.logCache.WriteMeta(runID, attempt, cache.CacheMeta{
			RunID:        runID,
			Attempt:      attempt,
			WorkflowName: run.Name,
			DisplayTitle: run.DisplayTitle,
			Branch:       run.HeadBranch,
			Actor:        run.Actor.Login,
			Event:        run.Event,
			CreatedAt:    run.CreatedAt,
			StoredAt:     time.Now(),
		})

		logs, err := a.logCache.GetAllJobLogs(runID, attempt)
		if err != nil {
			return ui.LogsLoadedMsg{RunID: runID, Attempt: attempt, Err: err}
		}
		return ui.LogsLoadedMsg{RunID: runID, Attempt: attempt, Logs: logs}
	}
}

func (a App) fetchLogs(run *model.Run) tea.Cmd {
	return func() tea.Msg {
		runID := run.ID
		attempt := run.RunAttempt

		// For multi-attempt runs, download all attempts in parallel and merge.
		// Later attempts only overwrite if the new content is longer, because
		// GitHub includes stub entries (just system.txt) for jobs that didn't
		// re-run in the later attempt — those stubs must not replace real logs.

		type attemptResult struct {
			att  int
			logs map[string]string
			err  error
		}

		results := make([]attemptResult, attempt)
		var wg sync.WaitGroup

		for att := 1; att <= attempt; att++ {
			wg.Add(1)
			go func(att int) {
				defer wg.Done()
				r := attemptResult{att: att}

				// Check cache first
				if a.logCache.HasRun(runID, att) {
					if logs, err := a.logCache.GetAllJobLogs(runID, att); err == nil && len(logs) > 0 {
						r.logs = logs
						results[att-1] = r
						return
					}
				}

				// Download from API
				var body io.ReadCloser
				var err error
				if att == attempt {
					body, err = a.client.DownloadRunLogs(context.Background(), runID)
				} else {
					body, err = a.client.DownloadRunAttemptLogs(context.Background(), runID, att)
				}
				if err != nil {
					r.err = err
					results[att-1] = r
					return
				}

				_, storeErr := a.logCache.StoreRunLogs(runID, att, body)
				body.Close()
				if storeErr != nil {
					r.err = storeErr
					results[att-1] = r
					return
				}

				a.logCache.WriteMeta(runID, att, cache.CacheMeta{
					RunID:        runID,
					Attempt:      att,
					WorkflowName: run.Name,
					DisplayTitle: run.DisplayTitle,
					Branch:       run.HeadBranch,
					Actor:        run.Actor.Login,
					Event:        run.Event,
					CreatedAt:    run.CreatedAt,
					StoredAt:     time.Now(),
				})

				if logs, err := a.logCache.GetAllJobLogs(runID, att); err == nil {
					r.logs = logs
				}
				results[att-1] = r
			}(att)
		}
		wg.Wait()

		// Merge in order (attempt 1, 2, ...) — longer content wins
		merged := make(map[string]string)
		var lastErr error
		for _, r := range results {
			if r.err != nil {
				// Earlier attempts may have been cleaned up; only fail on latest
				if r.att == attempt {
					lastErr = r.err
				}
				continue
			}
			for k, v := range r.logs {
				if existing, ok := merged[k]; !ok || len(v) > len(existing) {
					merged[k] = v
				}
			}
		}

		if len(merged) == 0 {
			err := lastErr
			if err == nil {
				err = fmt.Errorf("no logs available")
			}
			return ui.LogsLoadedMsg{RunID: runID, Attempt: attempt, Err: err}
		}
		return ui.LogsLoadedMsg{RunID: runID, Attempt: attempt, Logs: merged}
	}
}

func (a App) fetchWorkflows() tea.Cmd {
	return func() tea.Msg {
		resp, err := a.client.ListWorkflows(100, 1)
		if err != nil {
			return ui.WorkflowsLoadedMsg{Err: err}
		}
		return ui.WorkflowsLoadedMsg{Workflows: resp.Workflows}
	}
}

func (a App) fetchWorkflowStats(wfs []model.Workflow) tea.Cmd {
	client := a.client
	return func() tea.Msg {
		var mu sync.Mutex
		stats := make(map[int64]ui.WorkflowStats, len(wfs))
		var wg sync.WaitGroup

		for _, wf := range wfs {
			wg.Add(1)
			go func(wf model.Workflow) {
				defer wg.Done()
				resp, err := client.ListRuns(api.RunsFilter{
					WorkflowID: wf.ID,
					PerPage:    30,
				})
				if err != nil {
					return
				}
				var s ui.WorkflowStats
				s.TotalRuns = resp.TotalCount
				for _, r := range resp.Runs {
					switch r.Conclusion {
					case model.ConclusionSuccess:
						s.SuccessCount++
					case model.ConclusionFailure:
						s.FailureCount++
					}
				}
				mu.Lock()
				stats[wf.ID] = s
				mu.Unlock()
			}(wf)
		}

		wg.Wait()
		return ui.WorkflowStatsMsg{Stats: stats}
	}
}

func (a App) fetchDashboardData(window dashboard.TimeWindow) tea.Cmd {
	client := a.client
	return func() tea.Msg {
		since := time.Duration(window.Days) * 24 * time.Hour
		createdAfter := time.Now().Add(-since).UTC().Format("2006-01-02")
		resp, err := client.ListRuns(api.RunsFilter{
			PerPage: 100,
			Created: ">=" + createdAfter,
		})
		if err != nil {
			return ui.DashboardDataMsg{Err: err}
		}
		allRuns := resp.Runs
		totalCount := resp.TotalCount

		// Fetch page 2 if there are more runs
		if len(resp.Runs) >= 100 {
			resp2, err := client.ListRuns(api.RunsFilter{
				PerPage: 100,
				Page:    2,
				Created: ">=" + createdAfter,
			})
			if err == nil {
				allRuns = append(allRuns, resp2.Runs...)
			}
		}

		// Fetch jobs for up to 50 completed runs concurrently
		var allJobs []model.Job
		var mu sync.Mutex
		sem := make(chan struct{}, 10) // 10 concurrent
		var wg sync.WaitGroup

		jobCount := 0
		for _, r := range allRuns {
			if r.Status == model.RunStatusCompleted && jobCount < 50 {
				jobCount++
				wg.Add(1)
				go func(runID int64) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					jobResp, err := client.ListJobs(runID, api.JobsFilter{Filter: "latest", PerPage: 100})
					if err == nil {
						mu.Lock()
						allJobs = append(allJobs, jobResp.Jobs...)
						mu.Unlock()
					}
				}(r.ID)
			}
		}
		wg.Wait()

		return ui.DashboardDataMsg{Runs: allRuns, Jobs: allJobs, TotalCount: totalCount}
	}
}

func (a App) fetchActionsCaches() tea.Cmd {
	client := a.client
	return func() tea.Msg {
		// Fetch up to 100 caches sorted by last accessed
		resp, err := client.ListActionsCaches(100, 1, "last_accessed_at", "desc")
		if err != nil {
			return ui.ActionsCachesLoadedMsg{Err: err}
		}
		return ui.ActionsCachesLoadedMsg{
			Caches:     resp.ActionsCaches,
			TotalCount: resp.TotalCount,
		}
	}
}

func (a App) deleteActionsCache(cacheID int64) tea.Cmd {
	client := a.client
	return func() tea.Msg {
		err := client.DeleteActionsCache(cacheID)
		return ui.ActionsCacheDeletedMsg{CacheID: cacheID, Err: err}
	}
}

func (a App) deleteSelectedCaches(ids []int64) tea.Cmd {
	client := a.client
	return func() tea.Msg {
		var mu sync.Mutex
		var lastErr error
		deleted := 0
		sem := make(chan struct{}, 3)
		var wg sync.WaitGroup
		for _, id := range ids {
			wg.Add(1)
			go func(id int64) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				if err := client.DeleteActionsCache(id); err != nil {
					mu.Lock()
					lastErr = err
					mu.Unlock()
				} else {
					mu.Lock()
					deleted++
					mu.Unlock()
				}
			}(id)
		}
		wg.Wait()
		if lastErr != nil {
			return ui.ActionsCacheDeletedMsg{
				Err: fmt.Errorf("deleted %d/%d caches, last error: %w", deleted, len(ids), lastErr),
			}
		}
		return ui.ActionsCacheDeletedMsg{CacheID: 0}
	}
}

func (a App) deleteAllActionsCaches() tea.Cmd {
	client := a.client
	return func() tea.Msg {
		// Fetch all caches across pages
		var allIDs []int64
		for page := 1; ; page++ {
			resp, err := client.ListActionsCaches(100, page, "", "")
			if err != nil {
				return ui.ActionsCacheDeletedMsg{Err: fmt.Errorf("fetch caches: %w", err)}
			}
			for _, c := range resp.ActionsCaches {
				allIDs = append(allIDs, c.ID)
			}
			if len(resp.ActionsCaches) < 100 {
				break
			}
		}
		if len(allIDs) == 0 {
			return ui.ActionsCacheDeletedMsg{Err: fmt.Errorf("no caches to delete")}
		}
		// Delete 3 at a time
		var mu sync.Mutex
		var lastErr error
		deleted := 0
		sem := make(chan struct{}, 3)
		var wg sync.WaitGroup
		for _, id := range allIDs {
			wg.Add(1)
			go func(id int64) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				if err := client.DeleteActionsCache(id); err != nil {
					mu.Lock()
					lastErr = err
					mu.Unlock()
				} else {
					mu.Lock()
					deleted++
					mu.Unlock()
				}
			}(id)
		}
		wg.Wait()
		if lastErr != nil {
			return ui.ActionsCacheDeletedMsg{
				Err: fmt.Errorf("deleted %d/%d caches, last error: %w", deleted, len(allIDs), lastErr),
			}
		}
		return ui.ActionsCacheDeletedMsg{CacheID: 0}
	}
}

func (a App) fetchRunners() tea.Cmd {
	return func() tea.Msg {
		resp, err := a.client.ListRunners(100, 1)
		if err != nil {
			return ui.RunnersLoadedMsg{Err: err}
		}
		runners := resp.Runners

		// Also fetch org-level runners (shared with this repo).
		orgFailed := false
		if orgResp, err := a.client.ListOrgRunners(100, 1); err == nil && orgResp != nil {
			seen := make(map[int64]bool, len(runners))
			for _, r := range runners {
				seen[r.ID] = true
			}
			for _, r := range orgResp.Runners {
				if !seen[r.ID] {
					runners = append(runners, r)
				}
			}
		} else if err != nil {
			orgFailed = true
		}

		return ui.RunnersLoadedMsg{Runners: runners, OrgFailed: orgFailed}
	}
}

func (a App) scheduleRunsRefresh() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return ui.RunsTickMsg{}
	})
}

func (a App) scheduleJobsRefresh(runID int64) tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return ui.JobsTickMsg{RunID: runID}
	})
}

func (a App) scheduleLogRefresh(jobID int64, jobName string) tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
		return ui.LogTailTickMsg{JobID: jobID, JobName: jobName}
	})
}

func (a App) checkJobStatus(jobID int64, jobName string) tea.Cmd {
	client := a.client
	return func() tea.Msg {
		job, err := client.GetJob(jobID)
		if err != nil {
			return ui.JobTailStatusMsg{JobID: jobID, JobName: jobName, Err: err}
		}
		return ui.JobTailStatusMsg{
			JobID:     jobID,
			JobName:   jobName,
			Job:       job,
			Completed: job.Status == model.RunStatusCompleted,
		}
	}
}

func (a App) fetchJobLog(jobID int64, jobName string) tea.Cmd {
	return func() tea.Msg {
		body, err := a.client.DownloadJobLog(context.Background(), jobID)
		if err != nil {
			return ui.JobLogLoadedMsg{JobID: jobID, JobName: jobName, Err: err}
		}
		defer body.Close()
		data, err := io.ReadAll(body)
		if err != nil {
			return ui.JobLogLoadedMsg{JobID: jobID, JobName: jobName, Err: err}
		}
		return ui.JobLogLoadedMsg{JobID: jobID, JobName: jobName, Content: string(data)}
	}
}

// --- Action commands ---

func (a App) doRerunAll(runID int64) tea.Cmd {
	return func() tea.Msg {
		err := a.client.RerunWorkflow(runID, false)
		return ui.ActionResultMsg{Action: "Rerun all", Err: err}
	}
}

func (a App) doRerunFailed(runID int64) tea.Cmd {
	return func() tea.Msg {
		err := a.client.RerunFailedJobs(runID, false)
		return ui.ActionResultMsg{Action: "Rerun failed", Err: err}
	}
}

func (a App) doDeleteRun(runID int64) tea.Cmd {
	return func() tea.Msg {
		err := a.client.DeleteRun(runID)
		return ui.ActionResultMsg{Action: "Delete run", Err: err}
	}
}

func (a App) doCancelRun(runID int64) tea.Cmd {
	return func() tea.Msg {
		err := a.client.CancelRun(runID)
		return ui.ActionResultMsg{Action: "Cancel run", Err: err}
	}
}

func (a App) doForceCancelRun(runID int64) tea.Cmd {
	return func() tea.Msg {
		err := a.client.ForceCancelRun(runID)
		return ui.ActionResultMsg{Action: "Force cancel run", Err: err}
	}
}

func (a App) fetchRun(runID int64) tea.Cmd {
	return func() tea.Msg {
		run, err := a.client.GetRun(runID)
		if err != nil {
			return ui.RunLoadedMsg{RunID: runID, Err: err}
		}
		return ui.RunLoadedMsg{RunID: runID, Run: run}
	}
}

func (a App) doEnableWorkflow(wfID int64) tea.Cmd {
	return func() tea.Msg {
		err := a.client.EnableWorkflow(wfID)
		return ui.ActionResultMsg{Action: "Enable workflow", Err: err}
	}
}

func (a App) doDisableWorkflow(wfID int64) tea.Cmd {
	return func() tea.Msg {
		err := a.client.DisableWorkflow(wfID)
		return ui.ActionResultMsg{Action: "Disable workflow", Err: err}
	}
}

func deleteIDsConcurrently(client *api.Client, ids []int64, action string) ui.ActionResultMsg {
	var mu sync.Mutex
	var lastErr error
	deleted := 0
	sem := make(chan struct{}, 3) // 3 concurrent
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := client.DeleteRun(id); err != nil {
				mu.Lock()
				lastErr = err
				mu.Unlock()
			} else {
				mu.Lock()
				deleted++
				mu.Unlock()
			}
		}(id)
	}
	wg.Wait()
	if lastErr != nil {
		return ui.ActionResultMsg{
			Action: fmt.Sprintf("%s (%d/%d deleted)", action, deleted, len(ids)),
			Err:    lastErr,
		}
	}
	return ui.ActionResultMsg{
		Action: fmt.Sprintf("%s (%d runs)", action, deleted),
	}
}

func (a App) doBulkDeleteRuns(wf *model.Workflow) tea.Cmd {
	client := a.client
	return func() tea.Msg {
		var allIDs []int64
		for page := 1; ; page++ {
			resp, err := client.ListRuns(api.RunsFilter{
				WorkflowID: wf.ID, PerPage: 100, Page: page,
			})
			if err != nil {
				return ui.ActionResultMsg{Action: "Bulk delete", Err: err}
			}
			for _, r := range resp.Runs {
				allIDs = append(allIDs, r.ID)
			}
			if len(resp.Runs) < 100 {
				break
			}
		}
		if len(allIDs) == 0 {
			return ui.ActionResultMsg{Action: "Bulk delete", Err: fmt.Errorf("no runs found")}
		}
		return deleteIDsConcurrently(client, allIDs, "Bulk delete")
	}
}

func (a App) doBulkDeleteByIDs(ids []int64) tea.Cmd {
	client := a.client
	return func() tea.Msg {
		return deleteIDsConcurrently(client, ids, "Delete selected")
	}
}

// --- Update ---

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle confirm dialog result (arrives AFTER dialog deactivates itself)
	if result, ok := msg.(confirm.ResultMsg); ok {
		if result.Confirmed {
			switch result.Action {
			case "rerun-all":
				cmds = append(cmds, a.doRerunAll(result.Data.(int64)))
			case "rerun-failed":
				cmds = append(cmds, a.doRerunFailed(result.Data.(int64)))
			case "delete-run":
				cmds = append(cmds, a.doDeleteRun(result.Data.(int64)))
			case "cancel-run":
				cmds = append(cmds, a.doCancelRun(result.Data.(int64)))
			case "force-cancel-run":
				cmds = append(cmds, a.doForceCancelRun(result.Data.(int64)))
			case "enable-workflow":
				cmds = append(cmds, a.doEnableWorkflow(result.Data.(int64)))
			case "disable-workflow":
				cmds = append(cmds, a.doDisableWorkflow(result.Data.(int64)))
			case "bulk-delete-runs":
				wf := a.workflowsView.SelectedWorkflow()
				if wf != nil {
					a.status = fmt.Sprintf("Deleting runs for %s...", wf.Name)
					cmds = append(cmds, a.doBulkDeleteRuns(wf))
				}
			case "delete-selected-runs":
				ids := result.Data.([]int64)
				a.status = fmt.Sprintf("Deleting %d runs...", len(ids))
				a.runsView.ClearSelection()
				cmds = append(cmds, a.doBulkDeleteByIDs(ids))
			case "delete-selected-caches":
				ids := result.Data.([]int64)
				a.status = fmt.Sprintf("Deleting %d caches...", len(ids))
				a.cacheView.ClearSelection()
				cmds = append(cmds, a.deleteSelectedCaches(ids))
			case "delete-cache-entry":
				if entry := a.cacheView.SelectedEntry(); entry != nil {
					a.status = "Deleting cache..."
					cmds = append(cmds, a.deleteActionsCache(entry.ID))
				}
			case "clear-all-caches":
				a.status = "Deleting all caches..."
				cmds = append(cmds, a.deleteAllActionsCaches())
			}
		}
		return &a, tea.Batch(cmds...)
	}

	// Handle confirmation dialog input (key events while dialog is showing)
	if a.confirmDialog.IsActive() {
		var cmd tea.Cmd
		a.confirmDialog, cmd = a.confirmDialog.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return &a, tea.Batch(cmds...)
	}

	// Handle filter overlay result
	if result, ok := msg.(filteroverlay.ResultMsg); ok {
		if result.Applied {
			a.runsFilter = result.Filter
			a.runsPage = 1
			a.runsView = runs.New()
			a.propagateSize()
			a.status = "Loading runs..."
			cmds = append(cmds, a.fetchRuns())
		}
		return &a, tea.Batch(cmds...)
	}

	// Handle filter overlay input (key events while overlay is showing)
	if a.filterOverlay.IsActive() {
		var cmd tea.Cmd
		a.filterOverlay, cmd = a.filterOverlay.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return &a, tea.Batch(cmds...)
	}

	// Handle search input/results mode
	if a.searchView.IsActive() {
		var cmd tea.Cmd
		a.searchView, cmd = a.searchView.Update(msg)
		cmds = append(cmds, cmd)

		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			if a.searchView.IsInputMode() {
				// Input mode: dispatch search
				query := a.searchView.Query()
				if query != "" {
					if len(a.currentRunLogs) > 0 {
						cmds = append(cmds, a.executeSearch(query))
					} else {
						a.status = "No logs available yet (jobs may still be running)"
					}
				}
			} else {
				// Results mode: jump to the selected match's log
				if match := a.searchView.SelectedMatch(); match != nil {
					jobName := match.JobName
					line := match.Line
					content, ok := a.currentRunLogs[jobName]
					if !ok {
						// Fallback: partial match (zip dir names may differ)
						for k, v := range a.currentRunLogs {
							if strings.Contains(k, jobName) || strings.Contains(jobName, k) {
								content = v
								ok = true
								break
							}
						}
					}
					if ok {
						a.searchView.Deactivate()
						a.logView.SetContent(jobName, content)
						a.logView.GotoLine(line)
						a.logFullScreen = true
						a.cameFromSearch = true
						a.propagateSize()
					}
				}
			}
		}

		return &a, tea.Batch(cmds...)
	}

	// Handle full-screen log search mode: keys go directly to log view,
	// skip app-level handlers (quit, tab switching, etc.)
	if _, isKey := msg.(tea.KeyMsg); isKey && a.logFullScreen && a.logView.IsSearching() {
		var cmd tea.Cmd
		a.logView, cmd = a.logView.Update(msg)
		cmds = append(cmds, cmd)
		return &a, tea.Batch(cmds...)
	}

	// Handle list filter mode: keys go directly to the filtering list,
	// skip app-level handlers (tab switching, quit, etc.)
	if _, isKey := msg.(tea.KeyMsg); isKey && a.isListFiltering() {
		var cmd tea.Cmd
		switch a.currentView {
		case ViewRuns:
			a.runsView, cmd = a.runsView.Update(msg)
		case ViewWorkflows:
			a.workflowsView, cmd = a.workflowsView.Update(msg)
		case ViewCache:
			a.cacheView, cmd = a.cacheView.Update(msg)
		case ViewRunners:
			a.runnersView, cmd = a.runnersView.Update(msg)
		}
		cmds = append(cmds, cmd)
		return &a, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.propagateSize()
		if a.helpReady {
			contentH := a.height - 5
			if contentH < 1 {
				contentH = 1
			}
			a.helpViewport.Width = a.width - 4
			a.helpViewport.Height = contentH - 2 // reserve header line + border
			a.helpViewport.SetContent(a.renderHelp())
		}

	case tea.KeyMsg:
		// Help overlay: scroll keys pass to viewport, close keys dismiss
		if a.showHelp {
			switch msg.String() {
			case "esc", "?", "q":
				a.showHelp = false
				return &a, nil
			default:
				var cmd tea.Cmd
				a.helpViewport, cmd = a.helpViewport.Update(msg)
				return &a, cmd
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return &a, tea.Quit

		case "?":
			contentH := a.height - 5
			if contentH < 1 {
				contentH = 1
			}
			vpH := contentH - 2 // reserve header line + border
			if vpH < 1 {
				vpH = 1
			}
			if !a.helpReady {
				a.helpViewport = viewport.New(a.width-4, vpH)
				a.helpReady = true
			} else {
				a.helpViewport.Width = a.width - 4
				a.helpViewport.Height = vpH
			}
			a.helpViewport.SetContent(a.renderHelp())
			a.helpViewport.GotoTop()
			a.showHelp = true
			return &a, nil

		case "tab":
			if a.currentView == ViewRuns && !a.logFullScreen {
				if a.focusedPane == PaneLeft {
					a.focusedPane = PaneMiddle
				} else {
					a.focusedPane = PaneLeft
				}
			}
		case "shift+tab":
			if a.currentView == ViewRuns && !a.logFullScreen {
				if a.focusedPane == PaneMiddle {
					a.focusedPane = PaneLeft
				} else {
					a.focusedPane = PaneMiddle
				}
			}

		case "1", "2", "3", "4", "5":
			// Stop tailing when switching tabs
			if a.tailingJobID > 0 {
				a.tailingJobID = 0
				a.tailingJobName = ""
				a.logView.SetTailing(false)
			}
			a.logFullScreen = false
			a.infoFullScreen = false
			switch msg.String() {
			case "1":
				if a.currentView != ViewRuns {
					a.currentView = ViewRuns
					a.focusedPane = PaneLeft
					a.status = a.runsPageStatus()
					a.propagateSize()
				}
			case "2":
				if a.currentView != ViewWorkflows {
					a.currentView = ViewWorkflows
					a.focusedPane = PaneLeft
					a.status = "Workflows"
					cmds = append(cmds, a.fetchWorkflows())
				}
			case "3":
				if a.currentView != ViewMetrics {
					a.currentView = ViewMetrics
					a.focusedPane = PaneLeft
					a.status = "Loading metrics..."
					cmds = append(cmds, a.fetchDashboardData(a.dashboardView.Window()))
				}
			case "4":
				if a.currentView != ViewCache {
					a.currentView = ViewCache
					a.focusedPane = PaneLeft
					a.status = "Loading caches..."
					cmds = append(cmds, a.fetchActionsCaches())
				}
			case "5":
				if a.currentView != ViewRunners {
					a.currentView = ViewRunners
					a.focusedPane = PaneLeft
					a.status = "Loading runners..."
					cmds = append(cmds, a.fetchRunners())
				}
			}

		case "S":
			if a.currentView == ViewRuns && !a.logFullScreen && !a.infoFullScreen {
				a.filterOverlay = filteroverlay.New(a.workflows, a.runsFilter)
				a.filterOverlay.SetSize(a.width, a.height)
			}

		case "/":
			if a.currentView == ViewRuns && !a.logFullScreen {
				a.searchView.Activate()
			}

		case "enter":
			if a.currentView == ViewRuns && a.focusedPane == PaneLeft {
				if !a.runsView.IsFiltering() {
					if run := a.runsView.SelectedRun(); run != nil {
						a.viewingAttempt = 0
						a.detailsView.SetRun(run)
						a.focusedPane = PaneMiddle
						a.status = fmt.Sprintf("Loading jobs for #%d...", run.RunNumber)
						cmds = append(cmds, a.fetchJobs(run.ID))
						// Download run-level log archive (contains logs for completed jobs).
						// For in-progress runs, still-running job logs are fetched on demand.
						cmds = append(cmds, a.fetchLogs(run))
						if run.Status == model.RunStatusCompleted {
							a.autoRefreshRunID = 0
						} else {
							a.autoRefreshRunID = run.ID
						}
					}
				}
			} else if a.currentView == ViewWorkflows {
				if !a.workflowsView.IsFiltering() {
					if wf := a.workflowsView.SelectedWorkflow(); wf != nil {
						a.runsFilter = filteroverlay.FilterResult{
							WorkflowID:   wf.ID,
							WorkflowName: wf.Name,
						}
						a.currentView = ViewRuns
						a.runsView = runs.New()
						a.focusedPane = PaneLeft
						a.propagateSize()
						a.status = fmt.Sprintf("Loading runs for %s...", wf.Name)
						cmds = append(cmds, a.fetchRuns())
					}
				}
			} else if a.currentView == ViewRuns && a.focusedPane == PaneMiddle {
				if job := a.detailsView.SelectedJob(); job != nil {
					content, ok := a.findJobLog(job.Name)
					if job.Status != model.RunStatusCompleted {
						// In-progress job: show step progress from what we have, start tailing
						a.logView.SetContent(job.Name, renderStepProgress(job))
						a.logFullScreen = true
						a.propagateSize()
						a.tailingJobID = job.ID
						a.tailingJobName = job.Name
						a.logView.SetTailing(true)
						a.viewingJob = job
						a.status = fmt.Sprintf("Watching %s...", job.Name)
						cmds = append(cmds, a.checkJobStatus(job.ID, job.Name))
					} else if ok {
						a.logView.SetContent(job.Name, content)
						a.logFullScreen = true
						a.propagateSize()
						a.viewingJob = job
						// Jump to failed step if the job failed
						if stepName := firstFailedStepName(job); stepName != "" {
							if line := findFailedStepLine(content, stepName); line > 0 {
								a.logView.GotoLine(line)
							}
						}
					} else {
						// Completed job, no cached log -- fetch via per-job API.
						a.logView.SetLoading()
						a.logFullScreen = true
						a.propagateSize()
						a.viewingJob = job
						a.status = fmt.Sprintf("Fetching log for %s...", job.Name)
						cmds = append(cmds, a.fetchJobLog(job.ID, job.Name))
					}

					}
			}

		case "right", "l":
			if a.currentView == ViewRuns && a.focusedPane == PaneLeft && a.runsHasMore && !a.runsLoading {
				a.runsLoading = true
				a.status = fmt.Sprintf("Loading page %d...", a.runsPage+1)
				cmds = append(cmds, a.fetchRunsPage(a.runsPage+1))
			}
		case "left", "h":
			if a.currentView == ViewRuns && a.focusedPane == PaneLeft && a.runsPage > 1 && !a.runsLoading {
				a.runsLoading = true
				a.status = fmt.Sprintf("Loading page %d...", a.runsPage-1)
				cmds = append(cmds, a.fetchRunsPage(a.runsPage-1))
			}

		case "r":
			if a.currentView == ViewRuns {
				cmds = append(cmds, a.fetchRuns())
				a.status = "Refreshing runs..."
			} else if a.currentView == ViewWorkflows {
				cmds = append(cmds, a.fetchWorkflows())
				a.status = "Refreshing workflows..."
			} else if a.currentView == ViewCache {
				cmds = append(cmds, a.fetchActionsCaches())
				a.status = "Refreshing caches..."
			} else if a.currentView == ViewRunners {
				cmds = append(cmds, a.fetchRunners())
				a.status = "Refreshing runners..."
			}

		case "R":
			if a.currentView == ViewRuns {
				if run := a.runsView.SelectedRun(); run != nil {
					a.confirmDialog = confirm.New(
						"Rerun All Jobs",
						fmt.Sprintf("Rerun all jobs for run #%d (%s)?", run.RunNumber, run.DisplayTitle),
						"rerun-all", run.ID,
					)
				}
			}
		case "F":
			if a.currentView == ViewRuns {
				if run := a.runsView.SelectedRun(); run != nil {
					a.confirmDialog = confirm.New(
						"Rerun Failed Jobs",
						fmt.Sprintf("Rerun failed jobs for run #%d?", run.RunNumber),
						"rerun-failed", run.ID,
					)
				}
			}
		case "d":
			if a.currentView == ViewRuns {
				if count := a.runsView.SelectionCount(); count > 0 {
					a.confirmDialog = confirm.New(
						"Delete Selected Runs",
						fmt.Sprintf("Delete %d selected runs? This cannot be undone.", count),
						"delete-selected-runs", a.runsView.SelectedRuns(),
					)
				} else if run := a.runsView.SelectedRun(); run != nil {
					a.confirmDialog = confirm.New(
						"Delete Run",
						fmt.Sprintf("Delete run #%d? This cannot be undone.", run.RunNumber),
						"delete-run", run.ID,
					)
				}
			} else if a.currentView == ViewWorkflows {
				if wf := a.workflowsView.SelectedWorkflow(); wf != nil {
					a.confirmDialog = confirm.New(
						"Bulk Delete Runs",
						fmt.Sprintf("Delete ALL runs for workflow '%s'? This cannot be undone.", wf.Name),
						"bulk-delete-runs", wf.ID,
					)
				}
			} else if a.currentView == ViewCache {
				if count := a.cacheView.SelectionCount(); count > 0 {
					a.confirmDialog = confirm.New(
						"Delete Selected Caches",
						fmt.Sprintf("Delete %d selected caches? This cannot be undone.", count),
						"delete-selected-caches", a.cacheView.SelectedCaches(),
					)
				} else if entry := a.cacheView.SelectedEntry(); entry != nil {
					keyPreview := entry.Key
					if len(keyPreview) > 60 {
						keyPreview = keyPreview[:57] + "..."
					}
					a.confirmDialog = confirm.New(
						"Delete Cache",
						fmt.Sprintf("Delete cache '%s'?", keyPreview),
						"delete-cache-entry", nil,
					)
				}
			}
		case "C":
			if a.currentView == ViewRuns {
				if run := a.runsView.SelectedRun(); run != nil {
					a.confirmDialog = confirm.New(
						"Cancel Run",
						fmt.Sprintf("Cancel run #%d?", run.RunNumber),
						"cancel-run", run.ID,
					)
				}
			}
		case "X":
			if a.currentView == ViewRuns {
				if run := a.runsView.SelectedRun(); run != nil {
					a.confirmDialog = confirm.New(
						"Force Cancel Run",
						fmt.Sprintf("Force cancel run #%d? Use only if regular cancel failed.", run.RunNumber),
						"force-cancel-run", run.ID,
					)
				}
			}
		case "a":
			if a.currentView == ViewRuns && a.logFullScreen && !a.infoFullScreen {
				// Attempt cycling while viewing a job log
				if run := a.detailsView.Run(); run != nil && run.RunAttempt > 1 && a.viewingJob != nil {
					a.viewingAttempt++
					if a.viewingAttempt > run.RunAttempt {
						a.viewingAttempt = 0
					}
					a.detailsView.SetAttempt(a.viewingAttempt, run.RunAttempt)

					if a.viewingAttempt == 0 {
						a.status = "Loading latest (merged) logs..."
						cmds = append(cmds, a.fetchLogs(run))
					} else {
						a.status = fmt.Sprintf("Loading attempt %d logs...", a.viewingAttempt)
						cmds = append(cmds, a.fetchLogsForAttempt(run, a.viewingAttempt))
					}
				}
			} else if a.currentView == ViewRuns && a.focusedPane == PaneMiddle && !a.logFullScreen && !a.infoFullScreen {
				// Attempt cycling in jobs pane
				if run := a.detailsView.Run(); run != nil && run.RunAttempt > 1 {
					a.viewingAttempt++
					if a.viewingAttempt > run.RunAttempt {
						a.viewingAttempt = 0
					}
					a.detailsView.SetAttempt(a.viewingAttempt, run.RunAttempt)

					if a.viewingAttempt == 0 {
						a.status = "Loading latest (merged) jobs..."
						cmds = append(cmds, a.fetchJobs(run.ID))
						cmds = append(cmds, a.fetchLogs(run))
					} else {
						a.status = fmt.Sprintf("Loading attempt %d...", a.viewingAttempt)
						cmds = append(cmds, a.fetchJobsForAttempt(run.ID, a.viewingAttempt))
						cmds = append(cmds, a.fetchLogsForAttempt(run, a.viewingAttempt))
					}
				}
			}

		case "i":
			if a.currentView == ViewRuns && !a.logFullScreen && !a.infoFullScreen {
				if a.focusedPane == PaneMiddle {
					if job := a.detailsView.SelectedJob(); job != nil {
						a.infoView.SetJob(job)
						a.infoFullScreen = true
						a.propagateSize()
					}
				} else {
					if run := a.runsView.SelectedRun(); run != nil {
						a.infoView.SetRun(run)
						a.infoFullScreen = true
						a.propagateSize()
						cmds = append(cmds, a.fetchJobs(run.ID))
						if run.Status != model.RunStatusCompleted {
							if a.autoRefreshRunID == 0 {
								a.infoStartedRefresh = true
								a.autoRefreshRunID = run.ID
								cmds = append(cmds, a.scheduleJobsRefresh(run.ID))
							}
						}
					}
				}
			}

		case "e":
			if a.currentView == ViewWorkflows {
				if wf := a.workflowsView.SelectedWorkflow(); wf != nil {
					a.confirmDialog = confirm.New(
						"Enable Workflow",
						fmt.Sprintf("Enable workflow '%s'?", wf.Name),
						"enable-workflow", wf.ID,
					)
				}
			}
		case "D":
			if a.currentView == ViewWorkflows {
				if wf := a.workflowsView.SelectedWorkflow(); wf != nil {
					a.confirmDialog = confirm.New(
						"Disable Workflow",
						fmt.Sprintf("Disable workflow '%s'?", wf.Name),
						"disable-workflow", wf.ID,
					)
				}
			}
		case "x":
			if a.currentView == ViewWorkflows {
				if wf := a.workflowsView.SelectedWorkflow(); wf != nil {
					a.confirmDialog = confirm.New(
						"Bulk Delete Runs",
						fmt.Sprintf("Delete ALL runs for workflow '%s'? This cannot be undone.", wf.Name),
						"bulk-delete-runs", wf.ID,
					)
				}
			} else if a.currentView == ViewCache {
				a.confirmDialog = confirm.New(
					"Clear All Caches",
					"Delete ALL GitHub Actions caches? This cannot be undone.",
					"clear-all-caches", nil,
				)
			}
		}

	case runs.NeedNextPageMsg:
		if a.currentView == ViewRuns && a.runsHasMore && !a.runsLoading {
			a.runsLoading = true
			a.status = fmt.Sprintf("Loading page %d...", a.runsPage+1)
			cmds = append(cmds, a.fetchRunsPage(a.runsPage+1))
		}

	case ui.ActionResultMsg:
		if msg.Err != nil {
			a.status = fmt.Sprintf("Error: %v", msg.Err)
		} else {
			a.status = fmt.Sprintf("%s: success", msg.Action)
			if a.currentView == ViewWorkflows {
				cmds = append(cmds, a.fetchWorkflows())
			} else {
				cmds = append(cmds, a.fetchRuns())
			}
		}

	case ui.WorkflowsLoadedMsg:
		if msg.Err == nil {
			a.workflows = msg.Workflows
			a.status = fmt.Sprintf("%d workflows", len(msg.Workflows))
			cmds = append(cmds, a.fetchWorkflowStats(msg.Workflows))
		} else {
			a.status = fmt.Sprintf("Error: %v", msg.Err)
		}

	case ui.WorkflowStatsMsg:
		// Forward to workflows view (tab 2) to update stats display
		var cmd tea.Cmd
		a.workflowsView, cmd = a.workflowsView.Update(msg)
		cmds = append(cmds, cmd)

	case ui.RunsLoadedMsg:
		if msg.Err == nil {
			a.runsPage = 1
			a.runsTotalCount = msg.TotalCount
			a.runsHasMore = len(msg.Runs) >= runsPerPage
			a.runsLoading = false
			a.status = a.runsPageStatus()
		} else {
			a.runsLoading = false
			a.status = fmt.Sprintf("Error: %v", msg.Err)
		}
		cmds = append(cmds, a.scheduleRunsRefresh())

	case ui.RunsPageMsg:
		a.runsLoading = false
		if msg.Err == nil {
			a.runsPage = msg.Page
			a.runsTotalCount = msg.TotalCount
			a.runsHasMore = len(msg.Runs) >= runsPerPage
			a.status = a.runsPageStatus()
		} else {
			a.status = fmt.Sprintf("Error loading page: %v", msg.Err)
		}
		cmds = append(cmds, a.scheduleRunsRefresh())

	case ui.JobsLoadedMsg:
		if msg.Err == nil {
			a.status = fmt.Sprintf("%d jobs loaded", len(msg.Jobs))
			// Feed jobs to info view if it's showing this run
			if a.infoFullScreen && a.infoView.Run() != nil && a.infoView.Run().ID == msg.RunID {
				a.infoView.SetJobs(msg.Jobs)
			}
			// Auto-refresh: schedule next tick if any job is still running
			if a.autoRefreshRunID == msg.RunID {
				allDone := true
				for _, j := range msg.Jobs {
					if j.Status != model.RunStatusCompleted {
						allDone = false
						break
					}
				}
				if allDone {
					a.autoRefreshRunID = 0
					a.status = fmt.Sprintf("%d jobs — all completed", len(msg.Jobs))
					// Run is done, now fetch the full log archive.
					// We need the run to write cache metadata; find it from runsView.
					if run := a.runsView.RunByID(msg.RunID); run != nil {
						cmds = append(cmds, a.fetchLogs(run))
					}
				} else {
					cmds = append(cmds, a.scheduleJobsRefresh(msg.RunID))
				}
			}
		} else {
			a.status = fmt.Sprintf("Error loading jobs: %v", msg.Err)
		}

	case ui.RunsTickMsg:
		if a.currentView == ViewRuns && !a.runsLoading {
			cmds = append(cmds, a.refreshCurrentRuns())
		} else {
			// Not on runs tab right now, keep ticking
			cmds = append(cmds, a.scheduleRunsRefresh())
		}

	case ui.RunsRefreshedMsg:
		if msg.Err == nil {
			a.runsTotalCount = msg.TotalCount
			a.runsHasMore = len(msg.Runs) >= runsPerPage
		}
		cmds = append(cmds, a.scheduleRunsRefresh())

	case ui.JobsTickMsg:
		if a.autoRefreshRunID == msg.RunID && a.currentView == ViewRuns {
			cmds = append(cmds, a.fetchJobs(msg.RunID))
			if a.infoFullScreen {
				cmds = append(cmds, a.fetchRun(msg.RunID))
			}
		}

	case ui.LogTailTickMsg:
		if a.tailingJobID == msg.JobID && a.logFullScreen {
			cmds = append(cmds, a.checkJobStatus(msg.JobID, msg.JobName))
		}

	case ui.JobTailStatusMsg:
		if a.tailingJobID == msg.JobID {
			if msg.Err != nil {
				a.logView.UpdateContent(fmt.Sprintf("\n  Error fetching job status: %v\n\n  Retrying...", msg.Err))
				a.status = fmt.Sprintf("Error watching %s, retrying...", msg.JobName)
				cmds = append(cmds, a.scheduleLogRefresh(msg.JobID, msg.JobName))
			} else if msg.Completed {
				// Job done — update viewingJob with final step data, fetch logs
				a.tailingJobID = 0
				a.tailingJobName = ""
				a.logView.SetTailing(false)
				a.logView.SetLoading()
				if msg.Job != nil {
					a.viewingJob = msg.Job
				}
				a.status = fmt.Sprintf("Job %s completed — loading logs...", msg.JobName)
				cmds = append(cmds, a.fetchJobLog(msg.JobID, msg.JobName))
			} else if msg.Job != nil {
				// Still running — render step progress and schedule next tick
				a.logView.UpdateContent(renderStepProgress(msg.Job))
				a.status = fmt.Sprintf("Watching %s...", msg.JobName)
				cmds = append(cmds, a.scheduleLogRefresh(msg.JobID, msg.JobName))
			}
		}

	case ui.RunLoadedMsg:
		if msg.Err == nil && msg.Run != nil {
			if a.infoFullScreen && a.infoView.Run() != nil && a.infoView.Run().ID == msg.RunID {
				a.infoView.SetRun(msg.Run)
			}
		}

	case ui.JobLogLoadedMsg:
		if msg.Err == nil {
			// Cache the individual job log
			if a.currentRunLogs == nil {
				a.currentRunLogs = make(map[string]string)
			}
			a.currentRunLogs[msg.JobName] = msg.Content
			// If we're in full-screen log view waiting for this, show it
			if a.logFullScreen {
				a.logView.SetContent(msg.JobName, msg.Content)
				// Jump to failed step if the job failed
				if a.viewingJob != nil && a.viewingJob.Name == msg.JobName {
					if stepName := firstFailedStepName(a.viewingJob); stepName != "" {
						if line := findFailedStepLine(msg.Content, stepName); line > 0 {
							a.logView.GotoLine(line)
						}
					}
				}
			}
			a.status = fmt.Sprintf("Log loaded for %s", msg.JobName)
		} else {
			a.status = fmt.Sprintf("Error loading log: %v", msg.Err)
		}

	case ui.LogsLoadedMsg:
		if msg.Err == nil {
			a.currentRunLogs = msg.Logs
			a.currentRunID = msg.RunID
			a.status = fmt.Sprintf("Logs loaded (%d jobs)", len(msg.Logs))
			// If viewing a job log, refresh with new attempt's content
			if a.logFullScreen && a.viewingJob != nil && a.tailingJobID == 0 {
				displayName := a.viewingJob.Name
				if a.viewingAttempt > 0 {
					displayName = fmt.Sprintf("%s (attempt %d)", a.viewingJob.Name, a.viewingAttempt)
				}
				if content, ok := a.findJobLog(a.viewingJob.Name); ok && !isSystemStub(content) {
					a.logView.SetContent(displayName, content)
				} else {
					noLogMsg := "\n  No logs for this job in the selected attempt.\n"
					if a.viewingAttempt > 0 {
						noLogMsg = fmt.Sprintf("\n  This job did not run in attempt %d.\n  Press 'a' to switch attempts.\n", a.viewingAttempt)
					}
					a.logView.SetContent(displayName, noLogMsg)
				}
			}
		} else {
			a.status = fmt.Sprintf("Error loading logs: %v", msg.Err)
		}

	case ui.RetentionLoadedMsg:
		if msg.Err == nil && msg.RetentionDays > 0 {
			windows := dashboard.WindowsForRetention(msg.RetentionDays)
			a.dashboardView.SetWindows(windows)
		}

	case dashboard.WindowChangedMsg:
		a.status = fmt.Sprintf("Loading metrics (%s)...", msg.Window.Label)
		cmds = append(cmds, a.fetchDashboardData(msg.Window))

	case ui.DashboardDataMsg:
		if msg.Err == nil {
			metrics := dashboard.ComputeMetrics(msg.Runs, msg.Jobs, msg.TotalCount)
			a.dashboardView.SetMetrics(&metrics)
			a.status = fmt.Sprintf("Metrics: %d runs (%s)", metrics.TotalRuns, a.dashboardView.Window().Label)
		} else {
			a.status = fmt.Sprintf("Error loading metrics: %v", msg.Err)
		}

	case ui.ActionsCachesLoadedMsg:
		if msg.Err == nil {
			total := int64(0)
			for _, c := range msg.Caches {
				total += c.SizeInBytes
			}
			a.status = fmt.Sprintf("%d caches (%.1f MB)", len(msg.Caches), float64(total)/(1024*1024))
		} else {
			a.status = fmt.Sprintf("Error loading caches: %v", msg.Err)
		}

	case ui.ActionsCacheDeletedMsg:
		if msg.Err == nil {
			a.status = "Cache deleted"
			cmds = append(cmds, a.fetchActionsCaches())
		} else {
			a.status = fmt.Sprintf("Error deleting cache: %v", msg.Err)
		}

	case ui.RunnersLoadedMsg:
		if msg.Err == nil {
			a.status = fmt.Sprintf("%d runners", len(msg.Runners))
		} else {
			a.status = fmt.Sprintf("Error loading runners: %v", msg.Err)
		}

	case ui.SearchDoneMsg:
		if msg.Err == nil && msg.Results != nil {
			a.status = fmt.Sprintf("Search: %d matches across %d jobs",
				msg.Results.TotalCount, len(msg.Results.JobCounts))
		}

	case ui.StatusMsg:
		a.status = msg.Text
	}

	// Propagate to active sub-views.
	// Skip WindowSizeMsg — handled by propagateSize() with correct per-pane dimensions.
	if _, isResize := msg.(tea.WindowSizeMsg); !isResize {
		switch a.currentView {
		case ViewRuns:
			if a.logFullScreen {
				// Full-screen log mode: keys go to log view, esc exits
				if keyMsg, ok := msg.(tea.KeyMsg); ok {
					isExit := keyMsg.String() == "esc" || keyMsg.String() == "backspace" || keyMsg.String() == "delete"
					if isExit && !a.logView.IsSearching() {
						a.logFullScreen = false
						// Stop any active tailing
						a.tailingJobID = 0
						a.tailingJobName = ""
						a.viewingJob = nil
						a.logView.SetTailing(false)
						if a.cameFromSearch {
							a.cameFromSearch = false
							a.searchView.ActivateResults()
							a.focusedPane = PaneLeft
						} else {
							a.focusedPane = PaneMiddle
						}
						a.propagateSize()
					} else {
						var cmd tea.Cmd
						a.logView, cmd = a.logView.Update(msg)
						cmds = append(cmds, cmd)
					}
				} else {
					var cmd tea.Cmd
					a.runsView, cmd = a.runsView.Update(msg)
					cmds = append(cmds, cmd)
					a.detailsView, cmd = a.detailsView.Update(msg)
					cmds = append(cmds, cmd)
					a.logView, cmd = a.logView.Update(msg)
					cmds = append(cmds, cmd)
				}
			} else if a.infoFullScreen {
				// Full-screen info mode: esc exits, other keys go to info view
				if keyMsg, ok := msg.(tea.KeyMsg); ok {
					isExit := keyMsg.String() == "esc" || keyMsg.String() == "backspace" || keyMsg.String() == "delete"
					if isExit {
						a.infoFullScreen = false
						if a.infoView.IsShowingJob() {
							a.focusedPane = PaneMiddle
						} else {
							a.focusedPane = PaneLeft
						}
						if a.infoStartedRefresh {
							a.autoRefreshRunID = 0
							a.infoStartedRefresh = false
						}
						a.propagateSize()
					} else {
						var cmd tea.Cmd
						a.infoView, cmd = a.infoView.Update(msg)
						cmds = append(cmds, cmd)
					}
				} else {
					// Non-key messages propagate to info view and other views
					var cmd tea.Cmd
					a.infoView, cmd = a.infoView.Update(msg)
					cmds = append(cmds, cmd)
					a.runsView, cmd = a.runsView.Update(msg)
					cmds = append(cmds, cmd)
					a.detailsView, cmd = a.detailsView.Update(msg)
					cmds = append(cmds, cmd)
				}
			} else {
				hadFilter := a.runsView.HasActiveFilter()
				_, isKey := msg.(tea.KeyMsg)

				if isKey {
					// Key events go ONLY to the focused pane.
					var cmd tea.Cmd
					switch a.focusedPane {
					case PaneLeft:
						a.runsView, cmd = a.runsView.Update(msg)
					case PaneMiddle:
						a.detailsView, cmd = a.detailsView.Update(msg)
					}
					cmds = append(cmds, cmd)
				} else {
					// Data messages (RunsLoaded, JobsLoaded, etc.) go to all.
					var cmd tea.Cmd
					a.runsView, cmd = a.runsView.Update(msg)
					cmds = append(cmds, cmd)
					a.detailsView, cmd = a.detailsView.Update(msg)
					cmds = append(cmds, cmd)
					a.logView, cmd = a.logView.Update(msg)
					cmds = append(cmds, cmd)
				}

				// esc navigation: mid->left
				if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
					if a.focusedPane == PaneMiddle {
						a.focusedPane = PaneLeft
					} else if a.focusedPane == PaneLeft && !hadFilter {
						// Clear filter if active, otherwise do nothing
						if !a.runsFilter.IsEmpty() {
							a.runsFilter = filteroverlay.FilterResult{}
							a.runsView = runs.New()
							a.propagateSize()
							a.status = "Loading runs..."
							cmds = append(cmds, a.fetchRuns())
						}
					}
				}
			}
		case ViewWorkflows:
			var cmd tea.Cmd
			a.workflowsView, cmd = a.workflowsView.Update(msg)
			cmds = append(cmds, cmd)
			// Forward runs data messages so they're not lost when on another tab
			switch msg.(type) {
			case ui.RunsLoadedMsg, ui.RunsPageMsg, ui.RunsRefreshedMsg:
				a.runsView, cmd = a.runsView.Update(msg)
				cmds = append(cmds, cmd)
			}
		case ViewMetrics:
			var cmd tea.Cmd
			a.dashboardView, cmd = a.dashboardView.Update(msg)
			cmds = append(cmds, cmd)
			switch msg.(type) {
			case ui.RunsLoadedMsg, ui.RunsPageMsg, ui.RunsRefreshedMsg:
				a.runsView, cmd = a.runsView.Update(msg)
				cmds = append(cmds, cmd)
			}
		case ViewCache:
			var cmd tea.Cmd
			a.cacheView, cmd = a.cacheView.Update(msg)
			cmds = append(cmds, cmd)
			switch msg.(type) {
			case ui.RunsLoadedMsg, ui.RunsPageMsg, ui.RunsRefreshedMsg:
				a.runsView, cmd = a.runsView.Update(msg)
				cmds = append(cmds, cmd)
			}
		case ViewRunners:
			var cmd tea.Cmd
			a.runnersView, cmd = a.runnersView.Update(msg)
			cmds = append(cmds, cmd)
			switch msg.(type) {
			case ui.RunsLoadedMsg, ui.RunsPageMsg, ui.RunsRefreshedMsg:
				a.runsView, cmd = a.runsView.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return &a, tea.Batch(cmds...)
}

func (a App) executeSearch(pattern string) tea.Cmd {
	logs := a.currentRunLogs
	runID := a.currentRunID
	return func() tea.Msg {
		eng := search.New()
		isRegex := false
		if len(pattern) > 1 && pattern[0] == '/' {
			isRegex = true
		}
		p := pattern
		if isRegex {
			p = pattern[1:]
		}
		query := model.SearchQuery{
			Pattern: p,
			IsRegex: isRegex,
		}
		results := eng.Search(logs, query, runID, 1)
		return ui.SearchDoneMsg{Results: results}
	}
}

func (a App) runsPageStatus() string {
	totalPages := (a.runsTotalCount + runsPerPage - 1) / runsPerPage
	if totalPages < 1 {
		totalPages = 1
	}

	filterInfo := ""
	if summary := a.runsFilter.Summary(); summary != "" {
		filterInfo = summary
	}

	if totalPages <= 1 {
		if filterInfo != "" {
			return fmt.Sprintf("%d runs (%s)", a.runsTotalCount, filterInfo)
		}
		return fmt.Sprintf("%d runs", a.runsTotalCount)
	}
	if filterInfo != "" {
		return fmt.Sprintf("Page %d/%d  |  %d runs  |  %s  |  <-/->: page", a.runsPage, totalPages, a.runsTotalCount, filterInfo)
	}
	return fmt.Sprintf("Page %d/%d  |  %d runs  |  <-/->: page", a.runsPage, totalPages, a.runsTotalCount)
}

func (a App) isListFiltering() bool {
	switch a.currentView {
	case ViewRuns:
		return a.runsView.IsFiltering()
	case ViewWorkflows:
		return a.workflowsView.IsFiltering()
	case ViewCache:
		return a.cacheView.IsFiltering()
	case ViewRunners:
		return a.runnersView.IsFiltering()
	}
	return false
}

func (a *App) propagateSize() {
	// Total vertical budget:
	//   header(1) + tabs(1) + status(1) = 3 lines of chrome
	//   pane border top(1) + bottom(1) = 2 lines
	//   Inner content height = terminal height - 5
	contentH := a.height - 5
	if contentH < 1 {
		contentH = 1
	}

	// 2-pane layout: each border = 2 chars horizontal, 2 panes = 4
	leftW := a.width * 45 / 100
	midW := a.width - leftW - 4
	if midW < 1 {
		midW = 1
	}

	a.runsView, _ = a.runsView.Update(
		tea.WindowSizeMsg{Width: leftW, Height: contentH})
	// Workflows tab is full-width (single pane, border = 2 chars horizontal)
	a.workflowsView, _ = a.workflowsView.Update(
		tea.WindowSizeMsg{Width: a.width - 4, Height: contentH})
	a.detailsView, _ = a.detailsView.Update(
		tea.WindowSizeMsg{Width: midW, Height: contentH})
	// Log view: always full width (shown as full-screen overlay)
	a.logView, _ = a.logView.Update(
		tea.WindowSizeMsg{Width: a.width - 4, Height: contentH})
	// Info view: always full width (shown as full-screen overlay)
	a.infoView, _ = a.infoView.Update(
		tea.WindowSizeMsg{Width: a.width - 4, Height: contentH})
	a.searchView, _ = a.searchView.Update(
		tea.WindowSizeMsg{Width: a.width - 4, Height: contentH})
	a.dashboardView, _ = a.dashboardView.Update(
		tea.WindowSizeMsg{Width: a.width - 4, Height: contentH})
	a.cacheView, _ = a.cacheView.Update(
		tea.WindowSizeMsg{Width: a.width - 4, Height: contentH})
	a.runnersView, _ = a.runnersView.Update(
		tea.WindowSizeMsg{Width: a.width - 4, Height: contentH})
}

// --- View ---

func (a App) View() string {
	header := RenderHeader(a.cfg.RepoNWO(), a.rateRemaining, a.rateLimit, a.width)
	tabs := a.renderTabs()

	var content string
	switch a.currentView {
	case ViewRuns:
		content = a.renderRunsLayout()
	case ViewWorkflows:
		contentH := a.height - 5
		if contentH < 1 {
			contentH = 1
		}
		style := ui.StylePaneFocused.Width(a.width - 2).Height(contentH)
		content = style.Render(a.workflowsView.View())
	case ViewMetrics:
		contentH := a.height - 5
		if contentH < 1 {
			contentH = 1
		}
		style := ui.StylePaneFocused.Width(a.width - 2).Height(contentH)
		content = style.Render(a.dashboardView.View())
	case ViewCache:
		contentH := a.height - 5
		if contentH < 1 {
			contentH = 1
		}
		style := ui.StylePaneFocused.Width(a.width - 2).Height(contentH)
		content = style.Render(a.cacheView.View())
	case ViewRunners:
		contentH := a.height - 5
		if contentH < 1 {
			contentH = 1
		}
		style := ui.StylePaneFocused.Width(a.width - 2).Height(contentH)
		content = style.Render(a.runnersView.View())
	}

	if a.showHelp {
		contentH := a.height - 5
		if contentH < 1 {
			contentH = 1
		}
		pct := a.helpViewport.ScrollPercent() * 100
		header := lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("#F9FAFB")).
			Render(fmt.Sprintf(" Help  %3.0f%%", pct))
		hints := lipgloss.NewStyle().Foreground(ui.ColorMuted).
			Render("  j/k:scroll  PgUp/PgDn:page  g/G:top/bot  esc:close")
		helpContent := header + hints + "\n" + a.helpViewport.View()
		style := ui.StylePaneFocused.Width(a.width - 2).Height(contentH)
		content = style.Render(helpContent)
	} else if a.confirmDialog.IsActive() {
		content = a.confirmDialog.View()
	} else if a.filterOverlay.IsActive() {
		content = a.filterOverlay.View()
	}

	statusBar := RenderStatusBar(a.status, a.contextHints(), a.width)

	// Hard clamp: ensure content never overflows the terminal.
	// header(1) + tabs(1) + statusbar(1) = 3 lines of chrome.
	maxContentLines := a.height - 3
	if maxContentLines > 0 {
		lines := strings.Split(content, "\n")
		if len(lines) > maxContentLines {
			lines = lines[:maxContentLines]
			content = strings.Join(lines, "\n")
		}
	}

	return header + "\n" + tabs + "\n" + content + "\n" + statusBar
}

func (a App) renderTabs() string {
	tabStyle := lipgloss.NewStyle().Padding(0, 2)
	activeTab := tabStyle.Bold(true).Foreground(ui.ColorPrimary)
	inactiveTab := tabStyle.Foreground(ui.ColorMuted)

	runsLabel := "[1] Runs"
	if summary := a.runsFilter.Summary(); summary != "" {
		runsLabel = fmt.Sprintf("[1] Runs (%s)", summary)
	}

	runsTab := inactiveTab.Render(runsLabel)
	wfTab := inactiveTab.Render("[2] Workflows")
	dashTab := inactiveTab.Render("[3] Metrics")
	cacheTab := inactiveTab.Render("[4] Cache")
	runnersTab := inactiveTab.Render("[5] Runners")

	switch a.currentView {
	case ViewRuns:
		runsTab = activeTab.Render(runsLabel)
	case ViewWorkflows:
		wfTab = activeTab.Render("[2] Workflows")
	case ViewMetrics:
		dashTab = activeTab.Render("[3] Metrics")
	case ViewCache:
		cacheTab = activeTab.Render("[4] Cache")
	case ViewRunners:
		runnersTab = activeTab.Render("[5] Runners")
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, runsTab, wfTab, dashTab, cacheTab, runnersTab)
}

// findJobLog looks up a job's log in currentRunLogs by exact name, then partial match.
func (a App) findJobLog(name string) (string, bool) {
	if content, ok := a.currentRunLogs[name]; ok {
		return content, true
	}
	for k, v := range a.currentRunLogs {
		if strings.Contains(k, name) || strings.Contains(name, k) {
			return v, true
		}
	}
	return "", false
}

// isSystemStub returns true if log content is just a system.txt stub
// (GitHub includes these for jobs that didn't re-run in a later attempt).
func isSystemStub(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return true
	}
	// Subdirectory fallback format: only has "=== system.txt ===" section
	if strings.HasPrefix(trimmed, "=== system.txt ===") {
		// Check if there's any other === section (actual step logs)
		rest := trimmed[len("=== system.txt ==="):]
		return !strings.Contains(rest, "=== ")
	}
	// Short content without actual log lines (just runner metadata)
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 15 {
		allSystem := true
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "=== ") {
				continue
			}
			// System metadata lines contain evaluation info, runner info
			if !strings.Contains(line, "Evaluating") &&
				!strings.Contains(line, "Expanded:") &&
				!strings.Contains(line, "Result:") &&
				!strings.Contains(line, "Waiting for") &&
				!strings.Contains(line, "Requested labels:") &&
				!strings.Contains(line, "Job defined at:") &&
				!strings.Contains(line, "Job is about to start") &&
				!strings.Contains(line, "runner") {
				allSystem = false
				break
			}
		}
		return allSystem
	}
	return false
}

func firstFailedStepName(job *model.Job) string {
	if job == nil {
		return ""
	}
	for _, s := range job.Steps {
		if s.Conclusion == model.ConclusionFailure {
			return s.Name
		}
	}
	return ""
}

// findFailedStepLine searches log content for the line where a failed step starts.
// GitHub job logs use "##[group]StepName" markers. Run-level logs use "=== stepfile ===" markers.
// Returns 1-based line number, or 0 if not found.
func findFailedStepLine(content, stepName string) int {
	if stepName == "" {
		return 0
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Per-job log format: "2024-... ##[group]Step Name"
		if strings.Contains(line, "##[group]"+stepName) {
			return i + 1
		}
		// Run-level cached format: "=== N_Step Name.txt ==="
		if strings.Contains(line, "=== ") && strings.Contains(line, stepName) {
			return i + 1
		}
	}
	// Fallback: search for step name as a substring anywhere
	for i, line := range lines {
		if strings.Contains(line, stepName) {
			return i + 1
		}
	}
	return 0
}

// renderStepProgress renders a live step-by-step progress view for an in-progress job.
func renderStepProgress(job *model.Job) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n  Job: %s\n", job.Name))
	if job.RunnerName != "" {
		b.WriteString(fmt.Sprintf("  Runner: %s\n", job.RunnerName))
	}
	if !job.StartedAt.IsZero() {
		elapsed := time.Since(job.StartedAt).Truncate(time.Second)
		b.WriteString(fmt.Sprintf("  Elapsed: %s\n", elapsed))
	}
	b.WriteString("\n  Steps:\n\n")

	for _, step := range job.Steps {
		icon := ui.StatusIcon(string(step.Conclusion))
		if step.Status == model.RunStatusInProgress {
			icon = ui.StatusIcon("in_progress")
		} else if step.Status == model.RunStatusQueued {
			icon = ui.StatusIcon("queued")
		}

		dur := ""
		if !step.StartedAt.IsZero() && !step.CompletedAt.IsZero() {
			dur = fmt.Sprintf("  %s", step.CompletedAt.Sub(step.StartedAt).Truncate(time.Second))
		} else if !step.StartedAt.IsZero() && step.Status == model.RunStatusInProgress {
			dur = fmt.Sprintf("  %s...", time.Since(step.StartedAt).Truncate(time.Second))
		}

		b.WriteString(fmt.Sprintf("  %s %s%s\n", icon, step.Name, dur))
	}

	b.WriteString("\n  Logs will load automatically when the job completes.\n")
	return b.String()
}

func (a App) contextHints() string {
	if a.showHelp {
		return "j/k:scroll  PgUp/PgDn:page  g/G:top/bot  esc:close"
	}
	// Full-screen overlays take priority
	if a.currentView == ViewRuns {
		if a.logFullScreen {
			if a.logView.IsSearching() {
				return "enter:confirm  esc:cancel"
			}
			attemptHint := ""
			if run := a.detailsView.Run(); run != nil && run.RunAttempt > 1 {
				attemptHint = "  a:attempt"
			}
			if a.tailingJobID > 0 {
				return "[LIVE]  /:search  n/N:match  j/k:scroll  PgUp/PgDn:page  g/G:top/bot" + attemptHint + "  esc:back"
			}
			return "/:search  n/N:match  j/k:scroll  PgUp/PgDn:page  g/G:top/bot" + attemptHint + "  esc:back"
		}
		if a.infoFullScreen {
			return "j/k:scroll  PgUp/PgDn:page  esc:back"
		}
		if a.searchView.IsActive() {
			if a.searchView.IsInputMode() {
				return "enter:search  esc:close"
			}
			return "enter:view log  j/k:navigate  /:new search  esc:close"
		}
		if a.filterOverlay.IsActive() {
			return "tab:next field  enter:apply  esc:cancel"
		}
		// Normal two-pane mode
		if a.focusedPane == PaneLeft {
			legend := fmt.Sprintf("%s=pass %s=fail %s=cancel %s=run %s=queue %s=skip",
				ui.StatusIcon("success"),
				ui.StatusIcon("failure"),
				ui.StatusIcon("cancelled"),
				ui.StatusIcon("in_progress"),
				ui.StatusIcon("queued"),
				ui.StatusIcon("skipped"),
			)
			return legend + "  |  S:filter  r:refresh  i:info  /:search  ?:help"
		}
		legend := fmt.Sprintf("%s=pass %s=fail %s=cancel %s=run %s=queue %s=skip",
			ui.StatusIcon("success"),
			ui.StatusIcon("failure"),
			ui.StatusIcon("cancelled"),
			ui.StatusIcon("in_progress"),
			ui.StatusIcon("queued"),
			ui.StatusIcon("skipped"),
		)
		return legend + "  |  enter:view log  i:info  a:attempt  j/k:navigate  tab:pane  ?:help  esc:back"
	}

	switch a.currentView {
	case ViewWorkflows:
		return "enter:view runs  e:enable  D:disable  d:bulk delete  f:filter  ?:help"
	case ViewMetrics:
		return "[:prev window  ]:next window  j/k:scroll  ?:help"
	case ViewCache:
		return "space:select  d:delete  x:clear all  s:sort  r:refresh  f:filter  ?:help"
	case ViewRunners:
		return "r:refresh  f:filter  ?:help"
	}

	return "?:help  q:quit"
}

func (a App) renderRunsLayout() string {
	contentH := a.height - 5
	if contentH < 1 {
		contentH = 1
	}

	// Full-screen log view
	if a.logFullScreen {
		style := ui.StylePaneFocused.Width(a.width - 2).Height(contentH)
		return style.Render(a.logView.View())
	}

	// Full-screen info view
	if a.infoFullScreen {
		style := ui.StylePaneFocused.Width(a.width - 2).Height(contentH)
		return style.Render(a.infoView.View())
	}

	// Full-screen search view
	if a.searchView.IsActive() {
		style := ui.StylePaneFocused.Width(a.width - 2).Height(contentH)
		return style.Render(a.searchView.View())
	}

	// 2-pane layout (runs + job details)
	leftW := a.width * 45 / 100
	midW := a.width - leftW - 4
	if midW < 1 {
		midW = 1
	}

	leftStyle := ui.StylePane.Width(leftW).Height(contentH)
	midStyle := ui.StylePane.Width(midW).Height(contentH)

	if a.focusedPane == PaneLeft {
		leftStyle = ui.StylePaneFocused.Width(leftW).Height(contentH)
	} else if a.focusedPane == PaneMiddle {
		midStyle = ui.StylePaneFocused.Width(midW).Height(contentH)
	}

	left := leftStyle.Render(a.runsView.View())
	mid := midStyle.Render(a.detailsView.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, left, mid)
}

func (a App) renderHelp() string {
	bold := lipgloss.NewStyle().Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(ui.ColorPrimary).Bold(true).Width(14)
	desc := lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))

	row := func(k, d string) string {
		return "  " + keyStyle.Render(k) + desc.Render(d) + "\n"
	}

	// Left column: Navigation, Runs, Search & Filter, Log Viewer
	var left strings.Builder
	left.WriteString(bold.Render("  Navigation") + "\n\n")
	left.WriteString(row("1-5", "Switch tab"))
	left.WriteString(row("tab", "Next pane"))
	left.WriteString(row("shift+tab", "Previous pane"))
	left.WriteString(row("esc / bksp", "Back / close"))
	left.WriteString(row("j / k", "Move down / up"))
	left.WriteString(row("enter", "Select item"))
	left.WriteString(row("q", "Quit"))

	left.WriteString("\n" + bold.Render("  Runs") + "\n\n")
	left.WriteString(row("S", "Server-side filter"))
	left.WriteString(row("space", "Toggle select run"))
	left.WriteString(row("d", "Delete run"))
	left.WriteString(row("r", "Refresh"))
	left.WriteString(row("R", "Rerun all jobs"))
	left.WriteString(row("F", "Rerun failed jobs"))
	left.WriteString(row("C / X", "Cancel / force cancel"))
	left.WriteString(row("i", "Run / job info"))
	left.WriteString(row("a", "Cycle attempt"))
	left.WriteString(row("h / l", "Prev / next page"))

	left.WriteString("\n" + bold.Render("  Search & Filter") + "\n\n")
	left.WriteString(row("/", "Search logs"))
	left.WriteString(row("f", "Filter list"))

	// Right column: Log Viewer, Jobs, Workflows, Metrics, Cache, Runners
	var right strings.Builder
	right.WriteString(bold.Render("  Log Viewer") + "\n\n")
	right.WriteString(row("/", "Search in log"))
	right.WriteString(row("n / N", "Next / prev match"))
	right.WriteString(row("g / G", "Go to top / bottom"))
	right.WriteString(row("PgUp/PgDn", "Page up / down"))
	right.WriteString(row("a", "Cycle attempt"))
	right.WriteString(row("esc", "Exit log view"))

	right.WriteString("\n" + bold.Render("  Jobs") + "\n\n")
	right.WriteString(row("enter", "View job log"))
	right.WriteString(row("i", "Job info"))
	right.WriteString(row("a", "Cycle attempt"))

	right.WriteString("\n" + bold.Render("  Workflows") + "\n\n")
	right.WriteString(row("enter", "View runs"))
	right.WriteString(row("e / D", "Enable / disable"))
	right.WriteString(row("d / x", "Bulk delete runs"))

	right.WriteString("\n" + bold.Render("  Metrics") + "\n\n")
	right.WriteString(row("[ / ]", "Cycle time window"))

	right.WriteString("\n" + bold.Render("  Cache") + "\n\n")
	right.WriteString(row("space", "Toggle select"))
	right.WriteString(row("d", "Delete selected"))
	right.WriteString(row("x", "Clear all"))
	right.WriteString(row("s", "Cycle sort mode"))
	right.WriteString(row("r", "Refresh"))

	right.WriteString("\n" + bold.Render("  Runners") + "\n\n")
	right.WriteString(row("r", "Refresh"))

	colW := (a.width - 8) / 2
	if colW < 20 {
		colW = 20
	}
	leftCol := lipgloss.NewStyle().Width(colW).Render(left.String())
	rightCol := lipgloss.NewStyle().Width(colW).Render(right.String())

	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "  ", rightCol)
}
