// Package layout — TopBar panel body.
//
// Renders the top-bar interior as a 3-row dashboard at wide widths
// (identity / vitals / phase totals) and a single-row dense strip at narrow.
// The notched panel title carries the brand with an animated mark glyph that
// alternates ◉/◎ on the pulse ticker — the brand "blinks together" with the
// watch dot in the foot.
//
// The bordered envelope and notched title/foot are applied by the caller via
// components.Panel — this function returns only the body content. Title and
// foot strings are also produced here for the caller to thread into Panel:
// see TopBarTitle and TopBarFoot.
package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/theme"
)

// TopBarWideAt is the minimum inner panel width at which the wide top-bar
// path renders. Below this the body collapses to a single-row identity strip.
// 80 cells leaves enough room for the dashboard's left chips + a truncated
// nav strip; app/app.go uses this same constant to pick between
// topBarRowsWide and topBarRowsNarrow.
const TopBarWideAt = 80

// markGlyphPair returns (glyph, color) for the animated brand mark. Alternates
// on pulseOn so the brand pulses in lockstep with the watch dot. When the
// client isn't live (no cluster), the mark stays in the muted state — a quiet
// signal that the watcher is dormant.
func markGlyphPair(pulseOn, live bool) (string, lipgloss.Color) {
	if pulseOn && live {
		return "◉", theme.ColorAccent
	}
	return "◎", theme.ColorMuted2
}

// TopBarTitle returns the styled title for the top-bar Panel:
//
//	◉  K·L·E·N·S  ◣ v0.3.0 ◢  build a1b2c3d
//
// The mark glyph alternates ◉/◎ on pulseOn so the brand pulses in lockstep
// with the watch dot in the foot. The middle-dot-spaced wordmark gives the
// brand gravity on the chrome without consuming body rows. Caller hands the
// return value to PanelConfig.Title; Panel overlays it onto the top border
// and clamps to the available border width if the title would overflow.
func TopBarTitle(cfg TopBarConfig, pulseOn bool) string {
	mark, markColor := markGlyphPair(pulseOn, cfg.Live)
	markS := lipgloss.NewStyle().Foreground(markColor).Render(mark)

	word := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render("K·L·E·N·S")
	lBracket := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render("◣")
	rBracket := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render("◢")
	ver := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("v" + safeStr(cfg.KlensVer, "dev"))
	build := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render("build " + safeStr(cfg.BuildID, "dev"))

	return markS + "  " + word + "  " + lBracket + " " + ver + " " + rBracket + "  " + build
}

// TopBarFoot returns the styled foot for the top-bar Panel: the pulse dot
// + "watching" label. pulseOn toggles ● ↔ ○ once per pulse tick.
func TopBarFoot(pulseOn, live bool) string {
	dot := "○"
	if pulseOn && live {
		dot = "●"
	}
	color := theme.ColorMuted
	if live {
		color = theme.ColorOk
	}
	return lipgloss.NewStyle().Foreground(color).Render(dot) +
		lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(" watching")
}

// TopBar renders the top-bar body (no border — caller wraps via Panel).
// width is the INNER content width (caller passes outerW-2).
//
// Wide path returns 3 rows (identity / vitals / phase totals); narrow path
// returns a single-row fallback at widths < TopBarWideAt. The caller passes
// Height accordingly via the geometry constants in app/app.go
// (topBarRowsWide = 5 = 1 top border + 3 body + 1 bottom border).
func TopBar(width int, cfg TopBarConfig) string {
	if width < 8 {
		width = 8
	}
	if width < TopBarWideAt {
		return renderTopBarNarrow(width, cfg)
	}
	return renderTopBarWide(width, cfg)
}

func renderTopBarWide(inner int, cfg TopBarConfig) string {
	rows := []string{
		identityRow(inner, cfg),
		vitalsRow(inner, cfg),
		phaseRow(inner, cfg),
	}
	return strings.Join(rows, "\n")
}

