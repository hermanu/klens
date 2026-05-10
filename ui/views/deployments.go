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

var deploymentCols = []components.Column{
	{Header: "NAMESPACE", Width: 14},
	{Header: "NAME", Width: 36, Flex: true},
	{Header: "READY", Width: 8, Align: components.AlignRight},
	{Header: "UP-TO-DATE", Width: 12, Align: components.AlignRight},
	{Header: "AVAILABLE", Width: 10, Align: components.AlignRight},
	{Header: "AGE", Width: 6, Align: components.AlignRight},
}

// DeploymentsView lists deployments and supports `l` to fan out a multi-pod
// log tail across the deployment's matching pods (resolved via the spec
// selector).
type DeploymentsView struct {
	svc         port.DeploymentService
	pods        port.PodService // for ListPodsForSelector on `l`
	namespace   string
	deployments []resources.DeploymentItem
	table       components.Table
	filter      string
	err         error
}

// NewDeploymentsView wires the deployment service plus a PodService used to
// resolve the deployment's pods on `l` (multi-pod log fan-out).
func NewDeploymentsView(svc port.DeploymentService, pods port.PodService, namespace string) DeploymentsView {
	return DeploymentsView{
		svc:       svc,
		pods:      pods,
		namespace: namespace,
		table:     components.NewTable(deploymentCols, nil),
	}
}

// deploymentsListedMsg carries the result of an async ListDeployments call.
type deploymentsListedMsg struct {
	items []resources.DeploymentItem
	err   error
}

// deploymentPodsResolvedMsg carries the result of resolving a deployment's
// pods (label selector lookup) back to the view, which then emits the pair of
// SwitchToLogsMsg + LogTailRequestMsg for the resolved set. Async because the
// k8s API call must not block the Update loop.
type deploymentPodsResolvedMsg struct {
	namespace string
	title     string
	pods      []string
	err       error
}

func (v DeploymentsView) Update(msg tea.Msg) (DeploymentsView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.DeploymentsUpdatedMsg:
		ns := v.namespace
		svc := v.svc
		return v, func() tea.Msg {
			items, err := svc.ListDeployments(context.Background(), ns)
			return deploymentsListedMsg{items: items, err: err}
		}

	case deploymentsListedMsg:
		v.err = msg.err
		if msg.err == nil {
			v.deployments = msg.items
			v.table = v.table.SetRows(v.rows())
		}
		return v, nil

	case FilterMsg:
		v.filter = msg.Query
		v.table = v.table.SetRows(v.rows())
		return v, nil

	case NamespaceChangedMsg:
		v.namespace = msg.Namespace
		v.deployments = nil
		v.table = v.table.SetRows(nil)
		return v, nil

	case deploymentPodsResolvedMsg:
		// Empty resolution (selector matched nothing or err) — silently skip
		// the focus switch so the user stays on the deployment list. The
		// alternative (focus an empty logs view) feels broken.
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
		case "j", "down":
			v.table = v.table.MoveDown()
		case "k", "up":
			v.table = v.table.MoveUp()
		case "g":
			v.table = v.table.MoveTop()
		case "G":
			v.table = v.table.MoveBottom()
		case "l":
			// Fan out a multi-pod log tail across this deployment's pods.
			// Selector lookup is async to avoid blocking the Update loop on
			// the k8s API call.
			idx := v.table.SelectedIndex()
			items := v.visibleDeployments()
			if idx >= len(items) || v.pods == nil {
				return v, nil
			}
			d := items[idx]
			ns, name, sel := d.Namespace, d.Name, d.Selector
			pods := v.pods
			return v, func() tea.Msg {
				items, err := pods.ListPodsForSelector(context.Background(), ns, sel)
				names := make([]string, 0, len(items))
				for _, p := range items {
					names = append(names, p.Name)
				}
				return deploymentPodsResolvedMsg{
					namespace: ns,
					title:     "deployment/" + name,
					pods:      names,
					err:       err,
				}
			}
		case "enter":
			// k9s-style drill-down: switch to pods filtered by this
			// deployment's name. Pod names usually start with the deployment
			// name, so a substring filter finds the workload pods reliably.
			idx := v.table.SelectedIndex()
			items := v.visibleDeployments()
			if idx < len(items) {
				name := items[idx].Name
				return v, func() tea.Msg { return DrillToPodsMsg{Filter: name} }
			}
		}
	}
	return v, nil
}

// Title implements views.View.
func (v DeploymentsView) Title() string { return "deployments" }

// Filter implements views.Filterable.
func (v DeploymentsView) Filter() string { return v.filter }

// Count implements views.View.
func (v DeploymentsView) Count() (visible, total int) {
	return len(v.visibleDeployments()), len(v.deployments)
}

// Chips implements views.View.
func (v DeploymentsView) Chips() []layout.FilterChip {
	chips := []layout.FilterChip{}
	if v.filter != "" {
		chips = append(chips, layout.FilterChip{Key: "/", Value: v.filter, Strong: true})
	}
	return chips
}

