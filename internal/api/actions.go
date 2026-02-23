package api

import "fmt"

type RerunConfig struct {
	EnableDebugLogging bool `json:"enable_debug_logging,omitempty"`
}

func (c *Client) RerunWorkflow(runID int64, debug bool) error {
	body := RerunConfig{EnableDebugLogging: debug}
	return c.Post(fmt.Sprintf("actions/runs/%d/rerun", runID), body, nil)
}

func (c *Client) RerunFailedJobs(runID int64, debug bool) error {
	body := RerunConfig{EnableDebugLogging: debug}
	return c.Post(fmt.Sprintf("actions/runs/%d/rerun-failed-jobs", runID), body, nil)
}

func (c *Client) RerunJob(jobID int64, debug bool) error {
	body := RerunConfig{EnableDebugLogging: debug}
	return c.Post(fmt.Sprintf("actions/jobs/%d/rerun", jobID), body, nil)
}

func (c *Client) CancelRun(runID int64) error {
	return c.Post(fmt.Sprintf("actions/runs/%d/cancel", runID), nil, nil)
}

func (c *Client) ForceCancelRun(runID int64) error {
	return c.Post(fmt.Sprintf("actions/runs/%d/force-cancel", runID), nil, nil)
}

func (c *Client) DeleteRun(runID int64) error {
	return c.Delete(fmt.Sprintf("actions/runs/%d", runID))
}
