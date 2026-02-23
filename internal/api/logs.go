package api

import (
	"context"
	"fmt"
	"io"
	"net/http"

	ghAPI "github.com/cli/go-gh/v2/pkg/api"
)

// DownloadRunLogs downloads the log archive for a run attempt.
// GitHub returns a 302 redirect to a short-lived archive URL.
func (c *Client) DownloadRunLogs(ctx context.Context, runID int64) (io.ReadCloser, error) {
	return c.downloadLogs(ctx, fmt.Sprintf("repos/%s/%s/actions/runs/%d/logs", c.owner, c.repo, runID))
}

// DownloadRunAttemptLogs downloads logs for a specific run attempt.
func (c *Client) DownloadRunAttemptLogs(ctx context.Context, runID int64, attempt int) (io.ReadCloser, error) {
	return c.downloadLogs(ctx, fmt.Sprintf("repos/%s/%s/actions/runs/%d/attempts/%d/logs", c.owner, c.repo, runID, attempt))
}

// DownloadJobLog downloads the log for a specific job.
func (c *Client) DownloadJobLog(ctx context.Context, jobID int64) (io.ReadCloser, error) {
	return c.downloadLogs(ctx, fmt.Sprintf("repos/%s/%s/actions/jobs/%d/logs", c.owner, c.repo, jobID))
}

func (c *Client) downloadLogs(ctx context.Context, apiPath string) (io.ReadCloser, error) {
	// Use a separate HTTP client that doesn't follow redirects automatically,
	// since we need to handle the 302 redirect to the archive URL ourselves.
	// We use go-gh's DefaultHTTPClient to get proper auth headers.
	httpClient, err := ghAPI.DefaultHTTPClient()
	if err != nil {
		return nil, fmt.Errorf("create http client: %w", err)
	}
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	url := fmt.Sprintf("https://api.github.com/%s", apiPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build log request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("log request failed: %w", err)
	}

	// Follow the redirect to the archive URL (no auth needed)
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect {
		location := resp.Header.Get("Location")
		resp.Body.Close()
		if location == "" {
			return nil, fmt.Errorf("redirect with no Location header")
		}
		redirectReq, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
		if err != nil {
			return nil, fmt.Errorf("create redirect request: %w", err)
		}
		resp, err = http.DefaultClient.Do(redirectReq)
		if err != nil {
			return nil, fmt.Errorf("follow redirect: %w", err)
		}
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d downloading logs", resp.StatusCode)
	}

	return resp.Body, nil
}
