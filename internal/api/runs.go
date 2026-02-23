package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/altinukshini/gha-tui/internal/model"
)

type RunsFilter struct {
	WorkflowID   int64
	WorkflowFile string
	Actor        string
	Branch       string
	Event        string
	Status       string
	Created      string // e.g. ">=2025-01-01" for date range filtering
	PerPage      int
	Page         int
}

func (f RunsFilter) QueryString() string {
	v := url.Values{}
	if f.Actor != "" {
		v.Set("actor", f.Actor)
	}
	if f.Branch != "" {
		v.Set("branch", f.Branch)
	}
	if f.Event != "" {
		v.Set("event", f.Event)
	}
	if f.Status != "" {
		v.Set("status", f.Status)
	}
	if f.Created != "" {
		v.Set("created", f.Created)
	}
	if f.PerPage > 0 {
		v.Set("per_page", strconv.Itoa(f.PerPage))
	} else {
		v.Set("per_page", "30")
	}
	if f.Page > 0 {
		v.Set("page", strconv.Itoa(f.Page))
	}
	if qs := v.Encode(); qs != "" {
		return "?" + qs
	}
	return ""
}

func (c *Client) ListRuns(filter RunsFilter) (*model.RunsResponse, error) {
	var basePath string
	if filter.WorkflowID > 0 {
		basePath = fmt.Sprintf("actions/workflows/%d/runs", filter.WorkflowID)
	} else if filter.WorkflowFile != "" {
		basePath = fmt.Sprintf("actions/workflows/%s/runs", url.PathEscape(filter.WorkflowFile))
	} else {
		basePath = "actions/runs"
	}

	var resp model.RunsResponse
	err := c.Get(basePath+filter.QueryString(), &resp)
	if err != nil {
		// Workflow may have been deleted or have no runs — treat 404 as empty
		if strings.Contains(err.Error(), "404") {
			return &model.RunsResponse{}, nil
		}
		return nil, fmt.Errorf("list runs: %w", err)
	}
	return &resp, nil
}

func (c *Client) GetRun(runID int64) (*model.Run, error) {
	var run model.Run
	err := c.Get(fmt.Sprintf("actions/runs/%d", runID), &run)
	if err != nil {
		return nil, fmt.Errorf("get run %d: %w", runID, err)
	}
	return &run, nil
}
