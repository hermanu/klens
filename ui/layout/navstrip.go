package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// NavStrip renders the horizontal resource navigation bar that sits directly
// under the top bar. Each item is a compact `[N] label count` block; the
// active item is highlighted with bold accent text + brackets in accent.
//
//	▌[1] pods 4/23   [2] deployments 14   [3] services 12   ...
//
// Bracketed mnemonics (`[1]`, `[2]`, ...) keep the key glyph visually
// distinct from the count number that follows the label — without the
// brackets, `1 pods 56` reads as two numbers with no clue which is which.
//
// The active item's count is `V/T` when filtered (V bold accent, /T regular
// accent), or just `T` when unfiltered. Inactive items always show their
// total in muted. Two-tone palette only (muted + accent), no third color.
//
// The strip is centered across `width` so it visually aligns with the
// banner above. At narrow widths it falls back to bracketed mnemonics only.
func NavStrip(width int, cfg NavStripConfig) string {
	if width < 1 {
		width = 1
	}

	itemsFull := renderItems(cfg, true)
	full := strings.Join(itemsFull, "   ")
	if lipgloss.Width(full) <= width-2 {
		return centerStrip(width, full)
	}

	// Narrow fallback: bracketed mnemonics only.
	itemsCompact := renderItems(cfg, false)
	compact := strings.Join(itemsCompact, "  ")
	return centerStrip(width, compact)
}

// centerStrip places `line` horizontally centered within `width`. Falls back
// to left-padding(1) if `line` already fills the row.
func centerStrip(width int, line string) string {
	contentW := lipgloss.Width(line)
	if contentW >= width-2 {
		return lipgloss.NewStyle().Padding(0, 1).Width(width).Render(line)
	}
	leftPad := (width - contentW) / 2
	if leftPad < 1 {
		leftPad = 1
	}
	rightPad := width - leftPad - contentW
	if rightPad < 0 {
		rightPad = 0
	}
	return strings.Repeat(" ", leftPad) + line + strings.Repeat(" ", rightPad)
}

func renderItems(cfg NavStripConfig, withLabel bool) []string {
	out := make([]string, 0, len(cfg.Items))
	for _, it := range cfg.Items {
		out = append(out, renderNavItem(it, it.Key == cfg.Current, withLabel, cfg.VisibleCount, cfg.TotalCount))
	}
	return out
}

// Two-tone palette: every inactive cell renders in muted; the active item
// renders in accent (bold). The mnemonic is wrapped in `[N]` brackets so it
// reads as a key glyph instead of getting confused with the count number
// that follows the label.
func renderNavItem(it NavItem, active, withLabel bool, visibleCount, totalCount int) string {
	mnem := "[" + it.Mnemonic + "]"
	label := strings.ToLower(it.Label)

	if active {
		accent := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)
		mn := accent.Render(mnem)
		if !withLabel {
			return mn
		}
		return mn + " " + accent.Render(label) + " " + activeCount(visibleCount, totalCount)
	}

	muted := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	mn := muted.Render(mnem)
	if !withLabel {
		return mn
	}
	return mn + " " + muted.Render(label) + " " + muted.Render(fmt.Sprintf("%d", it.Count))
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
