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
)

var pvcCols = []components.Column{
	{Header: "NAMESPACE", Width: 14},
	{Header: colName, Width: 32, Flex: true},
	{Header: "STATUS", Width: 12},
	{Header: "VOLUME", Width: 24},
	{Header: "CAPACITY", Width: 10, Align: components.AlignRight},
	{Header: "ACCESS MODES", Width: 16},
	{Header: "STORAGECLASS", Width: 16},
	{Header: colAge, Width: 6, Align: components.AlignRight},
}

// PVCsView lists PersistentVolumeClaims in the current namespace.
type PVCsView struct {
	svc       port.PVCService
	namespace string
	pvcs      []resources.PVCItem
	table     components.Table
	filter    string
	err       error
}

// NewPVCsView creates a PVCsView wired to svc and scoped to namespace.
func NewPVCsView(svc port.PVCService, namespace string) PVCsView {
	return PVCsView{
		svc:       svc,
		namespace: namespace,
		table:     components.NewTable(pvcCols, nil),
	}
}

// pvcsListedMsg carries the result of an async ListPVCs call.
type pvcsListedMsg struct {
	items []resources.PVCItem
	err   error
}

// Update routes tea.Msg through the PVCs view.
func (v PVCsView) Update(msg tea.Msg) (PVCsView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.PVCsUpdatedMsg:
		ns := v.namespace
		svc := v.svc
		return v, func() tea.Msg {
			items, err := svc.ListPVCs(context.Background(), ns)
			return pvcsListedMsg{items: items, err: err}
		}

	case pvcsListedMsg:
		v.err = msg.err
		if msg.err == nil {
			v.pvcs = msg.items
			v.table = v.table.SetRows(v.rows())
		}
		return v, nil

	case FilterMsg:
		v.filter = msg.Query
		v.table = v.table.SetRows(v.rows())
		return v, nil

	case NamespaceChangedMsg:
		v.namespace = msg.Namespace
		v.pvcs = nil
		v.table = v.table.SetRows(nil)
		return v, nil

	case tea.KeyMsg:
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
			// Open a full-screen generic describe of the focused PVC. Reuses
			// the SPEC block for the body — same shape any future non-pod
			// describe will land on.
			p := v.selectedPVC()
			if p == nil {
				return v, nil
			}
			title := "pvc/" + p.Name
			kvs := v.focusKVs()
			return v, func() tea.Msg {
				return SwitchToGenericDescribeMsg{Title: title, KVs: kvs}
			}
		}
	}
	return v, nil
}

// selectedPVC resolves the table cursor back to a PVCItem. Returns nil for
// empty tables. Resolves via the filtered slice so the index aligns with the
// table's current view.
func (v PVCsView) selectedPVC() *resources.PVCItem {
	if v.table.SelectedRow() == nil {
		return nil
	}
	idx := v.table.SelectedIndex()
	visible := v.visiblePVCs()
	if idx >= len(visible) {
		return nil
	}
	p := visible[idx]
	for i := range v.pvcs {
		if v.pvcs[i].Name == p.Name && v.pvcs[i].Namespace == p.Namespace {
			return &v.pvcs[i]
		}
	}
	return nil
}

// Title implements views.View.
func (v PVCsView) Title() string { return "pvcs" }

// Filter implements views.Filterable.
func (v PVCsView) Filter() string { return v.filter }

// Count implements views.View.
func (v PVCsView) Count() (visible, total int) {
	return len(v.visiblePVCs()), len(v.pvcs)
}

// Chips implements views.View.
func (v PVCsView) Chips() []layout.FilterChip {
	chips := []layout.FilterChip{}
	if v.filter != "" {
		chips = append(chips, layout.FilterChip{Key: "/", Value: v.filter, Strong: true})
	}
	return chips
}

// KeyHints implements views.View. The Enter -> describe handler is wired in
// a follow-up wave; we still advertise it here because the user expects it
// and the gap is one wave wide. yaml/delete live in KeyMap as Soon entries.
func (v PVCsView) KeyHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "↵", Label: "describe"},
		{Key: "/", Label: labelFilter},
	}
}

// KeyMap implements views.KeyMap and powers the `?` help overlay.
func (v PVCsView) KeyMap() []components.KeySpec {
	return []components.KeySpec{
		{Key: "↵", Label: "describe"},
		{Key: "/", Label: labelFilter},
		{Key: "y", Label: labelYAML, Soon: true},
		{Key: "d", Label: "delete", Soon: true},
	}
}

// Table implements views.View.
func (v PVCsView) Table(width, height int) string {
	v.table = v.table.SetWidth(width).SetHeight(height)
	if v.err != nil {
		return "error: " + v.err.Error()
	}
	return v.table.View()
}

// Details implements views.View. Renders a uniform SPEC block driven by
// focusKVs() — status, volume, capacity, access modes, storage class, age.
func (v PVCsView) Details(width, height int) string {
	p := v.selectedPVC()
	if p == nil {
		return ""
	}
	subtitle := p.Namespace
	if p.Status != "" {
		subtitle = fmt.Sprintf("%s · %s · %s", p.Namespace, p.Status, fmtAge(p.Age))
	}
	return layout.DefaultDetails(width, height, layout.DetailsBlock{
		Title:    p.Name,
		Subtitle: subtitle,
		KVs:      v.focusKVs(),
	})
}

// focusKVs returns the SPEC fields for the focused PVC. Reused by the Enter
// handler to populate the GenericDescribeView body so the "describe" sub-view
// is just a full-width version of the right pane.
func (v PVCsView) focusKVs() []layout.KV {
	p := v.selectedPVC()
	if p == nil {
		return nil
	}
	return []layout.KV{
		{Key: "status", Value: fallbackOr(p.Status)},
		{Key: "volume", Value: fallbackOr(p.Volume)},
		{Key: "capacity", Value: fallbackOr(p.Capacity)},
		{Key: "access modes", Value: fallbackOr(p.AccessModes)},
		{Key: "storage class", Value: fallbackOr(p.StorageClass)},
		{Key: kvAge, Value: fmtAge(p.Age)},
	}
}

// visiblePVCs returns the pvcs slice after applying v.filter through
// matchesFields. Fields included: name, namespace, status, volume, capacity,
// access modes, storage class — every stringy column the user sees.
func (v PVCsView) visiblePVCs() []resources.PVCItem {
	if v.filter == "" {
		return v.pvcs
	}
	out := make([]resources.PVCItem, 0, len(v.pvcs))
	for _, p := range v.pvcs {
		if matchesFields(v.filter, p.Name, p.Namespace, p.Status, p.Volume, p.Capacity, p.AccessModes, p.StorageClass) {
			out = append(out, p)
		}
	}
	return out
}

func (v PVCsView) rows() []components.Row {
	pvcs := v.visiblePVCs()
	rows := make([]components.Row, len(pvcs))
	for i, p := range pvcs {
		rows[i] = components.Row{
			components.NSChip(p.Namespace),
			highlightMatch(p.Name, v.filter),
			components.StatusPill(p.Status),
			highlightMatch(p.Volume, v.filter),
			p.Capacity,
			p.AccessModes,
			highlightMatch(p.StorageClass, v.filter),
			fmtAge(p.Age),
		}
	}
	return rows
}
