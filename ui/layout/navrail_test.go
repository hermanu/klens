package layout_test

import (
	"strings"
	"testing"

	"github.com/hermanu/klens/ui/layout"
)

func sampleRailItems() []layout.NavItem {
	return []layout.NavItem{
		{Key: "pods", Label: "Pods", Mnemonic: "1", Count: 56},
		{Key: "deployments", Label: "Deployments", Mnemonic: "2", Count: 22},
		{Key: "services", Label: "Services", Mnemonic: "3", Count: 23},
		{Key: "secrets", Label: "Secrets", Mnemonic: "4", Count: 14},
		{Key: "configmaps", Label: "ConfigMaps", Mnemonic: "5", Count: 6},
		{Key: "namespaces", Label: "Namespaces", Mnemonic: "6", Count: 40},
		{Key: "nodes", Label: "Nodes", Mnemonic: "7", Count: 9},
		{Key: "pvcs", Label: "PVCs", Mnemonic: "8", Count: 11},
	}
}

// TestNavRail_RendersOneRowPerItem asserts every item gets its own line and
// the totals are visible.
func TestNavRail_RendersOneRowPerItem(t *testing.T) {
	got := stripANSI(layout.NavRail(22, 8, layout.NavRailConfig{
		Items:        sampleRailItems(),
		Current:      "pods",
		VisibleCount: 56,
		TotalCount:   56,
	}))
	for _, want := range []string{"pods", "56", "deployments", "22", "services", "23"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in NavRail output:\n%s", want, got)
		}
	}
	lines := strings.Split(got, "\n")
	// At least len(items) rows.
	if len(lines) < 8 {
		t.Errorf("expected ≥8 rows, got %d:\n%s", len(lines), got)
	}
}

// TestNavRail_ActiveCursor asserts the active row gets the `▌` cursor and the
// V/T format kicks in when filtered.
func TestNavRail_ActiveCursor(t *testing.T) {
	got := stripANSI(layout.NavRail(22, 8, layout.NavRailConfig{
		Items:        sampleRailItems(),
		Current:      "pods",
		VisibleCount: 4,
		TotalCount:   56,
	}))
	if !strings.Contains(got, "▌") {
		t.Errorf("active row should render `▌` cursor, got:\n%s", got)
	}
	if !strings.Contains(got, "4/56") {
		t.Errorf("filtered active row should render `4/56`, got:\n%s", got)
	}
}

// TestNavRail_BracketedMnemonics asserts every mnemonic is wrapped in
// `[N]` brackets — same convention as the bottom command bar's chips.
func TestNavRail_BracketedMnemonics(t *testing.T) {
	got := stripANSI(layout.NavRail(22, 8, layout.NavRailConfig{
		Items:        sampleRailItems(),
		Current:      "pods",
		VisibleCount: 56,
		TotalCount:   56,
	}))
	for _, want := range []string{"[1]", "[2]", "[3]", "[4]", "[5]", "[6]", "[7]", "[8]"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected bracketed mnemonic %q, got:\n%s", want, got)
		}
	}
}
