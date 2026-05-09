package layout_test

import (
	"strings"
	"testing"

	"github.com/hermanu/klens/ui/layout"
)

func sampleNavItems() []layout.NavItem {
	return []layout.NavItem{
		{Key: "pods", Label: "Pods", Mnemonic: "1", Count: 23},
		{Key: "deployments", Label: "Deployments", Mnemonic: "2", Count: 14},
		{Key: "services", Label: "Services", Mnemonic: "3", Count: 12},
		{Key: "secrets", Label: "Secrets", Mnemonic: "4", Count: 8},
		{Key: "configmaps", Label: "ConfigMaps", Mnemonic: "5", Count: 6},
		{Key: "namespaces", Label: "Namespaces", Mnemonic: "6", Count: 9},
		{Key: "nodes", Label: "Nodes", Mnemonic: "7", Count: 4},
		{Key: "pvcs", Label: "PVCs", Mnemonic: "8", Count: 7},
	}
}

// TestNavStrip_RendersItemsWithCounts asserts every item appears with its
// total at a wide-enough width.
func TestNavStrip_RendersItemsWithCounts(t *testing.T) {
	got := stripANSI(layout.NavStrip(180, layout.NavStripConfig{
		Items:        sampleNavItems(),
		Current:      "pods",
		VisibleCount: 23,
		TotalCount:   23,
	}))
	wants := []string{"pods", "23", "deployments", "14", "services", "12", "secrets", "8"}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("want substring %q in NavStrip, got %q", w, got)
		}
	}
}

// TestNavStrip_BracketedMnemonics asserts every mnemonic is wrapped in
// `[N]` brackets so the key glyph reads as a key and not as a number.
func TestNavStrip_BracketedMnemonics(t *testing.T) {
	got := stripANSI(layout.NavStrip(180, layout.NavStripConfig{
		Items:        sampleNavItems(),
		Current:      "pods",
		VisibleCount: 23,
		TotalCount:   23,
	}))
	for _, m := range []string{"[1]", "[2]", "[3]", "[4]", "[5]", "[6]", "[7]", "[8]"} {
		if !strings.Contains(got, m) {
			t.Errorf("expected bracketed mnemonic %q in NavStrip, got %q", m, got)
		}
	}
	if !strings.Contains(got, "[1] pods") {
		t.Errorf("active item should render `[1] pods`, got %q", got)
	}
}

// TestNavStrip_FilteredCountOnActive asserts the active item shows V/T when
// filtered, and just T when unfiltered. Inactive items always show T.
func TestNavStrip_FilteredCountOnActive(t *testing.T) {
	t.Run("unfiltered active shows total only", func(t *testing.T) {
		got := stripANSI(layout.NavStrip(180, layout.NavStripConfig{
			Items:        sampleNavItems(),
			Current:      "pods",
			VisibleCount: 23,
			TotalCount:   23,
		}))
		if strings.Contains(got, "/23") {
			t.Errorf("unfiltered active item must not render `/T`, got %q", got)
		}
	})
	t.Run("filtered active shows V/T", func(t *testing.T) {
		got := stripANSI(layout.NavStrip(180, layout.NavStripConfig{
			Items:        sampleNavItems(),
			Current:      "pods",
			VisibleCount: 4,
			TotalCount:   23,
		}))
		if !strings.Contains(got, "4/23") {
			t.Errorf("filtered active item should render `4/23`, got %q", got)
		}
	})
}

// TestNavStrip_NarrowFallbackHidesLabels asserts that at narrow widths the
// strip drops labels and shows mnemonics only — so the keymap stays visible
// even on a 60-col terminal.
func TestNavStrip_NarrowFallbackHidesLabels(t *testing.T) {
	got := stripANSI(layout.NavStrip(40, layout.NavStripConfig{
		Items:        sampleNavItems(),
		Current:      "pods",
		VisibleCount: 23,
		TotalCount:   23,
	}))
	if strings.Contains(got, "deployments") {
		t.Errorf("narrow width should drop labels, got %q", got)
	}
	// All eight mnemonics must still be present.
	for _, m := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
		if !strings.Contains(got, m) {
			t.Errorf("narrow width: mnemonic %q missing, got %q", m, got)
		}
	}
}

// TestNavStrip_SingleRow asserts the strip is exactly one row tall.
func TestNavStrip_SingleRow(t *testing.T) {
	for _, w := range []int{40, 80, 120, 200} {
		got := layout.NavStrip(w, layout.NavStripConfig{
			Items:        sampleNavItems(),
			Current:      "pods",
			VisibleCount: 23,
			TotalCount:   23,
		})
		lines := strings.Split(got, "\n")
		if len(lines) != 1 {
			t.Errorf("NavStrip(width=%d): want 1 row, got %d", w, len(lines))
		}
	}
}
