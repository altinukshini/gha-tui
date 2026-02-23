package api

import "testing"

func TestRunsFilterQueryString(t *testing.T) {
	tests := []struct {
		name   string
		filter RunsFilter
		want   string
	}{
		{
			name:   "empty filter",
			filter: RunsFilter{},
			want:   "?per_page=30",
		},
		{
			name:   "branch filter",
			filter: RunsFilter{Branch: "main", PerPage: 10},
			want:   "?branch=main&per_page=10",
		},
		{
			name:   "status and actor",
			filter: RunsFilter{Status: "failure", Actor: "octocat"},
			want:   "?actor=octocat&per_page=30&status=failure",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.QueryString()
			if got != tt.want {
				t.Errorf("QueryString() = %q, want %q", got, tt.want)
			}
		})
	}
}
