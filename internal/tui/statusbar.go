package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/altinukshini/gha-tui/internal/ui"
)

func RenderStatusBar(status, hints string, width int) string {
	left := lipgloss.NewStyle().Foreground(ui.ColorMuted).Render("  " + status)

	help := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(hints + " ")

	gap := width - lipgloss.Width(left) - lipgloss.Width(help)
	if gap < 0 {
		gap = 0
	}
	padding := lipgloss.NewStyle().Width(gap).Render("")

	return lipgloss.NewStyle().
		Background(lipgloss.Color("#111827")).
		Width(width).
		Render(left + padding + help)
}
