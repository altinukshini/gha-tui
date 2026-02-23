package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/altin/gh-actions-tui/internal/model"
)

type JobsFilter struct {
	Filter  string // "latest", "all"
	PerPage int
	Page    int
}

func (f JobsFilter) QueryString() string {
	v := url.Values{}
	if f.Filter != "" {
		v.Set("filter", f.Filter)
	}
	if f.PerPage > 0 {
		v.Set("per_page", strconv.Itoa(f.PerPage))
	} else {
		v.Set("per_page", "100")
	}
	if f.Page > 0 {
		v.Set("page", strconv.Itoa(f.Page))
	}
	if qs := v.Encode(); qs != "" {
		return "?" + qs
	}
	return ""
}

func (c *Client) ListJobs(runID int64, filter JobsFilter) (*model.JobsResponse, error) {
	var resp model.JobsResponse
	path := fmt.Sprintf("actions/runs/%d/jobs%s", runID, filter.QueryString())
	err := c.Get(path, &resp)
	if err != nil {
		// Run may have been deleted — treat 404 as empty
		if strings.Contains(err.Error(), "404") {
			return &model.JobsResponse{}, nil
		}
		return nil, fmt.Errorf("list jobs for run %d: %w", runID, err)
	}
	return &resp, nil
}

func (c *Client) ListJobsForAttempt(runID int64, attempt int, filter JobsFilter) (*model.JobsResponse, error) {
	var resp model.JobsResponse
	path := fmt.Sprintf("actions/runs/%d/attempts/%d/jobs%s", runID, attempt, filter.QueryString())
	err := c.Get(path, &resp)
	if err != nil {
		return nil, fmt.Errorf("list jobs for run %d attempt %d: %w", runID, attempt, err)
	}
	return &resp, nil
}

func (c *Client) GetJob(jobID int64) (*model.Job, error) {
	var job model.Job
	err := c.Get(fmt.Sprintf("actions/jobs/%d", jobID), &job)
	if err != nil {
		return nil, fmt.Errorf("get job %d: %w", jobID, err)
	}
	return &job, nil
}
