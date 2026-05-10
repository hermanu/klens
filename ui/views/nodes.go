package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hermanu/klens/k8s"
	"github.com/hermanu/klens/k8s/resources"
	"github.com/hermanu/klens/port"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/layout"
)

var nodeCols = []components.Column{
	{Header: colName, Width: 36, Flex: true},
	{Header: "STATUS", Width: 12},
	{Header: "ROLES", Width: 18},
	{Header: "VERSION", Width: 16},
	{Header: colAge, Width: 6, Align: components.AlignRight},
}

// NodesView lists nodes and supports `l` to fan out a multi-pod log tail
// across every pod scheduled to the focused node.
type NodesView struct {
	svc    port.NodeService
	pods   port.PodService // for ListPodsOnNode on `l`
	nodes  []resources.NodeItem
	table  components.Table
	filter string
	err    error
}

// NewNodesView wires the node service plus a PodService used to resolve pods
// on a node when the user presses `l`.
func NewNodesView(svc port.NodeService, pods port.PodService) NodesView {
	return NodesView{
		svc:   svc,
		pods:  pods,
		table: components.NewTable(nodeCols, nil),
	}
}

// nodesListedMsg carries the result of an async ListNodes call.
type nodesListedMsg struct {
	items []resources.NodeItem
	err   error
}

// nodePodsResolvedMsg carries the pods scheduled to a node back to the view,
// which then emits SwitchToLogsMsg + LogTailRequestMsg. Async to keep the
// Update loop unblocked during the field-selector lookup.
//
// The watcher's log fan-out streams pods within a single namespace per call,
// so we resolve to the most-populated namespace on the node and pass just that
// subset. Mixing namespaces in one tail would require widening the watcher
// API; this trade keeps the change small while still showing meaningful logs.
type nodePodsResolvedMsg struct {
	namespace string
	title     string
	pods      []string
	err       error
}

// Update routes tea.Msg through the nodes view, handling watcher events and
// pod resolution for log fan-out.
func (v NodesView) Update(msg tea.Msg) (NodesView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.NodesUpdatedMsg:
		svc := v.svc
		return v, func() tea.Msg {
			items, err := svc.ListNodes(context.Background())
			return nodesListedMsg{items: items, err: err}
		}

	case nodesListedMsg:
		v.err = msg.err
		if msg.err == nil {
			v.nodes = msg.items
			v.table = v.table.SetRows(v.rows())
		}
		return v, nil

	case FilterMsg:
		v.filter = msg.Query
		v.table = v.table.SetRows(v.rows())
		return v, nil

	case nodePodsResolvedMsg:
		if msg.err != nil || len(msg.pods) == 0 {
			return v, nil
		}
		ns, pods, title := msg.namespace, msg.pods, msg.title
		return v, tea.Batch(
			func() tea.Msg { return SwitchToLogsMsg{Namespace: ns, Pods: pods, Title: title} },
			func() tea.Msg {
				return LogTailRequestMsg{Namespace: ns, Pods: pods, SinceSeconds: 0}
			},
		)

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
		case "l":
			// Fan out a multi-pod log tail across every pod on this node,
			// scoped to the node's most-populated namespace (the watcher's
			// fan-out is single-namespace per call). Most kube clusters
			// concentrate node workload in one app namespace, so this
			// captures the interesting set in the common case.
			n := v.selectedNode()
			if n == nil || v.pods == nil {
				return v, nil
			}
			name := n.Name
			pods := v.pods
			return v, func() tea.Msg {
				items, err := pods.ListPodsOnNode(context.Background(), name)
				if err != nil {
					return nodePodsResolvedMsg{err: err}
				}
				ns, names := pickDominantNamespace(items)
				return nodePodsResolvedMsg{
					namespace: ns,
					title:     "node/" + name,
					pods:      names,
				}
			}
		case keyEnter:
			// Drill-down: switch to pods filtered by node name. Useful for
			// "show me everything running on this node".
			idx := v.table.SelectedIndex()
			items := v.visibleNodes()
			if idx < len(items) {
				name := items[idx].Name
				label := "node/" + name
				return v, func() tea.Msg { return DrillToPodsMsg{Filter: name, Label: label} }
			}
		}
	}
	return v, nil
}

// selectedNode resolves the table cursor back to a NodeItem. Returns nil for
// empty tables. Resolves via the filtered slice so the index aligns with the
// table's current view.
func (v NodesView) selectedNode() *resources.NodeItem {
	if v.table.SelectedRow() == nil {
		return nil
	}
	idx := v.table.SelectedIndex()
	visible := v.visibleNodes()
	if idx >= len(visible) {
		return nil
	}
	n := visible[idx]
	for i := range v.nodes {
		if v.nodes[i].Name == n.Name {
			return &v.nodes[i]
		}
	}
	return nil
}

// Title implements views.View.
func (v NodesView) Title() string { return "nodes" }

// Filter implements views.Filterable.
func (v NodesView) Filter() string { return v.filter }

// Count implements views.View.
func (v NodesView) Count() (visible, total int) {
	return len(v.visibleNodes()), len(v.nodes)
}

