package components_test

import (
	"strings"
	"testing"

	"github.com/hermanu/klens/ui/components"
)

// TestHelp_Smoke renders the overlay for a small key list and asserts the
// title, the key glyphs, and the "soon" tag for unwired keys all appear
// somewhere in the output. We only check substrings — the overlay's
// box/padding ANSI is intentionally not part of the contract.
func TestHelp_Smoke(t *testing.T) {
	got := components.Help(80, 30, "pods", []components.KeySpec{
		{Key: "j", Label: "down"},
		{Key: "y", Label: "yaml", Soon: true},
	})

	for _, want := range []string{"pods — keys", "j", "y", "soon"} {
		if !strings.Contains(got, want) {
			t.Errorf("want overlay to contain %q, got %q", want, got)
		}
	}
}
