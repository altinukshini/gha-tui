package workflows

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/altin/gh-actions-tui/internal/model"
	"github.com/altin/gh-actions-tui/internal/ui"
)

type workflowItem struct {
	wf        model.Workflow
	stats     ui.WorkflowStats
	showStats bool
}

func (w workflowItem) Title() string {
	state := ""
	switch w.wf.State {
	case model.WorkflowActive:
		state = ui.StyleSuccess.Render("[active]")
	case model.WorkflowDisabledManually:
		state = ui.StyleWarning.Render("[disabled]")
	case model.WorkflowDisabledInactivity:
		state = ui.StyleMuted.Render("[inactive]")
	}

	if w.showStats && w.stats.TotalRuns > 0 {
		stats := fmt.Sprintf("  %s  recent: %s %s",
			ui.StyleMuted.Render(fmt.Sprintf("%d total", w.stats.TotalRuns)),
			ui.StyleSuccess.Render(fmt.Sprintf("%d✓", w.stats.SuccessCount)),
			ui.StyleFailure.Render(fmt.Sprintf("%d✗", w.stats.FailureCount)),
		)
		return fmt.Sprintf("%s %s%s  %s", state, w.wf.Name, stats, ui.StyleMuted.Render(w.wf.Path))
	}

	return fmt.Sprintf("%s %s  %s", state, w.wf.Name, ui.StyleMuted.Render(w.wf.Path))
}

func (w workflowItem) Description() string { return "" }

func (w workflowItem) FilterValue() string {
	return w.wf.Name + " " + w.wf.Path
}

type Model struct {
	list      list.Model
	wfs       []model.Workflow
	showStats bool
	width     int
	height    int
	loading   bool
	err       error
}

// New creates a workflow picker for the Runs tab (no stats).
func New() Model {
	return newModel(false)
}

// NewWithStats creates a workflow list for the Workflows tab (with stats).
func NewWithStats() Model {
	return newModel(true)
}

func newModel(showStats bool) Model {
	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(1)
	delegate.SetSpacing(0)

	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.KeyMap.Filter = key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter"))
	l.DisableQuitKeybindings()

	return Model{list: l, showStats: showStats, loading: true}
}

func (m Model) SelectedWorkflow() *model.Workflow {
	if item, ok := m.list.SelectedItem().(workflowItem); ok {
		return &item.wf
	}
	return nil
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.WorkflowsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.wfs = msg.Workflows
		sort.Slice(m.wfs, func(i, j int) bool {
			return strings.ToLower(m.wfs[i].Name) < strings.ToLower(m.wfs[j].Name)
		})
		items := make([]list.Item, len(m.wfs))
		for i, w := range m.wfs {
			items[i] = workflowItem{wf: w, showStats: m.showStats}
		}
		cmd := m.list.SetItems(items)
		return m, cmd

	case ui.WorkflowStatsMsg:
		if !m.showStats {
			return m, nil
		}
		// Update existing items with stats
		items := m.list.Items()
		updated := make([]list.Item, len(items))
		for i, item := range items {
			if wi, ok := item.(workflowItem); ok {
				wi.stats = msg.Stats[wi.wf.ID]
				updated[i] = wi
			} else {
				updated[i] = item
			}
		}
		cmd := m.list.SetItems(updated)
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.loading {
		return "\n  Loading workflows..."
	}
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v", m.err)
	}
	return m.list.View()
}

func (m Model) IsFiltering() bool {
	return m.list.FilterState() == list.Filtering
}

func (m Model) HasActiveFilter() bool {
	return m.list.FilterState() != list.Unfiltered
}

func (m Model) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "enable")),
		key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "disable")),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "bulk delete runs")),
	}
}
