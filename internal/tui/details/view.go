package details

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/altinukshini/gha-tui/internal/model"
	"github.com/altinukshini/gha-tui/internal/ui"
)

// matrixRe matches patterns like "build (ubuntu-latest, 18, debug)"
var matrixRe = regexp.MustCompile(`^(.+?)\s*\((.+)\)$`)

// reusableRe matches patterns like "caller (variant) / called-job" or "caller / called-job"
var reusableRe = regexp.MustCompile(`^(.+?)\s*/\s*(.+)$`)

// JobGroup represents a group of related jobs. Groups can be:
// - A single standalone job
// - Matrix variants of the same job
// - Reusable workflow calls (caller / called-job), possibly with matrix variants
type JobGroup struct {
	// BaseName is the top-level group name (e.g., "deploy-ecs-service" or "build")
	BaseName string
	// CallerGroups holds sub-groups when reusable workflows have matrix variants.
	// Each sub-group represents one caller variant (e.g., "deploy-ecs-service (common)").
	CallerGroups []CallerGroup
	// Jobs holds jobs when there are no reusable workflow sub-groups.
	Jobs []model.Job
	// IsReusable indicates this group contains reusable workflow jobs (has / separator).
	IsReusable bool
}

// CallerGroup represents one caller variant within a reusable workflow group.
type CallerGroup struct {
	CallerName string // e.g., "deploy-ecs-service (common)"
	Jobs       []model.Job
}

func groupJobs(jobs []model.Job) []JobGroup {
	// First pass: detect reusable workflow jobs (contain " / ")
	type parsedJob struct {
		job        model.Job
		caller     string // part before " / " (empty if not reusable)
		calledJob  string // part after " / " (empty if not reusable)
		callerBase string // caller without matrix params
	}

	var parsed []parsedJob
	for _, j := range jobs {
		pj := parsedJob{job: j}
		if m := reusableRe.FindStringSubmatch(j.Name); m != nil {
			pj.caller = strings.TrimSpace(m[1])
			pj.calledJob = strings.TrimSpace(m[2])
			// Extract base name from caller (strip matrix params)
			if mm := matrixRe.FindStringSubmatch(pj.caller); mm != nil {
				pj.callerBase = mm[1]
			} else {
				pj.callerBase = pj.caller
			}
		}
		parsed = append(parsed, pj)
	}

	// Build groups preserving insertion order
	type groupEntry struct {
		name  string
		group *JobGroup
	}
	groupMap := make(map[string]*JobGroup)
	var order []groupEntry

	for _, pj := range parsed {
		if pj.caller != "" {
			// Reusable workflow job
			baseName := pj.callerBase
			g, exists := groupMap[baseName]
			if !exists {
				g = &JobGroup{BaseName: baseName, IsReusable: true}
				groupMap[baseName] = g
				order = append(order, groupEntry{baseName, g})
			}
			// Find or create caller sub-group
			found := false
			for i := range g.CallerGroups {
				if g.CallerGroups[i].CallerName == pj.caller {
					g.CallerGroups[i].Jobs = append(g.CallerGroups[i].Jobs, pj.job)
					found = true
					break
				}
			}
			if !found {
				g.CallerGroups = append(g.CallerGroups, CallerGroup{
					CallerName: pj.caller,
					Jobs:       []model.Job{pj.job},
				})
			}
		} else {
			// Regular job — group by matrix params
			matches := matrixRe.FindStringSubmatch(pj.job.Name)
			var baseName string
			if matches != nil {
				baseName = matches[1]
			} else {
				baseName = pj.job.Name
			}

			g, exists := groupMap[baseName]
			if !exists {
				g = &JobGroup{BaseName: baseName}
				groupMap[baseName] = g
				order = append(order, groupEntry{baseName, g})
			}
			g.Jobs = append(g.Jobs, pj.job)
		}
	}

	result := make([]JobGroup, 0, len(order))
	for _, e := range order {
		result = append(result, *e.group)
	}
	return result
}

type Model struct {
	run      *model.Run
	jobs     []model.Job
	groups   []JobGroup
	viewport viewport.Model
	cursor   int
	width    int
	height   int
	loading  bool
	ready    bool
	err      error
}

func New() Model {
	return Model{}
}

func (m *Model) SetRun(run *model.Run) {
	m.run = run
	m.loading = true
	m.cursor = 0
}

func (m Model) Run() *model.Run {
	return m.run
}

func (m Model) SelectedJob() *model.Job {
	flat := m.flatJobs()
	if m.cursor >= 0 && m.cursor < len(flat) {
		return &flat[m.cursor]
	}
	return nil
}

