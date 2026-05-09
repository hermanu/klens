package views

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/manu/klens/k8s"
	"github.com/manu/klens/k8s/resources"
	"github.com/manu/klens/port"
	"github.com/manu/klens/ui/components"
	"github.com/manu/klens/ui/layout"
	"github.com/manu/klens/ui/theme"
)

var namespaceCols = []components.Column{
	{Header: "NAME", Width: 44},
	{Header: "STATUS", Width: 12},
	{Header: "AGE", Width: 10},
}

type NamespacesView struct {
	svc        port.NamespaceService
	namespaces []resources.NamespaceItem
	table      components.Table
	err        error
	width      int
	height     int
}

func NewNamespacesView(svc port.NamespaceService) NamespacesView {
	return NamespacesView{
		svc:   svc,
		table: components.NewTable(namespaceCols, nil),
	}
}

func (v NamespacesView) Update(msg tea.Msg) (NamespacesView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.NamespacesUpdatedMsg:
		items, err := v.svc.ListNamespaces(context.Background())
		v.err = err
		if err == nil {
			v.namespaces = items
			v.table = v.table.SetRows(namespaceRows(items))
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

func (v NamespacesView) View() string {
	header := layout.Header(v.width, layout.HeaderConfig{
		Count:    len(v.namespaces),
		Total:    len(v.namespaces),
		Watching: true,
	})
	body := v.table.View()
	if v.err != nil {
		body = theme.Accent.Render("error: " + v.err.Error())
	}
	statusbar := layout.StatusBar(v.width, []layout.KeyHint{
		{Key: "j/k", Label: "navigate"},
		{Key: ":", Label: "command"},
	}, "namespaces")
	return header + "\n" + body + statusbar
}

func namespaceRows(items []resources.NamespaceItem) []components.Row {
	rows := make([]components.Row, len(items))
	for i, n := range items {
		rows[i] = components.Row{
			n.Name,
			n.Status,
			fmtAge(n.Age),
		}
	}
	return rows
}
