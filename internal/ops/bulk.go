package ops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/altinukshini/gha-tui/internal/api"
	"github.com/altinukshini/gha-tui/internal/model"
)

type BulkDeleteFilter struct {
	WorkflowName string
	Conclusion   string
	Branch       string
	Actor        string
	OlderThan    time.Duration
}

func FilterRuns(runs []model.Run, filter BulkDeleteFilter) []model.Run {
	var matched []model.Run
	now := time.Now()

	for _, r := range runs {
		if filter.WorkflowName != "" && !strings.EqualFold(r.Name, filter.WorkflowName) {
			continue
		}
		if filter.Conclusion != "" && string(r.Conclusion) != filter.Conclusion {
			continue
		}
		if filter.Branch != "" && r.HeadBranch != filter.Branch {
			continue
		}
		if filter.Actor != "" && r.Actor.Login != filter.Actor {
			continue
		}
		if filter.OlderThan > 0 && now.Sub(r.CreatedAt) < filter.OlderThan {
			continue
		}
		matched = append(matched, r)
	}
	return matched
}

type BulkDeleteResult struct {
	Completed int
	Failed    int
	Errors    []error
}

func BulkDeleteRuns(ctx context.Context, client *api.Client, runIDs []int64, onProgress func(completed, total int)) (*BulkDeleteResult, error) {
	result := &BulkDeleteResult{}
	total := len(runIDs)

	for i, id := range runIDs {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		err := client.DeleteRun(id)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("run %d: %w", id, err))
		} else {
			result.Completed++
		}

		if onProgress != nil {
			onProgress(i+1, total)
		}

		// Rate limit: ~30 deletes/min to stay safe
		if (i+1)%10 == 0 {
			time.Sleep(2 * time.Second)
		}
	}

	return result, nil
}
