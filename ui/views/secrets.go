package views

import (
	"context"
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// Column 0 is NAMESPACE so the row layout matches the other modern shell views
// (pods/services/deployments). openEditor and selected() resolve the secret by
// table index, not by reading row[0], to stay agnostic of the rendered NSChip.
var secretCols = []components.Column{
	{Header: "NAMESPACE", Width: 14},
	{Header: "NAME", Width: 36},
	{Header: "TYPE", Width: 28},
	{Header: "KEYS", Width: 6, Align: components.AlignRight},
	{Header: "AGE", Width: 6, Align: components.AlignRight},
}

// SecretsView lists secrets and edits them in-place via the form component.
//
// In edit mode, Table() returns the form body instead of the row table — the
// shell still draws chrome (top bar / chips / command bar) around it.
type SecretsView struct {
	svc       port.SecretService
	namespace string
	secrets   []resources.SecretItem
	table     components.Table
	form      components.Form
	current   *resources.SecretItem
	mode      secretsMode
	filter    string
	err       error
	saveMsg   string
}

// secretDetailMsg carries a fetched SecretItem back to the view.
type secretDetailMsg struct {
	item resources.SecretItem
}

// secretsListedMsg carries the result of an async ListSecrets call.
type secretsListedMsg struct {
	items []resources.SecretItem
	err   error
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
		ns := v.namespace
		svc := v.svc
		return v, func() tea.Msg {
			items, err := svc.ListSecrets(context.Background(), ns)
			return secretsListedMsg{items: items, err: err}
		}

	case secretsListedMsg:
		v.err = msg.err
		if msg.err == nil {
			v.secrets = msg.items
			v.table = v.table.SetRows(v.rows())
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

	case FilterMsg:
		v.filter = msg.Query
		v.table = v.table.SetRows(v.rows())
		return v, nil

	case NamespaceChangedMsg:
		v.namespace = msg.Namespace
		v.secrets = nil
		v.mode = secretsModeList
		v.current = nil
		v.table = v.table.SetRows(nil)
		return v, nil

	case tea.KeyMsg:
		if v.mode == secretsModeEdit {
			return v.updateEdit(msg)
		}
		return v.updateList(msg)
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

// openEditor resolves the focused row to a SecretItem by index (cell 0 is the
// NSChip with ANSI codes, so reading the raw values from v.secrets is the only
// reliable lookup).
func (v SecretsView) openEditor() (SecretsView, tea.Cmd) {
	sec := v.selected()
	if sec == nil {
		return v, nil
	}
	name := sec.Name
	ns := sec.Namespace
	svc := v.svc
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

// selected resolves the table cursor back to a SecretItem from the visible
// (post-filter) slice, then maps it to the underlying v.secrets entry so the
// edit flow operates on the canonical record.
func (v SecretsView) selected() *resources.SecretItem {
	idx := v.table.SelectedIndex()
	visible := v.visibleSecrets()
	if idx < 0 || idx >= len(visible) {
		return nil
	}
	s := visible[idx]
	for i := range v.secrets {
		if v.secrets[i].Name == s.Name && v.secrets[i].Namespace == s.Namespace {
			return &v.secrets[i]
		}
	}
	return nil
}

// Title implements views.View.
func (v SecretsView) Title() string { return "secrets" }

// Count implements views.View.
func (v SecretsView) Count() (visible, total int) {
	return len(v.visibleSecrets()), len(v.secrets)
}

// Chips implements views.View.
func (v SecretsView) Chips() []layout.FilterChip {
	chips := []layout.FilterChip{}
	if v.mode == secretsModeEdit {
		chips = append(chips, layout.FilterChip{Key: "mode", Value: "edit", Strong: true})
	}
	if v.filter != "" {
		chips = append(chips, layout.FilterChip{Key: "/", Value: v.filter, Strong: true})
	}
	return chips
}

// KeyHints implements views.View — hints differ between list and edit mode so
// the command bar always reflects what the focused pane actually does.
// In list mode only Enter (open editor) and `/` are wired; yaml/delete live
// in KeyMap as Soon entries.
func (v SecretsView) KeyHints() []layout.KeyHint {
	if v.mode == secretsModeEdit {
		return []layout.KeyHint{
			{Key: "ctrl+s", Label: "save"},
			{Key: "esc", Label: "cancel"},
			{Key: "tab", Label: "next field"},
		}
	}
	return []layout.KeyHint{
		{Key: "↵", Label: "edit"},
		{Key: "/", Label: "filter"},
	}
}

// KeyMap implements views.KeyMap and powers the `?` help overlay. In edit
// mode it returns the editor keymap so the overlay matches the focused pane.
func (v SecretsView) KeyMap() []components.KeySpec {
	if v.mode == secretsModeEdit {
		return []components.KeySpec{
			{Key: "ctrl+s", Label: "save"},
			{Key: "esc", Label: "cancel"},
			{Key: "tab", Label: "next field"},
			{Key: "ctrl+a", Label: "add row"},
		}
	}
	return []components.KeySpec{
		{Key: "↵", Label: "edit"},
		{Key: "/", Label: "filter"},
		{Key: "y", Label: "yaml", Soon: true},
		{Key: "d", Label: "delete", Soon: true},
	}
}

// Table implements views.View. In edit mode it returns the form body so the
// shell can swap the central pane without re-routing render calls.
func (v SecretsView) Table(width, height int) string {
	if v.mode == secretsModeEdit && v.current != nil {
		return v.formView()
	}
	v.table = v.table.SetWidth(width).SetHeight(height)
	if v.err != nil {
		return "error: " + v.err.Error()
	}
	return v.table.View()
}

// Details implements views.View. The pane stays informative in both list and
// edit mode — showing the selected secret's type / keys count / preview keys
// / age. The detail view never shows values, that's what edit-mode is for.
func (v SecretsView) Details(width, height int) string {
	sec := v.selected()
	// In edit mode v.current is the source of truth (cursor may have moved).
	if v.mode == secretsModeEdit && v.current != nil {
		sec = v.current
	}
	if sec == nil {
		return ""
	}
	return layout.DefaultDetails(width, height, layout.DetailsBlock{
		Title:    sec.Name,
		Subtitle: fmt.Sprintf("%s · %s", sec.Namespace, fmtAge(sec.Age)),
		KVs:      v.focusKVs(),
	})
}

// focusKVs returns the SPEC fields for the focused secret in list mode. Top-5
// alphabetical key names are rendered as "  · name" rows so the user sees what
// keys live in the secret without opening the editor. Values intentionally
// stay out of the SPEC block — exposing them belongs to edit mode.
func (v SecretsView) focusKVs() []layout.KV {
	sec := v.selected()
	if v.mode == secretsModeEdit && v.current != nil {
		sec = v.current
	}
	if sec == nil {
		return nil
	}
	// In edit mode we have the decoded Data map; in list mode KeyNames is
	// the cheap pre-sorted preview populated by ListSecrets.
	keys := sec.KeyNames
	if len(keys) == 0 && len(sec.Data) > 0 {
		keys = make([]string, 0, len(sec.Data))
		for k := range sec.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}

	kvs := []layout.KV{
		{Key: "type", Value: fallbackOr(sec.Type)},
		{Key: "keys", Value: fmt.Sprintf("%d", sec.Keys)},
	}
	for i, k := range keys {
		if i >= 5 {
			break
		}
		kvs = append(kvs, layout.KV{Key: "  · " + k, Value: ""})
	}
	kvs = append(kvs, layout.KV{Key: "age", Value: fmtAge(sec.Age)})
	return kvs
}

// formView renders just the editor body (title row + form + optional save
// notice). The shell renders header/status chrome around it.
func (v SecretsView) formView() string {
	title := theme.Base.Render(fmt.Sprintf("  Secret: %s", v.current.Name)) +
		"  " + theme.Faint.Render("namespace: "+v.namespace) + "\n"

	body := v.form.View()

	notice := ""
	if v.saveMsg != "" {
		notice = "\n" + lipgloss.NewStyle().Foreground(theme.ColorWarn).Render(v.saveMsg)
	}

	return title + "\n" + body + notice
}

// visibleSecrets applies v.filter through matchesFields. Fields included:
// name, namespace, type — every stringy column the user sees in the table.
func (v SecretsView) visibleSecrets() []resources.SecretItem {
	if v.filter == "" {
		return v.secrets
	}
	out := make([]resources.SecretItem, 0, len(v.secrets))
	for _, s := range v.secrets {
		if matchesFields(v.filter, s.Name, s.Namespace, s.Type) {
			out = append(out, s)
		}
	}
	return out
}

func (v SecretsView) rows() []components.Row {
	items := v.visibleSecrets()
	rows := make([]components.Row, len(items))
	for i, s := range items {
		rows[i] = components.Row{
			components.NSChip(s.Namespace),
			s.Name,
			s.Type,
			fmt.Sprintf("%d", s.Keys),
			fmtAge(s.Age),
		}
	}
	return rows
}
