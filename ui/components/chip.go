package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// NSChip renders a small colored square + namespace name, like the design's
// namespace dot. The square is "▆" (U+2586). Color comes from theme.NSColorFor.
func NSChip(ns string) string {
	if ns == "" {
		// Faint placeholder for missing/cluster-scoped resources.
		return lipgloss.NewStyle().Foreground(theme.ColorFaint).Render("▆ —")
	}
	color := theme.NSColorFor(ns)
	square := lipgloss.NewStyle().Foreground(color).Render("▆")
	name := lipgloss.NewStyle().Foreground(theme.ColorFG2).Render(ns)
	return square + " " + name
}

// NSChipBold renders a more prominent variant of NSChip — both square and
// name in the namespace's color, with the name bolded. Used in the top bar
// where the namespace is the visual anchor of the screen.
func NSChipBold(ns string) string {
	if ns == "" {
		return lipgloss.NewStyle().Foreground(theme.ColorFaint).Bold(true).Render("▆ —")
	}
	color := theme.NSColorFor(ns)
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render("▆ " + ns)
}

// StatusPill renders a colored dot ● + the phase name, both colored from
// theme.StatusStyleFor (Dot for the bullet, Text for the name).
func StatusPill(phase string) string {
	s := theme.StatusStyleFor(phase)
	dot := lipgloss.NewStyle().Foreground(s.Dot).Render("●")
	name := lipgloss.NewStyle().Foreground(s.Text).Render(phase)
	return dot + " " + name
}

// StatusDot returns just the colored ● glyph (no trailing name) for compact rows.
func StatusDot(phase string) string {
	s := theme.StatusStyleFor(phase)
	return lipgloss.NewStyle().Foreground(s.Dot).Render("●")
}
