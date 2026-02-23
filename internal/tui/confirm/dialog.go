package confirm

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ResultMsg struct {
	Confirmed bool
	Action    string
	Data      interface{}
}

type Model struct {
	Title    string
	Message  string
	Action   string
	Data     interface{}
	active   bool
	selected bool // true = confirm selected
}

func New(title, message, action string, data interface{}) Model {
	return Model{
		Title:   title,
		Message: message,
		Action:  action,
		Data:    data,
		active:  true,
	}
}

func (m Model) IsActive() bool { return m.active }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.active = false
			return m, func() tea.Msg {
				return ResultMsg{Confirmed: true, Action: m.Action, Data: m.Data}
			}
		case "n", "N", "esc":
			m.active = false
			return m, func() tea.Msg {
				return ResultMsg{Confirmed: false, Action: m.Action, Data: m.Data}
			}
		case "enter":
			m.active = false
			return m, func() tea.Msg {
				return ResultMsg{Confirmed: m.selected, Action: m.Action, Data: m.Data}
			}
		case "tab", "left", "right", "h", "l":
			m.selected = !m.selected
		}
	}
	return m, nil
}

func (m Model) View() string {
	if !m.active {
		return ""
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#F59E0B")).
		Padding(1, 2).
		Width(50)

	title := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color("#F59E0B")).
		Render(m.Title)

	yesStyle := lipgloss.NewStyle().Padding(0, 1)
	noStyle := lipgloss.NewStyle().Padding(0, 1)

	if m.selected {
		yesStyle = yesStyle.Bold(true).Background(lipgloss.Color("#10B981")).Foreground(lipgloss.Color("#F9FAFB"))
		noStyle = noStyle.Foreground(lipgloss.Color("#6B7280"))
	} else {
		yesStyle = yesStyle.Foreground(lipgloss.Color("#6B7280"))
		noStyle = noStyle.Bold(true).Background(lipgloss.Color("#EF4444")).Foreground(lipgloss.Color("#F9FAFB"))
	}

	content := fmt.Sprintf("%s\n\n%s\n\n%s  %s\n\ny/n to confirm, esc to cancel",
		title, m.Message,
		yesStyle.Render("Yes"), noStyle.Render("No"))

	return style.Render(content)
}
