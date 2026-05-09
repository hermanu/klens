package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/manu/klens/k8s"
	"github.com/manu/klens/k8s/resources"
	"github.com/manu/klens/port"
	"github.com/manu/klens/ui/components"
	"github.com/manu/klens/ui/layout"
	"github.com/manu/klens/ui/theme"
)

var deploymentCols = []components.Column{
	{Header: "NAME", Width: 40},
	{Header: "READY", Width: 8},
	{Header: "UP-TO-DATE", Width: 12},
	{Header: "AVAILABLE", Width: 10},
	{Header: "AGE", Width: 10},
}

type DeploymentsView struct {
	svc         port.DeploymentService
	namespace   string
	deployments []resources.DeploymentItem
	table       components.Table
	err         error
	width       int
	height      int
}

func NewDeploymentsView(svc port.DeploymentService, namespace string) DeploymentsView {
	return DeploymentsView{
		svc:       svc,
		namespace: namespace,
		table:     components.NewTable(deploymentCols, nil),
	}
}

func (v DeploymentsView) Update(msg tea.Msg) (DeploymentsView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.DeploymentsUpdatedMsg:
		items, err := v.svc.ListDeployments(context.Background(), v.namespace)
		v.err = err
		if err == nil {
			v.deployments = items
			v.table = v.table.SetRows(deploymentRows(items))
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

func (v DeploymentsView) View() string {
	header := layout.Header(v.width, layout.HeaderConfig{
		Namespace: v.namespace,
		Count:     len(v.deployments),
		Total:     len(v.deployments),
		Watching:  true,
	})
	body := v.table.View()
	if v.err != nil {
		body = theme.Accent.Render("error: " + v.err.Error())
	}
	statusbar := layout.StatusBar(v.width, []layout.KeyHint{
		{Key: "j/k", Label: "navigate"},
		{Key: ":", Label: "command"},
	}, "deployments")
	return header + "\n" + body + statusbar
}

func deploymentRows(items []resources.DeploymentItem) []components.Row {
	rows := make([]components.Row, len(items))
	for i, d := range items {
		rows[i] = components.Row{
			d.Name,
			d.Ready,
			fmt.Sprintf("%d", d.UpToDate),
			fmt.Sprintf("%d", d.Available),
			fmtAge(d.Age),
		}
	}
	return rows
}
