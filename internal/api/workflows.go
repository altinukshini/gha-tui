package api

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/altinukshini/gha-tui/internal/model"
)

func (c *Client) ListWorkflows(perPage, page int) (*model.WorkflowsResponse, error) {
	v := url.Values{}
	if perPage > 0 {
		v.Set("per_page", strconv.Itoa(perPage))
	} else {
		v.Set("per_page", "100")
	}
	if page > 0 {
		v.Set("page", strconv.Itoa(page))
	}

	var resp model.WorkflowsResponse
	err := c.Get("actions/workflows?"+v.Encode(), &resp)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	return &resp, nil
}

func (c *Client) EnableWorkflow(workflowID int64) error {
	return c.Put(fmt.Sprintf("actions/workflows/%d/enable", workflowID), nil, nil)
}

func (c *Client) DisableWorkflow(workflowID int64) error {
	return c.Put(fmt.Sprintf("actions/workflows/%d/disable", workflowID), nil, nil)
}

type RetentionSettings struct {
	ArtifactAndLogRetentionDays int `json:"artifact_and_log_retention_days"`
}

func (c *Client) GetRetentionSettings() (*RetentionSettings, error) {
	var resp RetentionSettings
	err := c.Get("actions/retention", &resp)
	if err != nil {
		return nil, fmt.Errorf("get retention settings: %w", err)
	}
	return &resp, nil
}
