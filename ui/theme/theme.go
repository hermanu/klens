package theme

import "github.com/charmbracelet/lipgloss"

// Color tokens — v3 "ANSI-faithful" palette. Constrained 16-color set that
// renders identically on terminals that don't support truecolor and looks
// faithful to the v3 design on those that do. Names preserved for source-
// compat with consumers; only hex values change.
var (
	ColorBG = lipgloss.Color("#0c0c0c")
	// ColorPanel intentionally equals ColorBG — separation comes from
	// borders, not background tint.
	ColorPanel = lipgloss.Color("#0c0c0c")
	// ColorRaised intentionally equals ColorBG — the v3 palette flattens all
	// surface tones; raised/panel/bg all share a single black.
	ColorRaised = lipgloss.Color("#0c0c0c")
	ColorHover  = lipgloss.Color("#1c1c1c")
	// Selection / alt-row backgrounds equal the body background — selection
	// is signalled by the accent left bar `▌` + cursor glyph `›`, not by row
	// tint. Keeps the table reading as fully black with the cursor jumping.
	ColorSel         = lipgloss.Color("#0c0c0c")
	ColorRowAlt      = lipgloss.Color("#0c0c0c")
	ColorBorder      = lipgloss.Color("#3a3a3a") // dimmer in design
	ColorBorderFaint = lipgloss.Color("#1c1c1c")
	ColorFG          = lipgloss.Color("#cccccc")
	// ColorFG2 intentionally equals ColorFG — the v3 palette uses a single
	// foreground tone. Kept as a distinct token for source-compat with the
	// 5 call sites that already address it by name.
	ColorFG2       = lipgloss.Color("#cccccc")
	ColorMid       = lipgloss.Color("#6a6a6a")
	ColorMuted     = lipgloss.Color("#6a6a6a") // "dim" in design
	ColorMuted2    = lipgloss.Color("#3a3a3a") // "dimmer" in design
	ColorDim       = lipgloss.Color("#6a6a6a")
	ColorFaint     = lipgloss.Color("#3a3a3a")
	ColorAccent    = lipgloss.Color("#70c0b1") // bright-cyan — primary accent
	ColorAccentDim = lipgloss.Color("#14304a") // sel-bg in design
	ColorWarn      = lipgloss.Color("#e7c547") // bright-yellow
	ColorError     = lipgloss.Color("#d54e53") // bright-red
	ColorOk        = lipgloss.Color("#b9ca4a") // bright-green
	ColorInfo      = lipgloss.Color("#7aa6da") // bright-blue
)

// NSColor maps namespace name → color. Drawn from the v3 ANSI palette so a
// single status-pill glance reads as the same hue across the screen.
var NSColor = map[string]lipgloss.Color{
	"kube-system": lipgloss.Color("#6a6a6a"),
	"default":     lipgloss.Color("#7aa6da"),
	"monitoring":  lipgloss.Color("#b9ca4a"),
	"ingress":     lipgloss.Color("#c397d8"),
	"data":        lipgloss.Color("#e7c547"),
	"platform":    lipgloss.Color("#70c0b1"),
	"argocd":      lipgloss.Color("#d54e53"),
}

// NSColorFor returns the chip color for a namespace, falling back to ColorMid.
func NSColorFor(ns string) lipgloss.Color {
	if c, ok := NSColor[ns]; ok {
		return c
	}
	return ColorMid
}

// NSColorAny returns a stable color for any string (typically a pod name) by
// hashing it into the namespace palette. Used by the multi-pod log tail to
// give every pod a distinct, repeatable per-line tag color so adjacent lines
// from different pods are visually separable. Same input → same color across
// renders so users can build muscle memory.
//
// Empty strings fall back to ColorMid to keep the renderer well-defined.
func NSColorAny(s string) lipgloss.Color {
	if s == "" {
		return ColorMid
	}
	// FNV-1a 32-bit hash, inlined to avoid pulling in hash/fnv for one call site.
	const offset = 2166136261
	const prime = 16777619
	h := uint32(offset)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime
	}
	// Iterate the palette in a deterministic order — Go map ranges are
	// randomised, so build the slice once and pick by index.
	palette := nsPaletteOrdered()
	return palette[int(h)%len(palette)]
}

// nsPaletteOrdered returns the per-namespace palette as a deterministic slice.
// Cached on first call so NSColorAny stays cheap on busy log tails.
var nsPaletteCache []lipgloss.Color

func nsPaletteOrdered() []lipgloss.Color {
	if nsPaletteCache != nil {
		return nsPaletteCache
	}
	// Sorted by namespace key for stability — adding a new entry to NSColor
	// never reshuffles the existing pod→color mappings unless it lands in the
	// middle of the alphabetical ordering, which is the cheapest invariant
	// available without a separate ordered palette.
	keys := make([]string, 0, len(NSColor))
	for k := range NSColor {
		keys = append(keys, k)
	}
	// Tiny insertion sort — len ~7, sort.Strings would also work but pulls
	// the sort package import for one-off use.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	out := make([]lipgloss.Color, 0, len(keys))
	for _, k := range keys {
		out = append(out, NSColor[k])
	}
	nsPaletteCache = out
	return out
}

// StatusStyle bundles the colors used by a status pill: the dot color, the
// text color, and the surrounding glow tint (used as cell background on the
// dot in TUI, since CSS box-shadow doesn't translate).
type StatusStyle struct {
	Dot  lipgloss.Color
	Text lipgloss.Color
	Glow lipgloss.Color
}

