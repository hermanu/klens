package views

import (
	"context"
	"fmt"
	"sort"

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

// Column 0 is NAMESPACE so the row layout matches the other modern shell
// views; selected() resolves the configmap by table index, not by reading
// row[0] (which contains an ANSI-coded NSChip).
var configMapCols = []components.Column{
	{Header: "NAMESPACE", Width: 14},
	{Header: colName, Width: 44, Flex: true},
	{Header: "KEYS", Width: 6, Align: components.AlignRight},
	{Header: colAge, Width: 6, Align: components.AlignRight},
}

// ConfigMapSavedMsg is sent after a configmap save attempt.
type ConfigMapSavedMsg struct {
	Name string
	Err  error
}

type configMapDetailMsg struct {
	item resources.ConfigMapItem
}

// configMapsListedMsg carries the result of an async ListConfigMaps call.
type configMapsListedMsg struct {
	items []resources.ConfigMapItem
	err   error
}

// ConfigMapsView lists configmaps and edits them in-place via the form
// component. ConfigMap data is map[string]string, so the form (which expects
// []byte values) round-trips through string(...) on save.
type ConfigMapsView struct {
	svc        port.ConfigMapService
	namespace  string
	configmaps []resources.ConfigMapItem
	table      components.Table
	form       components.Form
	current    *resources.ConfigMapItem
	mode       configMapsMode
	filter     string
	err        error
	saveMsg    string
}

// NewConfigMapsView creates a ConfigMapsView wired to svc and scoped to namespace.
func NewConfigMapsView(svc port.ConfigMapService, namespace string) ConfigMapsView {
	return ConfigMapsView{
		svc:       svc,
		namespace: namespace,
		table:     components.NewTable(configMapCols, nil),
	}
}

// Update routes tea.Msg through the configmaps view and its embedded form editor.
func (v ConfigMapsView) Update(msg tea.Msg) (ConfigMapsView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.ConfigMapsUpdatedMsg:
		ns := v.namespace
		svc := v.svc
		return v, func() tea.Msg {
			items, err := svc.ListConfigMaps(context.Background(), ns)
			return configMapsListedMsg{items: items, err: err}
		}

	case configMapsListedMsg:
		v.err = msg.err
		if msg.err == nil {
			v.configmaps = msg.items
			v.table = v.table.SetRows(v.rows())
		}
		return v, nil

	case configMapDetailMsg:
		item := msg.item
		v.current = &item
		// ConfigMap data is map[string]string; the form takes []byte values, so
		// wrap each value before handing it over.
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

	case components.FormSaveRequestedMsg:
		// User confirmed in the form's diff-preview / ex-mode `:w`.
		if v.mode == configMapsModeEdit {
			return v.saveConfigMap()
		}
		return v, nil

	case components.FormQuitRequestedMsg:
		// `:q` (clean) or `:q!` — pop back to the list.
		if v.mode == configMapsModeEdit {
			v.mode = configMapsModeList
			v.current = nil
			v.saveMsg = ""
		}
		return v, nil

	case FilterMsg:
		v.filter = msg.Query
		v.table = v.table.SetRows(v.rows())
		return v, nil

	case NamespaceChangedMsg:
		v.namespace = msg.Namespace
		v.configmaps = nil
		v.mode = configMapsModeList
		v.current = nil
		v.table = v.table.SetRows(nil)
		return v, nil

	case tea.KeyMsg:
		if v.mode == configMapsModeEdit {
			return v.updateEdit(msg)
		}
		return v.updateList(msg)
	}
	return v, nil
}

func (v ConfigMapsView) updateList(msg tea.KeyMsg) (ConfigMapsView, tea.Cmd) {
	switch msg.String() {
	case "j", keyDown:
		v.table = v.table.MoveDown()
	case "k", "up":
		v.table = v.table.MoveUp()
	case "g":
		v.table = v.table.MoveTop()
	case "G":
		v.table = v.table.MoveBottom()
	case keyEnter:
		return v.openEditor()
	}
	return v, nil
}

