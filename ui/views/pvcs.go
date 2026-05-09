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

var pvcCols = []components.Column{
	{Header: "NAME", Width: 32},
	{Header: "STATUS", Width: 10},
	{Header: "VOLUME", Width: 24},
	{Header: "CAPACITY", Width: 10},
	{Header: "ACCESS MODES", Width: 16},
	{Header: "STORAGECLASS", Width: 16},
	{Header: "AGE", Width: 8},
}

type PVCsView struct {
	svc       port.PVCService
	namespace string
	pvcs      []resources.PVCItem
	table     components.Table
	err       error
	width     int
	height    int
}

func NewPVCsView(svc port.PVCService, namespace string) PVCsView {
	return PVCsView{
		svc:       svc,
		namespace: namespace,
		table:     components.NewTable(pvcCols, nil),
	}
}

func (v PVCsView) Update(msg tea.Msg) (PVCsView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.PVCsUpdatedMsg:
		items, err := v.svc.ListPVCs(context.Background(), v.namespace)
		v.err = err
		if err == nil {
			v.pvcs = items
			v.table = v.table.SetRows(pvcRows(items))
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

func (v PVCsView) View() string {
	header := layout.Header(v.width, layout.HeaderConfig{
		Namespace: v.namespace,
		Count:     len(v.pvcs),
		Total:     len(v.pvcs),
		Watching:  true,
	})
	body := v.table.View()
	if v.err != nil {
		body = theme.Accent.Render("error: " + v.err.Error())
	}
	statusbar := layout.StatusBar(v.width, []layout.KeyHint{
		{Key: "j/k", Label: "navigate"},
		{Key: ":", Label: "command"},
	}, "pvcs")
	return header + "\n" + body + statusbar
}

func pvcRows(items []resources.PVCItem) []components.Row {
	rows := make([]components.Row, len(items))
	for i, p := range items {
		rows[i] = components.Row{
			p.Name,
			p.Status,
			p.Volume,
			p.Capacity,
			p.AccessModes,
			p.StorageClass,
			fmtAge(p.Age),
		}
	}
	return rows
}
