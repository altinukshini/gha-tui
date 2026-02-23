package api

import (
	"fmt"

	"github.com/altinukshini/gha-tui/internal/model"
)

// ListRunners returns the repository's GitHub Actions runners.
func (c *Client) ListRunners(perPage, page int) (*model.RunnersResponse, error) {
	endpoint := fmt.Sprintf("actions/runners?per_page=%d&page=%d", perPage, page)
	var resp model.RunnersResponse
	if err := c.Get(endpoint, &resp); err != nil {
		return nil, fmt.Errorf("list runners: %w", err)
	}
	return &resp, nil
}

// ListOrgRunners returns the organization's GitHub Actions runners.
// Returns nil, nil if the owner is not an org or the token lacks permissions.
func (c *Client) ListOrgRunners(perPage, page int) (*model.RunnersResponse, error) {
	endpoint := fmt.Sprintf("orgs/%s/actions/runners?per_page=%d&page=%d", c.owner, perPage, page)
	var resp model.RunnersResponse
	if err := c.rest.Get(endpoint, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
