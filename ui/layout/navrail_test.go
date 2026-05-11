package layout_test

import (
	"strings"
	"testing"

	"github.com/hermanu/klens/ui/layout"
)

func TestNavRail_DefaultActiveRow(t *testing.T) {
	out := layout.NavRail(22, 20, layout.NavRailConfig{
		Items: []layout.NavItem{
			{Mnemonic: "1", Label: "pods", Count: 23, Active: true},
			{Mnemonic: "2", Label: "deps", Count: 18},
			{Mnemonic: "3", Label: "svc", Count: 24},
		},
		Cluster: layout.ClusterMeta{
			NodesReady: 9,
			NodesTotal: 9,
			Pods:       25,
			CPUSamples: []float64{40, 52, 48, 60, 58, 62, 66, 70, 68, 62},
			CPUPercent: 62,
			MEMSamples: []float64{70, 72, 74, 76, 78, 76, 80, 78, 82, 78},
			MEMPercent: 78,
		},
	})

	plain := stripANSI(out)
	lines := strings.Split(plain, "\n")

	// Active row carries the ▌ glyph.
	if !strings.Contains(lines[0], "▌") || !strings.Contains(lines[0], "1") || !strings.Contains(lines[0], "pods") {
		t.Errorf("active row should contain '▌', '1', 'pods'; got %q", lines[0])
	}
	// Inactive row 2 has no ▌.
	if strings.Contains(lines[1], "▌") {
		t.Errorf("inactive row should not contain '▌'; got %q", lines[1])
	}
	// CLUSTER footer somewhere in the output.
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "CLUSTER") {
		t.Errorf("rail should render CLUSTER section; got\n%s", joined)
	}
	if !strings.Contains(joined, "nodes") || !strings.Contains(joined, "9/9") {
		t.Errorf("cluster footer should show nodes 9/9; got\n%s", joined)
	}
}

func TestNavRail_NoMetricsRendersDashes(t *testing.T) {
	out := layout.NavRail(22, 16, layout.NavRailConfig{
		Items: []layout.NavItem{{Mnemonic: "1", Label: "pods", Active: true}},
		Cluster: layout.ClusterMeta{
			NodesTotal: 0, // no node data
			CPUPercent: -1,
			MEMPercent: -1,
		},
	})
	plain := stripANSI(out)
	if !strings.Contains(plain, "—") {
		t.Errorf("empty cluster data should render —; got\n%s", plain)
	}
}
