package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/altinukshini/gha-tui/internal/ui"
)

func RenderHeader(repo string, rateRemaining, rateLimit int, width int) string {
	left := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color("#F9FAFB")).
		Render(fmt.Sprintf(" gha-tui | %s", repo))

	rate := ""
	if rateLimit > 0 {
		color := ui.ColorSuccess
		if rateRemaining < 100 {
			color = ui.ColorFailure
		} else if rateRemaining < 500 {
			color = ui.ColorWarning
		}
		rate = lipgloss.NewStyle().Foreground(color).
			Render(fmt.Sprintf("API: %d/%d ", rateRemaining, rateLimit))
	}

	gap := width - lipgloss.Width(left) - lipgloss.Width(rate)
	if gap < 0 {
		gap = 0
	}
	padding := lipgloss.NewStyle().Width(gap).Render("")

	return lipgloss.NewStyle().
		Background(lipgloss.Color("#1F2937")).
		Width(width).
		Render(left + padding + rate)
}
