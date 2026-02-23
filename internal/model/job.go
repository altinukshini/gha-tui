package model

import "time"

type Job struct {
	ID          int64         `json:"id"`
	RunID       int64         `json:"run_id"`
	RunAttempt  int           `json:"run_attempt"`
	Name        string        `json:"name"`
	Status      RunStatus     `json:"status"`
	Conclusion  RunConclusion `json:"conclusion"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Steps       []Step        `json:"steps"`
	RunnerName  string        `json:"runner_name"`
	HTMLURL     string        `json:"html_url"`
}

type Step struct {
	Name        string        `json:"name"`
	Status      RunStatus     `json:"status"`
	Conclusion  RunConclusion `json:"conclusion"`
	Number      int           `json:"number"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
}

type JobsResponse struct {
	TotalCount int   `json:"total_count"`
	Jobs       []Job `json:"jobs"`
}

func (j Job) Duration() time.Duration {
	if j.CompletedAt.IsZero() || j.StartedAt.IsZero() {
		return 0
	}
	return j.CompletedAt.Sub(j.StartedAt)
}

func (j Job) Failed() bool {
	return j.Conclusion == ConclusionFailure
}
