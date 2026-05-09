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

var serviceCols = []components.Column{
	{Header: "NAMESPACE", Width: 14},
	{Header: "NAME", Width: 36},
	{Header: "TYPE", Width: 14},
	{Header: "CLUSTER-IP", Width: 16},
	{Header: "EXTERNAL-IP", Width: 18},
	{Header: "PORTS", Width: 22},
	{Header: "AGE", Width: 6, Align: components.AlignRight},
}

type ServicesView struct {
	svc       port.SvcService
	namespace string
	services  []resources.ServiceItem
	table     components.Table
	filter    string
	err       error
}

func NewServicesView(svc port.SvcService, namespace string) ServicesView {
	return ServicesView{
		svc:       svc,
		namespace: namespace,
		table:     components.NewTable(serviceCols, nil),
	}
}

// servicesListedMsg carries the result of an async ListServices call.
type servicesListedMsg struct {
	items []resources.ServiceItem
	err   error
}

func (v ServicesView) Update(msg tea.Msg) (ServicesView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.ServicesUpdatedMsg:
		ns := v.namespace
		svc := v.svc
		return v, func() tea.Msg {
			items, err := svc.ListServices(context.Background(), ns)
			return servicesListedMsg{items: items, err: err}
		}

	case servicesListedMsg:
		v.err = msg.err
		if msg.err == nil {
			v.services = msg.items
			v.table = v.table.SetRows(v.rows())
		}
		return v, nil

	case FilterMsg:
		v.filter = msg.Query
		v.table = v.table.SetRows(v.rows())
		return v, nil

	case NamespaceChangedMsg:
		v.namespace = msg.Namespace
		v.services = nil
		v.table = v.table.SetRows(nil)
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
			// Drill-down to pods filtered by service name — services
			// typically share a prefix with the pods they front.
			idx := v.table.SelectedIndex()
			items := v.visibleServices()
			if idx < len(items) {
				name := items[idx].Name
				return v, func() tea.Msg { return DrillToPodsMsg{Filter: name} }
			}
		}
	}
	return v, nil
}

// Title implements views.View.
func (v ServicesView) Title() string { return "services" }

// Count implements views.View.
func (v ServicesView) Count() (visible, total int) {
	return len(v.visibleServices()), len(v.services)
}

// Chips implements views.View.
func (v ServicesView) Chips() []layout.FilterChip {
	chips := []layout.FilterChip{}
	if v.filter != "" {
		chips = append(chips, layout.FilterChip{Key: "/", Value: v.filter, Strong: true})
	}
	return chips
}

// KeyHints implements views.View. Only Enter (drill to pods) and `/` are
// wired today — advertise nothing else in the command bar.
func (v ServicesView) KeyHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "↵", Label: "pods"},
		{Key: "/", Label: "filter"},
	}
}

// KeyMap implements views.KeyMap and powers the `?` help overlay.
func (v ServicesView) KeyMap() []components.KeySpec {
	return []components.KeySpec{
		{Key: "↵", Label: "pods"},
		{Key: "l", Label: "logs", Soon: true},
		{Key: "/", Label: "filter"},
		{Key: "y", Label: "yaml", Soon: true},
		{Key: "d", Label: "delete", Soon: true},
	}
}

// Table implements views.View.
func (v ServicesView) Table(width, height int) string {
	v.table = v.table.SetWidth(width).SetHeight(height)
	if v.err != nil {
		return "error: " + v.err.Error()
	}
	return v.table.View()
}

// Details implements views.View.
func (v ServicesView) Details(width, height int) string {
	row := v.table.SelectedRow()
	if row == nil {
		return ""
	}
	title := ""
	// Column 1 holds the raw NAME (column 0 is the namespace chip with ANSI).
	if len(row) > 1 {
		title = row[1]
	}
	return layout.DefaultDetails(width, height, layout.DetailsBlock{
		Title: title,
		KVs:   kvFromRow(v.cols(), row),
	})
}

// cols exposes the column slice so kvFromRow can resolve headers.
func (v ServicesView) cols() []components.Column { return serviceCols }

// visibleServices returns the services slice after applying v.filter
// (case-insensitive substring match on name + namespace + type).
func (v ServicesView) visibleServices() []resources.ServiceItem {
	if v.filter == "" {
		return v.services
	}
	out := make([]resources.ServiceItem, 0, len(v.services))
	for _, s := range v.services {
		if matches(v.filter, s.Name, s.Namespace, s.Type) {
			out = append(out, s)
		}
	}
	return out
}

func (v ServicesView) rows() []components.Row {
	items := v.visibleServices()
	rows := make([]components.Row, len(items))
	for i, s := range items {
		rows[i] = components.Row{
			components.NSChip(s.Namespace),
			s.Name,
			s.Type,
			s.ClusterIP,
			s.ExternalIP,
			s.Ports,
			fmtAge(s.Age),
		}
	}
	return rows
}
