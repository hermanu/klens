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

// Two-tone palette: every inactive cell renders in muted; the active item
// renders in accent (bold). No third color, no per-token shade gradient — the
// user feedback was that mixing fg / muted / muted2 made the strip hard to
// scan. The only intra-item contrast on the active cell is bold-vs-regular
// for the V/T split when filtered (V bold, /T regular).
func renderNavItem(it NavItem, active, withLabel bool, visibleCount, totalCount int) string {
	mnem := it.Mnemonic
	label := strings.ToLower(it.Label)

	if active {
		accent := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)
		cursor := accent.Render("▌")
		mn := accent.Render(mnem)
		if !withLabel {
			return cursor + mn
		}
		return cursor + mn + " " + accent.Render(label) + " " + activeCount(visibleCount, totalCount)
	}

	muted := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	mn := muted.Render(mnem)
	if !withLabel {
		return " " + mn
	}
	return " " + mn + " " + muted.Render(label) + " " + muted.Render(fmt.Sprintf("%d", it.Count))
}

// activeCount renders the active item's count: `T` when unfiltered, `V/T`
// when filtered. Both halves use the accent — V is bold, `/T` is regular —
// so the eye still parses filtered-vs-total at a glance without introducing a
// third color.
func activeCount(visible, total int) string {
	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent)
	if visible == total {
		return accent.Bold(true).Render(fmt.Sprintf("%d", total))
	}
	return accent.Bold(true).Render(fmt.Sprintf("%d", visible)) +
		accent.Render(fmt.Sprintf("/%d", total))
}
