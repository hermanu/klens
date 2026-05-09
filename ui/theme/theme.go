package theme

import "github.com/charmbracelet/lipgloss"

// Color tokens — directly from modern.jsx tone{}
var (
	ColorBG     = lipgloss.Color("#0a0a0b")
	ColorPanel  = lipgloss.Color("#0d0d0f")
	ColorRaised = lipgloss.Color("#111114")
	ColorHover  = lipgloss.Color("#15151a")
	ColorSel    = lipgloss.Color("#1a1a22")
	ColorFG     = lipgloss.Color("#ebe8e1")
	ColorMid    = lipgloss.Color("#9a9aa0")
	ColorDim    = lipgloss.Color("#5b5b62")
	ColorFaint  = lipgloss.Color("#3a3a40")
	ColorAccent = lipgloss.Color("#e85a4f")
	ColorWarn   = lipgloss.Color("#c9a96a")
	ColorInfo   = lipgloss.Color("#7d8d9a")
)

// K8s status colors
var StatusColor = map[string]lipgloss.Color{
	"Running":          lipgloss.Color("#a3c08a"),
	"Pending":          ColorWarn,
	"Error":            ColorAccent,
	"CrashLoopBackOff": ColorAccent,
	"OOMKilled":        ColorAccent,
	"Terminating":      ColorDim,
	"Completed":        ColorDim,
	"Unknown":          ColorFaint,
}

func StatusColorFor(phase string) lipgloss.Color {
	if c, ok := StatusColor[phase]; ok {
		return c
	}
	return ColorFaint
}

// StatusGlyph maps k8s phase to the single-char glyph used in the severity column.
var StatusGlyph = map[string]string{
	"Running":          "›",
	"Pending":          "!",
	"Error":            "✕",
	"CrashLoopBackOff": "✕",
	"OOMKilled":        "✕",
	"Terminating":      "·",
	"Completed":        "·",
}

func GlyphFor(phase string) string {
	if g, ok := StatusGlyph[phase]; ok {
		return g
	}
	return "·"
}

// Base styles
var (
	Base = lipgloss.NewStyle().
		Background(ColorBG).
		Foreground(ColorFG)

	Panel = lipgloss.NewStyle().
		Background(ColorPanel).
		Foreground(ColorFG)

	Raised = lipgloss.NewStyle().
		Background(ColorRaised).
		Foreground(ColorFG)

	Selected = lipgloss.NewStyle().
		Background(ColorSel).
		Foreground(ColorFG)

	Dim = lipgloss.NewStyle().
		Foreground(ColorDim)

	Faint = lipgloss.NewStyle().
		Foreground(ColorFaint)

	Mid = lipgloss.NewStyle().
		Foreground(ColorMid)

	Accent = lipgloss.NewStyle().
		Foreground(ColorAccent)

	// Key chip style — used in status bar
	KeyChip = lipgloss.NewStyle().
		Foreground(ColorMid).
		Background(ColorRaised).
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorFaint).
		PaddingLeft(1).PaddingRight(1)

	// Filter chip (inactive)
	Chip = lipgloss.NewStyle().
		Foreground(ColorFG).
		Background(ColorRaised).
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorFaint).
		PaddingLeft(1).PaddingRight(1)

	// Filter chip (active/strong — e.g. error-level filter)
	ChipStrong = lipgloss.NewStyle().
		Foreground(ColorAccent).
		Background(lipgloss.Color("#1f0a09")).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#6b1e1b")).
		PaddingLeft(1).PaddingRight(1)

	// Section header (UPPERCASE, faint, letter-spaced)
	SectionLabel = lipgloss.NewStyle().
		Foreground(ColorFaint)

	// Table column header
	ColHeader = lipgloss.NewStyle().
		Foreground(ColorFaint)
)

// Divider renders a horizontal separator using faint color.
func Divider(width int) string {
	return lipgloss.NewStyle().
		Foreground(ColorFaint).
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
