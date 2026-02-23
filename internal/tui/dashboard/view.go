package dashboard

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/altin/gh-actions-tui/internal/model"
	"github.com/altin/gh-actions-tui/internal/ui"
)

type TimeWindow struct {
	Label string
	Days  int
}

func (w TimeWindow) String() string {
	return w.Label
}

// DefaultWindows is used as a fallback when retention days are unknown.
var DefaultWindows = []TimeWindow{
	{Label: "24h", Days: 1},
	{Label: "7d", Days: 7},
	{Label: "30d", Days: 30},
}

// WindowsForRetention builds time window options from the repo's retention days.
// It always includes 1d and 7d, then adds 30d and 90d if within the limit,
// and always includes the max retention itself.
func WindowsForRetention(retentionDays int) []TimeWindow {
	if retentionDays <= 0 {
		return DefaultWindows
	}

	candidates := []TimeWindow{
		{Label: "24h", Days: 1},
		{Label: "7d", Days: 7},
		{Label: "30d", Days: 30},
		{Label: "90d", Days: 90},
	}

	var windows []TimeWindow
	for _, c := range candidates {
		if c.Days <= retentionDays {
			windows = append(windows, c)
		}
	}

	// Add the max retention as the last option if it's not already there
	last := windows[len(windows)-1]
	if last.Days != retentionDays {
		windows = append(windows, TimeWindow{
			Label: fmt.Sprintf("%dd", retentionDays),
			Days:  retentionDays,
		})
	}

	return windows
}

type Metrics struct {
	TotalRuns      int // API total count (may exceed sampled runs)
	SampledRuns    int // number of runs actually fetched and analyzed
	SuccessCount   int
	FailureCount   int
	CancelCount    int
	SuccessRate    float64
	FailureRate    float64
	MedianDuration float64 // seconds
	P95Duration    float64 // seconds
	RetryRate      float64
	TopFailing     []WorkflowStat
	TopFailingJobs []JobStat

	// Usage breakdowns
	RunsByEvent  map[string]int
	RunsByActor  map[string]int
	RunsByBranch map[string]int

	// Performance
	MeanDuration    float64
	P99Duration     float64
	MeanQueueTime   float64
	MedianQueueTime float64
	P95QueueTime    float64
	SlowestWorkflows []WorkflowDurationStat

	// Job-level
	TotalJobs       int
	JobSuccessCount int
	JobFailureCount int
	MeanJobDuration   float64
	MedianJobDuration float64
	P95JobDuration    float64
}

type WorkflowStat struct {
	Name         string
	FailureCount int
	TotalRuns    int
	FailureRate  float64
}

type JobStat struct {
	Name         string
	FailureCount int
}

type WorkflowDurationStat struct {
	Name           string
	MedianDuration float64
	P95Duration    float64
	RunCount       int
}

type mapEntry struct {
	Key   string
	Value int
}

