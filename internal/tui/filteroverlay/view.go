package filteroverlay

import (
	"fmt"
	"strings"

	"github.com/altinukshini/gha-tui/internal/model"
	"github.com/altinukshini/gha-tui/internal/ui"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Filter result
// ---------------------------------------------------------------------------

// FilterResult holds the filter values selected by the user.
type FilterResult struct {
	WorkflowID   int64
	WorkflowName string
	Event        string
	Status       string
	Branch       string
	Actor        string
}

// IsEmpty returns true when no filter criteria are set.
func (f FilterResult) IsEmpty() bool {
	return f.WorkflowID == 0 && f.Event == "" && f.Status == "" && f.Branch == "" && f.Actor == ""
}

// Summary returns a short human-readable summary suitable for a tab label.
func (f FilterResult) Summary() string {
	var parts []string
	if f.WorkflowName != "" {
		parts = append(parts, f.WorkflowName)
	}
	if f.Event != "" {
		parts = append(parts, "event:"+f.Event)
	}
	if f.Status != "" {
		parts = append(parts, "status:"+f.Status)
	}
	if f.Branch != "" {
		parts = append(parts, "branch:"+f.Branch)
	}
	if f.Actor != "" {
		parts = append(parts, "actor:"+f.Actor)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// ---------------------------------------------------------------------------
// Result message
// ---------------------------------------------------------------------------

// ResultMsg is emitted when the user applies or cancels the filter.
type ResultMsg struct {
	Applied bool
	Filter  FilterResult
}

// ---------------------------------------------------------------------------
// Field enum
// ---------------------------------------------------------------------------

type field int

const (
	fieldWorkflow field = iota
	fieldEvent
	fieldStatus
	fieldBranch
	fieldActor
	fieldCount
)

// ---------------------------------------------------------------------------
// Option lists
// ---------------------------------------------------------------------------

var (
	eventOptions  = []string{"push", "pull_request", "schedule", "workflow_dispatch", "workflow_run", "release", "deployment"}
	statusOptions = []string{"completed", "in_progress", "queued", "waiting"}
)

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

// Model is the Bubble Tea model for the server-side filter overlay.
type Model struct {
	active      bool
	focused     field
	workflows   []model.Workflow
	workflowIdx int // -1 = all
	eventIdx    int // -1 = all
	statusIdx   int // -1 = all
	branch      textinput.Model
	actor       textinput.Model
	width       int
	height      int
}

// New creates a new filter overlay pre-populated with the given current filter
// values. The overlay starts in the active state.
func New(workflows []model.Workflow, current FilterResult) Model {
	branch := textinput.New()
	branch.Placeholder = "e.g. main"
	branch.CharLimit = 128
	branch.Width = 30
	branch.SetValue(current.Branch)

	actor := textinput.New()
	actor.Placeholder = "e.g. octocat"
	actor.CharLimit = 128
	actor.Width = 30
	actor.SetValue(current.Actor)

	m := Model{
		active:      true,
		workflows:   workflows,
		workflowIdx: -1,
		eventIdx:    -1,
		statusIdx:   -1,
		branch:      branch,
		actor:       actor,
	}

	// Resolve current workflow selection.
	if current.WorkflowID != 0 {
		for i, w := range workflows {
			if w.ID == current.WorkflowID {
				m.workflowIdx = i
				break
			}
		}
	}

	// Resolve current event selection.
	if current.Event != "" {
		for i, e := range eventOptions {
			if e == current.Event {
				m.eventIdx = i
				break
			}
		}
	}

	// Resolve current status selection.
	if current.Status != "" {
		for i, s := range statusOptions {
			if s == current.Status {
				m.statusIdx = i
				break
			}
		}
	}

	return m
}

// IsActive reports whether the overlay is currently visible.
func (m Model) IsActive() bool { return m.active }

// SetSize stores terminal dimensions so the overlay can centre itself.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Init satisfies the tea.Model interface.
func (m Model) Init() tea.Cmd { return nil }

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update handles key events while the overlay is active.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// When a text input is focused, let it handle most keys first.
		if m.isTextFieldFocused() {
			switch msg.String() {
			case "esc":
				m.active = false
				return m, emitResult(false, FilterResult{})
			case "up", "k":
				m.blurTextInputs()
				m.moveFocus(-1)
				return m, nil
			case "down", "j":
				m.blurTextInputs()
				m.moveFocus(1)
				return m, nil
			case "tab":
				m.blurTextInputs()
				m.moveFocus(1)
				m.focusCurrentTextInput()
				return m, nil
			case "shift+tab":
				m.blurTextInputs()
				m.moveFocus(-1)
				m.focusCurrentTextInput()
				return m, nil
			default:
				// Forward to the active text input.
				var cmd tea.Cmd
				if m.focused == fieldBranch {
					m.branch, cmd = m.branch.Update(msg)
				} else {
					m.actor, cmd = m.actor.Update(msg)
				}
				return m, cmd
			}
		}

		switch msg.String() {
		case "j", "down":
			m.moveFocus(1)
			return m, nil
		case "k", "up":
			m.moveFocus(-1)
			return m, nil

		// Cycle forward / enter text input.
		case "enter", "right", "l":
			switch m.focused {
			case fieldWorkflow:
				m.workflowIdx = cycleForward(m.workflowIdx, len(m.workflows))
			case fieldEvent:
				m.eventIdx = cycleForward(m.eventIdx, len(eventOptions))
			case fieldStatus:
				m.statusIdx = cycleForward(m.statusIdx, len(statusOptions))
			case fieldBranch:
				m.branch.Focus()
				return m, textinput.Blink
			case fieldActor:
				m.actor.Focus()
				return m, textinput.Blink
			}
			return m, nil

		// Cycle backward.
		case "left", "h":
			switch m.focused {
			case fieldWorkflow:
				m.workflowIdx = cycleBackward(m.workflowIdx, len(m.workflows))
			case fieldEvent:
				m.eventIdx = cycleBackward(m.eventIdx, len(eventOptions))
			case fieldStatus:
				m.statusIdx = cycleBackward(m.statusIdx, len(statusOptions))
			}
			return m, nil

		// Apply.
		case "a":
			m.active = false
			return m, emitResult(true, m.buildFilterResult())

		// Clear.
		case "c":
			m.workflowIdx = -1
			m.eventIdx = -1
			m.statusIdx = -1
			m.branch.SetValue("")
			m.actor.SetValue("")
			return m, nil

		// Cancel.
		case "esc":
			m.active = false
			return m, emitResult(false, FilterResult{})
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

// View renders the overlay.
func (m Model) View() string {
	if !m.active {
		return ""
	}

	labelStyle := lipgloss.NewStyle().Width(12).Foreground(ui.ColorMuted)
	focusedLabelStyle := lipgloss.NewStyle().Width(12).Bold(true).Foreground(ui.ColorPrimary)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F9FAFB"))
	allStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted).Italic(true)

	rows := make([]string, 0, int(fieldCount))

	for f := field(0); f < fieldCount; f++ {
		ls := labelStyle
		if f == m.focused {
			ls = focusedLabelStyle
		}

		var label, value string
		switch f {
		case fieldWorkflow:
			label = "Workflow:"
			if m.workflowIdx < 0 || m.workflowIdx >= len(m.workflows) {
				value = allStyle.Render("All workflows")
			} else {
				value = valueStyle.Render(m.workflows[m.workflowIdx].Name)
			}
		case fieldEvent:
			label = "Event:"
			if m.eventIdx < 0 || m.eventIdx >= len(eventOptions) {
				value = allStyle.Render("All events")
			} else {
				value = valueStyle.Render(eventOptions[m.eventIdx])
			}
		case fieldStatus:
			label = "Status:"
			if m.statusIdx < 0 || m.statusIdx >= len(statusOptions) {
				value = allStyle.Render("All statuses")
			} else {
				value = valueStyle.Render(statusOptions[m.statusIdx])
			}
		case fieldBranch:
			label = "Branch:"
			value = m.branch.View()
		case fieldActor:
			label = "Actor:"
			value = m.actor.View()
		}

		cursor := "  "
		if f == m.focused {
			cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Render("> ")
		}

		rows = append(rows, fmt.Sprintf("%s%s %s", cursor, ls.Render(label), value))
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.ColorPrimary).
		MarginBottom(1).
		Render("Server-Side Filter")

	help := lipgloss.NewStyle().
		Foreground(ui.ColorMuted).
		MarginTop(1).
		Render("a: apply  c: clear  esc: cancel")

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		strings.Join(rows, "\n"),
		help,
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorPrimary).
		Padding(1, 2).
		Width(56)

	box := boxStyle.Render(body)

	// Centre the box in the terminal.
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			box)
	}
	return box
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (m *Model) moveFocus(delta int) {
	next := int(m.focused) + delta
	if next < 0 {
		next = int(fieldCount) - 1
	}
	if next >= int(fieldCount) {
		next = 0
	}
	m.focused = field(next)
}

func (m Model) isTextFieldFocused() bool {
	return m.branch.Focused() || m.actor.Focused()
}

func (m *Model) blurTextInputs() {
	m.branch.Blur()
	m.actor.Blur()
}

func (m *Model) focusCurrentTextInput() {
	if m.focused == fieldBranch {
		m.branch.Focus()
	} else if m.focused == fieldActor {
		m.actor.Focus()
	}
}

func (m Model) buildFilterResult() FilterResult {
	r := FilterResult{
		Branch: strings.TrimSpace(m.branch.Value()),
		Actor:  strings.TrimSpace(m.actor.Value()),
	}
	if m.workflowIdx >= 0 && m.workflowIdx < len(m.workflows) {
		r.WorkflowID = m.workflows[m.workflowIdx].ID
		r.WorkflowName = m.workflows[m.workflowIdx].Name
	}
	if m.eventIdx >= 0 && m.eventIdx < len(eventOptions) {
		r.Event = eventOptions[m.eventIdx]
	}
	if m.statusIdx >= 0 && m.statusIdx < len(statusOptions) {
		r.Status = statusOptions[m.statusIdx]
	}
	return r
}

// cycleForward advances the index by one. -1 means "all", 0..max-1 are the
// actual entries, and going past the last entry wraps back to -1 (all).
func cycleForward(idx, count int) int {
	if count == 0 {
		return -1
	}
	idx++
	if idx >= count {
		idx = -1
	}
	return idx
}

// cycleBackward is the reverse of cycleForward.
func cycleBackward(idx, count int) int {
	if count == 0 {
		return -1
	}
	idx--
	if idx < -1 {
		idx = count - 1
	}
	return idx
}

func emitResult(applied bool, f FilterResult) tea.Cmd {
	return func() tea.Msg {
		return ResultMsg{Applied: applied, Filter: f}
	}
}
