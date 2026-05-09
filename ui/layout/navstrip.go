package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// NavStrip renders the horizontal resource navigation bar that sits directly
// under the top bar. Each item is a compact `MNEMONIC label COUNT` block; the
// active item is highlighted with an accent `▌` cursor and bold accent text.
//
//	▌1 pods 4/23   2 deployments 14   3 services 12   4 secrets 8   ...
//
// The active item's count is shown as `V/T` when filtered (V in accent), or
// just `T` when unfiltered. Inactive items always show their total. Anchoring
// the count on the active item keeps it grep-able at a stable position
// without reserving a fixed column on the top bar.
//
// At narrow widths the strip drops labels and shows mnemonics only so the
// keyboard map stays visible even on a 60-col terminal.
func NavStrip(width int, cfg NavStripConfig) string {
	if width < 1 {
		width = 1
	}

	itemsFull := renderItems(cfg, true)
	full := strings.Join(itemsFull, "   ")
	if lipgloss.Width(full) <= width-2 {
		return wrapStrip(width, full)
	}

	// Narrow fallback: mnemonics-only.
	itemsCompact := renderItems(cfg, false)
	compact := strings.Join(itemsCompact, "  ")
	return wrapStrip(width, compact)
}

func wrapStrip(width int, line string) string {
	return lipgloss.NewStyle().Padding(0, 1).Width(width).Render(line)
}

func renderItems(cfg NavStripConfig, withLabel bool) []string {
	out := make([]string, 0, len(cfg.Items))
	for _, it := range cfg.Items {
		out = append(out, renderNavItem(it, it.Key == cfg.Current, withLabel, cfg.VisibleCount, cfg.TotalCount))
	}
	return out
}

func renderNavItem(it NavItem, active, withLabel bool, visibleCount, totalCount int) string {
	mnem := it.Mnemonic
	label := strings.ToLower(it.Label)

	if active {
		cursor := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render("▌")
		mn := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render(mnem)
		if !withLabel {
			return cursor + mn
		}
		lab := lipgloss.NewStyle().Foreground(theme.ColorFG).Bold(true).Render(label)
		count := activeCount(visibleCount, totalCount)
		return cursor + mn + " " + lab + " " + count
	}

	mn := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render(mnem)
	if !withLabel {
		return " " + mn
	}
	lab := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(label)
	count := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render(fmt.Sprintf("%d", it.Count))
	return " " + mn + " " + lab + " " + count
}

// activeCount renders the canonical filtered/total counter on the active nav
// item: `T` (muted) when unfiltered, `V/T` (V in accent) when filtered.
func activeCount(visible, total int) string {
	if visible == total {
		return lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("%d", total))
	}
	return lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render(fmt.Sprintf("%d", visible)) +
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(fmt.Sprintf("/%d", total))
}
