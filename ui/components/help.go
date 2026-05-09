package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// KeySpec is one row in the help overlay: a key, what it does, and whether
// it's wired today. `Soon` keys render dimmed.
type KeySpec struct {
	Key   string
	Label string
	Soon  bool
}

// Help renders a centered modal listing the active view's keymap.
// `width` and `height` are the FULL terminal dimensions; the modal sizes
// itself and Place takes care of centering.
//
// `help.go` lives in `components` (not `layout`) because the views package
// already depends on both, while `layout` itself depends on `components` —
// putting the overlay here is the only direction that avoids an import cycle.
func Help(width, height int, viewTitle string, specs []KeySpec) string {
	// Modal width: max(40, longest "key  label" line + 6). Cap at 64.
	innerW := 40
	for _, s := range specs {
		w := lipgloss.Width(s.Key) + lipgloss.Width(s.Label) + 4
		if w > innerW {
			innerW = w
		}
	}
	if innerW > 64 {
		innerW = 64
	}
	_ = innerW // reserved for future width-aware rendering; modal currently sizes itself via padding.

	title := lipgloss.NewStyle().
		Foreground(theme.ColorAccent).
		Bold(true).
		Render(viewTitle + " — keys")

	var sb strings.Builder
	sb.WriteString(title + "\n\n")
	for _, s := range specs {
		keyStyle := lipgloss.NewStyle().Foreground(theme.ColorFG).Bold(true)
		labelStyle := lipgloss.NewStyle().Foreground(theme.ColorMuted)
		soonTag := ""
		if s.Soon {
			keyStyle = keyStyle.Foreground(theme.ColorMuted2)
			labelStyle = labelStyle.Foreground(theme.ColorMuted2)
			soonTag = lipgloss.NewStyle().Foreground(theme.ColorMuted2).Italic(true).Render("  soon")
		}
		key := keyStyle.Width(8).Render(s.Key)
		label := labelStyle.Render(s.Label)
		sb.WriteString(key + label + soonTag + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(theme.Faint.Render("? or esc to close"))

	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorAccent).
		Padding(1, 2).
		Render(sb.String())

	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		body,
	)
}
