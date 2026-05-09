package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hermanu/klens/k8s"
	"github.com/hermanu/klens/k8s/resources"
	"github.com/hermanu/klens/port"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/layout"
	"github.com/hermanu/klens/ui/theme"
)

var serviceCols = []components.Column{
	{Header: "NAME", Width: 36},
	{Header: "TYPE", Width: 14},
	{Header: "CLUSTER-IP", Width: 16},
	{Header: "EXTERNAL-IP", Width: 18},
	{Header: "PORTS", Width: 22},
	{Header: "AGE", Width: 8},
}

type ServicesView struct {
	svc       port.SvcService
	namespace string
	services  []resources.ServiceItem
	table     components.Table
	err       error
	width     int
	height    int
}

func NewServicesView(svc port.SvcService, namespace string) ServicesView {
	return ServicesView{
		svc:       svc,
		namespace: namespace,
		table:     components.NewTable(serviceCols, nil),
	}
}

func (v ServicesView) Update(msg tea.Msg) (ServicesView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.ServicesUpdatedMsg:
		items, err := v.svc.ListServices(context.Background(), v.namespace)
		v.err = err
		if err == nil {
			v.services = items
			v.table = v.table.SetRows(serviceRows(items))
		}
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
		}

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.table = v.table.SetWidth(msg.Width)
	}
	return v, nil
}

func (v ServicesView) View() string {
	header := layout.Header(v.width, layout.HeaderConfig{
		Namespace: v.namespace,
		Count:     len(v.services),
		Total:     len(v.services),
		Watching:  true,
	})
	body := v.table.View()
	if v.err != nil {
		body = theme.Accent.Render("error: " + v.err.Error())
	}
	statusbar := layout.StatusBar(v.width, []layout.KeyHint{
		{Key: "j/k", Label: "navigate"},
		{Key: ":", Label: "command"},
	}, "services")
	return header + "\n" + body + statusbar
}

func serviceRows(items []resources.ServiceItem) []components.Row {
	rows := make([]components.Row, len(items))
	for i, s := range items {
		rows[i] = components.Row{
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