// KeyHints implements views.View. `l logs` is now wired (multi-pod fan-out
// across the deployment's selector) so it's promoted to the working hint set;
// yaml/delete stay as Soon entries in the help overlay.
func (v DeploymentsView) KeyHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "↵", Label: "pods"},
		{Key: "l", Label: "logs"},
		{Key: "/", Label: "filter"},
	}
}

// KeyMap implements views.KeyMap and powers the `?` help overlay.
func (v DeploymentsView) KeyMap() []components.KeySpec {
	return []components.KeySpec{
		{Key: "↵", Label: "pods"},
		{Key: "l", Label: "logs"},
		{Key: "/", Label: "filter"},
		{Key: "y", Label: "yaml", Soon: true},
		{Key: "d", Label: "delete", Soon: true},
	}
}

// Table implements views.View.
func (v DeploymentsView) Table(width, height int) string {
	v.table = v.table.SetWidth(width).SetHeight(height)
	if v.err != nil {
		return "error: " + v.err.Error()
	}
	return v.table.View()
}

// Details implements views.View. Renders a uniform SPEC block driven by
// focusKVs() so the right pane stays informative across non-pod views.
func (v DeploymentsView) Details(width, height int) string {
	d := v.focusedDeployment()
	if d == nil {
		return ""
	}
	subtitle := d.Namespace
	if d.Ready != "" {
		subtitle = fmt.Sprintf("%s · %s · %s", d.Namespace, d.Ready, fmtAge(d.Age))
	}
	return layout.DefaultDetails(width, height, layout.DetailsBlock{
		Title:    d.Name,
		Subtitle: subtitle,
		KVs:      v.focusKVs(),
	})
}

// focusKVs returns the SPEC fields for the focused deployment. Fields chosen
// to match what `kubectl describe deployment` puts at the top: image, replicas
// (R/D updated:U avail:A), strategy, selector, age. Empty rows are omitted so
// the pane doesn't render dashes for "selectorless" deployments.
func (v DeploymentsView) focusKVs() []layout.KV {
	d := v.focusedDeployment()
	if d == nil {
		return nil
	}
	kvs := []layout.KV{}
	if d.Image != "" {
		kvs = append(kvs, layout.KV{Key: "image", Value: d.Image})
	}
	kvs = append(kvs, layout.KV{
		Key:   "replicas",
		Value: fmt.Sprintf("%s updated:%d avail:%d", d.Ready, d.UpToDate, d.Available),
	})
	if d.Strategy != "" {
		kvs = append(kvs, layout.KV{Key: "strategy", Value: d.Strategy})
	}
	if sel := joinSelector(d.Selector); sel != "" {
		kvs = append(kvs, layout.KV{Key: "selector", Value: truncSelector(sel)})
	}
	kvs = append(kvs, layout.KV{Key: "age", Value: fmtAge(d.Age)})
	return kvs
}

// focusedDeployment resolves the table cursor back to a DeploymentItem in the
// post-filter slice.
func (v DeploymentsView) focusedDeployment() *resources.DeploymentItem {
	idx := v.table.SelectedIndex()
	visible := v.visibleDeployments()
	if idx < 0 || idx >= len(visible) {
		return nil
	}
	target := visible[idx]
	for i := range v.deployments {
		if v.deployments[i].Name == target.Name && v.deployments[i].Namespace == target.Namespace {
			return &v.deployments[i]
		}
	}
	return nil
}

// joinSelector renders a label-selector map as `k=v,k=v` with sorted keys so
// the SPEC line is stable across renders.
func joinSelector(sel map[string]string) string {
	if len(sel) == 0 {
		return ""
	}
	keys := make([]string, 0, len(sel))
	for k := range sel {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+sel[k])
	}
	return strings.Join(parts, ",")
}

// truncSelector clamps the selector preview to 40 cols so a noisy multi-label
// deployment doesn't blow out the SPEC pane width.
func truncSelector(s string) string {
	const max = 40
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// visibleDeployments returns the deployments slice after applying v.filter
// through matchesFields. Fields included: name, namespace, ready (status as
// the user sees it: e.g. "2/3"), image, strategy — every stringy field that
// shows up in the table or SPEC pane so the filter feels predictable.
func (v DeploymentsView) visibleDeployments() []resources.DeploymentItem {
	if v.filter == "" {
		return v.deployments
	}
	out := make([]resources.DeploymentItem, 0, len(v.deployments))
	for _, d := range v.deployments {
		if matchesFields(v.filter, d.Name, d.Namespace, d.Ready, d.Image, d.Strategy) {
			out = append(out, d)
		}
	}
	return out
}

func (v DeploymentsView) rows() []components.Row {
	items := v.visibleDeployments()
	rows := make([]components.Row, len(items))
	for i, d := range items {
		rows[i] = components.Row{
			components.NSChip(d.Namespace),
			highlightMatch(d.Name, v.filter),
			d.Ready,
			fmt.Sprintf("%d", d.UpToDate),
			fmt.Sprintf("%d", d.Available),
			fmtAge(d.Age),
		}
	}
	return rows
}
