package layout_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/layout"
)

func TestTopBarTitle_RendersBuildID(t *testing.T) {
	got := stripANSI(layout.TopBarTitle(layout.TopBarConfig{KlensVer: "0.3.0", BuildID: "a1b2c3d"}))
	for _, want := range []string{"KLENS", "0.3.0", "build a1b2c3d"} {
		if !strings.Contains(got, want) {
			t.Errorf("TopBarTitle missing %q, got %q", want, got)
		}
	}
}

func TestTopBarTitle_NoBuildIDFallsBackToDev(t *testing.T) {
	got := stripANSI(layout.TopBarTitle(layout.TopBarConfig{KlensVer: "0.3.0"}))
	if !strings.Contains(got, "build dev") {
		t.Errorf("missing 'build dev' fallback: %q", got)
	}
}

func TestTopBarFoot_PulseSwap(t *testing.T) {
	on := stripANSI(layout.TopBarFoot(true, true))
	off := stripANSI(layout.TopBarFoot(false, true))
	if !strings.Contains(on, "●") || !strings.Contains(on, "watching") {
		t.Errorf("pulseOn+live should show ●+watching, got %q", on)
	}
	if !strings.Contains(off, "○") {
		t.Errorf("pulseOff should show ○, got %q", off)
	}
}

// TestTopBar_Wide_RendersBodyKVGrid verifies the body output at wide widths
// contains the KV grid contents. The brand and buildID live in the TITLE
// string (asserted separately); the right column was dropped because the
// nav rail's CLUSTER footer already shows nodes/cpu/mem.
func TestTopBar_Wide_RendersBodyKVGrid(t *testing.T) {
	out := layout.TopBar(120, layout.TopBarConfig{
		Context:    "production-eks",
		Cluster:    "acme-prod-2",
		User:       "alice@acme.io",
		K8sVersion: "v1.30.4",
		Region:     "us-east-1",
		KlensVer:   "0.3.0",
		BuildID:    "a1b2c3d",
		Uptime:     "62d 14h",
	})
	plain := stripANSI(out)
	for _, want := range []string{
		"production-eks",
		"acme-prod-2",
		"us-east-1",
		"alice@acme.io",
		"v1.30.4",
		"62d 14h",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("body missing %q\n--- output ---\n%s", want, plain)
		}
	}
}

// TestTopBar_Wide_DropsRedundantClusterAndUser verifies that when ctx,
// cluster, and user are all the same ARN (the EKS default), the cluster
// and user rows collapse and only the ctx line shows the basename. This
// is the specific redundancy bug surfaced by an EKS kubeconfig.
func TestTopBar_Wide_DropsRedundantClusterAndUser(t *testing.T) {
	arn := "arn:aws:eks:eu-west-1:857619978098:cluster/maisa-sdlc-eks"
	out := layout.TopBar(120, layout.TopBarConfig{
		Context:    arn,
		Cluster:    arn,
		User:       arn,
		K8sVersion: "v1.35.3",
	})
	plain := stripANSI(out)
	if !strings.Contains(plain, "maisa-sdlc-eks") {
		t.Errorf("ctx basename should render, got:\n%s", plain)
	}
	if strings.Contains(plain, "arn:aws:eks") {
		t.Errorf("raw ARN should be trimmed to basename, got:\n%s", plain)
	}
	if strings.Contains(plain, "cluster maisa-sdlc-eks") {
		t.Errorf("cluster row should collapse when identical to ctx, got:\n%s", plain)
	}
	if strings.Contains(plain, "user maisa-sdlc-eks") {
		t.Errorf("user row should collapse when identical to ctx, got:\n%s", plain)
	}
}

// TestTopBar_BodyHasTwoRows verifies the body is always 2 rows at wide widths
// (no divider — the bordering panel draws the border row separately).
func TestTopBar_BodyHasTwoRows(t *testing.T) {
	out := layout.TopBar(120, layout.TopBarConfig{
		Context:    "prod",
		NodesReady: 1, NodesTotal: 1,
	})
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Errorf("want 2 body rows at width=120, got %d:\n%s", len(lines), out)
	}
}

// TestTopBar_Narrow_NoLogoSingleRow verifies the narrow fallback drops the
// block logo and returns a single body row.
func TestTopBar_Narrow_NoLogoSingleRow(t *testing.T) {
	out := layout.TopBar(50, layout.TopBarConfig{
		Context:    "prod",
		NodesReady: 9, NodesTotal: 9,
	})
	plain := stripANSI(out)
	if strings.Contains(plain, "█▄▀") {
		t.Errorf("block logo should be hidden at width=50:\n%s", plain)
	}
	if lines := strings.Split(out, "\n"); len(lines) != 1 {
		t.Errorf("narrow body should be 1 row, got %d", len(lines))
	}
}

// TestTopBar_DropsRedundantArn verifies that long ARN-style context names
// don't cause overflow when rendered.
func TestTopBar_DropsRedundantArn(t *testing.T) {
	out := layout.TopBar(120, layout.TopBarConfig{
		Context:    "arn:aws:eks:eu-west-1:857619978098:cluster/maisa-sdlc-eks",
		NodesReady: 9, NodesTotal: 9,
	})
	for _, line := range strings.Split(out, "\n") {
		if lipgloss.Width(line) > 120 {
			t.Errorf("line exceeds width 120: width=%d %q", lipgloss.Width(line), line)
		}
	}
}
