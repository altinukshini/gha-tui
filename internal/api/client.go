package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	ghAPI "github.com/cli/go-gh/v2/pkg/api"
)

type Client struct {
	rest  *ghAPI.RESTClient
	owner string
	repo  string
}

type RateLimit struct {
	Remaining int
	Limit     int
	Reset     int64
}

func NewClient(owner, repo string) (*Client, error) {
	rest, err := ghAPI.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client (is gh authenticated?): %w", err)
	}
	return &Client{rest: rest, owner: owner, repo: repo}, nil
}

// CheckRepo verifies the repository exists and is accessible.
func (c *Client) CheckRepo() error {
	var result struct {
		FullName string `json:"full_name"`
	}
	err := c.rest.Get(fmt.Sprintf("repos/%s/%s", c.owner, c.repo), &result)
	if err != nil {
		var httpErr *ghAPI.HTTPError
		if errors.As(err, &httpErr) {
			switch httpErr.StatusCode {
			case 403:
				return fmt.Errorf("access denied for %s/%s — API rate limit may be exceeded or token lacks permissions (run: gh auth status)", c.owner, c.repo)
			case 404:
				return fmt.Errorf("repository %s/%s not found — check the name or token permissions (run: gh auth status)", c.owner, c.repo)
			case 401:
				return fmt.Errorf("authentication failed — run: gh auth login")
			default:
				return fmt.Errorf("repository %s/%s: HTTP %d — %s", c.owner, c.repo, httpErr.StatusCode, httpErr.Message)
			}
		}
		return fmt.Errorf("repository %s/%s not accessible: %w", c.owner, c.repo, err)
	}
	return nil
}

func (c *Client) repoPath(path string) string {
	return fmt.Sprintf("repos/%s/%s/%s", c.owner, c.repo, path)
}

func (c *Client) Get(path string, result interface{}) error {
	return c.rest.Get(c.repoPath(path), result)
}

func (c *Client) Post(path string, body interface{}, result interface{}) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	return c.rest.Post(c.repoPath(path), reader, result)
}

func (c *Client) Delete(path string) error {
	return c.rest.Delete(c.repoPath(path), nil)
}

func (c *Client) Put(path string, body interface{}, result interface{}) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	return c.rest.Put(c.repoPath(path), reader, result)
}

// RawRequest issues a raw HTTP request and returns the response without
// error handling for non-2xx status codes. Used for log downloads where
// GitHub returns 302 redirects.
func (c *Client) RawRequest(method, path string, body io.Reader) (*http.Response, error) {
	return c.rest.Request(method, c.repoPath(path), body)
}

func ParseRateLimit(resp *http.Response) RateLimit {
	rl := RateLimit{}
	if resp == nil {
		return rl
	}
	rl.Remaining, _ = strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining"))
	rl.Limit, _ = strconv.Atoi(resp.Header.Get("X-RateLimit-Limit"))
	rl.Reset, _ = strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)
	return rl
}
