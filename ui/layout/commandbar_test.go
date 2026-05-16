package layout_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/layout"
)

func TestCommandBar_RendersPromptInputAndHints(t *testing.T) {
	hints := []layout.KeyHint{
		{Key: "↵", Label: "describe"},
		{Key: "l", Label: "logs"},
		{Key: "s", Label: "shell"},
	}
	got := layout.CommandBar(120, "filter pods, e.g. ns:platform", hints)
	wants := []string{
		"›", "/",
		"filter pods", "ns:platform",
		"↵", "describe",
		"l", "logs",
		"s", "shell",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("want substring %q in CommandBar, got %q", w, got)
		}
	}
}

func TestCommandBar_WidthClamp(t *testing.T) {
	hints := []layout.KeyHint{
		{Key: "↵", Label: "describe"},
		{Key: "l", Label: "logs"},
	}
	for _, w := range []int{40, 80, 160} {
		got := layout.CommandBar(w, "x", hints)
		if lipgloss.Width(got) > w {
			t.Errorf("width=%d exceeded: got %d (%q)", w, lipgloss.Width(got), got)
		}
	}
}

func TestCommandBar_PromptUsesAccent(t *testing.T) {
	got := layout.CommandBar(80, "", nil)
	want := sgrFor(t, "#70c0b1") // theme.ColorAccent (v3 ANSI palette)
	if !strings.Contains(got, want) {
		t.Errorf("'›' prompt must render in accent color SGR %q, got %q", want, got)
	}
}

func TestCommandBar_NoHintsRendersJustInput(t *testing.T) {
	got := layout.CommandBar(60, "search…", nil)
	if !strings.Contains(got, "search…") {
		t.Errorf("input view must be embedded verbatim, got %q", got)
	}
}
