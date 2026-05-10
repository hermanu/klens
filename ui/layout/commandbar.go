package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// CommandBar renders the bottom command bar as a single row:
//
//	›  /  <filter input>     [↵ describe] [l logs] [s shell] ...
//
// `inputView` is the rendered textinput.View() string from bubbles/textinput
// (so the caller owns the cursor blink). `hints` are right-aligned key chips.
//
// At narrow widths the right side drops hints one-by-one (lowest priority
// first — the right-most chip) until the prompt + remaining hints fit one
// row; a trailing `…` signals when hints were dropped. We never wrap to a
// second row — the bar must stay glued to the bottom of the screen.
func CommandBar(width int, inputView string, hints []KeyHint) string {
	if width < 1 {
		width = 1
	}

	prompt := lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("›") + " " +
		theme.Faint.Render("/") + " "
	left := prompt + inputView

	chips := make([]string, 0, len(hints))
	for _, h := range hints {
		key := theme.KeyChip.Render(h.Key)
		label := theme.Dim.Render(" " + h.Label)
		chips = append(chips, key+label)
	}

	inner := width - 2 // panel padding 0,1
	if inner < 1 {
		inner = 1
	}

	// Drop chips from the right until the prompt + remaining chips + a 1-cell
	// gap fit. If we had to drop any, append a faint `…` so the user knows
	// some hints exist beyond the visible set (open `?` for the full keymap).
	leftW := lipgloss.Width(left)
	const minGap = 2
	dropped := false
	right := strings.Join(chips, "  ")
	for len(chips) > 0 && lipgloss.Width(right)+leftW+minGap > inner {
		chips = chips[:len(chips)-1]
		dropped = true
		right = strings.Join(chips, "  ")
	}
	if dropped && lipgloss.Width(right)+leftW+minGap+2 <= inner {
		right = right + "  " + theme.Faint.Render("…")
	}

	gap := inner - leftW - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return theme.Panel.Width(width).Padding(0, 1).Render(line)
}
