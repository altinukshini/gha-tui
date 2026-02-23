package runs

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/altinukshini/gha-tui/internal/model"
	"github.com/altinukshini/gha-tui/internal/ui"
)

func TestFilterShowsAfterPressingF(t *testing.T) {
	m := New()

	// Simulate WindowSizeMsg (propagateSize)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})

	// Simulate loading runs
	runs := []model.Run{
		{ID: 1, RunNumber: 101, DisplayTitle: "Build", HeadBranch: "main", CreatedAt: time.Now(), Actor: model.Actor{Login: "user"}},
		{ID: 2, RunNumber: 102, DisplayTitle: "Test", HeadBranch: "dev", CreatedAt: time.Now(), Actor: model.Actor{Login: "user"}},
	}
	m, _ = m.Update(ui.RunsLoadedMsg{Runs: runs, TotalCount: 2})

	// Verify items are loaded
	if len(m.list.Items()) != 2 {
		t.Fatalf("expected 2 items, got %d", len(m.list.Items()))
	}

	// Verify filter state before pressing 'f'
	if m.list.FilterState() != list.Unfiltered {
		t.Fatalf("expected Unfiltered before pressing f, got %v", m.list.FilterState())
	}

	// Verify filter is not in view before pressing 'f'
	viewBefore := m.View()
	if strings.Contains(viewBefore, "Filter") {
		t.Error("filter input should NOT be in view before pressing f")
	}

	// Press 'f' to activate filter
	fKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
	m, cmd := m.Update(fKey)

	if cmd == nil {
		t.Error("expected non-nil cmd after pressing f (textinput.Blink)")
	}

	// Verify filter state after pressing 'f'
	if m.list.FilterState() != list.Filtering {
		t.Fatalf("expected Filtering after pressing f, got %v", m.list.FilterState())
	}

	// Verify IsFiltering
	if !m.IsFiltering() {
		t.Error("IsFiltering() should return true")
	}

	// Verify filter is in the view
	viewAfter := m.View()
	if !strings.Contains(viewAfter, "Filter") {
		t.Errorf("filter input should be in view after pressing f.\nView:\n%s", viewAfter)
	}

	// Verify items are still visible
	if !strings.Contains(viewAfter, "Build") && !strings.Contains(viewAfter, "Test") {
		t.Error("items should still be visible while filtering")
	}
}

func TestPageChangeResetsCursor(t *testing.T) {
	m := New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})

	// Load initial page of runs
	page1 := make([]model.Run, 10)
	for i := range page1 {
		page1[i] = model.Run{
			ID: int64(i + 1), RunNumber: i + 100,
			DisplayTitle: "Run", HeadBranch: "main",
			CreatedAt: time.Now(), Actor: model.Actor{Login: "user"},
		}
	}
	m, _ = m.Update(ui.RunsLoadedMsg{Runs: page1, TotalCount: 30})

	if m.list.Index() != 0 {
		t.Fatalf("expected index 0 after initial load, got %d", m.list.Index())
	}

	// Move cursor down several times
	downKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	for i := 0; i < 5; i++ {
		m, _ = m.Update(downKey)
	}

	if m.list.Index() != 5 {
		t.Fatalf("expected index 5 after moving down, got %d", m.list.Index())
	}

	// Simulate page change (RunsPageMsg)
	page2 := make([]model.Run, 10)
	for i := range page2 {
		page2[i] = model.Run{
			ID: int64(i + 100), RunNumber: i + 200,
			DisplayTitle: "Page2Run", HeadBranch: "dev",
			CreatedAt: time.Now(), Actor: model.Actor{Login: "user"},
		}
	}
	m, _ = m.Update(ui.RunsPageMsg{Runs: page2, TotalCount: 30, Page: 2})

	// Cursor should be reset to 0
	if m.list.Index() != 0 {
		t.Fatalf("expected index 0 after page change, got %d", m.list.Index())
	}
}

func TestLKeyDoesNotTriggerInternalPageNav(t *testing.T) {
	m := New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 10}) // small height → internal pagination

	// Load more items than fit on screen (Height=10, delegate height=2 → ~4 items per page)
	runs := make([]model.Run, 12)
	for i := range runs {
		runs[i] = model.Run{
			ID: int64(i + 1), RunNumber: i + 100,
			DisplayTitle: "Run", HeadBranch: "main",
			CreatedAt: time.Now(), Actor: model.Actor{Login: "user"},
		}
	}
	m, _ = m.Update(ui.RunsLoadedMsg{Runs: runs, TotalCount: 12})

	// Record initial paginator page
	initialPage := m.list.Paginator.Page

	// Press 'l' — should NOT trigger internal page navigation
	lKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	m, _ = m.Update(lKey)

	if m.list.Paginator.Page != initialPage {
		t.Errorf("pressing 'l' should not change internal page: was %d, now %d",
			initialPage, m.list.Paginator.Page)
	}
}

func TestFilterEscCancels(t *testing.T) {
	m := New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})

	runs := []model.Run{
		{ID: 1, RunNumber: 101, DisplayTitle: "Build", HeadBranch: "main", CreatedAt: time.Now(), Actor: model.Actor{Login: "user"}},
	}
	m, _ = m.Update(ui.RunsLoadedMsg{Runs: runs, TotalCount: 1})

	// Activate filter
	fKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
	m, _ = m.Update(fKey)

	if !m.IsFiltering() {
		t.Fatal("should be filtering after pressing f")
	}

	// Press esc to cancel
	escKey := tea.KeyMsg{Type: tea.KeyEscape}
	m, _ = m.Update(escKey)

	if m.IsFiltering() {
		t.Error("should NOT be filtering after pressing esc")
	}
}
