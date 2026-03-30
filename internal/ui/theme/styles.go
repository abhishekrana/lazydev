package theme

import (
	"charm.land/glamour/v2/ansi"
	"charm.land/lipgloss/v2"
)

// ptr returns a pointer to the given value.
func ptr[T any](v T) *T { return &v }

// SolarizedMarkdownStyle returns a glamour style config using the solarized light palette.
func SolarizedMarkdownStyle() ansi.StyleConfig {
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
				Color:       ptr("#586E75"), // SolBase01 — body text
			},
			Margin: ptr(uint(0)),
		},
		BlockQuote: ansi.StyleBlock{
			Indent:      ptr(uint(1)),
			IndentToken: ptr("│ "),
			StylePrimitive: ansi.StylePrimitive{
				Color:  ptr("#93A1A1"), // SolBase1
				Italic: ptr(true),
			},
		},
		Paragraph: ansi.StyleBlock{},
		List: ansi.StyleList{
			LevelIndent: 2,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       ptr("#268BD2"), // SolBlue
				Bold:        ptr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "# ",
				Color:  ptr("#268BD2"), // SolBlue
				Bold:   ptr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "## ",
				Color:  ptr("#6C71C4"), // SolViolet
				Bold:   ptr(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
				Color:  ptr("#2AA198"), // SolCyan
				Bold:   ptr(true),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "#### ",
				Color:  ptr("#859900"), // SolGreen
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "##### ",
				Color:  ptr("#B58900"), // SolYellow
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
				Color:  ptr("#839496"), // SolBase0
			},
		},
		Text: ansi.StylePrimitive{},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: ptr(true),
		},
		Emph: ansi.StylePrimitive{
			Italic: ptr(true),
			Color:  ptr("#657B83"), // SolBase00
		},
		Strong: ansi.StylePrimitive{
			Bold:  ptr(true),
			Color: ptr("#073642"), // SolBase02
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  ptr("#93A1A1"), // SolBase1
			Format: "\n────────\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
		},
		Task: ansi.StyleTask{
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color:     ptr("#268BD2"), // SolBlue
			Underline: ptr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: ptr("#2AA198"), // SolCyan
			Bold:  ptr(true),
		},
		Image: ansi.StylePrimitive{
			Color:     ptr("#268BD2"), // SolBlue — render full URL, clickable
			Underline: ptr(true),
			Format:    "{{.text}}",
		},
		ImageText: ansi.StylePrimitive{
			Color:  ptr("#839496"), // SolBase0
			Format: "[image] ",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           ptr("#CB4B16"), // SolOrange
				BackgroundColor: ptr("#EEE8D5"), // SolBase2
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: ptr("#586E75"), // SolBase01
				},
				Margin: ptr(uint(2)),
			},
			Chroma: &ansi.Chroma{
				Text:                ansi.StylePrimitive{Color: ptr("#586E75")},
				Error:               ansi.StylePrimitive{Color: ptr("#DC322F")},
				Comment:             ansi.StylePrimitive{Color: ptr("#93A1A1")},
				CommentPreproc:      ansi.StylePrimitive{Color: ptr("#CB4B16")},
				Keyword:             ansi.StylePrimitive{Color: ptr("#268BD2")},
				KeywordReserved:     ansi.StylePrimitive{Color: ptr("#D33682")},
				KeywordNamespace:    ansi.StylePrimitive{Color: ptr("#DC322F")},
				KeywordType:         ansi.StylePrimitive{Color: ptr("#6C71C4")},
				Operator:            ansi.StylePrimitive{Color: ptr("#859900")},
				Punctuation:         ansi.StylePrimitive{Color: ptr("#586E75")},
				Name:                ansi.StylePrimitive{},
				NameBuiltin:         ansi.StylePrimitive{Color: ptr("#268BD2")},
				NameTag:             ansi.StylePrimitive{Color: ptr("#268BD2")},
				NameAttribute:       ansi.StylePrimitive{Color: ptr("#2AA198")},
				NameClass:           ansi.StylePrimitive{Color: ptr("#B58900"), Bold: ptr(true)},
				NameConstant:        ansi.StylePrimitive{Color: ptr("#2AA198")},
				NameDecorator:       ansi.StylePrimitive{Color: ptr("#CB4B16")},
				NameFunction:        ansi.StylePrimitive{Color: ptr("#268BD2")},
				LiteralNumber:       ansi.StylePrimitive{Color: ptr("#D33682")},
				LiteralString:       ansi.StylePrimitive{Color: ptr("#2AA198")},
				LiteralStringEscape: ansi.StylePrimitive{Color: ptr("#CB4B16")},
				GenericDeleted:      ansi.StylePrimitive{Color: ptr("#DC322F")},
				GenericEmph:         ansi.StylePrimitive{Italic: ptr(true)},
				GenericInserted:     ansi.StylePrimitive{Color: ptr("#859900")},
				GenericStrong:       ansi.StylePrimitive{Bold: ptr(true)},
				GenericSubheading:   ansi.StylePrimitive{Color: ptr("#93A1A1")},
			},
		},
	}
}

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
