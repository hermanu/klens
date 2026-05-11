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

// KlensLogo is the 6-row block-shadow KLENS banner shown in the top bar
// body at width >= topBarWideAt. Same figlet "ANSI Shadow" style used in
// the README hero — combines █ for the letter fills with box-drawing
// glyphs (╗║╔╚╝═) for the shadow edge. Renders cleanly on any terminal
// that supports unicode box-drawing (every modern one).
var KlensLogo = [6]string{
	"██╗  ██╗██╗     ███████╗███╗   ██╗███████╗",
	"██║ ██╔╝██║     ██╔════╝████╗  ██║██╔════╝",
	"█████╔╝ ██║     █████╗  ██╔██╗ ██║███████╗",
	"██╔═██╗ ██║     ██╔══╝  ██║╚██╗██║╚════██║",
	"██║  ██╗███████╗███████╗██║ ╚████║███████║",
	"╚═╝  ╚═╝╚══════╝╚══════╝╚═╝  ╚═══╝╚══════╝",
}

// LogoWidth is the column count of every KlensLogo row.
const LogoWidth = 42

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
//
// Wide path returns 6 rows (logo on the left, KV grid on the right); narrow
// path returns a single-row fallback at widths < topBarWideAt. The caller
// passes Height accordingly via the geometry constants in app/app.go.
func TopBar(width int, cfg TopBarConfig) string {
	if width < 8 {
		width = 8
	}
	if width < topBarWideAt {
		return renderTopBarNarrow(width, cfg)
	}
	return renderTopBarWide(width, cfg)
}

// topBarWideAt mirrors the app/app.go constant of the same name: minimum
// inner width at which the wide path renders. Below this, the body
// collapses to a single-row identity strip. 80 cells = logo(42) + gap(2) +
// minimum KV column(~36) — narrower than this would truncate the KVs.
const topBarWideAt = 80

func renderTopBarWide(inner int, cfg TopBarConfig) string {
	logoStyle := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)
	logoRows := make([]string, len(KlensLogo))
	for i, line := range KlensLogo {
		logoRows[i] = logoStyle.Render(line)
	}

	gap := "  "
	kvW := max(inner-LogoWidth-len(gap), 20)
	kvRows := kvColumn(cfg, kvW, len(KlensLogo))

	out := make([]string, len(KlensLogo))
	for i := range KlensLogo {
		out[i] = logoRows[i] + gap + padRight(kvRows[i], kvW)
	}
	return strings.Join(out, "\n")
}

// kvColumn renders the identity KV grid as exactly n vertically-stacked
// rows so it aligns 1:1 with the logo rows. Each KV pair occupies one
// row; empty / duplicate fields render as blank rows so the grid stays
// rectangular. The mapping (ctx → cluster → user → region → k8s → uptime)
// keeps the most-stable identity fields at the top and the volatile ones
// at the bottom.
func kvColumn(cfg TopBarConfig, w, n int) []string {
	dim := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	hi := lipgloss.NewStyle().Foreground(theme.ColorFG)
	hiBold := lipgloss.NewStyle().Foreground(theme.ColorFG).Bold(true)

	// EKS's `aws eks update-kubeconfig` sets kubeconfig context/cluster/user
	// all to the same ARN — trimming to the basename and skipping duplicates
	// keeps the grid useful instead of three identical 60-char ARNs.
	ctx := trimClusterIdent(cfg.Context)
	cluster := trimClusterIdent(cfg.Cluster)
	user := trimClusterIdent(cfg.User)

	row := func(label, value string, strong bool) string {
		if value == "" {
			return ""
		}
		labelStyled := dim.Render(label + " ")
		valueStyled := hi.Render(value)
		if strong {
			valueStyled = hiBold.Render(value)
		}
		s := labelStyled + valueStyled
		if lw := lipgloss.Width(s); lw < w {
			s += strings.Repeat(" ", w-lw)
		}
		return s
	}

	rows := []string{
		row("ctx", safeStr(ctx, "—"), true),
		"", // placeholder — cluster row filled below when distinct from ctx
		"",
		row("region", optionalStr(cfg.Region), false),
		row("k8s", safeStr(cfg.K8sVersion, "—"), false),
		row("uptime", optionalStr(cfg.Uptime), false),
	}
	if cluster != "" && cluster != ctx {
		rows[1] = row("cluster", cluster, false)
	}
	if user != "" && user != ctx {
		rows[2] = row("user", user, false)
	}
	// Clamp / pad to n rows so the column always returns exactly len(logo)
	// entries regardless of how many KV fields are populated.
	if len(rows) > n {
		rows = rows[:n]
	}
	for len(rows) < n {
		rows = append(rows, "")
	}
	return rows
}

// optionalStr returns "" for empty/placeholder values so the kvColumn can
// suppress entire rows when an identity field isn't wired yet (e.g. region
// stays empty for clusters that don't carry one in kubeconfig).
func optionalStr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "—" {
		return ""
	}
	return s
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
