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

var nodeCols = []components.Column{
	{Header: "NAME", Width: 36},
	{Header: "STATUS", Width: 10},
	{Header: "ROLES", Width: 18},
	{Header: "VERSION", Width: 16},
	{Header: "AGE", Width: 8},
}

type NodesView struct {
	svc    port.NodeService
	nodes  []resources.NodeItem
	table  components.Table
	err    error
	width  int
	height int
}

func NewNodesView(svc port.NodeService) NodesView {
	return NodesView{
		svc:   svc,
		table: components.NewTable(nodeCols, nil),
	}
}

func (v NodesView) Update(msg tea.Msg) (NodesView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.NodesUpdatedMsg:
		items, err := v.svc.ListNodes(context.Background())
		v.err = err
		if err == nil {
			v.nodes = items
			v.table = v.table.SetRows(nodeRows(items))
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

func (v NodesView) View() string {
	header := layout.Header(v.width, layout.HeaderConfig{
		Count:    len(v.nodes),
		Total:    len(v.nodes),
		Watching: true,
	})
	body := v.table.View()
	if v.err != nil {
		body = theme.Accent.Render("error: " + v.err.Error())
	}
	statusbar := layout.StatusBar(v.width, []layout.KeyHint{
		{Key: "j/k", Label: "navigate"},
		{Key: ":", Label: "command"},
	}, "nodes")
	return header + "\n" + body + statusbar
}

func nodeRows(items []resources.NodeItem) []components.Row {
	rows := make([]components.Row, len(items))
	for i, n := range items {
		rows[i] = components.Row{
			n.Name,
			n.Status,
			n.Roles,
			n.Version,
			fmtAge(n.Age),
		}
	}
	return rows
}
