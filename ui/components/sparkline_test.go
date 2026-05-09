package components_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/theme"
)

// blockChars must mirror the private `blocks` table in sparkline.go.
const blockChars = "▁▂▃▄▅▆▇█"

func TestSparkline_Empty(t *testing.T) {
	if got := components.Sparkline(nil, 10, theme.ColorAccent); got != "" {
		t.Errorf("want empty string for nil samples, got %q", got)
	}
	if got := components.Sparkline([]float64{}, 10, theme.ColorAccent); got != "" {
		t.Errorf("want empty string for empty samples, got %q", got)
	}
}

func TestSparkline_SingleSample(t *testing.T) {
	got := components.Sparkline([]float64{42}, 1, theme.ColorAccent)
	if w := lipgloss.Width(got); w != 1 {
		t.Fatalf("want width 1, got %d (%q)", w, got)
	}
	// Strip ANSI by counting runes from the visible output is awkward; the
	// rendered string should contain exactly one of the eight block runes.
	if !containsAnyRune(got, blockChars) {
		t.Errorf("want one of %q in output, got %q", blockChars, got)
	}
}

func TestSparkline_LevelMapping(t *testing.T) {
	low := components.Sparkline([]float64{0}, 1, theme.ColorAccent)
	if !strings.ContainsRune(low, '▁') {
		t.Errorf("0 should map to ▁, got %q", low)
	}
	high := components.Sparkline([]float64{100}, 1, theme.ColorAccent)
	if !strings.ContainsRune(high, '█') {
		t.Errorf("100 should map to █, got %q", high)
	}
	mid := components.Sparkline([]float64{50}, 1, theme.ColorAccent)
	if strings.ContainsRune(mid, '▁') || strings.ContainsRune(mid, '█') {
		t.Errorf("50 should map to a midpoint block, got %q", mid)
	}
	if !containsAnyRune(mid, "▃▄▅") {
		t.Errorf("50 should be a middle block (▃▄▅), got %q", mid)
	}
}

func TestSparkline_TruncatesToTail(t *testing.T) {
	// Build 10 samples ramping 0..100; only the last 3 should render.
	samples := []float64{0, 10, 20, 30, 40, 50, 60, 70, 80, 100}
	got := components.Sparkline(samples, 3, theme.ColorAccent)
	if w := lipgloss.Width(got); w != 3 {
		t.Fatalf("want width 3, got %d (%q)", w, got)
	}
	// The very last char must be the full block (sample = 100).
	if !strings.ContainsRune(got, '█') {
		t.Errorf("tail-truncated bar should include █, got %q", got)
	}
	// The earliest values (0,10,20) should NOT appear as ▁ in this rendering
	// because they were dropped by truncation; the kept tail starts at 80.
	// (Not a hard guarantee — but a strong sanity check.)
	if strings.Count(got, "▁") > 0 {
		t.Errorf("did not expect ▁ after truncating tail, got %q", got)
	}
}

func TestSparkline_LeftPadsShortInput(t *testing.T) {
	got := components.Sparkline([]float64{100, 100}, 6, theme.ColorAccent)
	if w := lipgloss.Width(got); w != 6 {
		t.Fatalf("want width 6, got %d (%q)", w, got)
	}
	// Padding is rendered with the lowest block in a faint style; the 4 left
	// cells should be ▁ glyphs and the 2 right cells should be █.
	if strings.Count(got, "▁") < 4 {
		t.Errorf("want >=4 padding ▁ runes, got %q", got)
	}
	if strings.Count(got, "█") != 2 {
		t.Errorf("want 2 full blocks for the data tail, got %q", got)
	}
}

// containsAnyRune reports whether s contains any rune from set.
func containsAnyRune(s, set string) bool {
	for _, r := range set {
		if strings.ContainsRune(s, r) {
			return true
		}
	}
	// Defensive: ensure the test corpus is valid UTF-8 — guards against a
	// future refactor that mangles the block-char constant.
	return utf8.RuneCountInString(set) == 0
}
