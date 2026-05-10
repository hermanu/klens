package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// NavRail renders the left-side vertical resource navigation. It carries the
// same data the (now retired) horizontal NavStrip used and applies the same
// two-tone styling: muted everywhere inactive, bold accent on the active
// item. The active item gets a `▌` cursor and shows a `V/T` count when
// filtered (V bold accent, /T regular accent); inactive items show their
// total in muted.
//
//	▌[1] pods         4/56
//	 [2] deployments    22
//	 [3] services       23
//	 [4] secrets        14
//	 ...
//
// The vertical rail uses the design's left-rail anchor — once the user gets
// past 80 cols of horizontal real estate, putting the nav on the left is
// clearer than competing for the top band, and it leaves the entire top row
// for identity + brand. Drops below `minRailAt` cols so narrow terminals get
// the full table width.
func NavRail(width, height int, cfg NavRailConfig) string {
	if width < 8 {
		width = 8
	}
	if height < 1 {
		height = 1
	}

	rows := make([]string, 0, len(cfg.Items)+1)
	for _, it := range cfg.Items {
		rows = append(rows, navRailRow(width, it, it.Key == cfg.Current, cfg.VisibleCount, cfg.TotalCount))
	}
	// Pad to fill the requested height so the rail's right edge aligns with
	// the table's bottom edge.
	for len(rows) < height {
		rows = append(rows, lipgloss.NewStyle().Width(width).Render(""))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// navRailRow renders a single resource entry as `▌[N] label  count` (active)
// or ` [N] label  count` (inactive). The label and count are pushed to the
// right edge so the column reads as three aligned bands: cursor / mnemonic /
// label / count.
func navRailRow(width int, it NavItem, active bool, visibleCount, totalCount int) string {
	mn := "[" + it.Mnemonic + "]"
	label := strings.ToLower(it.Label)

	cursor := " "
	if active {
		cursor = lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render("▌")
	}

	var mnPart, labelPart, countPart string
	if active {
		accent := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)
		mnPart = accent.Render(mn)
		labelPart = accent.Render(label)
		countPart = activeRailCount(visibleCount, totalCount)
	} else {
		muted := lipgloss.NewStyle().Foreground(theme.ColorMuted)
		mnPart = muted.Render(mn)
		labelPart = muted.Render(label)
		countPart = muted.Render(fmt.Sprintf("%d", it.Count))
	}

	// Layout: cursor (1) + space (1) + mnem (3 visible) + space + label … + count (right-aligned)
	leftBlock := cursor + " " + mnPart + " " + labelPart
	leftW := lipgloss.Width(leftBlock)
	countW := lipgloss.Width(countPart)
	// inner is width - 1 trailing space so the count doesn't kiss the right edge.
	gap := width - leftW - countW - 1
	if gap < 1 {
		// Rail too narrow — drop the label, keep cursor + mnem + count.
		mnOnly := cursor + " " + mnPart
		gap = width - lipgloss.Width(mnOnly) - countW - 1
		if gap < 1 {
			gap = 1
		}
		return mnOnly + strings.Repeat(" ", gap) + countPart + " "
	}
	return leftBlock + strings.Repeat(" ", gap) + countPart + " "
}

// activeRailCount renders the active item's `V/T` (V bold accent, /T regular
// accent), or just `T` when unfiltered. Same two-tone rule as the strip
// version we replaced — count belongs to the same color family as the rest
// of the active row, just with bold-vs-regular for the V/T split.
func activeRailCount(visible, total int) string {
	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent)
	if visible == total {
		return accent.Bold(true).Render(fmt.Sprintf("%d", total))
	}
	return accent.Bold(true).Render(fmt.Sprintf("%d", visible)) +
		accent.Render(fmt.Sprintf("/%d", total))
}
