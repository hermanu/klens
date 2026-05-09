package layout_test

import (
	"strings"
	"testing"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/layout"
)

// containsDigit reports whether s contains any decimal digit. Used to assert
// the filter strip no longer renders count numbers — those moved to the top
// bar's scope row.
func containsDigit(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// TestFilterChips_NoDigitsWhenEmpty asserts that with no chips the strip
// renders no count digits — the canonical filtered/total count was promoted
// to the top bar's scope row, so this strip should be number-free. ANSI is
// stripped before checking because RGB color codes contain digits like
// "38;2;125;211;252" that would yield false positives.
func TestFilterChips_NoDigitsWhenEmpty(t *testing.T) {
	got := stripANSI(layout.FilterChips(120, nil, 12, 12))
	if containsDigit(got) {
		t.Errorf("empty chips strip must contain no digits (count moved to top bar), got %q", got)
	}
	for _, banned := range []string{"matched", "showing"} {
		if strings.Contains(got, banned) {
			t.Errorf("empty chips strip must not contain %q (count moved to top bar), got %q", banned, got)
		}
	}
}

// TestFilterChips_RendersChipsWithoutCount asserts both chips render and that
// the legacy "matched"/"showing" status text is gone — the count moved to the
// top bar.
func TestFilterChips_RendersChipsWithoutCount(t *testing.T) {
	chips := []layout.FilterChip{
		{Key: "ns", Value: "platform"},
		{Key: "status", Value: "Error", Strong: true},
	}
	got := layout.FilterChips(120, chips, 3, 12)
	wants := []string{
		"ns", "platform",
		"status", "Error",
		"●", "watch",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("want substring %q, got %q", w, got)
		}
	}
	for _, banned := range []string{"matched", "showing"} {
		if strings.Contains(got, banned) {
			t.Errorf("filter strip must not contain %q (count moved to top bar), got %q", banned, got)
		}
	}
}

func TestFilterChips_WidthClamp(t *testing.T) {
	chips := []layout.FilterChip{
		{Key: "ns", Value: "platform"},
		{Key: "status", Value: "Running"},
		{Key: "node", Value: "ip-10-0-8-02"},
	}
	for _, w := range []int{80, 120, 200} {
		got := layout.FilterChips(w, chips, 4, 30)
		if lipgloss.Width(got) > w {
			t.Errorf("width=%d exceeded: got %d (%q)", w, lipgloss.Width(got), got)
		}
	}
}

func TestFilterChips_StrongChipUsesAccent(t *testing.T) {
	got := layout.FilterChips(120, []layout.FilterChip{{Key: "status", Value: "Error", Strong: true}}, 1, 10)
	want := sgrFor(t, "#7dd3fc") // theme.ColorAccent
	if !strings.Contains(got, want) {
		t.Errorf("strong chip must render in accent color SGR %q, got %q", want, got)
	}
}