func (m Model) flatJobs() []model.Job {
	var jobs []model.Job
	for _, g := range m.groups {
		if g.IsReusable {
			for _, cg := range g.CallerGroups {
				jobs = append(jobs, cg.Jobs...)
			}
		} else {
			jobs = append(jobs, g.Jobs...)
		}
	}
	return jobs
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.JobsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.jobs = msg.Jobs
		sort.Slice(m.jobs, func(i, j int) bool {
			return m.jobs[i].Name < m.jobs[j].Name
		})
		m.groups = groupJobs(m.jobs)
		m.cursor = 0
		if m.ready {
			m.viewport.SetContent(m.renderJobs())
		}

	case tea.KeyMsg:
		flat := m.flatJobs()
		switch {
		case key.Matches(msg, ui.Keys.Down):
			if m.cursor < len(flat)-1 {
				m.cursor++
				if m.ready {
					m.viewport.SetContent(m.renderJobs())
				}
			}
		case key.Matches(msg, ui.Keys.Up):
			if m.cursor > 0 {
				m.cursor--
				if m.ready {
					m.viewport.SetContent(m.renderJobs())
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-1)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 2
		}
		m.viewport.SetContent(m.renderJobs())
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// aggregateStatusIcon returns a single status icon summarizing a set of jobs:
// in_progress if any running, failure if any failed, cancelled if any cancelled,
// queued if any queued, success if all succeeded, skipped if all skipped.
func aggregateStatusIcon(jobs []model.Job) string {
	hasInProgress := false
	hasFailure := false
	hasCancelled := false
	hasQueued := false
	for _, j := range jobs {
		if j.Status == model.RunStatusInProgress {
			hasInProgress = true
		} else if j.Status == model.RunStatusQueued || j.Status == model.RunStatusWaiting || j.Status == model.RunStatusPending {
			hasQueued = true
		}
		switch j.Conclusion {
		case model.ConclusionFailure:
			hasFailure = true
		case model.ConclusionCancelled:
			hasCancelled = true
		}
	}
	if hasInProgress {
		return ui.StatusIcon("in_progress")
	}
	if hasFailure {
		return ui.StatusIcon("failure")
	}
	if hasCancelled {
		return ui.StatusIcon("cancelled")
	}
	if hasQueued {
		return ui.StatusIcon("queued")
	}
	return ui.StatusIcon("success")
}

func (m Model) renderJobs() string {
	if len(m.groups) == 0 {
		return "  No jobs"
	}

	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	highlight := lipgloss.NewStyle().Background(lipgloss.Color("#1F2937"))

	var b strings.Builder
	idx := 0

	for _, g := range m.groups {
		if g.IsReusable {
			// Reusable workflow group — aggregate all jobs for top-level icon
			var allJobs []model.Job
			for _, cg := range g.CallerGroups {
				allJobs = append(allJobs, cg.Jobs...)
			}
			groupIcon := aggregateStatusIcon(allJobs)
			b.WriteString(fmt.Sprintf("\n  %s %s %s\n",
				groupIcon,
				bold.Render(g.BaseName),
				muted.Render(fmt.Sprintf("(%d jobs, reusable workflow)", len(allJobs)))))

			hasMultipleCallers := len(g.CallerGroups) > 1
			for _, cg := range g.CallerGroups {
				if hasMultipleCallers {
					// Show caller variant as sub-header with aggregate status
					callerIcon := aggregateStatusIcon(cg.Jobs)
					label := cg.CallerName
					// Strip the base name prefix to show just the variant
					if strings.HasPrefix(label, g.BaseName) {
						suffix := strings.TrimPrefix(label, g.BaseName)
						suffix = strings.TrimSpace(suffix)
						if suffix != "" {
							label = suffix
						}
					}
					b.WriteString(fmt.Sprintf("    %s %s\n", callerIcon, muted.Render(label)))
				}

				for _, j := range cg.Jobs {
					cursor := "  "
					if idx == m.cursor {
						cursor = "> "
					}
					icon := ui.StatusIcon(string(j.Conclusion))
					if j.Status == model.RunStatusInProgress {
						icon = ui.StatusIcon("in_progress")
					}
					dur := j.Duration().Truncate(time.Second)

					// Show just the called job name (part after " / ")
					name := j.Name
					if parts := reusableRe.FindStringSubmatch(j.Name); parts != nil {
						name = strings.TrimSpace(parts[2])
					}

					indent := "    "
					if hasMultipleCallers {
						indent = "      "
					}

					line := fmt.Sprintf("%s%s%s %s  %s  %d steps",
						cursor, indent, icon, name, dur, len(j.Steps))

					if idx == m.cursor {
						line = highlight.Render(line)
					}
					b.WriteString(line + "\n")
					idx++
				}
			}
		} else {
			// Regular job group (matrix or standalone)
			isMatrix := len(g.Jobs) > 1
			if isMatrix {
				b.WriteString(fmt.Sprintf("\n  %s %s\n",
					bold.Render(g.BaseName),
					muted.Render(fmt.Sprintf("(%d variants)", len(g.Jobs)))))
			}

			for _, j := range g.Jobs {
				cursor := "  "
				if idx == m.cursor {
					cursor = "> "
				}
				icon := ui.StatusIcon(string(j.Conclusion))
				if j.Status == model.RunStatusInProgress {
					icon = ui.StatusIcon("in_progress")
				}
				dur := j.Duration().Truncate(time.Second)

				name := j.Name
				if isMatrix {
					name = "  " + name
				}

				line := fmt.Sprintf("%s%s %s  %s  %d steps",
					cursor, icon, name, dur, len(j.Steps))

				if idx == m.cursor {
					line = highlight.Render(line)
				}
				b.WriteString(line + "\n")
				idx++
			}
		}
	}
	return b.String()
}

func (m Model) View() string {
	if m.loading {
		return "\n  Loading jobs..."
	}
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v", m.err)
	}
	if m.run == nil {
		return "\n  Select a run"
	}

	header := fmt.Sprintf(" %s | %s | %s | attempt %d",
		m.run.DisplayTitle,
		m.run.HeadBranch,
		m.run.Event,
		m.run.RunAttempt,
	)

	return lipgloss.NewStyle().Bold(true).Render(header) + "\n" + m.viewport.View()
}
