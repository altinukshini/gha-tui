package model

import "time"

type RunStatus string

const (
	RunStatusQueued     RunStatus = "queued"
	RunStatusInProgress RunStatus = "in_progress"
	RunStatusCompleted  RunStatus = "completed"
	RunStatusWaiting    RunStatus = "waiting"
	RunStatusRequested  RunStatus = "requested"
	RunStatusPending    RunStatus = "pending"
)

type RunConclusion string

const (
	ConclusionSuccess   RunConclusion = "success"
	ConclusionFailure   RunConclusion = "failure"
	ConclusionCancelled RunConclusion = "cancelled"
	ConclusionSkipped   RunConclusion = "skipped"
	ConclusionTimedOut  RunConclusion = "timed_out"
	ConclusionNeutral   RunConclusion = "neutral"
)

type Run struct {
	ID           int64         `json:"id"`
	Name         string        `json:"name"`
	DisplayTitle string        `json:"display_title"`
	Status       RunStatus     `json:"status"`
	Conclusion   RunConclusion `json:"conclusion"`
	WorkflowID   int64         `json:"workflow_id"`
	RunNumber    int           `json:"run_number"`
	RunAttempt   int           `json:"run_attempt"`
	Event        string        `json:"event"`
	HeadBranch   string        `json:"head_branch"`
	HeadSHA      string        `json:"head_sha"`
	Actor        Actor         `json:"actor"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	RunStartedAt time.Time     `json:"run_started_at"`
	HTMLURL      string        `json:"html_url"`
	JobsURL      string        `json:"jobs_url"`
	LogsURL      string        `json:"logs_url"`
}

type Actor struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

type RunsResponse struct {
	TotalCount int   `json:"total_count"`
	Runs       []Run `json:"workflow_runs"`
}

func (r Run) Duration() time.Duration {
	if r.UpdatedAt.IsZero() || r.RunStartedAt.IsZero() {
		return 0
	}
	return r.UpdatedAt.Sub(r.RunStartedAt)
}

func (r Run) ShortSHA() string {
	if len(r.HeadSHA) >= 7 {
		return r.HeadSHA[:7]
	}
	return r.HeadSHA
}
