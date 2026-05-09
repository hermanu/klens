package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/theme"
)

// DefaultDetails renders the right-pane details for a focused row.
// All non-pod views build a DetailsBlock with just KVs and call this.
// Pods passes Sparks + LogTail to render the live metrics + tailing logs.
//
// Returns a multi-line string clamped to `width` columns and at most `height`
// rows. If the block is empty (no Title) returns "" (caller can hide the pane).
func DefaultDetails(width, height int, b DetailsBlock) string {
	if b.Title == "" {
		return ""
	}
	if width < 12 {
		width = 12
	}
	if height < 4 {
		height = 4
	}

	// The pane has a left border + 1-col inside padding (matches the JSX
	// borderLeft/padding). All sections render into `inner` columns.
	const borderW = 1
	const padW = 1
	inner := width - borderW - padW*2
	if inner < 4 {
		inner = 4
	}

	// ── 1. Header ─────────────────────────────────────────────────────
	header := []string{
		theme.Faint.Render("FOCUSED ITEM"),
		lipgloss.NewStyle().Foreground(theme.ColorFG).Render(truncToWidth(b.Title, inner)),
	}
	if b.Subtitle != "" {
		header = append(header, theme.Faint.Render(truncToWidth(b.Subtitle, inner)))
	}

	// ── 2. Live metrics ──────────────────────────────────────────────
	var metrics []string
	if len(b.Sparks) > 0 {
		metrics = append(metrics, theme.Faint.Render("LIVE · 60s"))
		// Layout: <4-char label> <sparkline> <right-aligned value>
		const labelW = 4
		const valueW = 8
		barW := inner - labelW - 1 - valueW - 1
		if barW < 4 {
			barW = 4
		}
		for _, m := range b.Sparks {
			color := lipgloss.Color(m.Color)
			if m.Color == "" {
				color = theme.ColorAccent
			}
			label := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(padRight(m.Label, labelW))
			bar := components.Sparkline(m.Samples, barW, color)
			value := lipgloss.NewStyle().Foreground(theme.ColorFG).Render(padLeft(m.Value, valueW))
			metrics = append(metrics, label+" "+bar+" "+value)
		}
	}

	// ── 3. Spec ──────────────────────────────────────────────────────
	var spec []string
	if len(b.KVs) > 0 {
		spec = append(spec, theme.Faint.Render("SPEC"))
		const keyW = 10
		for _, kv := range b.KVs {
			k := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(padRight(kv.Key, keyW))
			vColor := theme.ColorFG
			if kv.Warn {
				vColor = theme.ColorWarn
			}
			vMax := inner - keyW - 1
			if vMax < 1 {
				vMax = 1
			}
			v := lipgloss.NewStyle().Foreground(vColor).Render(truncToWidth(kv.Value, vMax))
			spec = append(spec, k+" "+v)
		}
	}

	// The right pane intentionally does NOT show a log tail — `l` opens a
	// dedicated full-screen logs view. Duplicating tail lines here added no
	// information density and stole vertical space from SPEC/metrics.

	// ── compose with single-line dividers between sections ───────────
	div := lipgloss.NewStyle().Foreground(theme.ColorBorderFaint).
		Render(strings.Repeat("─", inner))

	sections := [][]string{header, metrics, spec}
	body := []string{}
	for _, sec := range sections {
		if len(sec) == 0 {
			continue
		}
		if len(body) > 0 {
			body = append(body, div)
		}
		body = append(body, sec...)
	}

	// Clamp total rows to height; top of header always wins.
	if len(body) > height {
		body = body[:height]
	}

	// Pad with empty rows so the pane fills the full requested height —
	// otherwise the left border ends mid-pane and the user reads it as a
	// "floating" details panel.
	for len(body) < height {
		body = append(body, "")
	}

	// Apply the left border + 1-col padding to every row, including the
	// padding rows, so the border glyph runs top-to-bottom and the pane
	// "closes" at the bottom of the content area.
	leftBorder := lipgloss.NewStyle().Foreground(theme.ColorBorder).Render("│")
	out := make([]string, 0, len(body))
	for _, row := range body {
		padded := row + strings.Repeat(" ", maxInt(0, inner-lipgloss.Width(row)))
		out = append(out, leftBorder+" "+padded+" ")
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

func padLeft(s string, n int) string {
	if w := lipgloss.Width(s); w >= n {
		return s
	}
	return strings.Repeat(" ", n-lipgloss.Width(s)) + s
}

func padRight(s string, n int) string {
	if w := lipgloss.Width(s); w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-lipgloss.Width(s))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
