package theme

import (
	"charm.land/lipgloss/v2"
)

// Solarized Light palette.
var (
	// Base colors (light background).
	SolBase03  = lipgloss.Color("#002B36") // darkest
	SolBase02  = lipgloss.Color("#073642")
	SolBase01  = lipgloss.Color("#586E75") // emphasized content (dark)
	SolBase00  = lipgloss.Color("#657B83") // body text
	SolBase0   = lipgloss.Color("#839496") // comments
	SolBase1   = lipgloss.Color("#93A1A1") // secondary content
	SolBase2   = lipgloss.Color("#EEE8D5") // background highlights
	SolBase3   = lipgloss.Color("#FDF6E3") // background
	SolYellow  = lipgloss.Color("#B58900")
	SolOrange  = lipgloss.Color("#CB4B16")
	SolRed     = lipgloss.Color("#DC322F")
	SolMagenta = lipgloss.Color("#D33682")
	SolViolet  = lipgloss.Color("#6C71C4")
	SolBlue    = lipgloss.Color("#268BD2")
	SolCyan    = lipgloss.Color("#2AA198")
	SolGreen   = lipgloss.Color("#859900")
)

// Semantic color aliases.
var (
	ColorPrimary    = SolBlue
	ColorSecondary  = SolViolet
	ColorSuccess    = SolCyan
	ColorWarning    = SolYellow
	ColorError      = SolRed
	ColorFatal      = SolMagenta
	ColorMuted      = SolBase1
	ColorBorder     = SolBase1
	ColorActiveBg   = SolBase2
	ColorInactiveBg = SolBase3
	ColorText       = SolBase01
	ColorDimText    = SolBase0
	ColorHighlight  = SolOrange
)

// Styles used across the UI.
var (
	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(SolBase3).
			Background(SolBlue).
			Padding(0, 2)

	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(SolBase00).
				Padding(0, 2)

	// InactiveHeaderStyle is used for pane headers when the pane is not focused.
	InactiveHeaderStyle = lipgloss.NewStyle().
				Foreground(SolBase00).
				Background(SolBase2).
				Padding(0, 2)

	TabBarStyle = lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(SolBase1)

	SidebarStyle = lipgloss.NewStyle().
			BorderRight(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(SolBase1)

	SidebarItemStyle = lipgloss.NewStyle().
				Foreground(SolBase00).
				PaddingLeft(2)

	SidebarSelectedStyle = lipgloss.NewStyle().
				Foreground(SolBase3).
				Background(SolBlue).
				Bold(true).
				PaddingLeft(2)

	// LogCursorStyle highlights the cursor line in the log pane (no padding).
	LogCursorStyle = lipgloss.NewStyle().
			Foreground(SolBase3).
			Background(SolBlue)

	SidebarGroupStyle = lipgloss.NewStyle().
				Foreground(SolViolet).
				Bold(true).
				PaddingLeft(1)

	LogViewStyle      = lipgloss.NewStyle()
	LogTimestampStyle = lipgloss.NewStyle().Foreground(SolBase1)
	LogDebugStyle     = lipgloss.NewStyle().Foreground(SolCyan)
	LogInfoStyle      = lipgloss.NewStyle().Foreground(SolGreen)
	LogWarnStyle      = lipgloss.NewStyle().Foreground(SolYellow)
	LogErrorStyle     = lipgloss.NewStyle().Foreground(SolRed)
	LogFatalStyle     = lipgloss.NewStyle().Foreground(SolMagenta).Bold(true)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(SolBase3).
			Background(SolBase01).
			Padding(0, 1)

	StatusBarKeyStyle = lipgloss.NewStyle().
				Foreground(SolCyan).
				Bold(true)

	StateRunningStyle = lipgloss.NewStyle().Foreground(SolGreen)
	StateStoppedStyle = lipgloss.NewStyle().Foreground(SolBase1)
	StateErrorStyle   = lipgloss.NewStyle().Foreground(SolRed)
	StatePendingStyle = lipgloss.NewStyle().Foreground(SolYellow)

	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(SolBlue).
			Padding(1, 2).
			Width(50)

	ModalTitleStyle = lipgloss.NewStyle().
			Foreground(SolBlue).
			Bold(true)

	SearchStyle = lipgloss.NewStyle().
			Foreground(SolOrange).
			Bold(true)

	ScrollbarThumbStyle = lipgloss.NewStyle().Foreground(SolBase01)
	ScrollbarTrackStyle = lipgloss.NewStyle().Foreground(SolBase2)
)

// StateIcon returns a colored status icon for a ContainerState enum.
func StateIcon(state int) string {
	switch state {
	case 1: // StateRunning
		return StateRunningStyle.Render("●")
	case 2: // StateStopped
		return StateStoppedStyle.Render("○")
	case 3: // StateError
		return StateErrorStyle.Render("✗")
	case 4: // StatePending
		return StatePendingStyle.Render("◌")
	case 5: // StateRestarting
		return StateRunningStyle.Render("↻")
	default: // StateUnknown
		return StatePendingStyle.Render("◌")
	}
}
