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

func TestDefaultDetails_TitleAndKVs(t *testing.T) {
	b := layout.DetailsBlock{
		Title:    "api-gateway-7c6b-vxq2t",
		Subtitle: "platform · Running",
		KVs: []layout.KV{
			{Key: "image", Value: "ghcr.io/acme/api:1.0"},
			{Key: "node", Value: "ip-10-0-8-02"},
			{Key: "restarts", Value: "3", Warn: true},
		},
	}
	got := layout.DefaultDetails(60, 30, b)
	wants := []string{
		"FOCUSED ITEM",
		"api-gateway-7c6b-vxq2t",
		"platform",
		"SPEC",
		"image", "ghcr.io/acme/api:1.0",
		"node", "ip-10-0-8-02",
		"restarts", "3",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("want substring %q, got %q", w, got)
		}
	}
}

func TestDefaultDetails_NoSparksOmitsLiveSection(t *testing.T) {
	b := layout.DetailsBlock{
		Title: "cm-config",
		KVs:   []layout.KV{{Key: "items", Value: "12"}},
	}
	got := layout.DefaultDetails(60, 30, b)
	if strings.Contains(got, "LIVE") {
		t.Errorf("no Sparks → no LIVE section, got %q", got)
	}
	if strings.Contains(got, "LOGS") {
		t.Errorf("no LogTail → no LOGS section, got %q", got)
	}
}

func TestDefaultDetails_NoLogsOmitsLogsSection(t *testing.T) {
	b := layout.DetailsBlock{
		Title: "pod-x",
		Sparks: []layout.MetricSeries{
			{Label: "cpu", Value: "120m", Samples: []float64{10, 20, 30}},
		},
		KVs: []layout.KV{{Key: "image", Value: "x:1"}},
	}
	got := layout.DefaultDetails(80, 30, b)
	if !strings.Contains(got, "LIVE") {
		t.Errorf("Sparks present → want LIVE label, got %q", got)
	}
	if strings.Contains(got, "LOGS") {
		t.Errorf("no LogTail → must not include LOGS, got %q", got)
	}
}

// LogTail is intentionally not rendered in DefaultDetails — `l` opens the
// dedicated full-screen logs view, so this pane focuses on SPEC + metrics.
// The test below guards against accidentally re-introducing a log block.
func TestDefaultDetails_LogTailIsDropped(t *testing.T) {
	b := layout.DetailsBlock{
		Title: "pod-x",
		LogTail: []layout.LogLine{
			{Time: "14:08:21.412", Level: "INFO", Msg: "started"},
			{Time: "14:08:22.000", Level: "ERROR", Msg: "boom"},
		},
	}
	got := layout.DefaultDetails(80, 30, b)
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
		LogTail: []layout.LogLine{
			{Time: "00:00:00.000", Level: "INFO", Msg: strings.Repeat("x", 200)},
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
		LogTail: []layout.LogLine{
			{Time: "t1", Level: "INFO", Msg: "a"},
			{Time: "t2", Level: "INFO", Msg: "b"},
			{Time: "t3", Level: "INFO", Msg: "c"},
			{Time: "t4", Level: "INFO", Msg: "d"},
			{Time: "t5", Level: "INFO", Msg: "e"},
		},
	}
	got := layout.DefaultDetails(60, 6, b)
	lines := strings.Split(got, "\n")
	if len(lines) > 6 {
		t.Errorf("must clamp to height=6, got %d lines", len(lines))
	}
	if !strings.Contains(got, "FOCUSED ITEM") {
		t.Errorf("header should survive height clamp, got %q", got)
	}
}
