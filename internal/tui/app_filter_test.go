package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/altinukshini/gha-tui/internal/api"
	"github.com/altinukshini/gha-tui/internal/cache"
	"github.com/altinukshini/gha-tui/internal/config"
	"github.com/altinukshini/gha-tui/internal/model"
	"github.com/altinukshini/gha-tui/internal/ui"
)

func TestAppFilterKeyReachesRunsView(t *testing.T) {
	cfg := config.Config{}
	client := &api.Client{} // won't make real calls
	logCache, err := cache.NewLogCache(t.TempDir(), 10, time.Hour)
	if err != nil {
		t.Fatalf("failed to create log cache: %v", err)
	}

	app := NewApp(cfg, client, logCache)

	// Simulate initial window size
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	app = *m.(*App)

	// Simulate runs loaded (runs load immediately — no workflow picker gate)
	runs := []model.Run{
		{ID: 1, RunNumber: 101, Name: "CI", DisplayTitle: "Build CI", HeadBranch: "main", CreatedAt: time.Now(), Actor: model.Actor{Login: "user"}},
		{ID: 2, RunNumber: 102, Name: "CI", DisplayTitle: "Test Suite", HeadBranch: "dev", CreatedAt: time.Now(), Actor: model.Actor{Login: "user"}},
	}
	m, _ = app.Update(ui.RunsLoadedMsg{Runs: runs, TotalCount: 2})
	app = *m.(*App)

	// Verify we're in ViewRuns with PaneLeft focused
	if app.currentView != ViewRuns {
		t.Fatalf("expected ViewRuns, got %v", app.currentView)
	}
	if app.focusedPane != PaneLeft {
		t.Fatalf("expected PaneLeft, got %v", app.focusedPane)
	}

	// Verify the runs view has items
	view := app.runsView.View()
	t.Logf("Runs view (first 200 chars): %s", truncate(view, 200))

	if !strings.Contains(view, "Build CI") && !strings.Contains(view, "#101") {
		t.Error("runs view should contain loaded runs")
	}

	// Now press 'f' to activate filter
	fKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
	m, _ = app.Update(fKey)
	app = *m.(*App)

	// Verify the runs view is now filtering
	if !app.runsView.IsFiltering() {
		t.Error("expected runs view to be filtering after pressing f")
	}

	// Verify the app's isListFiltering returns true
	if !app.isListFiltering() {
		t.Error("expected isListFiltering() to return true")
	}

	// Verify the filter appears in the view
	viewAfter := app.runsView.View()
	t.Logf("Runs view after 'f' (first 300 chars): %s", truncate(viewAfter, 300))

	if !strings.Contains(viewAfter, "Filter") {
		t.Errorf("filter input should appear in runs view after pressing f")
	}

	// Verify the full app view contains the filter
	fullView := app.View()
	if !strings.Contains(fullView, "Filter") {
		t.Errorf("filter input should appear in the full app view")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
