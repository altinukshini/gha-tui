package ui

import (
	"github.com/altin/gh-actions-tui/internal/model"
)

// Data fetched messages
type RunsLoadedMsg struct {
	Runs       []model.Run
	TotalCount int
	Err        error
}

type RunsPageMsg struct {
	Runs       []model.Run
	TotalCount int
	Page       int
	Err        error
}

type JobsLoadedMsg struct {
	RunID int64
	Jobs  []model.Job
	Err   error
}

type LogsLoadedMsg struct {
	RunID   int64
	Attempt int
	Logs    map[string]string // jobName -> log content
	Err     error
}

type SearchDoneMsg struct {
	Results *model.SearchResults
	Err     error
}

type WorkflowStats struct {
	TotalRuns    int
	SuccessCount int
	FailureCount int
}

type WorkflowsLoadedMsg struct {
	Workflows []model.Workflow
	Err       error
}

type WorkflowStatsMsg struct {
	Stats map[int64]WorkflowStats // keyed by workflow ID
}

// Action result messages
type ActionResultMsg struct {
	Action  string
	Success bool
	Err     error
}

type BulkDeleteProgressMsg struct {
	Completed int
	Total     int
	Err       error
}

type DashboardDataMsg struct {
	Runs       []model.Run
	Jobs       []model.Job
	TotalCount int
	Err        error
}

type JobsTickMsg struct {
	RunID int64
}

type LogTailTickMsg struct {
	JobID   int64
	JobName string
}

type JobTailStatusMsg struct {
	JobID     int64
	JobName   string
	Job       *model.Job
	Completed bool
	Err       error
}

type JobLogLoadedMsg struct {
	JobID   int64
	JobName string
	Content string
	Err     error
}

type RunLoadedMsg struct {
	RunID int64
	Run   *model.Run
	Err   error
}

type RetentionLoadedMsg struct {
	RetentionDays int
	Err           error
}

type StatusMsg struct {
	Text string
}

// Actions cache management messages
type ActionsCachesLoadedMsg struct {
	Caches     []model.ActionsCache
	TotalCount int
	Err        error
}

type ActionsCacheDeletedMsg struct {
	CacheID int64
	Err     error
}

// Runners messages
type RunnersLoadedMsg struct {
	Runners []model.Runner
	Err     error
}
