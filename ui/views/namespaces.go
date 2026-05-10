package views

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hermanu/klens/k8s"
	"github.com/hermanu/klens/k8s/resources"
	"github.com/hermanu/klens/port"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/layout"
)

var namespaceCols = []components.Column{
	{Header: colName, Width: 44, Flex: true},
	{Header: "STATUS", Width: 14},
	{Header: colAge, Width: 6, Align: components.AlignRight},
}

// NamespacesView lists Kubernetes namespaces and supports drilling into pods
// scoped to the selected namespace.
type NamespacesView struct {
	svc        port.NamespaceService
	namespaces []resources.NamespaceItem
	table      components.Table
	filter     string
	err        error
}

// NewNamespacesView creates a NamespacesView wired to svc.
func NewNamespacesView(svc port.NamespaceService) NamespacesView {
	return NamespacesView{
		svc:   svc,
		table: components.NewTable(namespaceCols, nil),
	}
}

// namespacesListedMsg carries the result of an async ListNamespaces call.
type namespacesListedMsg struct {
	items []resources.NamespaceItem
	err   error
}

// Update routes tea.Msg through the namespaces view.
func (v NamespacesView) Update(msg tea.Msg) (NamespacesView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.NamespacesUpdatedMsg:
		svc := v.svc
		return v, func() tea.Msg {
			items, err := svc.ListNamespaces(context.Background())
			return namespacesListedMsg{items: items, err: err}
		}

	case namespacesListedMsg:
		v.err = msg.err
		if msg.err == nil {
			v.namespaces = msg.items
			v.table = v.table.SetRows(v.rows())
		}
		return v, nil

	case FilterMsg:
		v.filter = msg.Query
		v.table = v.table.SetRows(v.rows())
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", keyDown:
			v.table = v.table.MoveDown()
		case "k", "up":
			v.table = v.table.MoveUp()
		case "g":
			v.table = v.table.MoveTop()
		case "G":
			v.table = v.table.MoveBottom()
		case keyEnter:
			// k9s-style drill-down: switch to pods filtered by this namespace.
			if n := v.selectedNamespace(); n != nil {
				name := n.Name
				return v, func() tea.Msg { return NamespaceSelectedMsg{Name: name} }
			}
		}
	}
	return v, nil
}

// selectedNamespace resolves the table cursor back to a NamespaceItem.
func (v NamespacesView) selectedNamespace() *resources.NamespaceItem {
	idx := v.table.SelectedIndex()
	visible := v.visibleNamespaces()
	if idx >= len(visible) {
		return nil
	}
	target := visible[idx].Name
	for i := range v.namespaces {
		if v.namespaces[i].Name == target {
			return &v.namespaces[i]
		}
	}
	return nil
}

// Title implements views.View.
func (v NamespacesView) Title() string { return "namespaces" }

// Filter implements views.Filterable.
func (v NamespacesView) Filter() string { return v.filter }

// Count implements views.View.
func (v NamespacesView) Count() (visible, total int) {
	return len(v.visibleNamespaces()), len(v.namespaces)
}

// Chips implements views.View — namespaces are cluster-scoped, so the ns chip
// is always "all".
func (v NamespacesView) Chips() []layout.FilterChip {
	chips := []layout.FilterChip{{Key: "ns", Value: "all"}}
	if v.filter != "" {
		chips = append(chips, layout.FilterChip{Key: "/", Value: v.filter, Strong: true})
	}
	return chips
}

// KeyHints implements views.View. Enter selects the namespace (and the
// shell drills into pods scoped to it); `/` is wired. yaml/delete live in
// KeyMap as Soon.
func (v NamespacesView) KeyHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "↵", Label: "select"},
		{Key: "/", Label: labelFilter},
	}
}

// KeyMap implements views.KeyMap and powers the `?` help overlay.
func (v NamespacesView) KeyMap() []components.KeySpec {
	return []components.KeySpec{
		{Key: "↵", Label: "select"},
		{Key: "/", Label: labelFilter},
		{Key: "y", Label: labelYAML, Soon: true},
		{Key: "d", Label: "delete", Soon: true},
	}
}

// Table implements views.View.
func (v NamespacesView) Table(width, height int) string {
	v.table = v.table.SetWidth(width).SetHeight(height)
	if v.err != nil {
		return "error: " + v.err.Error()
	}
	return v.table.View()
}

// Details implements views.View. Renders a uniform SPEC block driven by
// focusKVs() — status, top labels, age. Labels are sorted alphabetically and
// capped at 5 rows so a busy namespace doesn't blow out the SPEC pane.
func (v NamespacesView) Details(width, height int) string {
	n := v.selectedNamespace()
	if n == nil {
		return ""
	}
	return layout.DefaultDetails(width, height, layout.DetailsBlock{
		Title:    n.Name,
		Subtitle: fmt.Sprintf("%s · %s", n.Status, fmtAge(n.Age)),
		KVs:      v.focusKVs(),
	})
}

// focusKVs returns the SPEC fields for the focused namespace.
func (v NamespacesView) focusKVs() []layout.KV {
	n := v.selectedNamespace()
	if n == nil {
		return nil
	}
	kvs := []layout.KV{
		{Key: "status", Value: fallbackOr(n.Status)},
	}
	if labels := topLabels(n.Labels, 5); labels != "" {
		kvs = append(kvs, layout.KV{Key: "labels", Value: labels})
	}
	kvs = append(kvs, layout.KV{Key: kvAge, Value: fmtAge(n.Age)})
	return kvs
}

// visibleNamespaces returns the namespaces slice after applying v.filter
// through matchesFields. Fields included: name, status — namespaces don't
// expose other free-text columns in the table.
func (v NamespacesView) visibleNamespaces() []resources.NamespaceItem {
	if v.filter == "" {
		return v.namespaces
	}
	out := make([]resources.NamespaceItem, 0, len(v.namespaces))
	for _, n := range v.namespaces {
		if matchesFields(v.filter, n.Name, n.Status) {
			out = append(out, n)
		}
	}
	return out
}

// topLabels renders the top-N labels from a map sorted alphabetically as
// `k=v,k=v`. Returns "" for an empty map so the caller can omit the row.
func topLabels(labels map[string]string, limit int) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > limit {
		keys = keys[:limit]
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, ",")
}

func (v NamespacesView) rows() []components.Row {
	items := v.visibleNamespaces()
	rows := make([]components.Row, len(items))
	for i, n := range items {
		rows[i] = components.Row{
			highlightMatch(n.Name, v.filter),
			components.StatusPill(n.Status),
			fmtAge(n.Age),
		}
	}
	return rows
}
