package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// NavRail renders the left-side resource list with mnemonic 1-0 keys, counts,
// and a cluster meta footer. `current` is the key of the active item (e.g.
// "pods"). `width` is the rail's column width (the design uses 18-22 cols).
// `height` is the total content height.
//
// A thin hairline divider is inserted between item 5 and item 6 to visually
// separate namespaced resources (Pods/Deployments/Services/Secrets/ConfigMaps)
// from cluster-scoped ones (Namespaces/Nodes/...).
func NavRail(width, height int, current string, items []NavItem, meta ClusterMeta) string {
	if width < 8 {
		width = 8
	}
	if height < 6 {
		height = 6
	}

	// Items list — no separate "RESOURCES" header. Dropping it lets the
	// first nav-rail row line up with the center pane's first row (chips +
	// table header), which the user reported as visually offset before.
	rows := make([]string, 0, len(items)+1)
	for i, it := range items {
		rows = append(rows, navItemRow(width, it, it.Key == current))
		// After the 5th item, insert a hairline to group namespaced vs
		// cluster-scoped resources. Only add it if there's actually a 6th
		// item, otherwise the divider would dangle at the bottom.
		if i == 4 && len(items) > 5 {
			rows = append(rows, theme.Panel.Width(width).Padding(0, 1).Render(theme.Divider(width-2)))
		}
	}

	// ── footer (cluster meta) ─────────────────────────────────────────
	footerLines := clusterMetaBlock(width, meta)

	// ── compose: items, spacer, divider, footer ───────────────────────
	used := len(rows) + 1 + len(footerLines) // +1 for footer divider line
	pad := height - used
	if pad < 0 {
		pad = 0
	}
	parts := make([]string, 0, used+pad)
	parts = append(parts, rows...)
	for i := 0; i < pad; i++ {
		parts = append(parts, theme.Panel.Width(width).Render(""))
	}
	parts = append(parts, theme.Panel.Width(width).Render(theme.Divider(width)))
	parts = append(parts, footerLines...)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// navItemRow renders one resource row. Geometry:
//
//	[ACT-PREFIX(3)] [MNEM-CHIP(3)] " " [Label.....gap.....Count]
//
// where ACT-PREFIX is "│› " (accent) on the active row and 3 spaces on
// inactive rows so columns line up. MNEM-CHIP is " 1 " (plain accent digit)
// on the active row and "[1]" (bracketed muted2 digit) on inactive rows so
// the digit reads as a pressable chip without competing with the cursor.
func navItemRow(width int, it NavItem, active bool) string {
	var prefix string
	if active {
		bar := lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("│")
		arrow := lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("›")
		prefix = bar + arrow + " "
	} else {
		prefix = "   "
	}

	mnemColor := theme.ColorMuted2
	labelColor := theme.ColorMuted
	if active {
		mnemColor = theme.ColorAccent
		labelColor = theme.ColorFG
	}

	// Mnemonic chip: " 1 " active, "[1]" inactive — both 3 cells wide.
	digit := it.Mnemonic
	if digit == "" {
		digit = " "
	}
	var mnem string
	if active {
		mnem = lipgloss.NewStyle().Foreground(mnemColor).Render(" " + digit + " ")
	} else {
		mnem = lipgloss.NewStyle().Foreground(mnemColor).Render("[" + digit + "]")
	}

	label := lipgloss.NewStyle().Foreground(labelColor).Render(it.Label)
	count := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render(fmt.Sprintf("%d", it.Count))

	inner := width - 2 // padding 0,1
	left := prefix + mnem + " " + label
	gap := inner - lipgloss.Width(left) - lipgloss.Width(count)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + count

	// Active row uses brighter foreground only — no background — so the
	// nav rail visually merges with the terminal's true black background.
	style := lipgloss.NewStyle().Width(width).Padding(0, 1)
	if active {
		style = style.Foreground(theme.ColorFG)
	}
	return style.Render(line)
}

// clusterMetaBlock renders the small footer with cluster identity and live
// resource utilisation. The "pods X/Y" row was dropped — the cap (PodsCap)
// today always equals Pods, so the value was misleading rather than
// informative. CPU/Mem render an em-dash when their percentage is 0,
// signalling "metrics-server unavailable" rather than "literally 0%".
func clusterMetaBlock(width int, meta ClusterMeta) []string {
	label := theme.Faint.Render("cluster")
	rows := []string{
		theme.Panel.Width(width).Padding(0, 1).Render(label),
		metaRow(width, "nodes", fmt.Sprintf("%d ready", meta.NodesReady), theme.ColorFG),
		metaPctRow(width, "cpu", meta.CPUPercent),
		metaPctRow(width, "mem", meta.MemPercent),
	}
	return rows
}

// metaPctRow renders a percentage row with the tri-state color, or a faint
// em-dash when the percentage is 0 (treated as "no data" rather than "0%").
func metaPctRow(width int, key string, pct int) string {
	if pct <= 0 {
		return metaRow(width, key, "—", theme.ColorFaint)
	}
	return metaRow(width, key, fmt.Sprintf("%d%%", pct), pctColor(pct))
}

func metaRow(width int, key, value string, valueColor lipgloss.Color) string {
	k := theme.Faint.Render(key)
	v := lipgloss.NewStyle().Foreground(valueColor).Render(value)
	inner := width - 2
	gap := inner - lipgloss.Width(k) - lipgloss.Width(v)
	if gap < 1 {
		gap = 1
	}
	line := k + strings.Repeat(" ", gap) + v
	return theme.Panel.Width(width).Padding(0, 1).Render(line)
}

// pctColor maps a percentage to the design's tri-state: ok < 70, warn < 90, err.
func pctColor(p int) lipgloss.Color {
	switch {
	case p >= 90:
		return theme.ColorError
	case p >= 70:
		return theme.ColorWarn
	default:
		return theme.ColorOk
	}
}

func padLeft(s string, n int) string {
	if w := lipgloss.Width(s); w >= n {
		return s
	}
	return strings.Repeat(" ", n-lipgloss.Width(s)) + s
}