// openEditor resolves the focused row to a ConfigMapItem by index because cell
// 0 is the NSChip with ANSI codes — reading raw values from v.configmaps is
// the only reliable lookup.
func (v ConfigMapsView) openEditor() (ConfigMapsView, tea.Cmd) {
	cm := v.selected()
	if cm == nil {
		return v, nil
	}
	name := cm.Name
	ns := cm.Namespace
	svc := v.svc
	return v, func() tea.Msg {
		item, err := svc.GetConfigMap(context.Background(), ns, name)
		if err != nil {
			return ConfigMapSavedMsg{Name: name, Err: err}
		}
		return configMapDetailMsg{item: item}
	}
}

func (v ConfigMapsView) updateEdit(msg tea.KeyMsg) (ConfigMapsView, tea.Cmd) {
	// Every keystroke goes to the form — exit decisions live there.
	// FormSaveRequestedMsg / FormQuitRequestedMsg are caught by the
	// view's outer Update so the editor closes after a save success
	// or a discard.
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
	// ConfigMap data is map[string]string on the wire; cast each []byte back.
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

// selected resolves the table cursor back to a ConfigMapItem from the visible
// (post-filter) slice, then maps it to the underlying v.configmaps entry.
func (v ConfigMapsView) selected() *resources.ConfigMapItem {
	idx := v.table.SelectedIndex()
	visible := v.visibleConfigMaps()
	if idx < 0 || idx >= len(visible) {
		return nil
	}
	c := visible[idx]
	for i := range v.configmaps {
		if v.configmaps[i].Name == c.Name && v.configmaps[i].Namespace == c.Namespace {
			return &v.configmaps[i]
		}
	}
	return nil
}

// Title implements views.View.
func (v ConfigMapsView) Title() string { return "configmaps" }

// Filter implements views.Filterable.
func (v ConfigMapsView) Filter() string { return v.filter }

// CapturesKeys implements views.Capturing — while editing a configmap
// the form owns every keystroke so app-level shortcuts (:, ctrl+p, ?,
// /) don't conflict with the inner editor.
func (v ConfigMapsView) CapturesKeys() bool { return v.mode == configMapsModeEdit }

// Count implements views.View.
func (v ConfigMapsView) Count() (visible, total int) {
	return len(v.visibleConfigMaps()), len(v.configmaps)
}

// Chips implements views.View.
func (v ConfigMapsView) Chips() []layout.FilterChip {
	chips := []layout.FilterChip{}
	if v.mode == configMapsModeEdit {
		chips = append(chips, layout.FilterChip{Key: "mode", Value: labelEdit, Strong: true})
	}
	if v.filter != "" {
		chips = append(chips, layout.FilterChip{Key: "/", Value: v.filter, Strong: true})
	}
	return chips
}

// KeyHints implements views.View. List mode only advertises Enter and `/`;
// yaml/delete live in KeyMap as Soon entries. Edit-mode hints align with
// the simplified 3-mode form: ↵ edits, esc backs out (or opens the
// save/discard confirm if dirty).
func (v ConfigMapsView) KeyHints() []layout.KeyHint {
	if v.mode == configMapsModeEdit {
		return []layout.KeyHint{
			{Key: "↵", Label: labelEdit},
			{Key: keyEsc, Label: "back"},
			{Key: "o", Label: "add"},
			{Key: "dd", Label: "del"},
		}
	}
	return []layout.KeyHint{
		{Key: "↵", Label: labelEdit},
		{Key: "/", Label: labelFilter},
	}
}

// KeyMap implements views.KeyMap and powers the `?` help overlay. The
// edit-mode keymap matches the simplified 3-mode form.
func (v ConfigMapsView) KeyMap() []components.KeySpec {
	if v.mode == configMapsModeEdit {
		return []components.KeySpec{
			{Key: "↵", Label: "edit selected row"},
			{Key: keyEsc, Label: "back / open exit confirm"},
			{Key: "j / k", Label: "next / prev row"},
			{Key: "o", Label: "add row"},
			{Key: "dd", Label: "delete row"},
			{Key: "s / d / esc", Label: "save / discard / cancel (in confirm)"},
		}
	}
	return []components.KeySpec{
		{Key: "↵", Label: labelEdit},
		{Key: "/", Label: labelFilter},
		{Key: "y", Label: labelYAML, Soon: true},
		{Key: "d", Label: "delete", Soon: true},
	}
}

// Table implements views.View. In edit mode it returns the form body so the
// shell can swap the central pane without re-routing render calls. The form
// gets the full pane width so the editor doesn't render in a narrow strip.
func (v ConfigMapsView) Table(width, height int) string {
	if v.mode == configMapsModeEdit && v.current != nil {
		v.form = v.form.SetWidth(width)
		return v.formView()
	}
	v.table = v.table.SetWidth(width).SetHeight(height)
	if v.err != nil {
		return "error: " + v.err.Error()
	}
	return v.table.View()
}

// Details implements views.View. Renders a uniform SPEC block driven by
// focusKVs() — keys count and a top-5 alphabetic key preview so users see
// what's inside without opening the editor.
func (v ConfigMapsView) Details(width, height int) string {
	cm := v.selected()
	if v.mode == configMapsModeEdit && v.current != nil {
		cm = v.current
	}
	if cm == nil {
		return ""
	}
	return layout.DefaultDetails(width, height, layout.DetailsBlock{
		Title:    cm.Name,
		Subtitle: fmt.Sprintf("%s · %s", cm.Namespace, fmtAge(cm.Age)),
		KVs:      v.focusKVs(),
	})
}

// focusKVs returns the SPEC fields for the focused configmap. Same shape as
// the secrets pane: count + top-5 key names + age. ConfigMaps don't have a
// type field so we omit that row.
func (v ConfigMapsView) focusKVs() []layout.KV {
	cm := v.selected()
	if v.mode == configMapsModeEdit && v.current != nil {
		cm = v.current
	}
	if cm == nil {
		return nil
	}
	keys := cm.KeyNames
	if len(keys) == 0 && len(cm.Data) > 0 {
		keys = make([]string, 0, len(cm.Data))
		for k := range cm.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}
	kvs := []layout.KV{
		{Key: "keys", Value: fmt.Sprintf("%d", cm.Keys)},
	}
	for i, k := range keys {
		if i >= 5 {
			break
		}
		kvs = append(kvs, layout.KV{Key: "  · " + k, Value: ""})
	}
	kvs = append(kvs, layout.KV{Key: kvAge, Value: fmtAge(cm.Age)})
	return kvs
}

// formView renders just the editor body. The shell renders chrome around it.
func (v ConfigMapsView) formView() string {
	title := theme.Base.Render(fmt.Sprintf("  ConfigMap: %s", v.current.Name)) +
		"  " + theme.Faint.Render("namespace: "+v.namespace) + "\n"

	body := v.form.View()

	notice := ""
	if v.saveMsg != "" {
		notice = "\n" + theme.Mid.Render(v.saveMsg)
	}

	return title + "\n" + body + notice
}

// visibleConfigMaps applies v.filter through matchesFields. Fields included:
// name, namespace — configmaps don't have a type or status column to match
// against, so the surface is intentionally narrow.
func (v ConfigMapsView) visibleConfigMaps() []resources.ConfigMapItem {
	if v.filter == "" {
		return v.configmaps
	}
	out := make([]resources.ConfigMapItem, 0, len(v.configmaps))
	for _, c := range v.configmaps {
		if matchesFields(v.filter, c.Name, c.Namespace) {
			out = append(out, c)
		}
	}
	return out
}

func (v ConfigMapsView) rows() []components.Row {
	items := v.visibleConfigMaps()
	rows := make([]components.Row, len(items))
	for i, c := range items {
		rows[i] = components.Row{
			components.NSChip(c.Namespace),
			highlightMatch(c.Name, v.filter),
			fmt.Sprintf("%d", c.Keys),
			fmtAge(c.Age),
		}
	}
	return rows
}
