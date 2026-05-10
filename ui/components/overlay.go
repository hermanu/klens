package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Overlay paints `top` over `base` at visual position (col, row), preserving
// ANSI escape codes on both. `base` and `top` are multi-line strings; rows
// are line indices (0-based) and cols are visible cells via lipgloss.Width.
//
// Lipgloss has no native cell-coordinate overlay — `Place` blanks the
// background. We need an actual overlay so the palette / help modal can
// float above the live table without losing context. This implementation
// walks each base line, slicing it by visible-cell width via ansi.Truncate
// and ansi.TruncateLeft so escape sequences carry through correctly.
func Overlay(base, top string, col, row int) string {
	if top == "" {
		return base
	}
	baseLines := strings.Split(base, "\n")
	topLines := strings.Split(top, "\n")

	modalW := 0
	for _, l := range topLines {
		if w := lipgloss.Width(l); w > modalW {
			modalW = w
		}
	}

	for i, topLine := range topLines {
		bi := row + i
		if bi < 0 || bi >= len(baseLines) {
			continue
		}
		baseLine := baseLines[bi]
		baseW := lipgloss.Width(baseLine)

		// Pad the base line to at least col+modalW cells so left/right slicing
		// is well-defined when the modal extends past the line's content.
		if baseW < col+modalW {
			baseLine += strings.Repeat(" ", col+modalW-baseW)
		}

		left := ansi.Truncate(baseLine, col, "")
		right := ansi.TruncateLeft(baseLine, col+modalW, "")

		// Pad the topLine to modalW so a shorter modal row still occupies the
		// full reserved width — otherwise the right slice would shift left.
		if w := lipgloss.Width(topLine); w < modalW {
			topLine += strings.Repeat(" ", modalW-w)
		}
		baseLines[bi] = left + topLine + right
	}
	return strings.Join(baseLines, "\n")
}
