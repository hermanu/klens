package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hermanu/klens/k8s"
	"github.com/hermanu/klens/k8s/resources"
	"github.com/hermanu/klens/port"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/layout"
)

var nodeCols = []components.Column{
	{Header: "NAME", Width: 36},
	{Header: "STATUS", Width: 12},
	{Header: "ROLES", Width: 18},
	{Header: "VERSION", Width: 16},
	{Header: "AGE", Width: 6, Align: components.AlignRight},
}

type NodesView struct {
	svc    port.NodeService
	nodes  []resources.NodeItem
	table  components.Table
	filter string
	err    error
}

func NewNodesView(svc port.NodeService) NodesView {
	return NodesView{
		svc:   svc,
		table: components.NewTable(nodeCols, nil),
	}
}

// nodesListedMsg carries the result of an async ListNodes call.
type nodesListedMsg struct {
	items []resources.NodeItem
	err   error
}

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

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			v.table = v.table.MoveDown()
		case "k", "up":
			v.table = v.table.MoveUp()
		case "g":
			v.table = v.table.MoveTop()
		case "G":
			v.table = v.table.MoveBottom()
		case "enter":
			// Drill-down: switch to pods filtered by node name. Useful for
			// "show me everything running on this node".
			idx := v.table.SelectedIndex()
			items := v.visibleNodes()
			if idx < len(items) {
				name := items[idx].Name
				return v, func() tea.Msg { return DrillToPodsMsg{Filter: name} }
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

// KeyHints implements views.View. Only Enter (drill to pods on this node) and
// `/` are wired today; cordon/drain/yaml are advertised in the `?` overlay.
func (v NodesView) KeyHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "↵", Label: "pods"},
		{Key: "/", Label: "filter"},
	}
}

// KeyMap implements views.KeyMap and powers the `?` help overlay.
func (v NodesView) KeyMap() []components.KeySpec {
	return []components.KeySpec{
		{Key: "↵", Label: "pods"},
		{Key: "l", Label: "logs", Soon: true},
		{Key: "/", Label: "filter"},
		{Key: "y", Label: "yaml", Soon: true},
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

// Details implements views.View.
func (v NodesView) Details(width, height int) string {
	n := v.selectedNode()
	if n == nil {
		return ""
	}
	return layout.DefaultDetails(width, height, layout.DetailsBlock{
		Title:    n.Name,
		Subtitle: n.Status,
		KVs: []layout.KV{
			{Key: "status", Value: n.Status},
			{Key: "roles", Value: n.Roles},
			{Key: "version", Value: n.Version},
			{Key: "age", Value: fmtAge(n.Age)},
		},
	})
}

// visibleNodes returns the nodes slice after applying v.filter
// (case-insensitive substring match on name, status, roles, version).
func (v NodesView) visibleNodes() []resources.NodeItem {
	if v.filter == "" {
		return v.nodes
	}
	out := make([]resources.NodeItem, 0, len(v.nodes))
	for _, n := range v.nodes {
		if matches(v.filter, n.Name, n.Status, n.Roles, n.Version) {
			out = append(out, n)
		}
	}
	return out
}

func (v NodesView) rows() []components.Row {
	nodes := v.visibleNodes()
	rows := make([]components.Row, len(nodes))
	for i, n := range nodes {
		rows[i] = components.Row{
			n.Name,
			components.StatusPill(n.Status),
			n.Roles,
			n.Version,
			fmtAge(n.Age),
		}
	}
	return rows
}
