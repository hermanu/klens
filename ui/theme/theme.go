package theme

import "github.com/charmbracelet/lipgloss"

// Color tokens — the "midnight" palette from the Klens design (klens-data.jsx).
// Existing names kept as backward-compatible aliases where the role overlaps.
var (
	ColorBG = lipgloss.Color("#06080b")
	// ColorPanel intentionally equals ColorBG: in HTML, panel=#0e1116 reads
	// as a hair lighter than body=#06080b, but on most terminals that
	// renders as a visibly gray block. Using the same value everywhere
	// keeps the chrome unified — separation comes from divider lines and
	// border colors instead.
	ColorPanel  = lipgloss.Color("#06080b")
	ColorRaised = lipgloss.Color("#0b0d10")
	ColorHover       = lipgloss.Color("#15151a")
	// Selection and alt-row backgrounds equal the body background — separation
	// is by accent left bar + cursor glyph, not by row tint, so the table
	// reads as fully black instead of gray-striped.
	ColorSel    = lipgloss.Color("#06080b")
	ColorRowAlt = lipgloss.Color("#06080b")
	ColorBorder      = lipgloss.Color("#1f2a3a")
	ColorBorderFaint = lipgloss.Color("#11171f")
	ColorFG          = lipgloss.Color("#e5e7eb")
	ColorFG2         = lipgloss.Color("#cbd5e1")
	ColorMid         = lipgloss.Color("#94a3b8")
	ColorMuted       = lipgloss.Color("#64748b")
	ColorMuted2      = lipgloss.Color("#475569")
	ColorDim         = lipgloss.Color("#5b6478")
	ColorFaint       = lipgloss.Color("#3a3f4a")
	ColorAccent      = lipgloss.Color("#7dd3fc") // sky-300 — design's primary accent
	ColorAccentDim   = lipgloss.Color("#0c2738")
	ColorWarn        = lipgloss.Color("#fbbf24")
	ColorError       = lipgloss.Color("#fb7185")
	ColorOk          = lipgloss.Color("#a3e635")
	ColorInfo        = lipgloss.Color("#7d8d9a")
)

// NSColor maps namespace name → color. Mirrors the design's NS_COLORS map.
// The default fallback is ColorMid for unknown namespaces.
var NSColor = map[string]lipgloss.Color{
	"kube-system": lipgloss.Color("#94a3b8"),
	"default":     lipgloss.Color("#7dd3fc"),
	"monitoring":  lipgloss.Color("#a3e635"),
	"ingress":     lipgloss.Color("#f0abfc"),
	"data":        lipgloss.Color("#fbbf24"),
	"platform":    lipgloss.Color("#22d3ee"),
	"argocd":      lipgloss.Color("#fb7185"),
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

// StatusStyles maps k8s phase → StatusStyle. Mirrors the design's STATUS_COLORS.
var StatusStyles = map[string]StatusStyle{
	"Running":          {Dot: lipgloss.Color("#a3e635"), Text: lipgloss.Color("#bef264"), Glow: lipgloss.Color("#1a2410")},
	"Pending":          {Dot: lipgloss.Color("#fbbf24"), Text: lipgloss.Color("#fde68a"), Glow: lipgloss.Color("#241c08")},
	"CrashLoopBackOff": {Dot: lipgloss.Color("#f472b6"), Text: lipgloss.Color("#fbcfe8"), Glow: lipgloss.Color("#2a1525")},
	"ImagePullBackOff": {Dot: lipgloss.Color("#fb7185"), Text: lipgloss.Color("#fecdd3"), Glow: lipgloss.Color("#2a1014")},
	"Error":            {Dot: lipgloss.Color("#fb7185"), Text: lipgloss.Color("#fecdd3"), Glow: lipgloss.Color("#2a1014")},
	"OOMKilled":        {Dot: lipgloss.Color("#fb7185"), Text: lipgloss.Color("#fecdd3"), Glow: lipgloss.Color("#2a1014")},
	"Terminating":      {Dot: lipgloss.Color("#94a3b8"), Text: lipgloss.Color("#cbd5e1"), Glow: lipgloss.Color("#1a1f28")},
	"Completed":        {Dot: lipgloss.Color("#7dd3fc"), Text: lipgloss.Color("#bae6fd"), Glow: lipgloss.Color("#0c2230")},
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
