// Package layout — DefaultDetails right-column dossier.
//
// Renders the focused-item dossier as a vertical stack of sections:
//
//	header   — title (bold) + subtitle (status line)
//	KVs      — short label/value table
//	METRICS  — cpu/mem/net sparklines (pods only)
//	CONTAINERS — first-container summary (pods only)
//
// Sections render only when their data is non-empty so non-pod views get a
// trimmed dossier (header + KVs). Width is clamped to a sensible minimum.
// DefaultDetails returns INTERIOR BODY ONLY — no border, no per-row padding.
// The caller wraps the return value in components.Panel.
package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/theme"
)

// DefaultDetails renders the right-pane dossier for a focused row. The
// caller wraps the return value in a Panel — this function returns interior
// content only (no border, no per-row padding).
func DefaultDetails(width, height int, b DetailsBlock) string {
	if b.Title == "" {
		return ""
	}
	if width < 24 {
		width = 24
	}
	if height < 4 {
		height = 4
	}
	inner := width

	header := renderDetailsHeader(b, inner)
	kvs := renderDetailsKVs(b.KVs, inner)
	metrics := renderDetailsMetrics(b.Sparks, inner)
	containers := renderDetailsContainers(b.Containers, inner)

	sections := [][]string{header, kvs, metrics, containers}

	var body []string
	for _, sec := range sections {
		if len(sec) == 0 {
			continue
		}
		if len(body) > 0 {
			body = append(body, "")
		}
		body = append(body, sec...)
	}

	if len(body) > height {
		body = body[:height]
	}
	for len(body) < height {
		body = append(body, "")
	}
	return strings.Join(body, "\n")
}

func renderDetailsHeader(b DetailsBlock, inner int) []string {
	title := lipgloss.NewStyle().Foreground(theme.ColorFG).Bold(true).Render(truncToWidth(b.Title, inner))
	rows := []string{title}
	if b.Subtitle != "" {
		rows = append(rows, lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(truncToWidth(b.Subtitle, inner)))
	}
	return rows
}

func renderDetailsKVs(kvs []KV, inner int) []string {
	if len(kvs) == 0 {
		return nil
	}
	const keyW = 10
	rows := make([]string, 0, len(kvs))
	for _, kv := range kvs {
		k := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(padRight(kv.Key, keyW))
		vColor := theme.ColorFG
		if kv.Warn {
			vColor = theme.ColorWarn
		}
		maxV := max(inner-keyW-1, 1)
		v := lipgloss.NewStyle().Foreground(vColor).Render(truncToWidth(kv.Value, maxV))
		rows = append(rows, k+" "+v)
	}
	return rows
}

func renderDetailsMetrics(sparks []MetricSeries, inner int) []string {
	if len(sparks) == 0 {
		return nil
	}
	const labelW = 5
	const valueW = 8
	// barW: leftover columns after label + space + value + space. Clamp to >=4
	// so very narrow widths still get a compressed sparkline rather than
	// overflowing the row.
	barW := max(inner-labelW-1-valueW-1, 4)

	header := lipgloss.NewStyle().Foreground(theme.ColorMuted).Bold(true).Render("METRICS · last 60s")
	divider := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render(strings.Repeat("─", min(inner, 20)))

	rows := []string{header, divider}
	for _, m := range sparks {
		color := lipgloss.Color(m.Color)
		if m.Color == "" {
			color = theme.ColorAccent
		}
		label := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(padRight(m.Label, labelW))
		bar := components.Sparkline(m.Samples, barW, color)
		value := lipgloss.NewStyle().Foreground(theme.ColorFG).Render(padLeft(m.Value, valueW))
		rows = append(rows, label+" "+bar+" "+value)
	}
	return rows
}

func renderDetailsContainers(cs []ContainerSummary, inner int) []string {
	if len(cs) == 0 {
		return nil
	}
	header := lipgloss.NewStyle().Foreground(theme.ColorMuted).Bold(true).Render("CONTAINERS")
	divider := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render(strings.Repeat("─", min(inner, 20)))

	rows := []string{header, divider}
	for _, c := range cs {
		statusColor := theme.StatusStyleFor(c.Status).Dot
		arrow := lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("▸ ")
		rstColor := theme.ColorMuted
		if c.Restarts > 0 {
			rstColor = theme.ColorWarn
		}
		rstStr := fmt.Sprintf("rst %d", c.Restarts)
		rst := lipgloss.NewStyle().Foreground(rstColor).Render(rstStr)
		// Budget: arrow(2) + gap(3) + name + gap(3) + status + rst. Name
		// absorbs slack when there's room; status truncates only when the
		// row would otherwise overflow inner. At least 1 cell each.
		fixedW := 2 + 3 + 3 + lipgloss.Width(rstStr)
		flexW := max(inner-fixedW, 2)
		statusW := min(lipgloss.Width(c.Status), max(flexW-1, 1))
		nameW := max(flexW-statusW, 1)
		statusStr := lipgloss.NewStyle().Foreground(statusColor).Render(truncToWidth(c.Status, statusW))
		name := lipgloss.NewStyle().Foreground(theme.ColorFG).Render(truncToWidth(c.Name, nameW))
		rows = append(rows, arrow+name+"   "+statusStr+"   "+rst)

		if c.Image != "" {
			// imgKeyW = 10 keeps imgKey + imgVal aligned with the KVs section
			// (keyW = 10) so the dossier reads as a single column grid.
			const imgKeyW = 10
			imgKey := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(padRight("  image", imgKeyW))
			imgVal := lipgloss.NewStyle().Foreground(theme.ColorFG).Render(truncToWidth(c.Image, max(inner-imgKeyW, 1)))
			rows = append(rows, imgKey+imgVal)
		}
	}
	return rows
}

func padLeft(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return strings.Repeat(" ", n-w) + s
}

func padRight(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}
