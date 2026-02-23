package searchview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/altinukshini/gha-tui/internal/model"
	"github.com/altinukshini/gha-tui/internal/ui"
)

type Mode int

const (
	ModeInput Mode = iota
	ModeResults
)

type Model struct {
	input    textinput.Model
	viewport viewport.Model
	results  *model.SearchResults
	mode     Mode
	cursor   int
	width    int
	height   int
	loading  bool
	active   bool
	ready    bool
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Search pattern (/ for regex)"
	ti.CharLimit = 256

	return Model{
		input: ti,
	}
}

func (m *Model) Activate() {
	m.active = true
	m.mode = ModeInput
	m.input.Focus()
}

func (m *Model) Deactivate() {
	m.active = false
	m.input.Blur()
}

func (m Model) IsActive() bool {
	return m.active
}

// IsInputMode returns true when the search view is in input mode (typing a query).
func (m Model) IsInputMode() bool {
	return m.mode == ModeInput
}

// ActivateResults re-enters the search view in results mode,
// preserving existing results and cursor position.
func (m *Model) ActivateResults() {
	m.active = true
	m.mode = ModeResults
}

func (m Model) Query() string {
	return m.input.Value()
}

func (m Model) SelectedMatch() *model.SearchResult {
	if m.results == nil || m.cursor >= len(m.results.Matches) {
		return nil
	}
	return &m.results.Matches[m.cursor]
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.SearchDoneMsg:
		m.loading = false
		if msg.Err != nil {
			return m, nil
		}
		m.results = msg.Results
		m.cursor = 0
		m.mode = ModeResults
		m.input.Blur()
		if m.ready {
			m.viewport.SetContent(m.renderResults())
		}

	case tea.KeyMsg:
		if m.mode == ModeInput {
			switch msg.String() {
			case "enter":
				if m.input.Value() != "" {
					m.loading = true
					return m, nil // parent handles dispatching search
				}
			case "esc":
				m.Deactivate()
				return m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		// Results mode
		switch {
		case key.Matches(msg, ui.Keys.Down):
			if m.results != nil && m.cursor < len(m.results.Matches)-1 {
				m.cursor++
				if m.ready {
					m.viewport.SetContent(m.renderResults())
				}
			}
		case key.Matches(msg, ui.Keys.Up):
			if m.cursor > 0 {
				m.cursor--
				if m.ready {
					m.viewport.SetContent(m.renderResults())
				}
			}
		case key.Matches(msg, ui.Keys.Search):
			m.mode = ModeInput
			m.input.Focus()
		case key.Matches(msg, ui.Keys.Back):
			m.Deactivate()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-4)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 4
		}
		if m.results != nil {
			m.viewport.SetContent(m.renderResults())
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) renderResults() string {
	if m.results == nil || m.results.TotalCount == 0 {
		return "  No matches"
	}

	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	highlight := lipgloss.NewStyle().Background(lipgloss.Color("#1F2937"))

	var b strings.Builder
	b.WriteString(fmt.Sprintf("  %d matches across %d jobs\n",
		m.results.TotalCount, len(m.results.JobCounts)))
	b.WriteString(muted.Render("  enter:view log  j/k:navigate  /:new search  esc:close") + "\n\n")

	for name, count := range m.results.JobCounts {
		b.WriteString(fmt.Sprintf("  %s: %d matches\n", name, count))
	}
	b.WriteString("\n")

	currentJob := ""
	for i, match := range m.results.Matches {
		if match.JobName != currentJob {
			currentJob = match.JobName
			b.WriteString(fmt.Sprintf("  --- %s ---\n", bold.Render(currentJob)))
		}

		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		line := fmt.Sprintf("%sL%d: %s", cursor, match.Line, match.Content)
		if i == m.cursor {
			line = highlight.Render(line)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m Model) View() string {
	if !m.active {
		return ""
	}

	var b strings.Builder
	b.WriteString("  " + m.input.View() + "\n")

	if m.loading {
		b.WriteString("\n  Searching...")
	} else if m.ready {
		b.WriteString(m.viewport.View())
	}
	return b.String()
}
