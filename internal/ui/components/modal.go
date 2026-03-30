package components

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/abhishek-rana/lazydk/internal/ui/theme"
)

// ModalAction is the callback type for modal confirmation.
type ModalAction func() tea.Cmd

// Modal is a confirmation dialog overlay.
type Modal struct {
	Title    string
	Message  string
	OnOK     ModalAction
	selected int // 0 = OK, 1 = Cancel
	visible  bool
	width    int
	height   int
}

// NewModal creates a new modal (initially hidden).
func NewModal() Modal {
	return Modal{}
}

// Show displays the modal with the given title, message, and OK callback.
func (m *Modal) Show(title, message string, onOK ModalAction) {
	m.Title = title
	m.Message = message
	m.OnOK = onOK
	m.selected = 1 // default to Cancel for safety
	m.visible = true
}

// Hide dismisses the modal.
func (m *Modal) Hide() {
	m.visible = false
	m.OnOK = nil
}

// Visible returns whether the modal is shown.
func (m Modal) Visible() bool {
	return m.visible
}

// SetSize sets the terminal size for centering.
func (m *Modal) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles input when the modal is visible.
func (m *Modal) Update(msg tea.Msg) tea.Cmd {
	if !m.visible {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "left", "h":
			m.selected = 0
		case "right", "l":
			m.selected = 1
		case "tab":
			m.selected = (m.selected + 1) % 2
		case "enter":
			if m.selected == 0 && m.OnOK != nil {
				cmd := m.OnOK()
				m.Hide()
				return cmd
			}
			m.Hide()
			return nil
		case "esc", "n", "q":
			m.Hide()
			return nil
		case "y":
			if m.OnOK != nil {
				cmd := m.OnOK()
				m.Hide()
				return cmd
			}
			m.Hide()
			return nil
		}
	}

	return nil
}

// View renders the modal centered on screen.
func (m Modal) View() string {
	if !m.visible {
		return ""
	}

	title := theme.ModalTitleStyle.Render(m.Title)
	msg := m.Message

	okLabel := "  OK  "
	cancelLabel := " Cancel "
	if m.selected == 0 {
		okLabel = theme.ActiveTabStyle.Render(okLabel)
		cancelLabel = theme.InactiveTabStyle.Render(cancelLabel)
	} else {
		okLabel = theme.InactiveTabStyle.Render(okLabel)
		cancelLabel = theme.ActiveTabStyle.Render(cancelLabel)
	}

	buttons := fmt.Sprintf("%s    %s", okLabel, cancelLabel)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		msg,
		"",
		buttons,
	)

	box := theme.ModalStyle.Render(content)

	// Center the modal on screen.
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
