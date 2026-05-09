package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/hermanu/klens/ui/components"
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

// matches reports whether `s` matches the user filter `q` case-insensitively.
// Empty q matches everything.
func matches(q string, s ...string) bool {
	if q == "" {
		return true
	}
	q = strings.ToLower(q)
	for _, v := range s {
		if strings.Contains(strings.ToLower(v), q) {
			return true
		}
	}
	return false
}

// kvFromRow pairs each column's header (lower-cased) with its rendered cell —
// the default Details() body for resource views without a custom panel.
func kvFromRow(cols []components.Column, row components.Row) []layout.KV {
	kvs := make([]layout.KV, 0, len(cols))
	for i, c := range cols {
		val := ""
		if i < len(row) {
			val = row[i]
		}
		kvs = append(kvs, layout.KV{
			Key:   strings.ToLower(c.Header),
			Value: val,
		})
	}
	return kvs
}