// StatusStyles maps k8s phase → StatusStyle. v3 ANSI palette; Glow field
// is retained for source compat but never rendered (terminals don't do
// soft shadows).
var StatusStyles = map[string]StatusStyle{
	"Running":          {Dot: lipgloss.Color("#b9ca4a"), Text: lipgloss.Color("#b9ca4a"), Glow: lipgloss.Color("#14304a")},
	"Pending":          {Dot: lipgloss.Color("#e7c547"), Text: lipgloss.Color("#e7c547"), Glow: lipgloss.Color("#14304a")},
	"CrashLoopBackOff": {Dot: lipgloss.Color("#d54e53"), Text: lipgloss.Color("#d54e53"), Glow: lipgloss.Color("#14304a")},
	"ImagePullBackOff": {Dot: lipgloss.Color("#d54e53"), Text: lipgloss.Color("#d54e53"), Glow: lipgloss.Color("#14304a")},
	"Error":            {Dot: lipgloss.Color("#d54e53"), Text: lipgloss.Color("#d54e53"), Glow: lipgloss.Color("#14304a")},
	"OOMKilled":        {Dot: lipgloss.Color("#d54e53"), Text: lipgloss.Color("#d54e53"), Glow: lipgloss.Color("#14304a")},
	"Terminating":      {Dot: lipgloss.Color("#6a6a6a"), Text: lipgloss.Color("#cccccc"), Glow: lipgloss.Color("#14304a")},
	"Completed":        {Dot: lipgloss.Color("#7aa6da"), Text: lipgloss.Color("#7aa6da"), Glow: lipgloss.Color("#14304a")},
	"Unknown":          {Dot: ColorFaint, Text: ColorMuted, Glow: ColorBorderFaint},
}

// StatusStyleFor returns the StatusStyle for a phase, falling back to Unknown.
func StatusStyleFor(phase string) StatusStyle {
	if s, ok := StatusStyles[phase]; ok {
		return s
	}
	return StatusStyles["Unknown"]
}

// StatusColor — backward-compat shim for callers that only want the dot color.
var StatusColor = func() map[string]lipgloss.Color {
	m := make(map[string]lipgloss.Color, len(StatusStyles))
	for k, v := range StatusStyles {
		m[k] = v.Dot
	}
	return m
}()

// StatusColorFor returns the accent color associated with the given k8s phase string.
func StatusColorFor(phase string) lipgloss.Color {
	return StatusStyleFor(phase).Dot
}

// StatusGlyph maps k8s phase to the single-char glyph used in legacy renderers.
// New code prefers the StatusPill component (● + colored name).
var StatusGlyph = map[string]string{
	"Running":          "●",
	"Pending":          "●",
	"Error":            "●",
	"CrashLoopBackOff": "●",
	"OOMKilled":        "●",
	"Terminating":      "●",
	"Completed":        "●",
}

// GlyphFor returns the single-character status glyph for phase, falling back to ● for unknown phases.
func GlyphFor(phase string) string {
	if g, ok := StatusGlyph[phase]; ok {
		return g
	}
	return "●"
}

// Base styles — surface tints intentionally inherit the terminal's default
// background. Setting an explicit Background(#06080b) makes panes look
// slightly grey on terminals whose own black is darker than #06080b, which
// is exactly the inconsistency users hit.
var (
	Base = lipgloss.NewStyle().
		Foreground(ColorFG)

	Panel = lipgloss.NewStyle().
		Foreground(ColorFG)

	Raised = lipgloss.NewStyle().
		Foreground(ColorFG)

	Selected = lipgloss.NewStyle().
			Foreground(ColorFG)

	Dim = lipgloss.NewStyle().
		Foreground(ColorMuted)

	Faint = lipgloss.NewStyle().
		Foreground(ColorMuted2)

	Mid = lipgloss.NewStyle().
		Foreground(ColorMid)

	Accent = lipgloss.NewStyle().
		Foreground(ColorAccent)

	Warn = lipgloss.NewStyle().
		Foreground(ColorWarn)

	Error = lipgloss.NewStyle().
		Foreground(ColorError)

	Ok = lipgloss.NewStyle().
		Foreground(ColorOk)

	// Key chip — single-row glyph for the bottom command bar. No background
	// (keeps the bar fully black) and no lipgloss border (a NormalBorder is
	// 3 rows tall and would stack chips vertically).
	KeyChip = lipgloss.NewStyle().
		Foreground(ColorFG2).
		PaddingLeft(1).PaddingRight(1)

	// Filter chip (inactive) — single-row "ns : all ×" pill above the table.
	// Transparent background — separation comes from spacing, not fill.
	Chip = lipgloss.NewStyle().
		Foreground(ColorFG).
		PaddingLeft(1).PaddingRight(1)

	// Filter chip (active/strong) — accent foreground; still no background
	// so the chip strip stays visually flat.
	ChipStrong = lipgloss.NewStyle().
			Foreground(ColorAccent).
			PaddingLeft(1).PaddingRight(1)

	// Section header — small, faint, uppercased outside this style.
	SectionLabel = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Table column header.
	ColHeader = lipgloss.NewStyle().
			Foreground(ColorMuted)
)

// Divider renders a horizontal separator using the border color.
func Divider(width int) string {
	return lipgloss.NewStyle().
		Foreground(ColorBorder).
		Render(repeatStr("─", width))
}

func repeatStr(s string, n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}
