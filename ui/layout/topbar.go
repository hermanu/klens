// Package layout — TopBar panel body.
//
// Renders the top-bar interior: 2-row block KLENS logo on the left, 2-row
// dense KV grid in the middle, right-aligned cluster meta (nodes + cpu) on
// the right. At widths < 60 collapses to a single-line body keeping only
// context + nodes.
//
// The bordered envelope and notched title/foot are applied by the caller
// via components.Panel — this function returns only the body content.
// Title and foot strings are also produced here for the caller to thread
// into Panel: see TopBarTitle and TopBarFoot.
package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// KlensLogo is the 2-row block-character KLENS banner shown in the top bar
// body at width >= 60. Uses half-block glyphs so the letters render as
// solid 2-cell-tall shapes across most terminals.
var KlensLogo = [2]string{
	"█▄▀ █   █▀▀ █▄ █ █▀",
	"█ █ █▄▄ █▄▄ █ ▀█ ▄█",
}

// LogoWidth is the column count of the KlensLogo entries.
const LogoWidth = 19

// TopBarTitle returns the styled title string for the top-bar Panel:
//
//	◎ KLENS v0.3.0 · build a1b2c3d
//
// Caller hands this to PanelConfig.Title.
func TopBarTitle(cfg TopBarConfig) string {
	dot := lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("◎ ")
	klens := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render("KLENS")
	ver := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(" " + safeStr(cfg.KlensVer, "dev"))
	sep := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render(" · ")
	bid := lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("build " + safeStr(cfg.BuildID, "dev"))
	return dot + klens + ver + sep + bid
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
func TopBar(width int, cfg TopBarConfig) string {
	if width < 8 {
		width = 8
	}
	if width < 60 {
		return renderTopBarNarrow(width, cfg)
	}
	return renderTopBarWide(width, cfg)
}

func renderTopBarWide(inner int, cfg TopBarConfig) string {
	logoStyle := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)
	logoRow0 := logoStyle.Render(KlensLogo[0])
	logoRow1 := logoStyle.Render(KlensLogo[1])

	// Right column intentionally dropped: the nav rail's CLUSTER footer
	// already shows nodes/cpu/mem aggregates, and rendering them here too
	// just doubled the noise (worse: this header would say "cpu —" when the
	// aggregate isn't wired yet). KV grid gets the leftover width.
	gap := "  "
	kvW := max(inner-LogoWidth-len(gap), 20)

	kvRow0 := kvLine(cfg, 0, kvW)
	kvRow1 := kvLine(cfg, 1, kvW)

	row0 := logoRow0 + gap + padRight(kvRow0, kvW)
	row1 := logoRow1 + gap + padRight(kvRow1, kvW)
	return row0 + "\n" + row1
}

func kvLine(cfg TopBarConfig, line, w int) string {
	dim := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	hi := lipgloss.NewStyle().Foreground(theme.ColorFG)
	hiBold := lipgloss.NewStyle().Foreground(theme.ColorFG).Bold(true)

	// Normalised identity strings — EKS's `aws eks update-kubeconfig` sets
	// kubeconfig context/cluster/user all to the same ARN, so showing each
	// raw would print three identical 60-char strings. trimClusterIdent
	// collapses them to the trailing path segment (e.g. "maisa-sdlc-eks").
	ctx := trimClusterIdent(cfg.Context)
	cluster := trimClusterIdent(cfg.Cluster)
	user := trimClusterIdent(cfg.User)

	if line == 0 {
		// ctx <ctx>   [cluster <cluster>]?   region <region>
		// cluster row is skipped when identical to ctx (EKS default state).
		parts := []string{
			dim.Render("ctx ") + hiBold.Render(safeStr(ctx, "—")),
		}
		if cluster != "" && cluster != ctx {
			parts = append(parts, dim.Render("cluster ")+hi.Render(cluster))
		}
		if r := strings.TrimSpace(cfg.Region); r != "" && r != "—" {
			parts = append(parts, dim.Render("region ")+hi.Render(r))
		}
		return joinFit(parts, "   ", w)
	}
	// line 1: [user <user>]?   k8s <k8s>   [uptime <uptime>]?
	// user row hidden when identical to ctx (EKS default).
	parts := []string{}
	if user != "" && user != ctx {
		parts = append(parts, dim.Render("user ")+hi.Render(user))
	}
	parts = append(parts, dim.Render("k8s ")+hi.Render(safeStr(cfg.K8sVersion, "—")))
	if up := strings.TrimSpace(cfg.Uptime); up != "" && up != "—" {
		parts = append(parts, dim.Render("uptime ")+hi.Render(up))
	}
	return joinFit(parts, "   ", w)
}

// trimClusterIdent collapses an ARN-style identity to its trailing
// path segment so the kvLine doesn't show "arn:aws:eks:.../cluster/foo"
// three times in a row. Non-ARN strings pass through unchanged.
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
	row := dim.Render("ctx ") +
		lipgloss.NewStyle().Foreground(theme.ColorFG).Bold(true).Render(safeStr(cfg.Context, "—")) +
		"   " + dim.Render("nodes ") +
		lipgloss.NewStyle().Foreground(valColor).Bold(true).Render(val)
	if w := lipgloss.Width(row); w < inner {
		row += strings.Repeat(" ", inner-w)
	}
	return row
}

// joinFit joins parts with sep, dropping trailing parts until the joined
// string's display width fits within w. Always returns at least the first
// part (truncated if needed).
func joinFit(parts []string, sep string, w int) string {
	for i := len(parts); i > 0; i-- {
		s := strings.Join(parts[:i], sep)
		if lipgloss.Width(s) <= w {
			return s
		}
	}
	if len(parts) == 0 {
		return ""
	}
	first := parts[0]
	for first != "" && lipgloss.Width(first) > w {
		first = first[:len(first)-1]
	}
	return first
}

func safeStr(s, def string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	return s
}

// truncToWidth clamps a styled string to n display columns by trimming bytes
// from the right. Lives here as the canonical home — other layout files
// reach across for it.
func truncToWidth(s string, n int) string {
	if lipgloss.Width(s) <= n {
		return s
	}
	for s != "" && lipgloss.Width(s) > n {
		s = s[:len(s)-1]
	}
	return s
}
