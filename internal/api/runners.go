package api

import (
	"fmt"

	"github.com/altin/gh-actions-tui/internal/model"
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
