package ui

import "github.com/charmbracelet/lipgloss"

var (
	ColorPrimary   = lipgloss.Color("#7C3AED")
	ColorSuccess   = lipgloss.Color("#10B981")
	ColorFailure   = lipgloss.Color("#EF4444")
	ColorWarning   = lipgloss.Color("#F59E0B")
	ColorInfo      = lipgloss.Color("#3B82F6")
	ColorMuted     = lipgloss.Color("#6B7280")
	ColorBorder    = lipgloss.Color("#374151")
	ColorHighlight = lipgloss.Color("#1F2937")

	StylePane = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder)

	StylePaneFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary)

	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F9FAFB")).
			Background(ColorPrimary).
			Padding(0, 1)

	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleFailure = lipgloss.NewStyle().Foreground(ColorFailure)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning)
	StyleInfo    = lipgloss.NewStyle().Foreground(ColorInfo)
	StyleMuted   = lipgloss.NewStyle().Foreground(ColorMuted)

	StyleMatch = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FCD34D")).
			Background(lipgloss.Color("#78350F"))
)

func ConclusionStyle(conclusion string) lipgloss.Style {
	switch conclusion {
	case "success":
		return StyleSuccess
	case "failure":
		return StyleFailure
	case "cancelled":
		return StyleWarning
	case "skipped":
		return StyleMuted
	default:
		return StyleInfo
	}
}

func StatusIcon(conclusion string) string {
	switch conclusion {
	case "success":
		return StyleSuccess.Render("V")
	case "failure":
		return StyleFailure.Render("X")
	case "cancelled":
		return StyleWarning.Render("!")
	case "skipped":
		return StyleMuted.Render("-")
	case "in_progress":
		return StyleInfo.Render("*")
	case "queued", "waiting", "pending", "requested":
		return StyleMuted.Render("o")
	default:
		return StyleMuted.Render("?")
	}
}
