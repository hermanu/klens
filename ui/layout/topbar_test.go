package layout_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/layout"
)

func TestTopBarTitle_RendersBuildIDAndWordmark(t *testing.T) {
	got := stripANSI(layout.TopBarTitle(layout.TopBarConfig{KlensVer: "0.3.0", BuildID: "a1b2c3d"}))
	for _, want := range []string{"K·L·E·N·S", "v0.3.0", "build a1b2c3d"} {
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

// TestTopBarTitle_NoPulseGlyph verifies the title carries no animated mark.
// The brand pulse used to alternate ◉/◎ tied to a "Live" field, but `Live`
// didn't communicate anything actionable to the user (it was effectively
// static once the cluster picker was past). The watching dot in the foot is
// the only pulse in the frame now.
func TestTopBarTitle_NoPulseGlyph(t *testing.T) {
	got := stripANSI(layout.TopBarTitle(layout.TopBarConfig{KlensVer: "0.3.0"}))
	for _, unwanted := range []string{"◉", "◎"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("title should not carry a pulse glyph anymore, found %q in %q", unwanted, got)
		}
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

// TestTopBar_Wide_RendersIdentityAndVitals verifies the 3-row dashboard
// surfaces the cluster identity (ctx, region, k8s, uptime), the live vitals
// (nodes ratio), and the namespace chip. Cluster + user are intentionally
// not rendered as separate chips — the ctx basename covers the identity for
// EKS kubeconfigs where ctx/cluster/user are identical ARNs.
func TestTopBar_Wide_RendersIdentityAndVitals(t *testing.T) {
	out := layout.TopBar(120, layout.TopBarConfig{
		Context:    "production-eks",
		Cluster:    "acme-prod-2",
		User:       "alice@acme.io",
		K8sVersion: "v1.30.4",
		Region:     "us-east-1",
		KlensVer:   "0.3.0",
		BuildID:    "a1b2c3d",
		Uptime:     "62d 14h",
		NodesReady: 9, NodesTotal: 9,
	})
	plain := stripANSI(out)
	for _, want := range []string{
		"production-eks", // ctx
		"us-east-1",      // region
		"v1.30.4",        // k8s version
		"62d 14h",        // uptime
		"nodes 9/9",      // vitals
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("body missing %q\n--- output ---\n%s", want, plain)
		}
	}
	// Row 1 must NOT inline-repeat the brand wordmark — the panel title already
	// renders K·L·E·N·S; duplicating it here was visually noisy.
	if strings.Contains(plain, "KLENS") {
		t.Errorf("row 1 should not inline the KLENS wordmark anymore, got:\n%s", plain)
	}
	// Namespace must NOT appear on the top bar — it lives on the table title's
	// breadcrumb (`ns:default`); two surfaces was the duplication users flagged.
	if strings.Contains(plain, "ns default") || strings.Contains(plain, "ns:") {
		t.Errorf("top bar should not carry the ns chip anymore, got:\n%s", plain)
	}
}

// TestTopBar_Wide_TrimsARNToBasename verifies that when ctx is an EKS ARN,
// only the cluster basename renders on row 1 — never the raw 60-char ARN.
func TestTopBar_Wide_TrimsARNToBasename(t *testing.T) {
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
}

// TestTopBar_BodyHasThreeRows verifies the wide body is exactly 3 rows
// (identity / vitals / phase). The bordering Panel adds the 2 border rows
// separately (topBarRowsWide = 5 in app/app.go).
func TestTopBar_BodyHasThreeRows(t *testing.T) {
	out := layout.TopBar(120, layout.TopBarConfig{
		Context:    "prod",
		NodesReady: 1, NodesTotal: 1,
	})
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Errorf("want 3 body rows at width=120, got %d:\n%s", len(lines), out)
	}
}

// TestTopBar_Wide_NavStripRendersActive verifies the horizontal nav strip
// in row 2 renders the active item with the ▌ accent.
func TestTopBar_Wide_NavStripRendersActive(t *testing.T) {
	out := layout.TopBar(120, layout.TopBarConfig{
		Context: "prod",
		NavItems: []layout.NavItem{
			{Mnemonic: "1", Label: "pods", Active: true},
			{Mnemonic: "2", Label: "deployments"},
			{Mnemonic: "3", Label: "services"},
		},
	})
	plain := stripANSI(out)
	if !strings.Contains(plain, "▌1 pods") {
		t.Errorf("nav strip should render ▌1 pods for active item, got:\n%s", plain)
	}
	if !strings.Contains(plain, "2 dp") {
		t.Errorf("nav strip should render short label '2 dp' for deployments, got:\n%s", plain)
	}
}

// TestTopBar_Narrow_SingleRow verifies the narrow fallback collapses to a
// single body row carrying ctx + nodes ratio. The brand mark is intentionally
// absent here — it lives only on the panel title so the body doesn't
// duplicate the pulse glyph competing with the `● watching` foot.
func TestTopBar_Narrow_SingleRow(t *testing.T) {
	out := layout.TopBar(50, layout.TopBarConfig{
		Context:    "prod",
		NodesReady: 9, NodesTotal: 9,
	})
	plain := stripANSI(out)
	if !strings.Contains(plain, "ctx prod") {
		t.Errorf("narrow body should keep ctx, got:\n%s", plain)
	}
	if !strings.Contains(plain, "nodes 9/9") {
		t.Errorf("narrow body should keep nodes ratio, got:\n%s", plain)
	}
	if strings.Contains(plain, "◉") || strings.Contains(plain, "◎") {
		t.Errorf("narrow body should NOT carry a pulse mark — title owns the brand pulse, got:\n%s", plain)
	}
	if lines := strings.Split(out, "\n"); len(lines) != 1 {
		t.Errorf("narrow body should be 1 row, got %d", len(lines))
	}
}

// TestTopBar_Wide_PhaseRowRendersWhenSet verifies row 3 emits the pod phase
// counts ("Running N · Pending N · Error N · Total N") when PhaseCounts is
// populated by the pods view via the PhaseCounter optional interface.
func TestTopBar_Wide_PhaseRowRendersWhenSet(t *testing.T) {
	out := layout.TopBar(120, layout.TopBarConfig{
		Context: "prod",
		PhaseCounts: &layout.PhaseCounts{
			Running: 23, Pending: 1, Errored: 0, Total: 54,
		},
	})
	plain := stripANSI(out)
	for _, want := range []string{"Running 23", "Pending 1", "Error 0", "Total 54"} {
		if !strings.Contains(plain, want) {
			t.Errorf("phase row missing %q\n--- output ---\n%s", want, plain)
		}
	}
}

// TestTopBar_Wide_PhaseRowEmptyWhenNil verifies non-pod views (PhaseCounts == nil)
// render row 3 empty so the body height stays at 3 across view switches.
func TestTopBar_Wide_PhaseRowEmptyWhenNil(t *testing.T) {
	out := layout.TopBar(120, layout.TopBarConfig{Context: "prod"})
	plain := stripANSI(out)
	for _, unwanted := range []string{"Running ", "Pending ", "Error ", "Total "} {
		if strings.Contains(plain, unwanted) {
			t.Errorf("non-pod view should not render phase row, found %q\n--- output ---\n%s", unwanted, plain)
		}
	}
}

// TestTopBar_LongARNDoesNotOverflowWidth verifies long ARN-style context
// names get trimmed/dropped rather than overflowing the body width.
func TestTopBar_LongARNDoesNotOverflowWidth(t *testing.T) {
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
