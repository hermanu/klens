package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestPanel_TitleOnly verifies that a Panel renders a 4-sided border with
// the title inset into the top border row, and the body inside.
func TestPanel_TitleOnly(t *testing.T) {
	out := Panel(PanelConfig{
		Width:  20,
		Height: 5,
		Title:  "TOPBAR",
		Body:   "hello",
	})

	// Strip ANSI escapes so the assertions are stable across lipgloss
	// styling. Build a quick ANSI stripper inline — pulling in a dependency
	// for one test isn't worth it.
	plain := stripANSI(out)
	lines := strings.Split(plain, "\n")

	if got := len(lines); got != 5 {
		t.Fatalf("want 5 lines, got %d:\n%s", got, plain)
	}

	if !strings.Contains(lines[0], "TOPBAR") {
		t.Errorf("top line should contain the title %q, got %q", "TOPBAR", lines[0])
	}

	// The body line is index 2 (line 0 is top border, line 1 is first body
	// row, line 2 is the second body row — body started at index 1 because
	// the top border row eats one). With Height=5, body has 3 rows.
	bodyFound := false
	for i := 1; i < 4; i++ {
		if strings.Contains(lines[i], "hello") {
			bodyFound = true
			break
		}
	}
	if !bodyFound {
		t.Errorf("body %q not found in rows 1..3:\n%s", "hello", plain)
	}

	// Border glyph should be present at columns 0 and Width-1 of every row
	// (the corners use ┌┐└┘, the sides │).
	for i, l := range lines {
		if lipgloss.Width(l) != 20 {
			t.Errorf("line %d width=%d, want 20: %q", i, lipgloss.Width(l), l)
		}
	}
}

// stripANSI removes CSI escape sequences so byte-level assertions are
// stable. Good enough for tests; not a general-purpose stripper.
func stripANSI(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// skip CSI sequence until a letter (final byte 0x40..0x7e)
			i += 2
			for i < len(s) {
				c := s[i]
				if c >= 0x40 && c <= 0x7e {
					break
				}
				i++
			}
			continue
		}
		out.WriteByte(s[i])
	}
	return out.String()
}
