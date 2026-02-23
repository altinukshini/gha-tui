package runs

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/altin/gh-actions-tui/internal/model"
	"github.com/altin/gh-actions-tui/internal/ui"
)

// NeedNextPageMsg is emitted when the cursor is at the bottom and user presses down.
type NeedNextPageMsg struct{}

// --- Custom delegate (avoids DefaultDelegate ANSI corruption during filtering) ---

type runDelegate struct {
	selected *map[int64]bool // pointer to the model's selection map
}

func (d runDelegate) Height() int                                      { return 2 }
func (d runDelegate) Spacing() int                                     { return 0 }
func (d runDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd         { return nil }

func (d runDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ri, ok := item.(runItem)
	if !ok {
		return
	}

	icon := ui.StatusIcon(string(ri.run.Conclusion))
	if ri.run.Status == model.RunStatusInProgress {
		icon = ui.StatusIcon("in_progress")
	}

	sel := *d.selected
	mark := " "
	if sel[ri.run.ID] {
		mark = ui.StyleWarning.Render("●")
	}

	ago := ui.StyleMuted.Render(formatDuration(time.Since(ri.run.CreatedAt).Truncate(time.Minute)) + " ago")
	branch := ui.StyleInfo.Render(ri.run.HeadBranch)
	wfName := ui.StyleMuted.Render(ri.run.Name)

	line1 := fmt.Sprintf(" %s%s #%d %s  %s  %s", mark, icon, ri.run.RunNumber, branch, ago, wfName)
	line2 := fmt.Sprintf("    %s", ri.run.DisplayTitle)

	isFocused := index == m.Index()
	if isFocused {
		hl := lipgloss.NewStyle().Background(lipgloss.Color("#1F2937")).Width(m.Width())
		line1 = hl.Render(line1)
		line2 = hl.Render(line2)
	}

	fmt.Fprintf(w, "%s\n%s", line1, line2)
}

// --- Item ---

type runItem struct {
	run model.Run
}

func (r runItem) FilterValue() string {
	return r.run.Name + " " + r.run.DisplayTitle + " " + r.run.HeadBranch + " " + r.run.Actor.Login
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// --- Model ---

type Model struct {
	list     list.Model
	runs     []model.Run
	selected map[int64]bool
	width    int
	height   int
	loading  bool
	err      error
}

func New() Model {
	sel := make(map[int64]bool)
	delegate := runDelegate{selected: &sel}

	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowFilter(true)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(true)
	l.KeyMap.Filter = key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter"))
	// Rebind internal page navigation to pgup/pgdown only.
	// The app uses l/h/left/right for API-level page navigation;
	// without this override the same keys also trigger the list's
	// built-in NextPage/PrevPage, causing double-handling.
	l.KeyMap.NextPage = key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "next page"))
	l.KeyMap.PrevPage = key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "prev page"))
	l.DisableQuitKeybindings()

	return Model{
		list:     l,
		selected: sel,
		loading:  true,
	}
}

func (m Model) SelectedRun() *model.Run {
	if item, ok := m.list.SelectedItem().(runItem); ok {
		return &item.run
	}
	return nil
}

// SelectedRuns returns all multi-selected run IDs.
func (m Model) SelectedRuns() []int64 {
	var ids []int64
	for id := range m.selected {
		ids = append(ids, id)
	}
	return ids
}

func (m Model) SelectionCount() int {
	return len(m.selected)
}

func (m *Model) ClearSelection() {
	for k := range m.selected {
		delete(m.selected, k)
	}
}

// RunByID returns a pointer to the run with the given ID, or nil.
func (m Model) RunByID(id int64) *model.Run {
	for i := range m.runs {
		if m.runs[i].ID == id {
			return &m.runs[i]
		}
	}
	return nil
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.RunsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.runs = msg.Runs
		for k := range m.selected {
			delete(m.selected, k)
		}
		items := make([]list.Item, len(msg.Runs))
		for i, r := range msg.Runs {
			items[i] = runItem{run: r}
		}
		cmd := m.list.SetItems(items)
		m.list.Select(0)
		return m, cmd

	case ui.RunsPageMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.runs = msg.Runs
		for k := range m.selected {
			delete(m.selected, k)
		}
		items := make([]list.Item, len(msg.Runs))
		for i, r := range msg.Runs {
			items[i] = runItem{run: r}
		}
		cmd := m.list.SetItems(items)
		m.list.Select(0)
		return m, cmd

	case tea.KeyMsg:
		// Ensure the filter key binding is enabled whenever items exist.
		// The list's updateKeybindings can disable it (e.g. after SetSize
		// with zero items); re-enabling here guarantees 'f' always works.
		if msg.String() == "f" && !m.IsFiltering() && len(m.list.Items()) > 0 {
			m.list.KeyMap.Filter.SetEnabled(true)
		}

		// Auto-advance: if at bottom and pressing down, signal for next page
		if !m.IsFiltering() {
			isDown := msg.String() == "j" || msg.Type == tea.KeyDown
			if isDown && len(m.list.Items()) > 0 && m.list.Index() >= len(m.list.Items())-1 {
				return m, func() tea.Msg { return NeedNextPageMsg{} }
			}
		}

		// Toggle selection with space (stay on current row)
		if msg.String() == " " && !m.IsFiltering() {
			if item, ok := m.list.SelectedItem().(runItem); ok {
				id := item.run.ID
				if m.selected[id] {
					delete(m.selected, id)
				} else {
					m.selected[id] = true
				}
			}
			return m, nil
		}

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
		return "\n  Loading runs..."
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
		ui.Keys.Enter,
		ui.Keys.Search,
		ui.Keys.Filter,
		ui.Keys.Refresh,
	}
}
