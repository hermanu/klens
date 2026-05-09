package layout_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestMain forces TrueColor on the default lipgloss renderer so tests can
// assert on emitted ANSI escapes. Without this, `go test` runs without a TTY
// and lipgloss strips all color, making style assertions impossible.
func TestMain(m *testing.M) {
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

// sgrRe extracts the foreground SGR fragment ("38;2;R;G;B") from rendered output.
var sgrRe = regexp.MustCompile(`\x1b\[(?:[\d;]+;)?38;2;(\d+;\d+;\d+)`)

// ansiRe matches any CSI escape sequence so tests can assert on the visible
// glyph stream without false positives from RGB color codes (which contain
// digits like "38;2;125;211;252").
var ansiRe = regexp.MustCompile(`\x1b\[[\d;]*[a-zA-Z]`)

// stripANSI returns s with all CSI escape sequences removed, leaving only the
// glyphs the user would see on screen.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// sgrFor renders an arbitrary glyph with `hex` as foreground and extracts the
// SGR sequence lipgloss emitted. We assert against this exact substring rather
// than computing RGB ourselves — lipgloss/termenv occasionally adjusts color
// values when round-tripping through their internal color model.
func sgrFor(t *testing.T, hex string) string {
	t.Helper()
	rendered := lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render("x")
	m := sgrRe.FindStringSubmatch(rendered)
	if m == nil {
		t.Fatalf("could not extract SGR for %s from %q", hex, rendered)
	}
	return m[1]
}
