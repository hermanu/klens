package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/hermanu/klens/ui/layout"
)

// FilterMsg is broadcast on every keystroke in the bottom command bar so views
// can re-filter their rows. Views that don't filter ignore it.
type FilterMsg struct{ Query string }

// NamespaceSelectedMsg is emitted by NamespacesView when the user presses
// Enter on a row. The root model intercepts it to switch the active namespace
// and route to PodsView (k9s-style drill-down).
type NamespaceSelectedMsg struct{ Name string }

// NamespaceChangedMsg is broadcast by the root model after the active
// namespace changes. Namespaced views (pods, deployments, services, secrets,
// configmaps, pvcs) update their internal `namespace` field on receipt — the
// follow-up UpdatedMsg refetches with the new scope.
type NamespaceChangedMsg struct{ Namespace string }

// LogTailRequestMsg is emitted by a view (today: pods on `l`, or LogsView on
// a range-shortcut keypress) asking the root model to start streaming logs.
// SinceSeconds is the lookback window — pass 1800 for 30 min, 0 for "no
// since" (the impl falls back to a tail-line cap). Pods carries one entry for
// a single-pod tail and N entries when fanning out across a workload.
type LogTailRequestMsg struct {
	Namespace    string
	Pods         []string
	SinceSeconds int64
}

// OpenDescribeMsg is emitted by a list view (today: pods on Enter) asking
// the root model to focus a full-screen describe view populated with the
// supplied resource info.
type OpenDescribeMsg struct {
	Title    string
	Subtitle string
	KVs      []layout.KV
}

// SwitchToGenericDescribeMsg asks the root model to focus the
// GenericDescribeView (a full-screen KV describe shell used by non-pod
// resources like PVCs). Sits alongside SwitchToDescribeMsg (pod-specific) so
// the shell can route based on the source view without growing a discriminator
// inside one message type.
type SwitchToGenericDescribeMsg struct {
	Title string
	KVs   []layout.KV
}

// BackMsg pops the navigation history stack. Sub-views (logs, describe) emit
// it on Esc so the user returns to whatever they came from.
type BackMsg struct{}

// DrillToPodsMsg is emitted by a non-pod list view when the user presses Enter
// on a row — k9s-style drill-down. The root model switches to PodsView and
// applies the supplied filter so the user sees the related workload pods.
type DrillToPodsMsg struct {
	Filter string
}

func fmtAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// matchesFields is the canonical filter helper used by every list view's
// visibleX(). Centralising the case-insensitive substring check here lets us
// guarantee the same fields-in / behaviour-out across resource types — e.g.
// deployments matching status, configmaps matching type — instead of each
// view drifting on its own.
//
// Empty filter matches everything; otherwise the trimmed query is compared
// case-insensitively against every supplied field.
func matchesFields(filter string, fields ...string) bool {
	if filter == "" {
		return true
	}
	f := strings.ToLower(strings.TrimSpace(filter))
	for _, s := range fields {
		if strings.Contains(strings.ToLower(s), f) {
			return true
		}
	}
	return false
}

