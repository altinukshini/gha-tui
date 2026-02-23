package api

import (
	"os"
	"testing"
)

func TestIntegrationListRuns(t *testing.T) {
	if os.Getenv("GHA_TUI_INTEGRATION") == "" {
		t.Skip("Set GHA_TUI_INTEGRATION=1 to run integration tests")
	}

	client, err := NewClient("cli", "cli")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	resp, err := client.ListRuns(RunsFilter{PerPage: 5})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}

	if resp.TotalCount == 0 {
		t.Error("expected at least 1 run")
	}
	if len(resp.Runs) == 0 {
		t.Error("expected runs in response")
	}

	t.Logf("Found %d total runs, got %d in page", resp.TotalCount, len(resp.Runs))
	for _, r := range resp.Runs {
		t.Logf("  #%d %s [%s] %s", r.RunNumber, r.DisplayTitle, r.Conclusion, r.HeadBranch)
	}
}

func TestIntegrationListWorkflows(t *testing.T) {
	if os.Getenv("GHA_TUI_INTEGRATION") == "" {
		t.Skip("Set GHA_TUI_INTEGRATION=1 to run integration tests")
	}

	client, err := NewClient("cli", "cli")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	resp, err := client.ListWorkflows(10, 1)
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}

	if resp.TotalCount == 0 {
		t.Error("expected at least 1 workflow")
	}

	t.Logf("Found %d workflows", resp.TotalCount)
	for _, w := range resp.Workflows {
		t.Logf("  %s [%s] %s", w.Name, w.State, w.Path)
	}
}
