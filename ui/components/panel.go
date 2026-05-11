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
	// Active, when true, swaps the border color from ColorBorder to ColorAccent
	// and bolds the title's first stylable segment. Caller pre-styles the title
	// for the inactive case; active swaps to accent on the same string.
	Active bool
	// Body is a pre-rendered multi-line string. The caller is responsible for
	// fitting it within (Width-2) x (Height-2). Excess is clipped at render
	// time; shortfall is padded with blank rows.
	Body string
}

// Panel renders cfg.Body wrapped in a border with cfg.Title inset into the
// top border row and cfg.Foot inset into the bottom border row.
func Panel(cfg PanelConfig) string {
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

	// Lipgloss's NormalBorder gives us ┌─┐│└┘ — exactly the glyph set the
	// inset overlay expects. Padding 0 keeps the inner area edge-to-edge so
	// the body is the caller's exact (Width-2) x (Height-2) canvas.
	frame := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		Width(cfg.Width - 2).
		Height(cfg.Height - 2).
		Render(cfg.Body)

	if cfg.Title != "" {
		titleInset := insetWrap(cfg.Title)
		frame = Overlay(frame, titleInset, 2, 0)
	}
	if cfg.Foot != "" {
		footInset := insetWrap(cfg.Foot)
		footCol := cfg.Width - 2 - lipgloss.Width(footInset)
		if footCol < 2 {
			footCol = 2
		}
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
