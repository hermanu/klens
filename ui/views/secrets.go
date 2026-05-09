package views

import (
	"context"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hermanu/klens/k8s"
	"github.com/hermanu/klens/k8s/resources"
	"github.com/hermanu/klens/port"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/layout"
	"github.com/hermanu/klens/ui/theme"
)

type secretsMode int

const (
	secretsModeList secretsMode = iota
	secretsModeEdit
)

var secretCols = []components.Column{
	{Header: "NAME", Width: 40},
	{Header: "TYPE", Width: 28},
	{Header: "KEYS", Width: 6},
	{Header: "AGE", Width: 10},
}

// SecretsView handles listing secrets and editing them in-place via the form component.
type SecretsView struct {
	svc       port.SecretService
	namespace string
	secrets   []resources.SecretItem
	table     components.Table
	form      components.Form
	current   *resources.SecretItem
	mode      secretsMode
	err       error
	saveMsg   string
	width     int
	height    int
}

// secretDetailMsg carries a fetched SecretItem back to the view.
type secretDetailMsg struct {
	item resources.SecretItem
}

// SecretSavedMsg is sent after a save attempt (success or failure).
type SecretSavedMsg struct {
	Name string
	Err  error
}

func NewSecretsView(svc port.SecretService, namespace string) SecretsView {
	return SecretsView{
		svc:       svc,
		namespace: namespace,
		table:     components.NewTable(secretCols, nil),
	}
}

func (v SecretsView) Update(msg tea.Msg) (SecretsView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.SecretsUpdatedMsg:
		secs, err := v.svc.ListSecrets(context.Background(), v.namespace)
		v.err = err
		if err == nil {
			v.secrets = secs
			v.table = v.table.SetRows(secretRows(secs))
		}
		return v, nil

	case secretDetailMsg:
		item := msg.item
		v.current = &item
		v.form = components.NewForm(item.Data)
		v.mode = secretsModeEdit
		v.saveMsg = ""
		return v, nil

	case SecretSavedMsg:
		if msg.Err != nil {
			v.saveMsg = "error: " + msg.Err.Error()
		} else {
			v.saveMsg = fmt.Sprintf("saved %s", msg.Name)
			v.mode = secretsModeList
			v.current = nil
		}
		return v, nil

	case tea.KeyMsg:
		if v.mode == secretsModeEdit {
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

func (v SecretsView) updateList(msg tea.KeyMsg) (SecretsView, tea.Cmd) {
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

func (v SecretsView) openEditor() (SecretsView, tea.Cmd) {
	row := v.table.SelectedRow()
	if row == nil {
		return v, nil
	}
	name := row[0]
	svc := v.svc
	ns := v.namespace
	return v, func() tea.Msg {
		item, err := svc.GetSecret(context.Background(), ns, name)
		if err != nil {
			return SecretSavedMsg{Name: name, Err: err}
		}
		return secretDetailMsg{item: item}
	}
}

func (v SecretsView) updateEdit(msg tea.KeyMsg) (SecretsView, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.mode = secretsModeList
		v.current = nil
		v.saveMsg = ""
		return v, nil
	case "ctrl+s":
		return v.saveSecret()
	case "ctrl+a":
		v.form = v.form.AddRow("", "")
		return v, nil
	}
	var cmd tea.Cmd
	v.form, cmd = v.form.Update(msg)
	return v, cmd
}

func (v SecretsView) saveSecret() (SecretsView, tea.Cmd) {
	if v.current == nil {
		return v, nil
	}
	name := v.current.Name
	data := v.form.Data()
	svc := v.svc
	ns := v.namespace
	return v, func() tea.Msg {
		err := svc.UpdateSecret(context.Background(), ns, name, data)
		return SecretSavedMsg{Name: name, Err: err}
	}
}

func (v SecretsView) View() string {
	if v.mode == secretsModeEdit && v.current != nil {
		return v.viewEditor()
	}
	return v.viewList()
}

func (v SecretsView) viewList() string {
	header := layout.Header(v.width, layout.HeaderConfig{
		Namespace: v.namespace,
		Count:     len(v.secrets),
		Total:     len(v.secrets),
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
		{Key: "d", Label: "delete"},
		{Key: ":", Label: "command"},
	}, "secrets")
	return header + "\n" + body + statusbar
}

func (v SecretsView) viewEditor() string {
	title := theme.Base.Render(fmt.Sprintf("  Secret: %s", v.current.Name)) +
		"  " + theme.Faint.Render("namespace: "+v.namespace) + "\n"

	body := v.form.View()

	notice := ""
	if v.saveMsg != "" {
		notice = "\n" + lipgloss.NewStyle().Foreground(theme.ColorWarn).Render(v.saveMsg)
	}

	statusbar := layout.StatusBar(v.width, []layout.KeyHint{
		{Key: "tab", Label: "next field"},
		{Key: "↑↓", Label: "row"},
		{Key: "ctrl+a", Label: "add key"},
		{Key: "ctrl+d", Label: "delete key"},
		{Key: "ctrl+h", Label: "toggle hide"},
		{Key: "ctrl+s", Label: "save"},
		{Key: "esc", Label: "cancel"},
	}, "")

	return title + "\n" + body + notice + "\n" + statusbar
}

func secretRows(secs []resources.SecretItem) []components.Row {
	rows := make([]components.Row, len(secs))
	for i, s := range secs {
		rows[i] = components.Row{
			s.Name,
			s.Type,
			fmt.Sprintf("%d", s.Keys),
			fmtAge(s.Age),
		}
	}
	return rows
}