// identityRow renders the dashboard's row 1: animated mark + KLENS wordmark
// followed by space-separated identity chips (ctx · region · k8s · uptime).
// Chips drop from the right (lowest-priority first) when the row would overflow.
func identityRow(inner int, cfg TopBarConfig) string {
	mark, markColor := markGlyphPair(cfg.PulseOn, cfg.Live)
	markChunk := lipgloss.NewStyle().Foreground(markColor).Render(mark) + " " +
		lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render("KLENS")

	sep := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render("  ·  ")
	sepW := lipgloss.Width(sep)

	chips := []string{markChunk}
	addChip := func(label, value string) {
		if value == "" {
			return
		}
		s := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(label+" ") +
			lipgloss.NewStyle().Foreground(theme.ColorFG).Render(value)
		chips = append(chips, s)
	}
	addChip("ctx", trimClusterIdent(safeStr(cfg.Context, "")))
	addChip("region", optionalStr(cfg.Region))
	addChip("k8s", optionalStr(cfg.K8sVersion))
	addChip("uptime", optionalStr(cfg.Uptime))

	// Drop chips from the right until the row fits. The mark+wordmark never drops.
	for len(chips) > 1 && joinedWidth(chips, sepW) > inner {
		chips = chips[:len(chips)-1]
	}
	return padRight(strings.Join(chips, sep), inner)
}

// vitalsRow renders the dashboard's row 2: nodes ratio + cpu sparkline +
// current namespace on the left, horizontal resource nav strip on the right.
// The nav strip absorbs the leftover width and truncates with a faint `…`
// when too many entries would overflow.
func vitalsRow(inner int, cfg TopBarConfig) string {
	sep := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render("  ·  ")

	var left []string

	if cfg.NodesTotal > 0 {
		valColor := theme.ColorOk
		if cfg.NodesReady != cfg.NodesTotal {
			valColor = theme.ColorWarn
		}
		s := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("nodes ") +
			lipgloss.NewStyle().Foreground(valColor).Bold(true).Render(fmt.Sprintf("%d/%d", cfg.NodesReady, cfg.NodesTotal))
		left = append(left, s)
	}

	if len(cfg.CPUSamples) > 0 {
		const barW = 6
		spark := components.Sparkline(cfg.CPUSamples, barW, theme.ColorAccent)
		s := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("cpu ") + spark
		if cfg.CPUPercent >= 0 {
			s += lipgloss.NewStyle().Foreground(theme.ColorFG).Render(fmt.Sprintf(" %d%%", cfg.CPUPercent))
		}
		left = append(left, s)
	}

	if ns := optionalStr(cfg.Namespace); ns != "" {
		s := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("ns ") +
			lipgloss.NewStyle().Foreground(theme.ColorFG).Bold(true).Render(ns)
		left = append(left, s)
	}

	leftStr := strings.Join(left, sep)
	leftW := lipgloss.Width(leftStr)

	const gap = 4 // visual breathing room between left vitals and nav strip
	navBudget := max(inner-leftW-gap, 0)
	nav := navStrip(cfg.NavItems, navBudget)
	navW := lipgloss.Width(nav)

	pad := max(inner-leftW-navW, 0)
	return leftStr + strings.Repeat(" ", pad) + nav
}

// navStrip renders the resource navigation as a horizontal `▌1 pods  2 dp  …`
// strip. Entries that don't fit budget are dropped from the right, replaced
// by a faint `…` to signal truncation. Returns "" when budget < ~4 cells.
func navStrip(items []NavItem, budget int) string {
	if len(items) == 0 || budget < 4 {
		return ""
	}
	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)
	muted := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	mnemonicMuted := lipgloss.NewStyle().Foreground(theme.ColorMuted2)
	const innerSep = "  "
	innerSepW := lipgloss.Width(innerSep)

	rendered := make([]string, 0, len(items))
	totalW := 0
	for _, it := range items {
		label := shortLabel(it.Label)
		var chunk string
		if it.Active {
			chunk = accent.Render("▌") + accent.Render(it.Mnemonic) + " " + accent.Render(label)
		} else {
			chunk = mnemonicMuted.Render(it.Mnemonic) + " " + muted.Render(label)
		}
		chunkW := lipgloss.Width(chunk)
		add := chunkW
		if len(rendered) > 0 {
			add += innerSepW
		}
		// Reserve 2 cells for trailing "  …" when more entries remain after this one.
		reserve := 0
		if len(rendered)+1 < len(items) {
			reserve = innerSepW + 1
		}
		if totalW+add+reserve > budget {
			break
		}
		rendered = append(rendered, chunk)
		totalW += add
	}
	out := strings.Join(rendered, innerSep)
	if len(rendered) < len(items) {
		out += innerSep + lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render("…")
	}
	return out
}

// shortLabel compresses verbose resource names so the horizontal nav strip
// fits a useful number of entries within the body's remaining width.
func shortLabel(label string) string {
	switch label {
	case "deployments":
		return "dp"
	case "services":
		return "svc"
	case "secrets":
		return "sec"
	case "configmaps":
		return "cm"
	case "namespaces":
		return "ns"
	case "nodes":
		return "no"
	case "pvcs":
		return "pvc"
	}
	return label
}

