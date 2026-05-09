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

var deploymentCols = []components.Column{
	{Header: "NAMESPACE", Width: 14},
	{Header: "NAME", Width: 36},
	{Header: "READY", Width: 8, Align: components.AlignRight},
	{Header: "UP-TO-DATE", Width: 12, Align: components.AlignRight},
	{Header: "AVAILABLE", Width: 10, Align: components.AlignRight},
	{Header: "AGE", Width: 6, Align: components.AlignRight},
}

type DeploymentsView struct {
	svc         port.DeploymentService
	namespace   string
	deployments []resources.DeploymentItem
	table       components.Table
	filter      string
	err         error
}

func NewDeploymentsView(svc port.DeploymentService, namespace string) DeploymentsView {
	return DeploymentsView{
		svc:       svc,
		namespace: namespace,
		table:     components.NewTable(deploymentCols, nil),
	}
}

// deploymentsListedMsg carries the result of an async ListDeployments call.
type deploymentsListedMsg struct {
	items []resources.DeploymentItem
	err   error
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

// KeyHints implements views.View. Advertises only keys with a live handler in
// Update. `l logs` and YAML/delete are reserved for upcoming waves and live in
// KeyMap below as `Soon` so the `?` overlay can tease them honestly without
// the command bar lying about today's keymap.
func (v DeploymentsView) KeyHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "↵", Label: "pods"},
		{Key: "/", Label: "filter"},
	}
}

// KeyMap implements views.KeyMap and powers the `?` help overlay.
func (v DeploymentsView) KeyMap() []components.KeySpec {
	return []components.KeySpec{
		{Key: "↵", Label: "pods"},
		{Key: "l", Label: "logs", Soon: true},
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

// Details implements views.View.
func (v DeploymentsView) Details(width, height int) string {
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
func (v DeploymentsView) cols() []components.Column { return deploymentCols }

// visibleDeployments returns the deployments slice after applying v.filter
// (case-insensitive substring match on name + namespace).
func (v DeploymentsView) visibleDeployments() []resources.DeploymentItem {
	if v.filter == "" {
		return v.deployments
	}
	out := make([]resources.DeploymentItem, 0, len(v.deployments))
	for _, d := range v.deployments {
		if matches(v.filter, d.Name, d.Namespace) {
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
			d.Name,
			d.Ready,
			fmt.Sprintf("%d", d.UpToDate),
			fmt.Sprintf("%d", d.Available),
			fmtAge(d.Age),
		}
	}
	return rows
}
