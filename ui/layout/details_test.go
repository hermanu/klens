package layout_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/layout"
)

func TestDefaultDetails_EmptyBlockReturnsEmpty(t *testing.T) {
	if got := layout.DefaultDetails(40, 20, layout.DetailsBlock{}); got != "" {
		t.Errorf("empty block must yield empty string, got %q", got)
	}
}

func TestDefaultDetails_PodDossier(t *testing.T) {
	out := layout.DefaultDetails(46, 24, layout.DetailsBlock{
		Title:    "api-gateway-7c4b9d-xk29p",
		Subtitle: "platform · Running · ready 2/2 · 4d12h",
		KVs: []layout.KV{
			{Key: "node", Value: "ip-10-0-1-21"},
			{Key: "ip", Value: "10.42.1.18"},
			{Key: "restarts", Value: "0"},
		},
		Sparks: []layout.MetricSeries{
			{Label: "cpu", Value: "142m", Samples: []float64{40, 50, 60, 70, 60, 70, 80}},
			{Label: "mem", Value: "412Mi", Samples: []float64{55, 60, 65, 70, 72, 70, 75}},
			{Label: "net↓", Value: "58KB/s", Samples: []float64{30, 40, 45, 50, 55}},
			{Label: "net↑", Value: "32KB/s", Samples: []float64{20, 25, 30, 35}},
		},
		Containers: []layout.ContainerSummary{
			{Name: "api-gateway", Image: "ghcr.io/acme/api-gateway:1.42.0", Status: "Running", Restarts: 0},
		},
	})
	plain := stripANSI(out)

	for _, want := range []string{
		"api-gateway-7c4b9d-xk29p",
		"platform · Running",
		"node",
		"ip-10-0-1-21",
		"METRICS",
		"cpu",
		"142m",
		"net↓",
		"net↑",
		"CONTAINERS",
		"api-gateway",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, plain)
		}
	}
}

func TestDefaultDetails_NonPodView(t *testing.T) {
	// No Sparks, no Containers — should render only header + KVs.
	out := layout.DefaultDetails(46, 20, layout.DetailsBlock{
		Title: "api-gateway",
		KVs: []layout.KV{
			{Key: "replicas", Value: "3"},
			{Key: "strategy", Value: "RollingUpdate"},
		},
	})
	plain := stripANSI(out)
	if strings.Contains(plain, "METRICS") {
		t.Errorf("METRICS section should not appear without Sparks")
	}
	if strings.Contains(plain, "CONTAINERS") {
		t.Errorf("CONTAINERS section should not appear without Containers")
	}
	if !strings.Contains(plain, "api-gateway") {
		t.Errorf("title should still render: %s", plain)
	}
}

// LogTail is intentionally not rendered — `l` opens the dedicated full-screen
// logs view. This test guards against accidentally re-introducing a log block.
func TestDefaultDetails_LogTailIsDropped(t *testing.T) {
	b := layout.DetailsBlock{
		Title: "pod-x",
		LogTail: []layout.LogLine{
			{Time: "14:08:21.412", Level: "INFO", Msg: "started"},
			{Time: "14:08:22.000", Level: "ERROR", Msg: "boom"},
		},
	}
	got := stripANSI(layout.DefaultDetails(80, 30, b))
	for _, banned := range []string{"LOGS", "tailing", "14:08:21.412", "INFO", "ERROR", "boom"} {
		if strings.Contains(got, banned) {
			t.Errorf("DefaultDetails must not render LogTail, but %q appeared in:\n%s", banned, got)
		}
	}
}

func TestDefaultDetails_WidthClamp(t *testing.T) {
	b := layout.DetailsBlock{
		Title:    "very-long-resource-name-that-should-be-trimmed-or-fit",
		Subtitle: "sub",
		KVs: []layout.KV{
			{Key: "image", Value: "ghcr.io/acme/api-gateway-with-a-very-long-image-tag:1.42.0-rc.5"},
		},
		Containers: []layout.ContainerSummary{
			{
				Name:     "horizontal-pod-autoscaler-controller-very-very-long",
				Image:    "ghcr.io/acme/horizontal-pod-autoscaler-controller-with-a-very-very-long-image-tag:2.0.0",
				Status:   "CrashLoopBackOff",
				Restarts: 12345,
			},
		},
		Sparks: []layout.MetricSeries{
			{Label: "cpu", Value: "999m", Samples: []float64{10, 20, 30}},
			{Label: "mem", Value: "9999Mi", Samples: []float64{10, 20, 30}},
		},
	}
	for _, w := range []int{30, 60, 120} {
		got := layout.DefaultDetails(w, 30, b)
		for _, line := range strings.Split(got, "\n") {
			if lipgloss.Width(line) > w {
				t.Errorf("width=%d exceeded: line width=%d %q", w, lipgloss.Width(line), line)
			}
		}
	}
}

func TestDefaultDetails_HeightClampKeepsHeader(t *testing.T) {
	b := layout.DetailsBlock{
		Title: "pod-x",
		Sparks: []layout.MetricSeries{
			{Label: "cpu", Value: "10m", Samples: []float64{1, 2, 3}},
			{Label: "mem", Value: "10Mi", Samples: []float64{1, 2, 3}},
			{Label: "net↓", Value: "1KB/s", Samples: []float64{1, 2, 3}},
			{Label: "net↑", Value: "1KB/s", Samples: []float64{1, 2, 3}},
		},
	}
	got := stripANSI(layout.DefaultDetails(60, 6, b))
	lines := strings.Split(got, "\n")
	if len(lines) > 6 {
		t.Errorf("must clamp to height=6, got %d lines", len(lines))
	}
	if !strings.Contains(got, "pod-x") {
		t.Errorf("title should survive height clamp, got %q", got)
	}
}
