// Package components — Panel primitive.
//
// Panel wraps a pre-rendered body string in a 4-sided border and overlays
// a notched title (top-left) and optional foot (bottom-right) onto the
// border rows. Built on top of Overlay() so the inset preserves ANSI
// escapes on both the border and the inset string.
//
// The notch is a faithful render of the lazygit/btop/tmux pattern: the
// title appears to "sit on" the border, with the border glyphs on either
// side of it still visible.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// PanelConfig drives Panel rendering. All fields are optional except
// Width/Height/Body.
type PanelConfig struct {
	Width int
	// Height is the total outer height including the top and bottom border rows.
	Height int
	// Title is inset into the top border row, starting at column 2. Pre-styled
	// (caller decides foreground color, bold, etc.). The panel adds a single
	// dim border-glyph on each side so the inset reads as notched.
	Title string
	// Foot is inset into the bottom border row, right-aligned so it ends at
	// column Width-2. Same notching treatment as Title.
	Foot string
	// Active, when true, swaps the border color from ColorBorder to ColorAccent.
	// The title string is passed through unchanged — callers that want a
	// different title style when active should branch and pre-style accordingly
	// before constructing the PanelConfig.
	Active bool
	// TitleCenter, when true, positions the title centered on the top border
	// instead of the default top-left inset (col 2). Use for panels where the
	// title is the primary content anchor (e.g. the table's k9s-style
	// breadcrumb) so the eye lands on it naturally. Falls back to col 2 when
	// the title is too wide to center cleanly.
	TitleCenter bool
	// Body is a pre-rendered multi-line string. The caller is responsible for
	// fitting it within (Width-2) x (Height-2). Excess is clipped at render
	// time; shortfall is padded with blank rows.
	Body string
}

// Panel renders cfg.Body wrapped in a border with cfg.Title inset into the
// top border row and cfg.Foot inset into the bottom border row.
func Panel(cfg PanelConfig) string {
	// Width: 2 border cols + col-2 title-inset offset + at least 4 cols of
	// notch headroom = 8 cells minimum before the body is meaningfully
	// renderable. Height: 1 top border + 1 body row + 1 bottom border = 3.
	if cfg.Width < 8 {
		cfg.Width = 8
	}
	if cfg.Height < 3 {
		cfg.Height = 3
	}

	borderColor := theme.ColorBorder
	if cfg.Active {
		borderColor = theme.ColorAccent
	}

	// Hard-clamp body to exactly (Height-2) rows × (Width-2) cells. Lipgloss
	// `.Height(N)` pads but does NOT truncate; `.Width(N)` pads short rows
	// but WRAPS rows wider than N to multiple lines. Either overflow would
	// cause the rendered panel to exceed cfg.Width × cfg.Height, pushing
	// siblings (and the frame's bottom edge) off-screen. Clip both axes
	// here so the panel is always exactly cfg.Width × cfg.Height.
	innerH := cfg.Height - 2
	innerW := cfg.Width - 2
	bodyLines := strings.Split(cfg.Body, "\n")
	if len(bodyLines) > innerH {
		bodyLines = bodyLines[:innerH]
	}
	for i, line := range bodyLines {
		if lipgloss.Width(line) > innerW {
			bodyLines[i] = clipToWidth(line, innerW)
		}
	}
	clampedBody := strings.Join(bodyLines, "\n")

	// Lipgloss's NormalBorder gives us ┌─┐│└┘ — exactly the glyph set the
	// inset overlay expects. Padding 0 keeps the inner area edge-to-edge so
	// the body is the caller's exact (Width-2) x (Height-2) canvas.
	frame := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		Width(cfg.Width - 2).
		Height(innerH).
		Render(clampedBody)

	if cfg.Title != "" {
		titleInset := insetWrap(cfg.Title)
		col := 2
		if cfg.TitleCenter {
			// Center the title on the top border. Fall back to col 2 if the
			// title is too wide for the panel — better to inset than to clip.
			centered := (cfg.Width - lipgloss.Width(titleInset)) / 2
			if centered > col {
				col = centered
			}
		}
		frame = Overlay(frame, titleInset, col, 0)
	}
	if cfg.Foot != "" {
		footInset := insetWrap(cfg.Foot)
		footCol := max(cfg.Width-1-lipgloss.Width(footInset), 2)
		frame = Overlay(frame, footInset, footCol, cfg.Height-1)
	}

	return frame
}

// insetWrap wraps a title/foot string with a single space on each side so the
// inset reads as a notch against the surrounding border glyphs. The spaces
// are styled with the panel bg so they erase the border `─` chars beneath.
//
// Caller passes pre-styled text. The space-pad uses Panel bg so terminals
// without a "fill" color still mask the border underneath.
func insetWrap(s string) string {
	pad := lipgloss.NewStyle().Background(theme.ColorPanel).Render(" ")
	return pad + s + pad
}

// clipToWidth trims a styled string from the right until its display width
// is at most n. Preserves ANSI escape sequences naively: byte-trims, so a
// trailing partial CSI sequence may remain — harmless because terminals
// ignore unterminated CSI and the body will overflow into the border at
// most by an invisible byte run, never visibly. Used to ensure body rows
// never exceed Width-2 cells before lipgloss wraps them.
func clipToWidth(s string, n int) string {
	for s != "" && lipgloss.Width(s) > n {
		s = s[:len(s)-1]
	}
	return s
}
