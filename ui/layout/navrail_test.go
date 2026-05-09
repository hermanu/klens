package layout_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/layout"
)

func sampleNavItems() []layout.NavItem {
	return []layout.NavItem{
		{Key: "pods", Label: "Pods", Mnemonic: "1", Count: 12},
		{Key: "deployments", Label: "Deployments", Mnemonic: "2", Count: 18},
		{Key: "services", Label: "Services", Mnemonic: "3", Count: 24},
	}
}

// fullNavItems mirrors what app.go feeds the rail in production: 5
// namespaced resources followed by cluster-scoped ones. Used to assert the
// hairline divider between row 5 and row 6.
func fullNavItems() []layout.NavItem {
	return []layout.NavItem{
		{Key: "pods", Label: "Pods", Mnemonic: "1", Count: 12},
		{Key: "deployments", Label: "Deployments", Mnemonic: "2", Count: 18},
		{Key: "services", Label: "Services", Mnemonic: "3", Count: 24},
		{Key: "secrets", Label: "Secrets", Mnemonic: "4", Count: 6},
		{Key: "configmaps", Label: "ConfigMaps", Mnemonic: "5", Count: 9},
		{Key: "namespaces", Label: "Namespaces", Mnemonic: "6", Count: 11},
		{Key: "nodes", Label: "Nodes", Mnemonic: "7", Count: 9},
	}
}

func sampleMeta() layout.ClusterMeta {
	return layout.ClusterMeta{
		NodesReady: 9, NodesTotal: 9,
		CPUPercent: 62, MemPercent: 78,
		Pods: 47, PodsCap: 250,
	}
}

func TestNavRail_RendersResourcesAndCounts(t *testing.T) {
	got := layout.NavRail(22, 30, "pods", sampleNavItems(), sampleMeta())
	wants := []string{
		// "RESOURCES" header was dropped — first row is the items list itself.
		"Pods", "Deployments", "Services",
		"1", "2", "3",
		"12", "18", "24",
		"cluster",
		"nodes", "9 ready",
		"cpu", "62%",
		"mem", "78%",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("want substring %q in NavRail output, got %q", w, got)
		}
	}
}

// TestNavRail_FooterDropsPodsTotal asserts the misleading "pods X/Y" footer
// row was removed. PodsCap was always equal to Pods in production, so the
// fraction read as noise; the row is gone entirely.
func TestNavRail_FooterDropsPodsTotal(t *testing.T) {
	got := layout.NavRail(22, 30, "pods", sampleNavItems(), sampleMeta())
	// "47/250" was the sample meta's Pods/PodsCap — the row that rendered it.
	if strings.Contains(got, "47/250") {
		t.Errorf("footer must not contain pods cap fraction, got %q", got)
	}
}

// TestNavRail_HairlineDividerBetweenGroups asserts a hairline row separates
// item 5 (the last namespaced resource in fullNavItems) from item 6 (the
// first cluster-scoped one). The divider uses '─'; we look for a row that
// is hairline-only (no labels, no digits) after ANSI is stripped — the
// raw output contains RGB codes whose digits would yield false positives.
func TestNavRail_HairlineDividerBetweenGroups(t *testing.T) {
	got := layout.NavRail(22, 40, "pods", fullNavItems(), sampleMeta())
	lines := strings.Split(got, "\n")

	findRow := func(label string) int {
		for i, l := range lines {
			if strings.Contains(l, label) {
				return i
			}
		}
		return -1
	}
	cm := findRow("ConfigMaps") // 5th item
	ns := findRow("Namespaces") // 6th item
	if cm < 0 || ns < 0 {
		t.Fatalf("expected ConfigMaps and Namespaces rows in output, got\n%s", got)
	}
	if ns != cm+2 {
		t.Errorf("expected exactly one divider row between ConfigMaps (line %d) and Namespaces (line %d), output:\n%s", cm, ns, got)
	}
	divider := stripANSI(lines[cm+1])
	if !strings.Contains(divider, "─") {
		t.Errorf("divider row should contain '─', got %q", divider)
	}
	if strings.ContainsAny(divider, "0123456789") {
		t.Errorf("divider row should be hairline-only (no digits), got %q", divider)
	}
}

// TestNavRail_CPUMemDashWhenZero asserts that 0% renders as a faint em-dash
// rather than literal "0%" — the latter reads as "metrics-server says 0",
// which is wrong; "—" reads as "no data".
func TestNavRail_CPUMemDashWhenZero(t *testing.T) {
	meta := sampleMeta()
	meta.CPUPercent = 0
	meta.MemPercent = 0
	got := layout.NavRail(22, 30, "pods", sampleNavItems(), meta)
	if strings.Contains(got, "0%") {
		t.Errorf("zero CPU/Mem must render as '—', not '0%%', got %q", got)
	}
	if !strings.Contains(got, "—") {
		t.Errorf("zero CPU/Mem must render as '—', got %q", got)
	}
}

func TestNavRail_WidthClamp(t *testing.T) {
	got := layout.NavRail(20, 24, "pods", sampleNavItems(), sampleMeta())
	for _, line := range strings.Split(got, "\n") {
		if lipgloss.Width(line) > 20 {
			t.Errorf("NavRail line exceeds width 20: width=%d %q", lipgloss.Width(line), line)
		}
	}
}

func TestNavRail_ActiveRowUsesAccent(t *testing.T) {
	got := layout.NavRail(22, 24, "pods", sampleNavItems(), sampleMeta())
	want := sgrFor(t, "#7dd3fc") // theme.ColorAccent
	if !strings.Contains(got, want) {
		t.Errorf("active row must render in accent color SGR %q, got %q", want, got)
	}
}

func TestNavRail_CPUMemColorThresholds(t *testing.T) {
	cases := []struct {
		name     string
		cpu, mem int
		hex      string // expected color hex
	}{
		{"ok", 50, 60, "#a3e635"},
		{"warn", 75, 85, "#fbbf24"},
		{"err", 95, 92, "#fb7185"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			meta := sampleMeta()
			meta.CPUPercent = c.cpu
			meta.MemPercent = c.mem
			got := layout.NavRail(22, 30, "pods", sampleNavItems(), meta)
			want := sgrFor(t, c.hex)
			// Tolerate single-bit jitter in the blue channel: termenv occasionally
			// adjusts the LSB when colors are routed through a chained style
			// (Background+Foreground), so we match on R;G only.
			prefix := want[:strings.LastIndex(want, ";")]
			if !strings.Contains(got, prefix) {
				t.Errorf("want SGR prefix %q for cpu=%d mem=%d, got %q",
					prefix, c.cpu, c.mem, got)
			}
		})
	}
}
