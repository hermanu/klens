// Package layout — NavRail body renderer.
//
// NavRail draws the rail's interior content (the bordered envelope is
// applied by components.Panel at the call site, not here). The rail has
// three vertical sections:
//
//  1. The 8 mnemonic resource rows (pods/deps/.../pvcs) with active
//     highlight (▌ + accent fg) and right-aligned counts.
//  2. A spacer that absorbs leftover height.
//  3. The CLUSTER footer — nodes/pods/cpu/mem aggregate stats with
//     mini sparklines.
//
// When height is too small to render the footer, the footer drops first;
// when smaller still, the items below the active one drop. Width is
// expected to be 22 cols (the rail's fixed allotment).
package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/theme"
)

// NavItem is one row in the rail's resource list.
type NavItem struct {
	Mnemonic string // "1".."8"
	Label    string // "pods", "deployments", …
	Count    int    // total rows in the underlying view; -1 = "—"
	Active   bool
}

// ClusterMeta is the rail's footer block — derived stats over the whole
// cluster. The model populates this from current node + metrics state.
type ClusterMeta struct {
	NodesReady int
	NodesTotal int
	// Pods is the cluster's visible pod count. Zero renders "—" — used as
	// the "not yet fetched" sentinel because a healthy cluster never has
	// zero pods (kube-system alone keeps the count non-zero).
	Pods int
	// CPUSamples / MEMSamples are 0..100 normalised; empty slice = "—".
	CPUSamples []float64
	MEMSamples []float64
	// CPUPercent / MEMPercent: -1 renders "—" (unknown).
	CPUPercent int
	MEMPercent int
}

// NavRailConfig groups the immutable render snapshot passed to NavRail so
// the signature stays stable as new footer sections (e.g. recent events,
// pinned namespaces) get added in future iterations.
type NavRailConfig struct {
	Items   []NavItem
	Cluster ClusterMeta
}

// NavRail renders the rail body, sized to fit inside a width × height
// content area. The caller wraps the return value in components.Panel.
func NavRail(width, height int, cfg NavRailConfig) string {
	// Width 18: cursor(2) + mnemonic(2) + gap(1) + label(12) + gap(1) +
	// count(>=3) plus 2 cols of bezel slack — narrower than this would
	// corrupt the row layout. Height 4: minimum to show at least one item
	// row plus the footer header + divider.
	if width < 18 {
		width = 18
	}
	if height < 4 {
		height = 4
	}

	var rows []string
	for _, it := range cfg.Items {
		rows = append(rows, renderNavItem(it, width))
	}

	footer := renderClusterMeta(cfg.Cluster, width)
	footerH := strings.Count(footer, "\n") + 1

	// Fit: items + blank-spacer + footer = height.
	spacer := height - len(rows) - footerH
	if spacer < 0 {
		// Drop footer first.
		footer = ""
		spacer = height - len(rows)
		if spacer < 0 {
			// Truncate items from the end. Keep active item visible.
			activeIdx := -1
			for i, it := range cfg.Items {
				if it.Active {
					activeIdx = i
					break
				}
			}
			if activeIdx >= height {
				// Slide window down so active stays visible.
				start := activeIdx - height + 1
				rows = rows[start : start+height]
			} else {
				rows = rows[:height]
			}
			spacer = 0
		}
	}

	body := make([]string, 0, height)
	body = append(body, rows...)
	for range spacer {
		body = append(body, "")
	}
	if footer != "" {
		body = append(body, strings.Split(footer, "\n")...)
	}
	if len(body) > height {
		body = body[:height]
	}
	for len(body) < height {
		body = append(body, "")
	}
	return strings.Join(body, "\n")
}

func renderNavItem(it NavItem, width int) string {
	const mnemonicW = 2
	const labelW = 12
	// 2 for cursor prefix, gaps between segments.
	countW := max(width-2-mnemonicW-1-labelW-1, 3)

	cursor := "  "
	mnemonicStyle := lipgloss.NewStyle().Foreground(theme.ColorMuted2)
	labelStyle := lipgloss.NewStyle().Foreground(theme.ColorFG)
	countStyle := lipgloss.NewStyle().Foreground(theme.ColorMuted2)

	if it.Active {
		cursor = lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("▌ ")
		mnemonicStyle = lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)
		labelStyle = lipgloss.NewStyle().Foreground(theme.ColorFG).Bold(true)
		countStyle = lipgloss.NewStyle().Foreground(theme.ColorMuted)
	}

	count := "—"
	if it.Count >= 0 {
		count = fmt.Sprintf("%d", it.Count)
	}

	return cursor +
		mnemonicStyle.Render(padRightLayout(it.Mnemonic, mnemonicW)) +
		" " +
		labelStyle.Render(padRightLayout(it.Label, labelW)) +
		" " +
		countStyle.Render(padLeftLayout(count, countW))
}

func renderClusterMeta(cm ClusterMeta, width int) string {
	header := lipgloss.NewStyle().Foreground(theme.ColorMuted).Bold(true).Render("  CLUSTER")
	divider := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render("  " + strings.Repeat("─", width-4))

	nodesStr := "—"
	nodesColor := theme.ColorFG
	if cm.NodesTotal > 0 {
		nodesStr = fmt.Sprintf("%d/%d", cm.NodesReady, cm.NodesTotal)
		if cm.NodesReady == cm.NodesTotal {
			nodesColor = theme.ColorOk
		} else {
			nodesColor = theme.ColorWarn
		}
	}
	nodesRow := "  " +
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(padRightLayout("nodes", 10)) +
		lipgloss.NewStyle().Foreground(nodesColor).Bold(true).Render(nodesStr)

	podsStr := "—"
	if cm.Pods > 0 {
		podsStr = fmt.Sprintf("%d", cm.Pods)
	}
	podsRow := "  " +
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(padRightLayout("pods", 10)) +
		lipgloss.NewStyle().Foreground(theme.ColorFG).Render(podsStr)

	cpuRow := metricSpark("cpu", cm.CPUSamples, cm.CPUPercent, theme.ColorOk)
	memRow := metricSpark("mem", cm.MEMSamples, cm.MEMPercent, theme.ColorWarn)

	return strings.Join([]string{header, divider, nodesRow, podsRow, cpuRow, memRow}, "\n")
}

// metricSpark renders a single metric row with an optional sparkline and
// percentage. When samples is empty or percent is -1 the fields render "—"
// so the footer is always well-defined even without metrics data.
func metricSpark(label string, samples []float64, percent int, color lipgloss.Color) string {
	const sparkW = 10
	dash := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("—")
	pct := dash
	if percent >= 0 {
		pct = lipgloss.NewStyle().Foreground(theme.ColorFG).Render(fmt.Sprintf("%d%%", percent))
	}

	spark := dash
	if len(samples) > 0 {
		spark = components.Sparkline(samples, sparkW, color)
	}

	return "  " +
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(padRightLayout(label, 5)) +
		spark + " " + pct
}

func padRightLayout(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func padLeftLayout(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return strings.Repeat(" ", n-w) + s
}
