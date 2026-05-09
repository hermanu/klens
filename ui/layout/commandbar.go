package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// CommandBar renders the bottom command bar:
//
//	›  /  <filter input>     [↵ describe] [l logs] [s shell] ...
//
// `inputView` is the rendered textinput.View() string from bubbles/textinput
// (so the caller owns the cursor blink). `hints` are right-aligned key chips.
func CommandBar(width int, inputView string, hints []KeyHint) string {
	if width < 1 {
		width = 1
	}

	prompt := lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("›") + " " +
		theme.Faint.Render("/") + " "
	left := prompt + inputView

	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		// Compose chip + label so each key keeps a small box around it. The
		// JSX uses one rounded box per `[k] label`; KeyChip is the closest
		// equivalent and matches existing StatusBar styling.
		key := theme.KeyChip.Render(h.Key)
		label := theme.Dim.Render(" " + h.Label)
		parts = append(parts, key+label)
	}
	right := strings.Join(parts, "  ")

	inner := width - 2 // padding 0,1
	if inner < 1 {
		inner = 1
	}
	gap := inner - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return theme.Panel.Width(width).Padding(0, 1).Render(line)
}