// Chips implements views.View. Nodes are cluster-scoped — show that explicitly
// instead of a namespace chip so the chip row isn't misleading.
func (v NodesView) Chips() []layout.FilterChip {
	chips := []layout.FilterChip{{Key: "scope", Value: "cluster"}}
	if v.filter != "" {
		chips = append(chips, layout.FilterChip{Key: "/", Value: v.filter, Strong: true})
	}
	return chips
}

// KeyHints implements views.View. `l logs` is wired (multi-pod fan-out across
// the node's dominant namespace); cordon/drain/yaml stay Soon.
func (v NodesView) KeyHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "↵", Label: labelPods},
		{Key: "l", Label: labelLogs},
		{Key: "/", Label: labelFilter},
	}
}

// KeyMap implements views.KeyMap and powers the `?` help overlay.
func (v NodesView) KeyMap() []components.KeySpec {
	return []components.KeySpec{
		{Key: "↵", Label: labelPods},
		{Key: "l", Label: labelLogs},
		{Key: "/", Label: labelFilter},
		{Key: "y", Label: labelYAML, Soon: true},
		{Key: "c", Label: "cordon", Soon: true},
		{Key: "d", Label: "drain", Soon: true},
	}
}

// Table implements views.View.
func (v NodesView) Table(width, height int) string {
	v.table = v.table.SetWidth(width).SetHeight(height)
	if v.err != nil {
		return "error: " + v.err.Error()
	}
	return v.table.View()
}

// Details implements views.View. Renders a uniform SPEC block driven by
// focusKVs() — roles, version, kernel/runtime when present, capacity, age.
func (v NodesView) Details(width, height int) string {
	n := v.selectedNode()
	if n == nil {
		return ""
	}
	subtitle := n.Status
	if n.Status != "" {
		subtitle = fmt.Sprintf("%s · %s", n.Status, fmtAge(n.Age))
	}
	return layout.DefaultDetails(width, height, layout.DetailsBlock{
		Title:    n.Name,
		Subtitle: subtitle,
		KVs:      v.focusKVs(),
	})
}

// focusKVs returns the SPEC fields for the focused node. Kernel and runtime
// are only included when populated (older clusters / minikube sometimes leave
// them empty). Capacity values come straight from Status.Capacity in resources.
func (v NodesView) focusKVs() []layout.KV {
	n := v.selectedNode()
	if n == nil {
		return nil
	}
	kvs := []layout.KV{
		{Key: "roles", Value: fallbackOr(n.Roles)},
		{Key: "version", Value: fallbackOr(n.Version)},
	}
	if n.Kernel != "" {
		kvs = append(kvs, layout.KV{Key: "kernel", Value: n.Kernel})
	}
	if n.Runtime != "" {
		kvs = append(kvs, layout.KV{Key: "runtime", Value: n.Runtime})
	}
	if n.CPU != "" {
		kvs = append(kvs, layout.KV{Key: "cpu cap", Value: n.CPU})
	}
	if n.Memory != "" {
		kvs = append(kvs, layout.KV{Key: "mem cap", Value: n.Memory})
	}
	if n.Pods != "" {
		kvs = append(kvs, layout.KV{Key: "pods cap", Value: n.Pods})
	}
	kvs = append(kvs, layout.KV{Key: kvAge, Value: fmtAge(n.Age)})
	return kvs
}

// visibleNodes returns the nodes slice after applying v.filter through
// matchesFields. Fields included: name, status, roles, version, kernel,
// runtime — every stringy field a user might filter by when triaging nodes.
func (v NodesView) visibleNodes() []resources.NodeItem {
	if v.filter == "" {
		return v.nodes
	}
	out := make([]resources.NodeItem, 0, len(v.nodes))
	for _, n := range v.nodes {
		if matchesFields(v.filter, n.Name, n.Status, n.Roles, n.Version, n.Kernel, n.Runtime) {
			out = append(out, n)
		}
	}
	return out
}

// pickDominantNamespace returns the namespace that hosts the most pods in the
// supplied list, plus the matching pod names. Used by the node log-tail flow:
// the watcher fan-out is single-namespace per call, so we pick whichever
// namespace dominates the node and stream that subset.
//
// Empty input returns an empty namespace and slice — callers should treat that
// as "nothing to tail".
func pickDominantNamespace(items []resources.PodItem) (namespace string, podNames []string) {
	if len(items) == 0 {
		return "", nil
	}
	counts := make(map[string]int, 4)
	for _, p := range items {
		counts[p.Namespace]++
	}
	top := ""
	maxCount := 0
	for ns, n := range counts {
		if n > maxCount {
			top = ns
			maxCount = n
		}
	}
	names := make([]string, 0, maxCount)
	for _, p := range items {
		if p.Namespace == top {
			names = append(names, p.Name)
		}
	}
	return top, names
}

func (v NodesView) rows() []components.Row {
	nodes := v.visibleNodes()
	rows := make([]components.Row, len(nodes))
	for i, n := range nodes {
		rows[i] = components.Row{
			highlightMatch(n.Name, v.filter),
			components.StatusPill(n.Status),
			highlightMatch(n.Roles, v.filter),
			n.Version,
			fmtAge(n.Age),
		}
	}
	return rows
}
