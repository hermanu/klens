package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/k8s"
	"github.com/hermanu/klens/k8s/resources"
	"github.com/hermanu/klens/port"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/layout"
	"github.com/hermanu/klens/ui/theme"
)

var podCols = []components.Column{
	{Header: "NAME", Width: 40},
	{Header: "READY", Width: 7},
	{Header: "STATUS", Width: 22},
	{Header: "RESTARTS", Width: 9},
	{Header: "AGE", Width: 8},
	{Header: "NODE", Width: 24},
}

// PodsView is the pod list view.
type PodsView struct {
	svc       port.PodService
	namespace string
	pods      []resources.PodItem
	table     components.Table
	err       error
	width     int
	height    int
}

func NewPodsView(svc port.PodService, namespace string) PodsView {
	return PodsView{
		svc:       svc,
		namespace: namespace,
		table:     components.NewTable(podCols, nil),
	}
}

func (v PodsView) Update(msg tea.Msg) (PodsView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.PodsUpdatedMsg:
		pods, err := v.svc.ListPods(context.Background(), v.namespace)
		v.err = err
		if err == nil {
			v.pods = pods
			v.table = v.table.SetRows(podRows(pods))
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

func (v PodsView) SelectedPod() *resources.PodItem {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	for i := range v.pods {
		if v.pods[i].Name == row[0] {
			return &v.pods[i]
		}
	}
	return nil
}

func (v PodsView) View() string {
	header := layout.Header(v.width, layout.HeaderConfig{
		Namespace: v.namespace,
		Count:     len(v.pods),
		Total:     len(v.pods),
		Watching:  true,
	})
	statusbar := layout.StatusBar(v.width, []layout.KeyHint{
		{Key: "j/k", Label: "navigate"},
		{Key: "enter", Label: "detail"},
		{Key: "l", Label: "logs"},
		{Key: "d", Label: "delete"},
		{Key: ":", Label: "command"},
		{Key: "?", Label: "help"},
	}, "pods")

	body := v.table.View()
	if v.err != nil {
		body = theme.Accent.Render("error: " + v.err.Error())
	}
	return header + "\n" + body + statusbar
}

func podRows(pods []resources.PodItem) []components.Row {
	rows := make([]components.Row, len(pods))
	for i, p := range pods {
		color := theme.StatusColorFor(p.Status)
		statusCell := lipgloss.NewStyle().Foreground(color).Render(
			theme.GlyphFor(p.Status) + " " + p.Status,
		)
		rows[i] = components.Row{
			p.Name,
			p.Ready,
			statusCell,
			fmt.Sprintf("%d", p.Restarts),
			fmtAge(p.Age),
			p.Node,
		}
	}
	return rows
}
