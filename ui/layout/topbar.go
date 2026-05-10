package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/theme"
)

// TopBar renders the modern shell's compact top bar — a single row + divider:
//
//	ctx maisa-sdlc · v1.30 · ▆ europa     ── K L E N S ──     ● live
//
// The horizontal nav strip (rendered by NavStrip) sits directly under this
// divider and carries the per-resource mnemonic + count. The aggregate
// filtered/total count is anchored on the active nav item — not duplicated
// here — so the chrome stays one line tall.
//
// Cluster ARN, user ARN, region and klens-version are intentionally left out:
// they don't change during a session and just steal horizontal real estate.
// Discoverable via the palette (`ctx`, `:about`).
func TopBar(width int, cfg TopBarConfig) string {
	if width < 1 {
		width = 1
	}

	left := identityStrip(cfg)
	mid := klensBanner()
	right := rightChips(cfg.Live)
	row := flex3(width, left, mid, right)

	div := lipgloss.NewStyle().
		Foreground(theme.ColorBorder).
		Render(strings.Repeat("─", width))
	return lipgloss.JoinVertical(lipgloss.Left, row, div)
}

// klensBanner is the brand banner — a letter-spaced KLENS in accent flanked
// by symmetric thin horizontal-line bookends so it reads as one balanced unit.
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

// identityStrip renders the minimal cluster identity: context + short k8s
// version + the active namespace chip. The namespace chip used to live on a
// dedicated row 2 (with the resource label and count); we collapsed it here
// because the resource label is now redundant with the active nav item, and
// the count is now anchored on that nav item too.
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
	parts = append(parts,
		theme.Faint.Render("·")+" "+nsChip(cfg.Namespace))
	if r := strings.TrimSpace(cfg.Resource); r != "" {
		parts = append(parts,
			theme.Faint.Render("·")+" "+resourceCount(r, cfg.VisibleCount, cfg.TotalCount))
	}
	return strings.Join(parts, "  ")
}

// resourceCount renders the active resource label + V/T count, e.g.
// `pods 4/56` or `pods 56`. Now that the rail is gone, this is the only
// place the user sees what view they're on, so it lives in the top bar's
// identity strip alongside the namespace chip.
func resourceCount(resource string, visible, total int) string {
	res := lipgloss.NewStyle().Foreground(theme.ColorFG).Bold(true).Render(resource)
	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)
	muted := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	if visible == total {
		return res + " " + muted.Render(fmt.Sprintf("%d", total))
	}
	return res + " " + accent.Render(fmt.Sprintf("%d", visible)) +
		muted.Render(fmt.Sprintf("/%d", total))
}

// shortK8sVersion compresses "v1.35.3-eks-bbe087e" to "v1.35.3" — the minor
// version is what users care about at a glance.
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

// rightChips renders the top bar's right-hand corner: the `: palette` hint
// and (when live) the watch dot. Two-tone palette only — accent for the key
// glyph + dot, muted for the descriptive label.
func rightChips(live bool) string {
	muted := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent)

	paletteHint := accent.Render(":") + " " + muted.Render("palette")
	if !live {
		return paletteHint
	}
	dot := accent.Render("●")
	return paletteHint + "   " + dot + " " + muted.Render("live")
}

// nsChip renders a colored namespace chip — the visual anchor of the modern
// shell. Color is derived from theme.NSColor (per-namespace palette) so
// users develop muscle memory between color and scope.
func nsChip(ns string) string {
	ns = strings.TrimSpace(ns)
	if ns == "" || ns == "all" {
		return lipgloss.NewStyle().
			Foreground(theme.ColorMuted).
			Bold(true).
			Render("▆ all namespaces")
	}
	return components.NSChipBold(ns)
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

// truncToWidth clamps a styled string to n display columns by trimming bytes
// from the right. Used as a fallback when the bar is too narrow for full layout.
func truncToWidth(s string, n int) string {
	if lipgloss.Width(s) <= n {
		return s
	}
	for s != "" && lipgloss.Width(s) > n {
		s = s[:len(s)-1]
	}
	return s
}
