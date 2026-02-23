package logview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	viewport viewport.Model
	content  string
	jobName  string
	width    int
	height   int
	ready    bool
	loading  bool

	// In-log search
	searchInput   textinput.Model
	searching     bool
	searchQuery   string
	matchLines    []int // 0-based line indices of matches
	matchIndex    int   // current match position
	matchTotal    int

	// Jump highlight (from cross-log search result)
	jumpLine int // 0-based line to highlight, -1 = none

	// Live tailing for in-progress jobs
	tailing bool
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Search in log..."
	ti.CharLimit = 256
	return Model{searchInput: ti, jumpLine: -1}
}

func (m *Model) SetContent(jobName, content string) {
	m.jobName = jobName
	m.content = content
	m.loading = false
	m.searchQuery = ""
	m.matchLines = nil
	m.matchIndex = 0
	m.matchTotal = 0
	m.jumpLine = -1
	if m.ready {
		m.viewport.SetContent(content)
		m.viewport.GotoTop()
	}
}

func (m *Model) SetLoading() {
	m.loading = true
}

func (m *Model) GotoLine(line int) {
	if line > 0 {
		m.jumpLine = line - 1 // convert 1-based to 0-based
		m.viewport.SetContent(m.applyHighlights())
		m.viewport.SetYOffset(line - 1)
	}
}

// UpdateContent replaces the log content while preserving scroll position.
// If the viewport was at the bottom (following), it auto-scrolls to bottom.
// Otherwise it restores the previous YOffset.
func (m *Model) UpdateContent(content string) {
	m.content = content
	if !m.ready {
		return
	}

	wasAtBottom := m.viewport.AtBottom()
	prevOffset := m.viewport.YOffset

	m.viewport.SetContent(m.applyHighlights())

	if wasAtBottom {
		m.viewport.GotoBottom()
	} else {
		maxOffset := m.viewport.TotalLineCount() - m.viewport.VisibleLineCount()
		if maxOffset < 0 {
			maxOffset = 0
		}
		if prevOffset > maxOffset {
			m.viewport.GotoBottom()
		} else {
			m.viewport.SetYOffset(prevOffset)
		}
	}
}

func (m *Model) SetTailing(tailing bool) {
	m.tailing = tailing
}

func (m Model) IsTailing() bool {
	return m.tailing
}

func (m Model) IsSearching() bool {
	return m.searching
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.searching {
			switch msg.String() {
			case "enter":
				query := m.searchInput.Value()
				if query != "" {
					m.searchQuery = query
					m.findMatches()
					m.viewport.SetContent(m.applyHighlights())
					if len(m.matchLines) > 0 {
						m.matchIndex = 0
						m.viewport.SetYOffset(m.matchLines[0])
					}
				}
				m.searching = false
				m.searchInput.Blur()
				return m, nil
			case "esc":
				m.searching = false
				m.searchInput.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "/":
			m.searching = true
			m.jumpLine = -1
			m.searchInput.SetValue("")
			m.searchInput.Focus()
			return m, textinput.Blink
		case "n":
			if len(m.matchLines) > 0 {
				m.matchIndex = (m.matchIndex + 1) % len(m.matchLines)
				m.viewport.SetContent(m.applyHighlights())
				m.viewport.SetYOffset(m.matchLines[m.matchIndex])
			}
			return m, nil
		case "N":
			if len(m.matchLines) > 0 {
				m.matchIndex = (m.matchIndex - 1 + len(m.matchLines)) % len(m.matchLines)
				m.viewport.SetContent(m.applyHighlights())
				m.viewport.SetYOffset(m.matchLines[m.matchIndex])
			}
			return m, nil
		case "g":
			m.viewport.GotoTop()
			return m, nil
		case "G":
			m.viewport.GotoBottom()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerH := 1
		if m.searching {
			headerH = 2
		}
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerH)
			m.ready = true
			if m.content != "" {
				m.viewport.SetContent(m.applyHighlights())
			}
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerH
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *Model) findMatches() {
	m.matchLines = nil
	if m.searchQuery == "" || m.content == "" {
		m.matchTotal = 0
		return
	}
	query := strings.ToLower(m.searchQuery)
	lines := strings.Split(m.content, "\n")
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			m.matchLines = append(m.matchLines, i)
		}
	}
	m.matchTotal = len(m.matchLines)
}

// applyHighlights returns the content with matching lines and jump line highlighted.
func (m Model) applyHighlights() string {
	hasSearch := m.searchQuery != "" && len(m.matchLines) > 0
	hasJump := m.jumpLine >= 0

	if !hasSearch && !hasJump {
		return m.content
	}

	matchSet := make(map[int]bool)
	for _, idx := range m.matchLines {
		matchSet[idx] = true
	}

	currentMatchLine := -1
	if m.matchIndex >= 0 && m.matchIndex < len(m.matchLines) {
		currentMatchLine = m.matchLines[m.matchIndex]
	}

	highlight := lipgloss.NewStyle().Background(lipgloss.Color("#374151"))
	current := lipgloss.NewStyle().Background(lipgloss.Color("#92400E")).Bold(true)

	lines := strings.Split(m.content, "\n")
	for i, line := range lines {
		if i == currentMatchLine {
			lines[i] = current.Render(line)
		} else if hasJump && i == m.jumpLine {
			lines[i] = current.Render(line)
		} else if matchSet[i] {
			lines[i] = highlight.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) View() string {
	if m.loading {
		return "\n  Loading logs..."
	}
	if m.content == "" {
		return "\n  Select a job to view logs"
	}

	// Header line
	liveTag := ""
	if m.tailing {
		liveTag = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981")).Render(" [LIVE]")
	}
	headerParts := fmt.Sprintf(" %s%s  %3.f%%", m.jobName, liveTag, m.viewport.ScrollPercent()*100)
	if m.searchQuery != "" && m.matchTotal > 0 {
		headerParts += fmt.Sprintf("  [%d/%d matches]", m.matchIndex+1, m.matchTotal)
	} else if m.searchQuery != "" {
		headerParts += "  [no matches]"
	}
	hints := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(
		"  /:search  n/N:match  j/k:line  PgUp/PgDn:page  g/G:top/bot  esc:back")
	header := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color("#F9FAFB")).
		Render(headerParts) + hints

	if m.searching {
		searchLine := "  /" + m.searchInput.View()
		return header + "\n" + searchLine + "\n" + m.viewport.View()
	}

	return header + "\n" + m.viewport.View()
}
