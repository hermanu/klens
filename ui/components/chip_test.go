package components_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/theme"
	"github.com/muesli/termenv"
)

// TestMain forces TrueColor on the default lipgloss renderer so tests can
// assert on emitted ANSI escapes. Without this, `go test` runs without a TTY
// and lipgloss strips all color, making style assertions impossible.
func TestMain(m *testing.M) {
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

// sgrFor renders an arbitrary glyph with `hex` as foreground and extracts the
// SGR sequence lipgloss emitted. We assert against this exact substring rather
// than computing RGB ourselves — lipgloss/termenv occasionally adjusts color
// values when round-tripping through their internal color model.
var sgrRe = regexp.MustCompile(`\x1b\[(38;2;\d+;\d+;\d+)m`)

func sgrFor(t *testing.T, hex string) string {
	t.Helper()
	rendered := lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render("x")
	m := sgrRe.FindStringSubmatch(rendered)
	if m == nil {
		t.Fatalf("could not extract SGR for %s from %q", hex, rendered)
	}
	return m[1]
}

func TestNSChip_Known(t *testing.T) {
	got := components.NSChip("platform")
	if got == "" {
		t.Fatal("want non-empty NSChip output")
	}
	if !strings.Contains(got, "platform") {
		t.Errorf("want output to contain 'platform', got %q", got)
	}
	// The chip should encode the namespace's color (#22d3ee for platform).
	want := sgrFor(t, "#22d3ee")
	if !strings.Contains(got, want) {
		t.Errorf("want platform color SGR %q in output, got %q", want, got)
	}
}

func TestNSChip_EmptyFallback(t *testing.T) {
	// Must not panic and should still render something (faint placeholder).
	got := components.NSChip("")
	if got == "" {
		t.Error("want a faint placeholder for empty ns, got empty string")
	}
}

func TestStatusPill_Running(t *testing.T) {
	got := components.StatusPill("Running")
	if !strings.Contains(got, "Running") {
		t.Errorf("want output to contain 'Running', got %q", got)
	}
	// theme.StatusStyles["Running"].Dot is #a3e635 (green).
	want := sgrFor(t, "#a3e635")
	if !strings.Contains(got, want) {
		t.Errorf("want green dot SGR %q in output, got %q", want, got)
	}
}

func TestStatusPill_ImagePullBackOff(t *testing.T) {
	got := components.StatusPill("ImagePullBackOff")
	if !strings.Contains(got, "ImagePullBackOff") {
		t.Errorf("want output to contain 'ImagePullBackOff', got %q", got)
	}
	// theme.StatusStyles["ImagePullBackOff"].Dot is #fb7185 (red).
	want := sgrFor(t, "#fb7185")
	if !strings.Contains(got, want) {
		t.Errorf("want red dot SGR %q in output, got %q", want, got)
	}
}

func TestStatusPill_UnknownFallback(t *testing.T) {
	// Must not panic and should render the literal phase string.
	got := components.StatusPill("Mystery")
	if !strings.Contains(got, "Mystery") {
		t.Errorf("want output to contain the phase name, got %q", got)
	}
	// Fallback must be the Unknown style (Dot = ColorFaint = #3a3f4a).
	unknown := theme.StatusStyles["Unknown"]
	want := sgrFor(t, string(unknown.Dot))
	if !strings.Contains(got, want) {
		t.Errorf("want Unknown dot SGR %q in output, got %q", want, got)
	}
}

func TestStatusDot_NoText(t *testing.T) {
	got := components.StatusDot("Running")
	if strings.Contains(got, "Running") {
		t.Errorf("StatusDot must NOT include the phase name, got %q", got)
	}
	if !strings.Contains(got, "●") {
		t.Errorf("want ● glyph in output, got %q", got)
	}
}
