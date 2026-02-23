package api

import "testing"

func TestRepoPath(t *testing.T) {
	c := &Client{owner: "octocat", repo: "hello-world"}
	got := c.repoPath("actions/runs")
	want := "repos/octocat/hello-world/actions/runs"
	if got != want {
		t.Errorf("repoPath() = %q, want %q", got, want)
	}
}
