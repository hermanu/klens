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

var serviceCols = []components.Column{
	{Header: "NAMESPACE", Width: 14},
	{Header: colName, Width: 36, Flex: true},
	{Header: "TYPE", Width: 14},
	{Header: "CLUSTER-IP", Width: 16},
	{Header: "EXTERNAL-IP", Width: 18},
	{Header: "PORTS", Width: 22},
	{Header: colAge, Width: 6, Align: components.AlignRight},
}

// ServicesView lists services and supports `l` to fan out a multi-pod log
// tail across the service's selector-matched pods.
type ServicesView struct {
	svc       port.SvcService
	pods      port.PodService // for ListPodsForSelector on `l`
	namespace string
	services  []resources.ServiceItem
	table     components.Table
	filter    string
	err       error
}

// NewServicesView wires the service service plus a PodService used to resolve
// the service's pods on `l` (multi-pod log fan-out).
func NewServicesView(svc port.SvcService, pods port.PodService, namespace string) ServicesView {
	return ServicesView{
		svc:       svc,
		pods:      pods,
		namespace: namespace,
		table:     components.NewTable(serviceCols, nil),
	}
}

// servicesListedMsg carries the result of an async ListServices call.
type servicesListedMsg struct {
	items []resources.ServiceItem
	err   error
}

// servicePodsResolvedMsg carries the result of resolving a service's pods to
// the view, which then emits SwitchToLogsMsg + LogTailRequestMsg. Async to
// keep the Update loop from blocking on the API call.
type servicePodsResolvedMsg struct {
	namespace string
	title     string
	pods      []string
	err       error
}

// Update routes tea.Msg through the services view, handling watcher events
// and log fan-out for selector-matched pods.
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

	case servicePodsResolvedMsg:
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
			// Fan out a multi-pod log tail across this service's pods.
			idx := v.table.SelectedIndex()
			items := v.visibleServices()
			if idx >= len(items) || v.pods == nil {
				return v, nil
			}
			s := items[idx]
			ns, name, sel := s.Namespace, s.Name, s.Selector
			pods := v.pods
			return v, func() tea.Msg {
				items, err := pods.ListPodsForSelector(context.Background(), ns, sel)
				names := make([]string, 0, len(items))
				for _, p := range items {
					names = append(names, p.Name)
				}
				return servicePodsResolvedMsg{
					namespace: ns,
					title:     "service/" + name,
					pods:      names,
					err:       err,
				}
			}
		case keyEnter:
			// Drill-down to pods filtered by service name — services
			// typically share a prefix with the pods they front.
			idx := v.table.SelectedIndex()
			items := v.visibleServices()
			if idx < len(items) {
				name := items[idx].Name
				label := "service/" + name
				return v, func() tea.Msg { return DrillToPodsMsg{Filter: name, Label: label} }
			}
		}
	}
	return v, nil
}

// Title implements views.View.
func (v ServicesView) Title() string { return "services" }

// Filter implements views.Filterable.
func (v ServicesView) Filter() string { return v.filter }

// Count implements views.View.
func (v ServicesView) Count() (visible, total int) {
	return len(v.visibleServices()), len(v.services)
}

// CursorIndex implements views.Cursored.
func (v ServicesView) CursorIndex() int {
	if v.table.RowCount() == 0 {
		return 0
	}
	return v.table.SelectedIndex() + 1
}

// Chips implements views.View.
func (v ServicesView) Chips() []layout.FilterChip {
	chips := []layout.FilterChip{}
	if v.filter != "" {
		chips = append(chips, layout.FilterChip{Key: "/", Value: v.filter, Strong: true})
	}
	return chips
}

// KeyHints implements views.View. `l logs` is wired (multi-pod fan-out across
// the service selector). yaml/delete remain Soon entries.
func (v ServicesView) KeyHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "↵", Label: labelPods},
		{Key: "l", Label: labelLogs},
		{Key: "/", Label: labelFilter},
	}
}

// KeyMap implements views.KeyMap and powers the `?` help overlay.
func (v ServicesView) KeyMap() []components.KeySpec {
	return []components.KeySpec{
		{Key: "↵", Label: labelPods},
		{Key: "l", Label: labelLogs},
		{Key: "/", Label: labelFilter},
		{Key: "y", Label: labelYAML, Soon: true},
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

// Details implements views.View. Renders a uniform SPEC block driven by
// focusKVs() — type, ips, ports, selector, age — so the right pane stays
// informative without duplicating the table.
func (v ServicesView) Details(width, height int) string {
	s := v.focusedService()
	if s == nil {
		return ""
	}
	subtitle := s.Namespace
	if s.Type != "" {
		subtitle = fmt.Sprintf("%s · %s · %s", s.Namespace, s.Type, fmtAge(s.Age))
	}
	return layout.DefaultDetails(width, height, layout.DetailsBlock{
		Title:    s.Name,
		Subtitle: subtitle,
		KVs:      v.focusKVs(),
	})
}

// focusKVs returns the SPEC fields for the focused service. Empty fields are
// included where they have a meaningful "—" representation; selector is
// truncated so a noisy multi-label service doesn't blow out the SPEC pane.
func (v ServicesView) focusKVs() []layout.KV {
	s := v.focusedService()
	if s == nil {
		return nil
	}
	kvs := []layout.KV{
		{Key: "type", Value: fallbackOr(s.Type)},
		{Key: "cluster ip", Value: fallbackOr(s.ClusterIP)},
		{Key: "external", Value: fallbackOr(s.ExternalIP)},
	}
	if s.Ports != "" {
		kvs = append(kvs, layout.KV{Key: "ports", Value: s.Ports})
	}
	if sel := joinSelector(s.Selector); sel != "" {
		kvs = append(kvs, layout.KV{Key: "selector", Value: truncSelector(sel)})
	}
	kvs = append(kvs, layout.KV{Key: kvAge, Value: fmtAge(s.Age)})
	return kvs
}

// focusedService resolves the table cursor back to a ServiceItem in the
// post-filter slice.
func (v ServicesView) focusedService() *resources.ServiceItem {
	idx := v.table.SelectedIndex()
	visible := v.visibleServices()
	if idx < 0 || idx >= len(visible) {
		return nil
	}
	target := visible[idx]
	for i := range v.services {
		if v.services[i].Name == target.Name && v.services[i].Namespace == target.Namespace {
			return &v.services[i]
		}
	}
	return nil
}

// visibleServices returns the services slice after applying v.filter through
// matchesFields. Fields included: name, namespace, type, cluster IP, external
// IP, ports — every stringy column the user sees in the table.
func (v ServicesView) visibleServices() []resources.ServiceItem {
	if v.filter == "" {
		return v.services
	}
	out := make([]resources.ServiceItem, 0, len(v.services))
	for _, s := range v.services {
		if matchesFields(v.filter, s.Name, s.Namespace, s.Type, s.ClusterIP, s.ExternalIP, s.Ports) {
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
			highlightMatch(s.Name, v.filter),
			s.Type,
			highlightMatch(s.ClusterIP, v.filter),
			highlightMatch(s.ExternalIP, v.filter),
			s.Ports,
			fmtAge(s.Age),
		}
	}
	return rows
}
