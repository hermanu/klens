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

var namespaceCols = []components.Column{
	{Header: "NAME", Width: 44},
	{Header: "STATUS", Width: 14},
	{Header: "AGE", Width: 6, Align: components.AlignRight},
}

type NamespacesView struct {
	svc        port.NamespaceService
	namespaces []resources.NamespaceItem
	table      components.Table
	filter     string
	err        error
}

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
		case "j", "down":
			v.table = v.table.MoveDown()
		case "k", "up":
			v.table = v.table.MoveUp()
		case "g":
			v.table = v.table.MoveTop()
		case "G":
			v.table = v.table.MoveBottom()
		case "enter":
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
		{Key: "/", Label: "filter"},
	}
}

// KeyMap implements views.KeyMap and powers the `?` help overlay.
func (v NamespacesView) KeyMap() []components.KeySpec {
	return []components.KeySpec{
		{Key: "↵", Label: "select"},
		{Key: "/", Label: "filter"},
		{Key: "y", Label: "yaml", Soon: true},
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

// Details implements views.View.
func (v NamespacesView) Details(width, height int) string {
	row := v.table.SelectedRow()
	if row == nil {
		return ""
	}
	title := ""
	if len(row) > 0 {
		title = row[0]
	}
	return layout.DefaultDetails(width, height, layout.DetailsBlock{
		Title: title,
		KVs:   kvFromRow(v.cols(), row),
	})
}

// cols exposes the column slice so kvFromRow can resolve headers.
func (v NamespacesView) cols() []components.Column { return namespaceCols }

// visibleNamespaces returns the namespaces slice after applying v.filter
// (case-insensitive substring match on name + status).
func (v NamespacesView) visibleNamespaces() []resources.NamespaceItem {
	if v.filter == "" {
		return v.namespaces
	}
	out := make([]resources.NamespaceItem, 0, len(v.namespaces))
	for _, n := range v.namespaces {
		if matches(v.filter, n.Name, n.Status) {
			out = append(out, n)
		}
	}
	return out
}

func (v NamespacesView) rows() []components.Row {
	items := v.visibleNamespaces()
	rows := make([]components.Row, len(items))
	for i, n := range items {
		rows[i] = components.Row{
			n.Name,
			components.StatusPill(n.Status),
			fmtAge(n.Age),
		}
	}
	return rows
}
