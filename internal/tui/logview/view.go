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

	// Word wrap
	wrap            bool
	sourceToWrapped []int // maps source line index → first wrapped line index

	// In-log search
	searchInput textinput.Model
	searching   bool
	searchQuery string
	matchLines  []int // 0-based line indices of matches (in source lines)
	matchIndex  int   // current match position
	matchTotal  int

	// Jump highlight (from cross-log search result)
	jumpLine int // 0-based source line to highlight, -1 = none

	// Live tailing for in-progress jobs
	tailing bool
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Search in log..."
	ti.CharLimit = 256
	return Model{searchInput: ti, jumpLine: -1, wrap: true}
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
		m.refreshViewport()
		m.viewport.GotoTop()
	}
}

func (m *Model) SetLoading() {
	m.loading = true
}

func (m *Model) GotoLine(line int) {
	if line > 0 {
		m.jumpLine = line - 1 // convert 1-based to 0-based
		m.refreshViewport()
		m.viewport.SetYOffset(m.wrappedLineFor(line - 1))
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

	m.refreshViewport()

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
					m.refreshViewport()
					if len(m.matchLines) > 0 {
						m.matchIndex = 0
						m.viewport.SetYOffset(m.wrappedLineFor(m.matchLines[0]))
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
				m.refreshViewport()
				m.viewport.SetYOffset(m.wrappedLineFor(m.matchLines[m.matchIndex]))
			}
			return m, nil
		case "N":
			if len(m.matchLines) > 0 {
				m.matchIndex = (m.matchIndex - 1 + len(m.matchLines)) % len(m.matchLines)
				m.refreshViewport()
				m.viewport.SetYOffset(m.wrappedLineFor(m.matchLines[m.matchIndex]))
			}
			return m, nil
		case "w":
			m.wrap = !m.wrap
			if m.content != "" {
				m.refreshViewport()
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
				m.refreshViewport()
			}
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerH
			if m.content != "" {
				m.refreshViewport()
			}
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

// refreshViewport wraps content (if enabled), applies search/jump highlights,
// and sets the result into the viewport. It also builds the source-to-wrapped
// line mapping used for search navigation.
func (m *Model) refreshViewport() {
	if !m.ready || m.content == "" {
		if m.ready {
			m.viewport.SetContent("")
		}
		m.sourceToWrapped = nil
		return
	}

	sourceLines := strings.Split(m.content, "\n")

	// Build highlight lookup
	hasSearch := m.searchQuery != "" && len(m.matchLines) > 0
	hasJump := m.jumpLine >= 0

	matchSet := make(map[int]bool)
	for _, idx := range m.matchLines {
		matchSet[idx] = true
	}
	currentMatchLine := -1
	if m.matchIndex >= 0 && m.matchIndex < len(m.matchLines) {
		currentMatchLine = m.matchLines[m.matchIndex]
	}

	highlightStyle := lipgloss.NewStyle().Background(lipgloss.Color("#374151"))
	currentStyle := lipgloss.NewStyle().Background(lipgloss.Color("#92400E")).Bold(true)

	wrapWidth := m.width
	doWrap := m.wrap && wrapWidth > 0

	m.sourceToWrapped = make([]int, len(sourceLines))
	var wrappedLines []string

	for i, line := range sourceLines {
		m.sourceToWrapped[i] = len(wrappedLines)

		// Wrap the raw line first (before adding ANSI codes)
		var segments []string
		if doWrap {
			segments = wrapLine(line, wrapWidth)
		} else {
			segments = []string{line}
		}

		// Apply highlight to all segments of this source line
		if hasSearch || hasJump {
			if i == currentMatchLine || (hasJump && i == m.jumpLine) {
				for j := range segments {
					segments[j] = currentStyle.Render(segments[j])
				}
			} else if matchSet[i] {
				for j := range segments {
					segments[j] = highlightStyle.Render(segments[j])
				}
			}
		}

		wrappedLines = append(wrappedLines, segments...)
	}

	m.viewport.SetContent(strings.Join(wrappedLines, "\n"))
}

// wrappedLineFor translates a source line index to a viewport line index.
func (m Model) wrappedLineFor(sourceLine int) int {
	if m.sourceToWrapped == nil || sourceLine >= len(m.sourceToWrapped) {
		return sourceLine
	}
	return m.sourceToWrapped[sourceLine]
}

// wrapLine hard-wraps a single line into segments of at most width runes.
func wrapLine(line string, width int) []string {
	runes := []rune(line)
	if len(runes) <= width {
		return []string{line}
	}
	var segments []string
	for len(runes) > width {
		segments = append(segments, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 {
		segments = append(segments, string(runes))
	}
	return segments
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
	wrapTag := ""
	if m.wrap {
		wrapTag = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" [wrap]")
	}
	headerParts := fmt.Sprintf(" %s%s%s  %3.f%%", m.jobName, liveTag, wrapTag, m.viewport.ScrollPercent()*100)
	if m.searchQuery != "" && m.matchTotal > 0 {
		headerParts += fmt.Sprintf("  [%d/%d matches]", m.matchIndex+1, m.matchTotal)
	} else if m.searchQuery != "" {
		headerParts += "  [no matches]"
	}
	hints := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(
		"  /:search  n/N:match  w:wrap  g/G:top/bot  esc:back")
	header := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color("#F9FAFB")).
		Render(headerParts) + hints

	if m.searching {
		searchLine := "  /" + m.searchInput.View()
		return header + "\n" + searchLine + "\n" + m.viewport.View()
	}

	return header + "\n" + m.viewport.View()
}
