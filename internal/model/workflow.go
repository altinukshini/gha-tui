package model

import "time"

type WorkflowState string

const (
	WorkflowActive             WorkflowState = "active"
	WorkflowDisabledManually   WorkflowState = "disabled_manually"
	WorkflowDisabledInactivity WorkflowState = "disabled_inactivity"
)

type Workflow struct {
	ID        int64         `json:"id"`
	Name      string        `json:"name"`
	Path      string        `json:"path"`
	State     WorkflowState `json:"state"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	HTMLURL   string        `json:"html_url"`
	BadgeURL  string        `json:"badge_url"`
}

type WorkflowsResponse struct {
	TotalCount int        `json:"total_count"`
	Workflows  []Workflow `json:"workflows"`
}
