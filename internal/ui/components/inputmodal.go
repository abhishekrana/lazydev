package components

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/abhishek-rana/lazydk/internal/ui/theme"
)

// InputModalAction is called with the user's input when they confirm.
type InputModalAction func(value string) tea.Cmd

// InputModal is a modal dialog with a text input field.
type InputModal struct {
	Title       string
	Placeholder string
	OnSubmit    InputModalAction
	input       textinput.Model
	visible     bool
	width       int
	height      int
}

// NewInputModal creates a new input modal (initially hidden).
func NewInputModal() InputModal {
	ti := textinput.New()
	ti.CharLimit = 64
	return InputModal{input: ti}
}

// Show displays the input modal.
func (m *InputModal) Show(title, placeholder string, onSubmit InputModalAction) {
	m.Title = title
	m.Placeholder = placeholder
	m.OnSubmit = onSubmit
	m.input.SetValue("")
	m.input.Placeholder = placeholder
	m.input.Focus()
	m.visible = true
}

// Hide dismisses the input modal.
func (m *InputModal) Hide() {
	m.visible = false
	m.OnSubmit = nil
	m.input.Blur()
}

// Visible returns whether the modal is shown.
func (m InputModal) Visible() bool {
	return m.visible
}

// SetSize sets terminal size for centering.
func (m *InputModal) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles input.
func (m *InputModal) Update(msg tea.Msg) tea.Cmd {
	if !m.visible {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter": //nolint:goconst // idiomatic key name
			if m.OnSubmit != nil {
				cmd := m.OnSubmit(m.input.Value())
				m.Hide()
				return cmd
			}
			m.Hide()
			return nil
		case "esc": //nolint:goconst // idiomatic key name
			m.Hide()
			return nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

// View renders the input modal centered on screen.
func (m InputModal) View() string {
	if !m.visible {
		return ""
	}

	title := theme.ModalTitleStyle.Render(m.Title)
	inputView := m.input.View()
	hint := theme.LogTimestampStyle.Render("Enter to confirm, Esc to cancel")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		inputView,
		"",
		hint,
	)

	box := theme.ModalStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
