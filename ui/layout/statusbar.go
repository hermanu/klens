package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// KeyHint is a key + label pair shown in the status bar.
type KeyHint struct {
	Key   string
	Label string
}

// StatusBar renders the bottom key-hint bar.
// Left: key chips with labels. Right: status text (e.g. "pods · 2.1k/s").
// Design: matches modern.jsx StatusBar component.
func StatusBar(width int, hints []KeyHint, right string) string {
	var parts []string
	for _, h := range hints {
		key := theme.KeyChip.Render(h.Key)
		label := theme.Dim.Render(" " + h.Label)
		parts = append(parts, key+label)
	}
	left := strings.Join(parts, "  ")

	rightStyled := theme.Faint.Render(right)
	gap := width - lipgloss.Width(left) - lipgloss.Width(rightStyled)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + rightStyled
	return theme.Panel.Width(width).Render(line)
}