// phaseRow renders the dashboard's row 3 — pod phase totals — when the
// active view exposes PhaseCounts (only the pods view today). Other views
// pass cfg.PhaseCounts == nil and the row renders empty so the body height
// stays at 3 across view switches.
//
//	Running 23  ·  Pending 1  ·  Error 0  ·  Total 54
//
// Color rules:
//   - Running: ColorOk always (Running is the happy state).
//   - Pending: ColorWarn when > 0, ColorMuted otherwise.
//   - Error:   ColorError when > 0, ColorMuted otherwise.
//   - Total:   ColorFG always.
func phaseRow(inner int, cfg TopBarConfig) string {
	if cfg.PhaseCounts == nil {
		return strings.Repeat(" ", inner)
	}
	pc := cfg.PhaseCounts
	pendingColor := theme.ColorMuted
	if pc.Pending > 0 {
		pendingColor = theme.ColorWarn
	}
	errorColor := theme.ColorMuted
	if pc.Errored > 0 {
		errorColor = theme.ColorError
	}

	sep := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render("  ·  ")
	parts := []string{
		phaseChip("Running", pc.Running, theme.ColorOk),
		phaseChip("Pending", pc.Pending, pendingColor),
		phaseChip("Error", pc.Errored, errorColor),
		phaseChip("Total", pc.Total, theme.ColorFG),
	}
	row := strings.Join(parts, sep)
	return padRight(truncToWidth(row, inner), inner)
}

// phaseChip renders one "<label> <n>" chip — muted label, bold colored value.
// Centralised so all four phase chips share the same layout and bold weight.
func phaseChip(label string, n int, c lipgloss.Color) string {
	l := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(label + " ")
	v := lipgloss.NewStyle().Foreground(c).Bold(true).Render(fmt.Sprintf("%d", n))
	return l + v
}

// joinedWidth returns the rendered display width of chips joined by a separator
// of width sepW. Used by identityRow's drop-from-right fitting loop.
func joinedWidth(chips []string, sepW int) int {
	total := 0
	for i, c := range chips {
		if i > 0 {
			total += sepW
		}
		total += lipgloss.Width(c)
	}
	return total
}

// trimClusterIdent collapses an ARN-style identity to its trailing path
// segment so the topbar doesn't burn an entire row on a 60-char ARN. Non-ARN
// strings pass through unchanged.
func trimClusterIdent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "—" {
		return s
	}
	if i := strings.LastIndex(s, "/"); i >= 0 && i < len(s)-1 {
		return s[i+1:]
	}
	return s
}

func renderTopBarNarrow(inner int, cfg TopBarConfig) string {
	mark, markColor := markGlyphPair(cfg.PulseOn, cfg.Live)
	markS := lipgloss.NewStyle().Foreground(markColor).Render(mark)

	val := "—"
	valColor := theme.ColorFG
	if cfg.NodesTotal > 0 {
		val = fmt.Sprintf("%d/%d", cfg.NodesReady, cfg.NodesTotal)
		if cfg.NodesReady == cfg.NodesTotal {
			valColor = theme.ColorOk
		} else {
			valColor = theme.ColorWarn
		}
	}
	dim := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	row := markS + " " + dim.Render("ctx ") +
		lipgloss.NewStyle().Foreground(theme.ColorFG).Bold(true).Render(trimClusterIdent(safeStr(cfg.Context, "—"))) +
		"   " + dim.Render("nodes ") +
		lipgloss.NewStyle().Foreground(valColor).Bold(true).Render(val)
	if w := lipgloss.Width(row); w < inner {
		row += strings.Repeat(" ", inner-w)
	}
	return row
}

// safeStr returns the trimmed input, or def when the input is empty.
func safeStr(s, def string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	return s
}

// optionalStr returns "" for empty/placeholder values so the topbar can
// suppress entire chips when an identity field isn't wired yet (e.g. region
// stays empty for clusters that don't carry one in kubeconfig).
func optionalStr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "—" {
		return ""
	}
	return s
}

// truncToWidth clamps a styled string to n display columns by trimming bytes
// from the right. Lives here as the canonical home — other layout files reach
// across for it. Note: not ANSI-aware; only safe for strings that are either
// plain text or whose ANSI tail is acceptable to drop.
func truncToWidth(s string, n int) string {
	if lipgloss.Width(s) <= n {
		return s
	}
	for s != "" && lipgloss.Width(s) > n {
		s = s[:len(s)-1]
	}
	return s
}
