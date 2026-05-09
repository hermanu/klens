package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/theme"
)

// TopBar renders the modern shell's top bar. Two compact rows + a divider:
//
//	ctx maisa-sdlc · v1.35           ── K L E N S ──            : palette · ● live
//	▆ europa  pods · 4 of 23
//
// The count on row 2 is the canonical filtered/total counter for the current
// view. Anchoring it here (instead of inside the filter strip) keeps the
// number at the same screen column on every render so the eye doesn't have
// to track it as filters change.
//
// Cluster ARN, user ARN, region and klens-version are intentionally left out:
// they don't change during a session and just steal horizontal real estate.
// Discoverable via the palette (`ctx`, `:about`).
func TopBar(width int, cfg TopBarConfig) string {
	if width < 1 {
		width = 1
	}

	// ── row 1: identity (left) — KLENS banner (center) — ⌘K + live (right) ─
	identity := identityStrip(cfg)
	banner := klensBanner()
	right1 := commandHints(cfg.Live)
	r1 := flex3(width, identity, banner, right1)

	// ── row 2: scope (left) — right side intentionally empty ──────────────
	// We dropped the aggregate counters chip from row 2: it was redundant
	// with the canonical "V of N" count we now anchor inside scopeStrip,
	// and it competed with the live dot on row 1 for attention.
	scope := scopeStrip(cfg)
	r2 := flex(width, scope, "")

	div := lipgloss.NewStyle().
		Foreground(theme.ColorBorder).
		Render(strings.Repeat("─", width))
	return lipgloss.JoinVertical(lipgloss.Left, r1, r2, div)
}

// klensBanner is the brand banner — a letter-spaced KLENS in accent flanked
// by symmetric thin horizontal-line "bookends" so it reads as a single
// balanced unit. The previous lone leading "◉" sat off-center and made the
// title look like it had a stray dot on the side.
func klensBanner() string {
	bookend := lipgloss.NewStyle().
		Foreground(theme.ColorBorder).
		Render("──")
	name := lipgloss.NewStyle().
		Foreground(theme.ColorAccent).
		Bold(true).
		Render("K L E N S")
	return bookend + "  " + name + "  " + bookend
}

// identityStrip renders the minimal cluster identity — just the context name
// and the short k8s server version. Cluster ARN and user ARN are deliberately
// omitted (they're long, redundant with the context name, and not actionable
// at runtime).
func identityStrip(cfg TopBarConfig) string {
	parts := []string{}
	if c := strings.TrimSpace(cfg.Context); c != "" {
		parts = append(parts,
			theme.Faint.Render("ctx")+" "+
				lipgloss.NewStyle().Foreground(theme.ColorFG).Render(c))
	}
	if v := shortK8sVersion(cfg.K8sVersion); v != "" {
		parts = append(parts,
			theme.Faint.Render("·")+" "+
				lipgloss.NewStyle().Foreground(theme.ColorFG2).Render(v))
	}
	return strings.Join(parts, "  ")
}

// shortK8sVersion compresses "v1.35.3-eks-bbe087e" to "v1.35.3". Only the
// minor version is what users care about at a glance.
func shortK8sVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "—" {
		return ""
	}
	if i := strings.Index(v, "-"); i > 0 {
		v = v[:i]
	}
	return v
}

// commandHints renders the right-hand chip strip on row 1.
func commandHints(live bool) string {
	// Show only the keys that actually open the palette. We drop "⌘K"
	// because it conflicts with terminal emulator shortcuts (Warp, iTerm).
	palette := lipgloss.NewStyle().Foreground(theme.ColorFG2).Render(":") +
		" " + theme.Faint.Render("palette")
	if !live {
		return palette
	}
	dot := lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("●")
	return palette + "   " + dot + " " + theme.Faint.Render("live")
}

// scopeStrip renders the row 2 scope: namespace chip + resource label +
// canonical filtered/total count. The count column is anchored here so it
// stays at the same screen column on every render of every view, instead of
// drifting around as filters change.
//
//	▆ europa  pods · 23           (unfiltered: V == T)
//	▆ europa  pods · 4 of 23      (filtered:   V != T, V in accent)
func scopeStrip(cfg TopBarConfig) string {
	ns := strings.TrimSpace(cfg.Namespace)
	chip := scopeNamespaceChip(ns)
	res := lipgloss.NewStyle().
		Foreground(theme.ColorFG).
		Bold(true).
		Render(fallback(cfg.Resource, "pods"))

	muted := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent)
	var count string
	if cfg.VisibleCount == cfg.TotalCount {
		count = muted.Render(fmt.Sprintf("· %d", cfg.TotalCount))
	} else {
		count = muted.Render("· ") +
			accent.Render(fmt.Sprintf("%d", cfg.VisibleCount)) +
			muted.Render(fmt.Sprintf(" of %d", cfg.TotalCount))
	}
	return chip + "   " + res + "  " + count
}

// scopeNamespaceChip renders a colored namespace chip — the visual anchor of
// the modern shell. Color is derived from theme.NSColor (mirrors the design's
// per-namespace palette) so users develop muscle-memory between color and
// scope.
func scopeNamespaceChip(ns string) string {
	if ns == "all" || ns == "" {
		return lipgloss.NewStyle().
			Foreground(theme.ColorMuted).
			Bold(true).
			Render("▆ all namespaces")
	}
	return components.NSChipBold(ns)
}

// flex lays out a left and right segment across `width` cells, padding the
// middle with spaces. If they don't fit, the left side is truncated.
func flex(width int, left, right string) string {
	inner := width - 2
	if inner < 1 {
		inner = 1
	}
	gap := inner - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		left = truncToWidth(left, inner-lipgloss.Width(right)-1)
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return lipgloss.NewStyle().Padding(0, 1).Width(width).Render(line)
}

// flex3 lays out left | center | right segments across `width`, biasing the
// center to be horizontally centered when there's enough slack.
func flex3(width int, left, mid, right string) string {
	inner := width - 2
	if inner < 1 {
		inner = 1
	}
	lW := lipgloss.Width(left)
	mW := lipgloss.Width(mid)
	rW := lipgloss.Width(right)
	slack := inner - lW - mW - rW
	if slack < 2 {
		// Fallback: drop center, just left/right.
		return flex(width, left, right)
	}
	gapL := slack / 2
	gapR := slack - gapL
	idealLeftEnd := (inner - mW) / 2
	if idealLeftEnd > lW {
		gapL = idealLeftEnd - lW
		gapR = slack - gapL
		if gapR < 1 {
			gapR = 1
			gapL = slack - gapR
		}
	}
	line := left + strings.Repeat(" ", gapL) + mid + strings.Repeat(" ", gapR) + right
	return lipgloss.NewStyle().Padding(0, 1).Width(width).Render(line)
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// truncToWidth clamps a styled string to n display columns by trimming bytes
// from the right. Used as a fallback when the bar is too narrow for full layout.
func truncToWidth(s string, n int) string {
	if lipgloss.Width(s) <= n {
		return s
	}
	for len(s) > 0 && lipgloss.Width(s) > n {
		s = s[:len(s)-1]
	}
	return s
}
