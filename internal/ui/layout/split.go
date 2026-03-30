package layout

import (
	"charm.land/lipgloss/v2"
)

// HorizontalSplit renders two views side by side.
func HorizontalSplit(left, right string, leftWidth, totalWidth, height int) string {
	rightWidth := totalWidth - leftWidth
	if rightWidth < 0 {
		rightWidth = 0
	}

	l := lipgloss.NewStyle().
		Width(leftWidth).
		Height(height).
		Render(left)

	r := lipgloss.NewStyle().
		Width(rightWidth).
		Height(height).
		Render(right)

	return lipgloss.JoinHorizontal(lipgloss.Top, l, r)
}
