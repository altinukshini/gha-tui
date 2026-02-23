package api

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/altinukshini/gha-tui/internal/model"
)

// ListActionsCaches returns GitHub Actions caches for the repository.
func (c *Client) ListActionsCaches(perPage, page int, sort, direction string) (*model.ActionsCacheList, error) {
	v := url.Values{}
	v.Set("per_page", strconv.Itoa(perPage))
	v.Set("page", strconv.Itoa(page))
	if sort != "" {
		v.Set("sort", sort)
	}
	if direction != "" {
		v.Set("direction", direction)
	}
	endpoint := fmt.Sprintf("actions/caches?%s", v.Encode())
	var resp model.ActionsCacheList
	if err := c.Get(endpoint, &resp); err != nil {
		return nil, fmt.Errorf("list actions caches: %w", err)
	}
	return &resp, nil
}

// DeleteActionsCache deletes a GitHub Actions cache by ID.
func (c *Client) DeleteActionsCache(cacheID int64) error {
	endpoint := fmt.Sprintf("actions/caches/%d", cacheID)
	return c.Delete(endpoint)
}
