// Package layout — NavStrip horizontal resource selector.
//
// NavStrip renders the 8 mnemonic resource entries (pods / deployments /
// services / nodes / configmaps / secrets / namespaces / pvcs) inline as
// a single body row. Replaces the previous vertical NavRail so the table
// reclaims those 22 cells of horizontal space. The bordered envelope is
// applied by components.Panel at the call site.
package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// NavItem is one entry in the horizontal resource selector. Was named for
// the previous vertical rail; the name stays since the data shape is the
// same and migrating call sites would be churn for no benefit.
type NavItem struct {
	Mnemonic string // "1".."8"
	Label    string // "pods", "deployments", …
	Active   bool
	// Count is preserved for future per-item annotations (e.g. drift
	// indicators) but the strip renderer does not display it today — the
	// table panel's [N] title is the source of truth for resource counts.
	Count int
}

// NavStrip renders the horizontal resource selector body. width is the
// INNER panel content width. The active item carries a ▌ + accent label;
// inactive items render in muted color. When the strip won't fit at the
// given width, items are dropped from the right (the active item always
// stays — caller is expected to use the strip at widths >= ~80 cells).
func NavStrip(width int, items []NavItem) string {
	if width < 1 {
		width = 1
	}
	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)
	muted := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	mnemonicMuted := lipgloss.NewStyle().Foreground(theme.ColorMuted2)
	dot := muted.Render(" · ")
	cursor := accent.Render("▌ ")

	rendered := make([]string, len(items))
	for i, it := range items {
		label := muted.Render(it.Label)
		mnemonic := mnemonicMuted.Render(it.Mnemonic)
		if it.Active {
			label = accent.Render(it.Label)
			mnemonic = accent.Render(it.Mnemonic)
		}
		rendered[i] = mnemonic + " " + label
	}

	// Find the active index so we can prioritise keeping it visible if the
	// strip overflows. Falls back to 0 when nothing is active.
	activeIdx := 0
	for i, it := range items {
		if it.Active {
			activeIdx = i
			break
		}
	}

	// Build the candidate row with the ▌ cursor prepended to the active
	// item. Trim items from the right until the row fits. If even the
	// shortest possible row exceeds width, trim from the left as a last
	// resort so the active item never disappears.
	row := buildStripRow(rendered, activeIdx, cursor, dot)
	for lipgloss.Width(row) > width && len(rendered) > activeIdx+1 {
		rendered = rendered[:len(rendered)-1]
		row = buildStripRow(rendered, activeIdx, cursor, dot)
	}
	for lipgloss.Width(row) > width && activeIdx > 0 {
		rendered = rendered[1:]
		activeIdx--
		row = buildStripRow(rendered, activeIdx, cursor, dot)
	}
	if w := lipgloss.Width(row); w < width {
		row += strings.Repeat(" ", width-w)
	}
	return row
}

// buildStripRow joins `items` with the · separator and prepends the ▌
// cursor to the entry at activeIdx so it visually pops without disturbing
// the inter-item spacing.
func buildStripRow(items []string, activeIdx int, cursor, sep string) string {
	parts := make([]string, len(items))
	for i, it := range items {
		if i == activeIdx {
			parts[i] = cursor + it
		} else {
			parts[i] = "  " + it
		}
	}
	return strings.Join(parts, sep)
}
