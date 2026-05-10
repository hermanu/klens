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

// HelpBody renders the bordered help modal as an opaque block. The shell
// then overlays it over the live frame so the user keeps context while
// reading the keymap. Use `Help` for the legacy place-on-blank-canvas form.
//
// `help.go` lives in `components` (not `layout`) because the views package
// already depends on both, while `layout` itself depends on `components` —
// putting the overlay here is the only direction that avoids an import cycle.
func HelpBody(viewTitle string, specs []KeySpec) string {
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

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorAccent).
		Padding(1, 2).
		Render(sb.String())
}

// Help renders the help modal centered on a blank canvas (legacy form,
// kept for the existing test). Prefer `HelpBody` + Overlay for live use.
func Help(width, height int, viewTitle string, specs []KeySpec) string {
	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		HelpBody(viewTitle, specs),
	)
}
