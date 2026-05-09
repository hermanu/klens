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
	"github.com/hermanu/klens/ui/theme"
)

type configMapsMode int

const (
	configMapsModeList configMapsMode = iota
	configMapsModeEdit
)

var configMapCols = []components.Column{
	{Header: "NAME", Width: 44},
	{Header: "KEYS", Width: 6},
	{Header: "AGE", Width: 10},
}

// ConfigMapSavedMsg is sent after a configmap save attempt.
type ConfigMapSavedMsg struct {
	Name string
	Err  error
}

type configMapDetailMsg struct {
	item resources.ConfigMapItem
}

// ConfigMapsView handles listing and editing configmaps.
// ConfigMap data is map[string]string, so the form editor stores values as strings.
type ConfigMapsView struct {
	svc        port.ConfigMapService
	namespace  string
	configmaps []resources.ConfigMapItem
	table      components.Table
	form       components.Form
	current    *resources.ConfigMapItem
	mode       configMapsMode
	err        error
	saveMsg    string
	width      int
	height     int
}

func NewConfigMapsView(svc port.ConfigMapService, namespace string) ConfigMapsView {
	return ConfigMapsView{
		svc:       svc,
		namespace: namespace,
		table:     components.NewTable(configMapCols, nil),
	}
}

func (v ConfigMapsView) Update(msg tea.Msg) (ConfigMapsView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.ConfigMapsUpdatedMsg:
		items, err := v.svc.ListConfigMaps(context.Background(), v.namespace)
		v.err = err
		if err == nil {
			v.configmaps = items
			v.table = v.table.SetRows(configMapRows(items))
		}
		return v, nil

	case configMapDetailMsg:
		item := msg.item
		v.current = &item
		// Convert map[string]string to map[string][]byte for the form
		data := make(map[string][]byte, len(item.Data))
		for k, val := range item.Data {
			data[k] = []byte(val)
		}
		v.form = components.NewForm(data)
		v.mode = configMapsModeEdit
		v.saveMsg = ""
		return v, nil

	case ConfigMapSavedMsg:
		if msg.Err != nil {
			v.saveMsg = "error: " + msg.Err.Error()
		} else {
			v.saveMsg = fmt.Sprintf("saved %s", msg.Name)
			v.mode = configMapsModeList
			v.current = nil
		}
		return v, nil

	case tea.KeyMsg:
		if v.mode == configMapsModeEdit {
			return v.updateEdit(msg)
		}
		return v.updateList(msg)

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.table = v.table.SetWidth(msg.Width)
	}
	return v, nil
}

func (v ConfigMapsView) updateList(msg tea.KeyMsg) (ConfigMapsView, tea.Cmd) {
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
		return v.openEditor()
	}
	return v, nil
}

func (v ConfigMapsView) openEditor() (ConfigMapsView, tea.Cmd) {
	row := v.table.SelectedRow()
	if row == nil {
		return v, nil
	}
	name := row[0]
	svc := v.svc
	ns := v.namespace
	return v, func() tea.Msg {
		item, err := svc.GetConfigMap(context.Background(), ns, name)
		if err != nil {
			return ConfigMapSavedMsg{Name: name, Err: err}
		}
		return configMapDetailMsg{item: item}
	}
}

func (v ConfigMapsView) updateEdit(msg tea.KeyMsg) (ConfigMapsView, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.mode = configMapsModeList
		v.current = nil
		v.saveMsg = ""
		return v, nil
	case "ctrl+s":
		return v.saveConfigMap()
	case "ctrl+a":
		v.form = v.form.AddRow("", "")
		return v, nil
	}
	var cmd tea.Cmd
	v.form, cmd = v.form.Update(msg)
	return v, cmd
}

func (v ConfigMapsView) saveConfigMap() (ConfigMapsView, tea.Cmd) {
	if v.current == nil {
		return v, nil
	}
	name := v.current.Name
	rawData := v.form.Data()
	// Convert map[string][]byte back to map[string]string
	data := make(map[string]string, len(rawData))
	for k, val := range rawData {
		data[k] = string(val)
	}
	svc := v.svc
	ns := v.namespace
	return v, func() tea.Msg {
		err := svc.UpdateConfigMap(context.Background(), ns, name, data)
		return ConfigMapSavedMsg{Name: name, Err: err}
	}
}

func (v ConfigMapsView) View() string {
	if v.mode == configMapsModeEdit && v.current != nil {
		return v.viewEditor()
	}
	return v.viewList()
}

func (v ConfigMapsView) viewList() string {
	header := layout.Header(v.width, layout.HeaderConfig{
		Namespace: v.namespace,
		Count:     len(v.configmaps),
		Total:     len(v.configmaps),
		Watching:  true,
	})
	body := v.table.View()
	if v.err != nil {
		body = theme.Accent.Render("error: " + v.err.Error())
	}
	if v.saveMsg != "" {
		body = theme.Mid.Render(v.saveMsg) + "\n" + body
	}
	statusbar := layout.StatusBar(v.width, []layout.KeyHint{
		{Key: "j/k", Label: "navigate"},
		{Key: "enter", Label: "edit"},
		{Key: ":", Label: "command"},
	}, "configmaps")
	return header + "\n" + body + statusbar
}

func (v ConfigMapsView) viewEditor() string {
	title := theme.Base.Render(fmt.Sprintf("  ConfigMap: %s", v.current.Name)) +
		"  " + theme.Faint.Render("namespace: "+v.namespace) + "\n"

	body := v.form.View()

	notice := ""
	if v.saveMsg != "" {
		notice = "\n" + theme.Mid.Render(v.saveMsg)
	}

	statusbar := layout.StatusBar(v.width, []layout.KeyHint{
		{Key: "tab", Label: "next field"},
		{Key: "↑↓", Label: "row"},
		{Key: "ctrl+a", Label: "add key"},
		{Key: "ctrl+d", Label: "delete key"},
		{Key: "ctrl+s", Label: "save"},
		{Key: "esc", Label: "cancel"},
	}, "")

	return title + "\n" + body + notice + "\n" + statusbar
}

func configMapRows(items []resources.ConfigMapItem) []components.Row {
	rows := make([]components.Row, len(items))
	for i, c := range items {
		rows[i] = components.Row{
			c.Name,
			fmt.Sprintf("%d", c.Keys),
			fmtAge(c.Age),
		}
	}
	return rows
}
