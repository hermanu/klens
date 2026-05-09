package layout_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/layout"
)

func sampleTopBarCfg() layout.TopBarConfig {
	return layout.TopBarConfig{
		Context:      "prod-eks",
		Cluster:      "eks-east",
		User:         "manu",
		K8sVersion:   "v1.30.4",
		Region:       "us-east-1",
		KlensVer:     "0.3.0",
		Namespace:    "europa",
		Resource:     "pods",
		Live:         true,
		VisibleCount: 23,
		TotalCount:   23,
		Totals:       layout.Totals{Pods: 12, Deployments: 3, Services: 5, Events: 7},
	}
}

// TestTopBar_ContainsKeyFragments asserts the redesigned single-row bar still
// surfaces identity, namespace anchor, and the live indicator. Counts moved
// to the nav strip so this row no longer carries the resource label or count.
func TestTopBar_ContainsKeyFragments(t *testing.T) {
	got := layout.TopBar(160, sampleTopBarCfg())
	wants := []string{
		"K L E N S",       // letter-spaced banner
		"ctx", "prod-eks", // identity strip — context only
		"v1.30.4", // short k8s version (no -eks suffix)
		"europa",  // namespace chip is the visual anchor
		"live",    // ● live indicator
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("want substring %q in TopBar output, got %q", w, got)
		}
	}
}

// TestTopBar_DropsRedundantArn asserts that long ARN-style identifiers
// (passed in via Cluster/User) are NOT rendered — they were causing the bar
// to read as cluttered noise.
func TestTopBar_DropsRedundantArn(t *testing.T) {
	cfg := sampleTopBarCfg()
	cfg.Cluster = "arn:aws:eks:eu-west-1:857619978098:cluster/maisa-sdlc-eks"
	cfg.User = "arn:aws:eks:eu-west-1:857619978098:cluster/maisa-sdlc-eks"
	got := layout.TopBar(160, cfg)
	if strings.Contains(got, "arn:") {
		t.Errorf("top bar must not show full cluster/user ARNs, got %q", got)
	}
}

// TestTopBar_ShortensK8sVersion asserts the eks-vendored suffix is trimmed.
func TestTopBar_ShortensK8sVersion(t *testing.T) {
	cfg := sampleTopBarCfg()
	cfg.K8sVersion = "v1.35.3-eks-bbe087e"
	got := layout.TopBar(160, cfg)
	if !strings.Contains(got, "v1.35.3") {
		t.Errorf("expected short version v1.35.3 in output, got %q", got)
	}
	if strings.Contains(got, "bbe087e") {
		t.Errorf("expected eks suffix to be trimmed, got %q", got)
	}
}

// TestTopBar_TwoRowsAtAnyWidth asserts the bar is exactly 2 rows tall (1
// content + 1 divider). The horizontal nav strip is rendered separately by
// the shell, so the top bar must never grow vertically.
func TestTopBar_TwoRowsAtAnyWidth(t *testing.T) {
	cfg := sampleTopBarCfg()
	for _, w := range []int{80, 120, 160, 200} {
		got := layout.TopBar(w, cfg)
		lines := strings.Split(got, "\n")
		if len(lines) != 2 {
			t.Errorf("TopBar(width=%d): want 2 rows, got %d: %q", w, len(lines), got)
		}
		for _, line := range lines {
			if lipgloss.Width(line) > w {
				t.Errorf("TopBar(width=%d): line width %d exceeds clamp: %q",
					w, lipgloss.Width(line), line)
			}
		}
	}
}

func TestTopBar_LiveOmittedWhenFalse(t *testing.T) {
	cfg := sampleTopBarCfg()
	cfg.Live = false
	got := layout.TopBar(160, cfg)
	if strings.Contains(got, "● live") {
		t.Errorf("Live=false should omit '● live', got %q", got)
	}
}

// TestTopBar_AllNamespacesFallback asserts the chip falls back to the
// "all namespaces" pseudo-scope when no namespace is set.
func TestTopBar_AllNamespacesFallback(t *testing.T) {
	cfg := sampleTopBarCfg()
	cfg.Namespace = ""
	got := layout.TopBar(160, cfg)
	if !strings.Contains(got, "all namespaces") {
		t.Errorf("empty namespace should render as 'all namespaces', got %q", got)
	}
}
