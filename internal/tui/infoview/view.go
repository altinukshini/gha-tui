package infoview

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/altin/gh-actions-tui/internal/model"
	"github.com/altin/gh-actions-tui/internal/ui"
)

type Model struct {
	run      *model.Run
	jobs     []model.Job
	viewport viewport.Model
	width    int
	height   int
	ready    bool
}

func New() Model {
	return Model{}
}

func (m *Model) SetRun(run *model.Run) {
	m.run = run
	if m.ready {
		m.viewport.SetContent(m.render())
	}
}

func (m *Model) SetJobs(jobs []model.Job) {
	m.jobs = jobs
	if m.ready {
		m.viewport.SetContent(m.render())
	}
}

func (m Model) Run() *model.Run {
	return m.run
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg.(type) {
	case tea.WindowSizeMsg:
		wsm := msg.(tea.WindowSizeMsg)
		m.width = wsm.Width
		m.height = wsm.Height
		headerH := 1
		if !m.ready {
			m.viewport = viewport.New(wsm.Width, wsm.Height-headerH)
			m.ready = true
			if m.run != nil {
				m.viewport.SetContent(m.render())
			}
		} else {
			m.viewport.Width = wsm.Width
			m.viewport.Height = wsm.Height - headerH
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.run == nil {
		return "\n  Select a run and press 'i' to view info"
	}

	pct := m.viewport.ScrollPercent() * 100
	header := fmt.Sprintf(" Run #%d Info  %3.0f%%", m.run.RunNumber, pct)
	hints := lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(
		"  j/k:scroll  g/G:top/bot  PgUp/Dn:page  esc:back")
	headerLine := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color("#F9FAFB")).
		Render(header) + hints

	return headerLine + "\n" + m.viewport.View()
}

func (m Model) render() string {
	if m.run == nil {
		return "  No run selected"
	}

	r := m.run
	bold := lipgloss.NewStyle().Bold(true)
	label := lipgloss.NewStyle().Foreground(ui.ColorMuted).Width(16)
	value := lipgloss.NewStyle().Foreground(lipgloss.Color("#F9FAFB"))

	row := func(l, v string) string {
		return "  " + label.Render(l) + value.Render(v) + "\n"
	}

	var b strings.Builder

	// Title
	b.WriteString("\n")
	b.WriteString("  " + bold.Render(r.DisplayTitle) + "\n")
	b.WriteString("\n")

	// Status with colored icon
	icon := ui.StatusIcon(string(r.Conclusion))
	statusStr := string(r.Status)
	if r.Status == model.RunStatusInProgress {
		icon = ui.StatusIcon("in_progress")
		statusStr = "in_progress"
	} else if r.Status == model.RunStatusQueued {
		icon = ui.StatusIcon("queued")
		statusStr = "queued"
	}
	conclusionPart := ""
	if r.Conclusion != "" {
		conclusionPart = " / " + ui.ConclusionStyle(string(r.Conclusion)).Render(string(r.Conclusion))
	}

	b.WriteString(row("Run", fmt.Sprintf("#%d  (attempt %d)", r.RunNumber, r.RunAttempt)))
	b.WriteString("  " + label.Render("Status") + icon + " " + value.Render(statusStr) + conclusionPart + "\n")
	b.WriteString(row("Actor", r.Actor.Login))
	b.WriteString(row("Branch", r.HeadBranch))
	b.WriteString(row("Commit", r.ShortSHA()))
	b.WriteString(row("Event", r.Event))
	b.WriteString("\n")

	// Timestamps
	b.WriteString("  " + bold.Render("Timestamps") + "\n\n")
	b.WriteString(row("Created", formatTime(r.CreatedAt)))
	b.WriteString(row("Started", formatTime(r.RunStartedAt)))
	b.WriteString(row("Updated", formatTime(r.UpdatedAt)))
	dur := r.Duration().Truncate(time.Second)
	if dur > 0 {
		b.WriteString(row("Duration", dur.String()))
	} else {
		b.WriteString(row("Duration", "-"))
	}
	b.WriteString("\n")

	// URL
	b.WriteString(row("URL", r.HTMLURL))
	b.WriteString("\n")

	// Jobs
	b.WriteString("  " + bold.Render("Jobs") + "\n\n")
	if len(m.jobs) == 0 {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(ui.ColorMuted).Render("Loading jobs...") + "\n")
	} else {
		total := len(m.jobs)
		var passed, failed, cancelled, running, other int
		for _, j := range m.jobs {
			switch {
			case j.Status == model.RunStatusInProgress:
				running++
			case j.Conclusion == model.ConclusionSuccess:
				passed++
			case j.Conclusion == model.ConclusionFailure:
				failed++
			case j.Conclusion == model.ConclusionCancelled:
				cancelled++
			default:
				other++
			}
		}

		parts := []string{fmt.Sprintf("%d total", total)}
		if passed > 0 {
			parts = append(parts, ui.StyleSuccess.Render(fmt.Sprintf("%d passed", passed)))
		}
		if failed > 0 {
			parts = append(parts, ui.StyleFailure.Render(fmt.Sprintf("%d failed", failed)))
		}
		if running > 0 {
			parts = append(parts, ui.StyleInfo.Render(fmt.Sprintf("%d running", running)))
		}
		if cancelled > 0 {
			parts = append(parts, ui.StyleWarning.Render(fmt.Sprintf("%d cancelled", cancelled)))
		}
		if other > 0 {
			parts = append(parts, lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(fmt.Sprintf("%d other", other)))
		}
		b.WriteString("  " + strings.Join(parts, ", ") + "\n\n")

		// Sort jobs alphabetically for consistent display
		sorted := make([]model.Job, len(m.jobs))
		copy(sorted, m.jobs)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Name < sorted[j].Name
		})

		for _, j := range sorted {
			jIcon := ui.StatusIcon(string(j.Conclusion))
			if j.Status == model.RunStatusInProgress {
				jIcon = ui.StatusIcon("in_progress")
			}
			jDur := j.Duration().Truncate(time.Second)
			durStr := jDur.String()
			if jDur == 0 {
				durStr = "-"
			}
			b.WriteString(fmt.Sprintf("  %s %-40s %8s  %d steps\n",
				jIcon, j.Name, durStr, len(j.Steps)))
		}
	}

	return b.String()
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}
