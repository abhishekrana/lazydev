package theme

import (
	"charm.land/lipgloss/v2"
)

// Theme colors.
var (
	ColorPrimary    = lipgloss.Color("#7D56F4")
	ColorSecondary  = lipgloss.Color("#6C71C4")
	ColorSuccess    = lipgloss.Color("#2AA198")
	ColorWarning    = lipgloss.Color("#B58900")
	ColorError      = lipgloss.Color("#DC322F")
	ColorFatal      = lipgloss.Color("#D33682")
	ColorMuted      = lipgloss.Color("#586E75")
	ColorBorder     = lipgloss.Color("#444444")
	ColorActiveBg   = lipgloss.Color("#1A1A2E")
	ColorInactiveBg = lipgloss.Color("#0F0F1A")
	ColorText       = lipgloss.Color("#FAFAFA")
	ColorDimText    = lipgloss.Color("#888888")
	ColorHighlight  = lipgloss.Color("#E6DB74")
)

// Styles used across the UI.
var (
	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText).
			Background(ColorPrimary).
			Padding(0, 2)

	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(ColorDimText).
				Padding(0, 2)

	TabBarStyle = lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder)

	SidebarStyle = lipgloss.NewStyle().
			BorderRight(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder)

	SidebarItemStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				PaddingLeft(2)

	SidebarSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				Background(ColorPrimary).
				Bold(true).
				PaddingLeft(2)

	SidebarGroupStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Bold(true).
				PaddingLeft(1)

	LogViewStyle      = lipgloss.NewStyle()
	LogTimestampStyle = lipgloss.NewStyle().Foreground(ColorDimText)
	LogDebugStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#268BD2"))
	LogInfoStyle      = lipgloss.NewStyle().Foreground(ColorSuccess)
	LogWarnStyle      = lipgloss.NewStyle().Foreground(ColorWarning)
	LogErrorStyle     = lipgloss.NewStyle().Foreground(ColorError)
	LogFatalStyle     = lipgloss.NewStyle().Foreground(ColorFatal).Bold(true)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorDimText).
			Background(lipgloss.Color("#1A1A2E")).
			Padding(0, 1)

	StatusBarKeyStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true)

	StateRunningStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	StateStoppedStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	StateErrorStyle   = lipgloss.NewStyle().Foreground(ColorError)
	StatePendingStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2).
			Width(50)

	ModalTitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	SearchStyle = lipgloss.NewStyle().
			Foreground(ColorHighlight)
)

// StateIcon returns a colored status icon.
func StateIcon(state string) string {
	switch state {
	case "running", "Running":
		return StateRunningStyle.Render("●")
	case "exited", "Stopped", "stopped":
		return StateStoppedStyle.Render("○")
	case "error", "Error", "CrashLoopBackOff":
		return StateErrorStyle.Render("✗")
	case "pending", "Pending", "ContainerCreating":
		return StatePendingStyle.Render("◌")
	case "restarting", "Restarting":
		return StateRunningStyle.Render("↻")
	default:
		return StateStoppedStyle.Render("?")
	}
}
