package ops

import (
	"testing"
	"time"

	"github.com/altinukshini/gha-tui/internal/model"
)

func TestFilterRunsForDeletion(t *testing.T) {
	now := time.Now()
	runs := []model.Run{
		{ID: 1, Name: "CI", Conclusion: model.ConclusionFailure, HeadBranch: "main", Actor: model.Actor{Login: "alice"}, CreatedAt: now.Add(-48 * time.Hour)},
		{ID: 2, Name: "CI", Conclusion: model.ConclusionSuccess, HeadBranch: "main", Actor: model.Actor{Login: "bob"}, CreatedAt: now.Add(-1 * time.Hour)},
		{ID: 3, Name: "Deploy", Conclusion: model.ConclusionFailure, HeadBranch: "dev", Actor: model.Actor{Login: "alice"}, CreatedAt: now.Add(-72 * time.Hour)},
	}

	tests := []struct {
		name   string
		filter BulkDeleteFilter
		want   int
	}{
		{
			name:   "by workflow name",
			filter: BulkDeleteFilter{WorkflowName: "CI"},
			want:   2,
		},
		{
			name:   "by status",
			filter: BulkDeleteFilter{Conclusion: "failure"},
			want:   2,
		},
		{
			name:   "by age",
			filter: BulkDeleteFilter{OlderThan: 24 * time.Hour},
			want:   2,
		},
		{
			name:   "combined",
			filter: BulkDeleteFilter{WorkflowName: "CI", Conclusion: "failure"},
			want:   1,
		},
		{
			name:   "by branch",
			filter: BulkDeleteFilter{Branch: "dev"},
			want:   1,
		},
		{
			name:   "by actor",
			filter: BulkDeleteFilter{Actor: "alice"},
			want:   2,
		},
		{
			name:   "no match",
			filter: BulkDeleteFilter{WorkflowName: "Nonexistent"},
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterRuns(runs, tt.filter)
			if len(got) != tt.want {
				t.Errorf("FilterRuns() returned %d runs, want %d", len(got), tt.want)
			}
		})
	}
}
