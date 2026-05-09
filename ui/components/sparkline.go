package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// blocks are the 8 Unicode block-elevation glyphs U+2581..U+2588.
// Index 0 is the lowest bar, index 7 the full block.
var blocks = [8]rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders samples as a Unicode-block sparkline of the given character
// width, colored with `color`. Returns "" for empty samples.
func Sparkline(samples []float64, width int, color lipgloss.Color) string {
	if len(samples) == 0 || width <= 0 {
		return ""
	}

	// Samples are pre-normalised to 0..100 by the caller (matches design).
	const minV, maxV = 0.0, 100.0
	span := maxV - minV

	// Downsample by tail-truncation; the freshest data wins on the right.
	if len(samples) > width {
		samples = samples[len(samples)-width:]
	}

	var b strings.Builder
	if pad := width - len(samples); pad > 0 {
		// Left-pad with the lowest block in a faint color so the bar stays
		// right-aligned without an empty/jagged left edge.
		b.WriteString(lipgloss.NewStyle().Foreground(theme.ColorFaint).Render(strings.Repeat(string(blocks[0]), pad)))
	}

	style := lipgloss.NewStyle().Foreground(color)
	var bar strings.Builder
	for _, s := range samples {
		switch {
		case s <= minV:
			bar.WriteRune(blocks[0])
		case s >= maxV:
			bar.WriteRune(blocks[len(blocks)-1])
		default:
			idx := int(((s - minV) / span) * float64(len(blocks)-1))
			if idx < 0 {
				idx = 0
			} else if idx >= len(blocks) {
				idx = len(blocks) - 1
			}
			bar.WriteRune(blocks[idx])
		}
	}
	b.WriteString(style.Render(bar.String()))
	return b.String()
}
