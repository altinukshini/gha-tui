package runnersview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/altin/gh-actions-tui/internal/model"
	"github.com/altin/gh-actions-tui/internal/ui"
)

type runnerItem struct {
	runner model.Runner
}

func (r runnerItem) Title() string {
	// Status icon: green ● for online, red ● for offline
	var status string
	if r.runner.Status == "online" {
		status = ui.StyleSuccess.Render("●")
	} else {
		status = ui.StyleFailure.Render("●")
	}

	// Busy indicator
	busy := ""
	if r.runner.Busy {
		busy = ui.StyleInfo.Render(" [busy]")
	}

	// Labels
	var labelNames []string
	for _, l := range r.runner.Labels {
		labelNames = append(labelNames, l.Name)
	}
	labels := ui.StyleMuted.Render(strings.Join(labelNames, ", "))

	return fmt.Sprintf("%s %s%s  %s  %s", status, r.runner.Name, busy, ui.StyleMuted.Render(r.runner.OS), labels)
}

func (r runnerItem) Description() string {
	parts := []string{r.runner.OS}
	if r.runner.Ephemeral {
		parts = append(parts, "ephemeral")
	}
	return strings.Join(parts, " | ")
}

func (r runnerItem) FilterValue() string {
	// Filter by name, OS, and label names
	parts := []string{r.runner.Name, r.runner.OS}
	for _, l := range r.runner.Labels {
		parts = append(parts, l.Name)
	}
	return strings.Join(parts, " ")
}

// Model is the runners list view.
type Model struct {
	list    list.Model
	runners []model.Runner
	width   int
	height  int
	loading bool
	err     error
}

// New creates a new runners view.
func New() Model {
	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2)
	delegate.SetSpacing(0)

	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.KeyMap.Filter = key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter"))
	l.DisableQuitKeybindings()

	return Model{list: l, loading: true}
}

// Init satisfies the tea.Model interface.
func (m Model) Init() tea.Cmd { return nil }

// Update handles messages for the runners view.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.RunnersLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.runners = msg.Runners
		items := make([]list.Item, len(msg.Runners))
		for i, r := range msg.Runners {
			items[i] = runnerItem{runner: r}
		}
		cmd := m.list.SetItems(items)
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Reserve one line for the header.
		m.list.SetSize(msg.Width, msg.Height-1)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the runners view.
func (m Model) View() string {
	if m.loading {
		return "\n  Loading runners..."
	}
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v", m.err)
	}
	if len(m.runners) == 0 {
		return "\n  No self-hosted runners found.\n\n  This view shows self-hosted runners only.\n  GitHub-hosted runners are not listed by the API."
	}

	online := 0
	busy := 0
	for _, r := range m.runners {
		if r.Status == "online" {
			online++
		}
		if r.Busy {
			busy++
		}
	}
	header := fmt.Sprintf("  %d runners | %d online | %d busy | r: refresh  f: filter",
		len(m.runners), online, busy)
	header = ui.StyleMuted.Render(header)

	return header + "\n" + m.list.View()
}

// IsFiltering returns true when the filter input is active.
func (m Model) IsFiltering() bool {
	return m.list.FilterState() == list.Filtering
}

// HasActiveFilter returns true when a filter is applied.
func (m Model) HasActiveFilter() bool {
	return m.list.FilterState() != list.Unfiltered
}

// ShortHelp returns key bindings for this view.
func (m Model) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
	}
}
