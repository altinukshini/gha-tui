package model

// RunnerLabel represents a label attached to a runner.
type RunnerLabel struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "read-only" or "custom"
}

// Runner represents a GitHub Actions runner.
type Runner struct {
	ID            int64         `json:"id"`
	Name          string        `json:"name"`
	OS            string        `json:"os"`
	Status        string        `json:"status"` // "online" or "offline"
	Busy          bool          `json:"busy"`
	Ephemeral     bool          `json:"ephemeral"`
	RunnerGroupID int64         `json:"runner_group_id"`
	Labels        []RunnerLabel `json:"labels"`
}

// RunnersResponse is the API response for listing runners.
type RunnersResponse struct {
	TotalCount int      `json:"total_count"`
	Runners    []Runner `json:"runners"`
}
