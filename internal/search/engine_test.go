package search

import (
	"testing"

	"github.com/altin/gh-actions-tui/internal/model"
)

func TestSearchPlainText(t *testing.T) {
	logs := map[string]string{
		"build (ubuntu, 18)": "line 1: compiling\nline 2: error: undefined reference\nline 3: done",
		"build (ubuntu, 20)": "line 1: compiling\nline 2: success\nline 3: done",
		"test (ubuntu, 18)":  "line 1: running tests\nline 2: FAIL: TestFoo error: assertion failed\nline 3: done",
	}

	engine := New()
	query := model.SearchQuery{
		Pattern:       "error",
		IsRegex:       false,
		CaseSensitive: true,
		ContextLines:  0,
	}

	results := engine.Search(logs, query, 123, 1)

	if results.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", results.TotalCount)
	}
	if len(results.JobCounts) != 2 {
		t.Errorf("matched %d jobs, want 2", len(results.JobCounts))
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	logs := map[string]string{
		"build": "Error: file not found\nerror: missing dep\nwarning: unused var",
	}

	engine := New()
	query := model.SearchQuery{
		Pattern:       "error",
		CaseSensitive: false,
	}

	results := engine.Search(logs, query, 1, 1)
	if results.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", results.TotalCount)
	}
}

func TestSearchRegex(t *testing.T) {
	logs := map[string]string{
		"build": "Error: file not found\nerror: missing dep\nwarning: unused var",
	}

	engine := New()
	query := model.SearchQuery{
		Pattern:       `[Ee]rror:\s+\w+`,
		IsRegex:       true,
		CaseSensitive: true,
	}

	results := engine.Search(logs, query, 1, 1)
	if results.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", results.TotalCount)
	}
}

func TestSearchFailedOnly(t *testing.T) {
	engine := New()
	query := model.SearchQuery{
		Pattern:    "compile",
		FailedOnly: true,
	}

	logs := map[string]string{
		"build":  "compile started\ncompile done",
		"deploy": "compile assets\ndone",
	}
	failedJobs := map[string]bool{"build": true}

	results := engine.SearchWithFilter(logs, query, 1, 1, failedJobs)
	if results.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", results.TotalCount)
	}
	if _, ok := results.JobCounts["deploy"]; ok {
		t.Error("should not have matched non-failed job 'deploy'")
	}
}

func TestSearchJobPattern(t *testing.T) {
	engine := New()
	query := model.SearchQuery{
		Pattern:    "done",
		JobPattern: `build.*`,
	}

	logs := map[string]string{
		"build (ubuntu)": "compile\ndone",
		"build (macos)":  "compile\ndone",
		"test":           "run\ndone",
	}

	results := engine.Search(logs, query, 1, 1)
	if results.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", results.TotalCount)
	}
}
