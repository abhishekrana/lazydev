package components

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/abhishek-rana/lazydk/internal/ui/theme"
)

// CmdExecutor is called with the parsed command and arguments.
type CmdExecutor func(cmd string, args []string) tea.Cmd

// CmdPalette is a command input bar shown at the bottom of the screen.
type CmdPalette struct {
	input   textinput.Model
	visible bool
	width   int
	onExec  CmdExecutor
}

// NewCmdPalette creates a new command palette.
func NewCmdPalette() CmdPalette {
	ti := textinput.New()
	ti.Placeholder = "command..."
	ti.CharLimit = 256
	return CmdPalette{input: ti}
}

// Show opens the command palette.
func (c *CmdPalette) Show(onExec CmdExecutor) {
	c.onExec = onExec
	c.input.SetValue("")
	c.input.Focus()
	c.visible = true
}

// Hide closes the command palette.
func (c *CmdPalette) Hide() {
	c.visible = false
	c.onExec = nil
	c.input.Blur()
}

// Visible returns whether the palette is shown.
func (c CmdPalette) Visible() bool {
	return c.visible
}

// SetWidth sets the palette width.
func (c *CmdPalette) SetWidth(width int) {
	c.width = width
}

// Update handles input.
func (c *CmdPalette) Update(msg tea.Msg) tea.Cmd {
	if !c.visible {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter": //nolint:goconst // idiomatic key name
			value := strings.TrimSpace(c.input.Value())
			c.Hide()
			if value == "" || c.onExec == nil {
				return nil
			}
			parts := strings.Fields(value)
			return c.onExec(parts[0], parts[1:])
		case "esc": //nolint:goconst // idiomatic key name
			c.Hide()
			return nil
		}
	}

	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return cmd
}

// View renders the command palette bar.
func (c CmdPalette) View() string {
	if !c.visible {
		return ""
	}

	return theme.StatusBarStyle.Width(c.width).Render(":" + c.input.View())
}
