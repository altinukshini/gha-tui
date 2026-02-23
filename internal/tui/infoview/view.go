package infoview

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/altinukshini/gha-tui/internal/model"
	"github.com/altinukshini/gha-tui/internal/ui"
)

type Model struct {
	run        *model.Run
	jobs       []model.Job
	job        *model.Job
	showingJob bool
	viewport   viewport.Model
	width      int
	height     int
	ready      bool
}

func New() Model {
	return Model{}
}

func (m *Model) SetRun(run *model.Run) {
	m.run = run
	m.showingJob = false
	if m.ready {
		m.viewport.SetContent(m.render())
	}
}

func (m *Model) SetJob(job *model.Job) {
	m.job = job
	m.showingJob = true
	if m.ready {
		m.viewport.SetContent(m.render())
	}
}

func (m Model) Job() *model.Job {
	return m.job
}

func (m Model) IsShowingJob() bool {
	return m.showingJob
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
			if m.run != nil || m.job != nil {
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
	if m.showingJob {
		if m.job == nil {
			return "\n  Select a job and press 'i' to view info"
		}
	} else {
		if m.run == nil {
			return "\n  Select a run and press 'i' to view info"
		}
	}

	pct := m.viewport.ScrollPercent() * 100
	var header string
	if m.showingJob {
		header = fmt.Sprintf(" Job Info  %3.0f%%", pct)
	} else {
		header = fmt.Sprintf(" Run #%d Info  %3.0f%%", m.run.RunNumber, pct)
	}
	hints := lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(
		"  j/k:scroll  g/G:top/bot  PgUp/Dn:page  esc:back")
	headerLine := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color("#F9FAFB")).
		Render(header) + hints

	return headerLine + "\n" + m.viewport.View()
}

func (m Model) render() string {
	if m.showingJob {
		return m.renderJob()
	}
	return m.renderRun()
}

func (m Model) renderRun() string {
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

func (m Model) renderJob() string {
	if m.job == nil {
		return "  No job selected"
	}

	j := m.job
	bold := lipgloss.NewStyle().Bold(true)
	label := lipgloss.NewStyle().Foreground(ui.ColorMuted).Width(16)
	value := lipgloss.NewStyle().Foreground(lipgloss.Color("#F9FAFB"))

	row := func(l, v string) string {
		return "  " + label.Render(l) + value.Render(v) + "\n"
	}

	var b strings.Builder

	// Title
	b.WriteString("\n")
	b.WriteString("  " + bold.Render(j.Name) + "\n")
	b.WriteString("\n")

	// Status with colored icon
	icon := ui.StatusIcon(string(j.Conclusion))
	statusStr := string(j.Status)
	if j.Status == model.RunStatusInProgress {
		icon = ui.StatusIcon("in_progress")
		statusStr = "in_progress"
	} else if j.Status == model.RunStatusQueued {
		icon = ui.StatusIcon("queued")
		statusStr = "queued"
	}
	conclusionPart := ""
	if j.Conclusion != "" {
		conclusionPart = " / " + ui.ConclusionStyle(string(j.Conclusion)).Render(string(j.Conclusion))
	}

	b.WriteString("  " + label.Render("Status") + icon + " " + value.Render(statusStr) + conclusionPart + "\n")
	if j.RunnerName != "" {
		b.WriteString(row("Runner", j.RunnerName))
	}
	b.WriteString("\n")

	// Timestamps
	b.WriteString("  " + bold.Render("Timestamps") + "\n\n")
	b.WriteString(row("Started", formatTime(j.StartedAt)))
	b.WriteString(row("Completed", formatTime(j.CompletedAt)))
	dur := j.Duration().Truncate(time.Second)
	if dur > 0 {
		b.WriteString(row("Duration", dur.String()))
	} else if j.Status == model.RunStatusInProgress && !j.StartedAt.IsZero() {
		elapsed := time.Since(j.StartedAt).Truncate(time.Second)
		b.WriteString(row("Elapsed", elapsed.String()))
	} else {
		b.WriteString(row("Duration", "-"))
	}
	b.WriteString("\n")

	// URL
	if j.HTMLURL != "" {
		b.WriteString(row("URL", j.HTMLURL))
		b.WriteString("\n")
	}

	// Steps
	b.WriteString("  " + bold.Render("Steps") + "\n\n")
	if len(j.Steps) == 0 {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(ui.ColorMuted).Render("No steps available") + "\n")
	} else {
		// Steps summary
		total := len(j.Steps)
		var passed, failed, cancelled, running, skipped, other int
		for _, s := range j.Steps {
			switch {
			case s.Status == model.RunStatusInProgress:
				running++
			case s.Conclusion == model.ConclusionSuccess:
				passed++
			case s.Conclusion == model.ConclusionFailure:
				failed++
			case s.Conclusion == model.ConclusionCancelled:
				cancelled++
			case s.Conclusion == model.ConclusionSkipped:
				skipped++
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
		if skipped > 0 {
			parts = append(parts, lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(fmt.Sprintf("%d skipped", skipped)))
		}
		if other > 0 {
			parts = append(parts, lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(fmt.Sprintf("%d other", other)))
		}
		b.WriteString("  " + strings.Join(parts, ", ") + "\n\n")

		// Steps table
		for _, s := range j.Steps {
			sIcon := ui.StatusIcon(string(s.Conclusion))
			if s.Status == model.RunStatusInProgress {
				sIcon = ui.StatusIcon("in_progress")
			} else if s.Status == model.RunStatusQueued {
				sIcon = ui.StatusIcon("queued")
			}

			sDur := ""
			if !s.CompletedAt.IsZero() && !s.StartedAt.IsZero() {
				d := s.CompletedAt.Sub(s.StartedAt).Truncate(time.Second)
				sDur = d.String()
			} else if s.Status == model.RunStatusInProgress && !s.StartedAt.IsZero() {
				d := time.Since(s.StartedAt).Truncate(time.Second)
				sDur = d.String()
			} else {
				sDur = "-"
			}

			stepName := s.Name
			suffix := ""
			if s.Conclusion == model.ConclusionFailure {
				stepName = ui.StyleFailure.Render(s.Name)
				suffix = ui.StyleFailure.Render("  ← FAILED")
			} else if s.Status == model.RunStatusInProgress {
				stepName = ui.StyleInfo.Render(s.Name)
				suffix = ui.StyleInfo.Render("  ← RUNNING")
			}

			b.WriteString(fmt.Sprintf("  %2d  %s  %-40s %8s%s\n",
				s.Number, sIcon, stepName, sDur, suffix))
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