func sortMapByValue(m map[string]int) []mapEntry {
	entries := make([]mapEntry, 0, len(m))
	for k, v := range m {
		entries = append(entries, mapEntry{k, v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Value > entries[j].Value
	})
	return entries
}

func ComputeMetrics(runs []model.Run, jobs []model.Job, totalCount int) Metrics {
	m := Metrics{}
	m.SampledRuns = len(runs)
	m.TotalRuns = totalCount
	if m.TotalRuns < m.SampledRuns {
		m.TotalRuns = m.SampledRuns
	}
	if m.SampledRuns == 0 {
		return m
	}

	var durations []float64
	retries := 0
	wfCounts := make(map[string]*WorkflowStat)
	jobFails := make(map[string]int)

	m.RunsByEvent = make(map[string]int)
	m.RunsByActor = make(map[string]int)
	m.RunsByBranch = make(map[string]int)
	wfDurations := make(map[string][]float64)
	var queueTimes []float64

	for _, r := range runs {
		switch r.Conclusion {
		case model.ConclusionSuccess:
			m.SuccessCount++
		case model.ConclusionFailure:
			m.FailureCount++
		case model.ConclusionCancelled:
			m.CancelCount++
		}
		if r.RunAttempt > 1 {
			retries++
		}
		dur := r.Duration().Seconds()
		if dur > 0 {
			durations = append(durations, dur)
			wfDurations[r.Name] = append(wfDurations[r.Name], dur)
		}

		ws, ok := wfCounts[r.Name]
		if !ok {
			ws = &WorkflowStat{Name: r.Name}
			wfCounts[r.Name] = ws
		}
		ws.TotalRuns++
		if r.Conclusion == model.ConclusionFailure {
			ws.FailureCount++
		}

		// Usage breakdowns
		if r.Event != "" {
			m.RunsByEvent[r.Event]++
		}
		if r.Actor.Login != "" {
			m.RunsByActor[r.Actor.Login]++
		}
		if r.HeadBranch != "" {
			m.RunsByBranch[r.HeadBranch]++
		}

		// Queue time
		if !r.RunStartedAt.IsZero() && !r.CreatedAt.IsZero() {
			qt := r.RunStartedAt.Sub(r.CreatedAt).Seconds()
			if qt >= 0 {
				queueTimes = append(queueTimes, qt)
			}
		}
	}

	for _, j := range jobs {
		if j.Failed() {
			jobFails[j.Name]++
		}
	}

	m.SuccessRate = float64(m.SuccessCount) / float64(m.SampledRuns) * 100
	m.FailureRate = float64(m.FailureCount) / float64(m.SampledRuns) * 100
	m.RetryRate = float64(retries) / float64(m.SampledRuns) * 100

	sort.Float64s(durations)
	if len(durations) > 0 {
		m.MedianDuration = percentile(durations, 50)
		m.P95Duration = percentile(durations, 95)
		m.P99Duration = percentile(durations, 99)

		sum := 0.0
		for _, d := range durations {
			sum += d
		}
		m.MeanDuration = sum / float64(len(durations))
	}

	// Queue time stats
	sort.Float64s(queueTimes)
	if len(queueTimes) > 0 {
		qtSum := 0.0
		for _, qt := range queueTimes {
			qtSum += qt
		}
		m.MeanQueueTime = qtSum / float64(len(queueTimes))
		m.MedianQueueTime = percentile(queueTimes, 50)
		m.P95QueueTime = percentile(queueTimes, 95)
	}

	// Slowest workflows by p95 duration
	for name, durs := range wfDurations {
		sorted := make([]float64, len(durs))
		copy(sorted, durs)
		sort.Float64s(sorted)
		m.SlowestWorkflows = append(m.SlowestWorkflows, WorkflowDurationStat{
			Name:           name,
			MedianDuration: percentile(sorted, 50),
			P95Duration:    percentile(sorted, 95),
			RunCount:       len(sorted),
		})
	}
	sort.Slice(m.SlowestWorkflows, func(i, j int) bool {
		return m.SlowestWorkflows[i].P95Duration > m.SlowestWorkflows[j].P95Duration
	})
	if len(m.SlowestWorkflows) > 5 {
		m.SlowestWorkflows = m.SlowestWorkflows[:5]
	}

	for _, ws := range wfCounts {
		if ws.FailureCount > 0 {
			ws.FailureRate = float64(ws.FailureCount) / float64(ws.TotalRuns) * 100
			m.TopFailing = append(m.TopFailing, *ws)
		}
	}
	sort.Slice(m.TopFailing, func(i, j int) bool {
		return m.TopFailing[i].FailureCount > m.TopFailing[j].FailureCount
	})
	if len(m.TopFailing) > 5 {
		m.TopFailing = m.TopFailing[:5]
	}

	for name, count := range jobFails {
		m.TopFailingJobs = append(m.TopFailingJobs, JobStat{Name: name, FailureCount: count})
	}
	sort.Slice(m.TopFailingJobs, func(i, j int) bool {
		return m.TopFailingJobs[i].FailureCount > m.TopFailingJobs[j].FailureCount
	})
	if len(m.TopFailingJobs) > 5 {
		m.TopFailingJobs = m.TopFailingJobs[:5]
	}

	// Job-level metrics
	m.TotalJobs = len(jobs)
	var jobDurations []float64
	for _, j := range jobs {
		if j.Conclusion == model.ConclusionSuccess {
			m.JobSuccessCount++
		}
		if j.Failed() {
			m.JobFailureCount++
		}
		jd := j.Duration().Seconds()
		if jd > 0 {
			jobDurations = append(jobDurations, jd)
		}
	}
	sort.Float64s(jobDurations)
	if len(jobDurations) > 0 {
		jdSum := 0.0
		for _, d := range jobDurations {
			jdSum += d
		}
		m.MeanJobDuration = jdSum / float64(len(jobDurations))
		m.MedianJobDuration = percentile(jobDurations, 50)
		m.P95JobDuration = percentile(jobDurations, 95)
	}

	return m
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

type Model struct {
	metrics   *Metrics
	windows   []TimeWindow
	windowIdx int
	viewport  viewport.Model
	width     int
	height    int
	loading   bool
	ready     bool
}

func New() Model {
	w := DefaultWindows
	return Model{
		windows:   w,
		windowIdx: 1, // default to 7d
		loading:   true,
	}
}

func (m *Model) SetWindows(windows []TimeWindow) {
	m.windows = windows
	// Reset to a sensible default: pick 7d if available, otherwise the last window
	m.windowIdx = 0
	for i, w := range windows {
		if w.Days == 7 {
			m.windowIdx = i
			break
		}
	}
}

func (m *Model) SetMetrics(metrics *Metrics) {
	m.metrics = metrics
	m.loading = false
	if m.ready {
		m.viewport.SetContent(m.render())
	}
}

func (m Model) Init() tea.Cmd { return nil }

type WindowChangedMsg struct {
	Window TimeWindow
}

func (m Model) Window() TimeWindow {
	if m.windowIdx >= 0 && m.windowIdx < len(m.windows) {
		return m.windows[m.windowIdx]
	}
	return DefaultWindows[1] // 7d fallback
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		var newIdx int = -1
		switch msg.String() {
		case "[":
			if m.windowIdx > 0 {
				newIdx = m.windowIdx - 1
			}
		case "]":
			if m.windowIdx < len(m.windows)-1 {
				newIdx = m.windowIdx + 1
			}
		}
		if newIdx >= 0 && newIdx != m.windowIdx {
			m.windowIdx = newIdx
			m.loading = true
			w := m.windows[newIdx]
			return m, func() tea.Msg {
				return WindowChangedMsg{Window: w}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-2)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 2
		}
		if m.metrics != nil {
			m.viewport.SetContent(m.render())
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) render() string {
	if m.metrics == nil {
		return "  No data"
	}
	met := m.metrics
	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	w := m.Window()

	var b strings.Builder

	// ── Overview ──────────────────────────────────────────────────────
	b.WriteString(bold.Render(fmt.Sprintf("  Overview (%s)", w.Label)) + "\n\n")

	totalLabel := bold.Render(fmt.Sprintf("%d", met.TotalRuns))
	if met.SampledRuns < met.TotalRuns {
		totalLabel += muted.Render(fmt.Sprintf("  (analyzed %d)", met.SampledRuns))
	}
	b.WriteString(fmt.Sprintf("  Total Runs: %s\n", totalLabel))
	b.WriteString(fmt.Sprintf("  Success:    %s (%s)\n",
		ui.StyleSuccess.Render(fmt.Sprintf("%d", met.SuccessCount)),
		fmt.Sprintf("%.1f%%", met.SuccessRate)))
	b.WriteString(fmt.Sprintf("  Failures:   %s (%s)\n",
		ui.StyleFailure.Render(fmt.Sprintf("%d", met.FailureCount)),
		fmt.Sprintf("%.1f%%", met.FailureRate)))
	b.WriteString(fmt.Sprintf("  Cancelled:  %s\n",
		ui.StyleWarning.Render(fmt.Sprintf("%d", met.CancelCount))))
	b.WriteString(fmt.Sprintf("  Retry Rate: %.1f%%\n\n", met.RetryRate))

	// ── Performance ──────────────────────────────────────────────────
	b.WriteString(bold.Render("  Performance") + "\n\n")

	b.WriteString(fmt.Sprintf("  Duration:    mean %s / median %s / p95 %s / p99 %s\n",
		formatSeconds(met.MeanDuration),
		formatSeconds(met.MedianDuration),
		formatSeconds(met.P95Duration),
		formatSeconds(met.P99Duration)))
	b.WriteString(fmt.Sprintf("  Queue Time:  mean %s / median %s / p95 %s\n\n",
		formatSeconds(met.MeanQueueTime),
		formatSeconds(met.MedianQueueTime),
		formatSeconds(met.P95QueueTime)))

	// ── Slowest Workflows ────────────────────────────────────────────
	if len(met.SlowestWorkflows) > 0 {
		b.WriteString(bold.Render("  Slowest Workflows") + "\n\n")

		for i, sw := range met.SlowestWorkflows {
			name := sw.Name
			if len(name) > 40 {
				name = name[:37] + "..."
			}
			b.WriteString(fmt.Sprintf("  %d. %-40s  median %s  p95 %s  %s\n",
				i+1,
				name,
				ui.StyleWarning.Render(fmt.Sprintf("%7s", formatSeconds(sw.MedianDuration))),
				ui.StyleFailure.Render(fmt.Sprintf("%7s", formatSeconds(sw.P95Duration))),
				muted.Render(fmt.Sprintf("(%d runs)", sw.RunCount))))
		}
		b.WriteString("\n")
	}

	// ── Runs by Event ────────────────────────────────────────────────
	if len(met.RunsByEvent) > 0 {
		b.WriteString(bold.Render("  Runs by Event") + "\n\n")

		eventEntries := sortMapByValue(met.RunsByEvent)
		for _, e := range eventEntries {
			b.WriteString(fmt.Sprintf("  %-20s %s\n",
				e.Key,
				muted.Render(fmt.Sprintf("%d", e.Value))))
		}
		b.WriteString("\n")
	}

	// ── Top Actors ───────────────────────────────────────────────────
	if len(met.RunsByActor) > 0 {
		b.WriteString(bold.Render("  Top Actors") + "\n\n")

		actorEntries := sortMapByValue(met.RunsByActor)
		limit := 10
		if len(actorEntries) < limit {
			limit = len(actorEntries)
		}
		for _, e := range actorEntries[:limit] {
			b.WriteString(fmt.Sprintf("  %-30s %s\n",
				e.Key,
				muted.Render(fmt.Sprintf("%d runs", e.Value))))
		}
		b.WriteString("\n")
	}

	// ── Top Branches ─────────────────────────────────────────────────
	if len(met.RunsByBranch) > 0 {
		b.WriteString(bold.Render("  Top Branches") + "\n\n")

		branchEntries := sortMapByValue(met.RunsByBranch)
		limit := 10
		if len(branchEntries) < limit {
			limit = len(branchEntries)
		}
		for _, e := range branchEntries[:limit] {
			b.WriteString(fmt.Sprintf("  %-30s %s\n",
				e.Key,
				muted.Render(fmt.Sprintf("%d runs", e.Value))))
		}
		b.WriteString("\n")
	}

	// ── Top Failing Workflows ────────────────────────────────────────
	if len(met.TopFailing) > 0 {
		b.WriteString(bold.Render("  Top Failing Workflows") + "\n\n")

		barMaxLen := 20

		// Find max failure rate for proportional bars and max count width for alignment
		maxRate := 0.0
		maxCountLen := 0
		for _, ws := range met.TopFailing {
			if ws.FailureRate > maxRate {
				maxRate = ws.FailureRate
			}
			countStr := fmt.Sprintf("%d/%d", ws.FailureCount, ws.TotalRuns)
			if len(countStr) > maxCountLen {
				maxCountLen = len(countStr)
			}
		}

		for i, ws := range met.TopFailing {
			name := ws.Name
			if len(name) > 40 {
				name = name[:37] + "..."
			}

			barLen := 1
			if maxRate > 0 {
				barLen = int(ws.FailureRate / maxRate * float64(barMaxLen))
				if barLen < 1 {
					barLen = 1
				}
			}
			bar := strings.Repeat("█", barLen) + strings.Repeat("░", barMaxLen-barLen)

			countStr := fmt.Sprintf("%*d/%-d", 0, ws.FailureCount, ws.TotalRuns)

			b.WriteString(fmt.Sprintf("  %d. %s  %s  %s %s\n",
				i+1,
				ui.StyleFailure.Render(fmt.Sprintf("%5.1f%%", ws.FailureRate)),
				ui.StyleFailure.Render(bar),
				muted.Render(fmt.Sprintf("%-*s", maxCountLen, countStr)),
				name))
		}
		b.WriteString("\n")
	}

	// ── Top Failing Jobs ─────────────────────────────────────────────
	if len(met.TopFailingJobs) > 0 {
		b.WriteString(bold.Render("  Top Failing Jobs") + "\n\n")

		for i, js := range met.TopFailingJobs {
			name := js.Name
			if len(name) > 40 {
				name = name[:37] + "..."
			}
			b.WriteString(fmt.Sprintf("  %d. %s  %s  %s\n",
				i+1,
				ui.StatusIcon("failure"),
				ui.StyleFailure.Render(fmt.Sprintf("%-3d failures", js.FailureCount)),
				name))
		}
		b.WriteString("\n")
	}

	// ── Job Performance ──────────────────────────────────────────────
	if met.TotalJobs > 0 {
		b.WriteString(bold.Render("  Job Performance") + "\n\n")

		b.WriteString(fmt.Sprintf("  Total Jobs: %s\n", bold.Render(fmt.Sprintf("%d", met.TotalJobs))))
		b.WriteString(fmt.Sprintf("  Succeeded:  %s\n",
			ui.StyleSuccess.Render(fmt.Sprintf("%d", met.JobSuccessCount))))
		b.WriteString(fmt.Sprintf("  Failed:     %s\n",
			ui.StyleFailure.Render(fmt.Sprintf("%d", met.JobFailureCount))))
		b.WriteString(fmt.Sprintf("  Duration:   mean %s / median %s / p95 %s\n",
			formatSeconds(met.MeanJobDuration),
			formatSeconds(met.MedianJobDuration),
			formatSeconds(met.P95JobDuration)))
	}

	return b.String()
}

func formatSeconds(s float64) string {
	if s < 60 {
		return fmt.Sprintf("%.0fs", s)
	}
	if s < 3600 {
		return fmt.Sprintf("%.0fm", s/60)
	}
	return fmt.Sprintf("%.1fh", s/3600)
}

func (m Model) View() string {
	if m.loading {
		return "\n  Loading metrics..."
	}

	// Build tabs line showing all windows with current highlighted
	muted := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	active := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F9FAFB"))

	var parts []string
	for i, w := range m.windows {
		if i == m.windowIdx {
			parts = append(parts, active.Render(w.Label))
		} else {
			parts = append(parts, muted.Render(w.Label))
		}
	}
	tabs := "  " + strings.Join(parts, "  ") + "    " + muted.Render("press [ or ] to switch")

	if m.ready {
		return tabs + "\n" + m.viewport.View()
	}
	return tabs + "\n  Initializing..."
}
