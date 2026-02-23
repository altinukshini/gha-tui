package search

import (
	"regexp"
	"strings"

	"github.com/altin/gh-actions-tui/internal/model"
)

type Engine struct{}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) Search(logs map[string]string, query model.SearchQuery, runID int64, attempt int) *model.SearchResults {
	return e.SearchWithFilter(logs, query, runID, attempt, nil)
}

func (e *Engine) SearchWithFilter(logs map[string]string, query model.SearchQuery, runID int64, attempt int, failedJobs map[string]bool) *model.SearchResults {
	results := &model.SearchResults{
		Query:     query,
		JobCounts: make(map[string]int),
	}

	matcher, err := buildMatcher(query)
	if err != nil {
		return results
	}

	for jobName, content := range logs {
		if query.FailedOnly && failedJobs != nil && !failedJobs[jobName] {
			continue
		}
		if query.JobPattern != "" {
			matched, _ := regexp.MatchString(query.JobPattern, jobName)
			if !matched {
				continue
			}
		}

		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if matcher(line) {
				results.Matches = append(results.Matches, model.SearchResult{
					RunID:   runID,
					JobName: jobName,
					Line:    i + 1,
					Content: line,
				})
				results.JobCounts[jobName]++
				results.TotalCount++
			}
		}
	}

	return results
}

func buildMatcher(query model.SearchQuery) (func(string) bool, error) {
	if query.IsRegex {
		flags := ""
		if !query.CaseSensitive {
			flags = "(?i)"
		}
		re, err := regexp.Compile(flags + query.Pattern)
		if err != nil {
			return nil, err
		}
		return func(line string) bool { return re.MatchString(line) }, nil
	}

	pattern := query.Pattern
	if !query.CaseSensitive {
		pattern = strings.ToLower(pattern)
	}
	return func(line string) bool {
		if !query.CaseSensitive {
			line = strings.ToLower(line)
		}
		return strings.Contains(line, pattern)
	}, nil
}
