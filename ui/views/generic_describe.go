package views

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/layout"
	"github.com/hermanu/klens/ui/theme"
)

// GenericDescribeView is the full-screen KV describe shell used by non-pod
// resources (today: PVCs). The structure mirrors DescribeView (the pod-specific
// describe) — same scroll/back ergonomics — but the body is a flat KV list
// instead of the rich pod section breakdown, since most non-pod resources don't
// need anything beyond what the SPEC pane already shows.
//
// Title is the chip caption shown in the top of the body, e.g. "pvc/data-x".
// KVs is the body content, populated by the source view's focusKVs() at the
// time of switch — the describe is a snapshot, not a live re-fetch.
type GenericDescribeView struct {
	title  string
	kvs    []layout.KV
	offset int // first visible line for j/k scroll
}

// NewGenericDescribeView returns an empty describe shell. The root model swaps
// in fresh content via WithFocus when the user presses Enter on a non-pod row.
func NewGenericDescribeView() GenericDescribeView { return GenericDescribeView{} }

// WithFocus resets the view to a fresh title + KV list and rewinds the scroll
// offset to the top. Returning a value (no cmd) means callers don't have to
// thread an async fetch — KVs are already in hand from the source view.
func (v GenericDescribeView) WithFocus(title string, kvs []layout.KV) GenericDescribeView {
	v.title = title
	v.kvs = kvs
	v.offset = 0
	return v
}

func (v GenericDescribeView) Update(msg tea.Msg) (GenericDescribeView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			v.offset++
		case "k", "up":
			if v.offset > 0 {
				v.offset--
			}
		case "g":
			v.offset = 0
		case "G":
			// Snap to bottom — clamping happens at render time so we set a
			// large value here and let the renderer pin it.
			v.offset = len(v.kvs)
		case "esc":
			return v, func() tea.Msg { return BackToPodsMsg{} }
		}
	}
	return v, nil
}

// Title implements views.View. Mirrors the pod describe's title so the
// breadcrumb in the top bar reads "describe" regardless of source resource.
func (v GenericDescribeView) Title() string { return "describe" }

// Count implements views.View. Total = number of KV rows; visible = same after
// clamp. Cheap and accurate for the bottom hint.
func (v GenericDescribeView) Count() (visible, total int) {
	n := len(v.kvs)
	return n, n
}

// Chips implements views.View. One chip carrying the resource title so users
// can see the source target at a glance.
func (v GenericDescribeView) Chips() []layout.FilterChip {
	return []layout.FilterChip{{Key: "target", Value: fallbackOr(v.title)}}
}

// KeyHints implements views.View. Only the keys that actually do something.
func (v GenericDescribeView) KeyHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "j/k", Label: "scroll"},
		{Key: "esc", Label: "back"},
	}
}

// KeyMap implements views.KeyMap and powers the `?` help overlay.
func (v GenericDescribeView) KeyMap() []components.KeySpec {
	return []components.KeySpec{
		{Key: "j/k", Label: "scroll"},
		{Key: "g", Label: "top"},
		{Key: "G", Label: "bottom"},
		{Key: "esc", Label: "back"},
	}
}

// Table implements views.View — full-screen KV body. The shell hides the right
// details pane when IsDescribe() returns true, so this gets the full content
// width.
func (v GenericDescribeView) Table(width, height int) string {
	if len(v.kvs) == 0 {
		return lipgloss.NewStyle().
			Foreground(theme.ColorMuted).
			Padding(1, 2).
			Render("no data")
	}

	// Scroll: clamp offset, then slice. Render each KV inline (no border) so the
	// describe view feels like a flat document instead of a wide modal.
	pageSize := height - 1
	if pageSize < 1 {
		pageSize = 20
	}
	start := v.offset
	if start > len(v.kvs)-1 {
		start = max0(len(v.kvs) - 1)
	}
	end := start + pageSize
	if end > len(v.kvs) {
		end = len(v.kvs)
	}
	visible := v.kvs[start:end]

	keyStyle := lipgloss.NewStyle().Foreground(theme.ColorMuted).Width(18)
	valStyle := lipgloss.NewStyle().Foreground(theme.ColorFG)
	warnStyle := lipgloss.NewStyle().Foreground(theme.ColorWarn)
	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)

	var sb strings.Builder
	if v.title != "" && start == 0 {
		sb.WriteString("  " + titleStyle.Render(v.title) + "\n\n")
	}
	for _, kv := range visible {
		v := valStyle.Render(kv.Value)
		if kv.Warn {
			v = warnStyle.Render(kv.Value)
		}
		sb.WriteString("  " + keyStyle.Render(kv.Key) + " " + v + "\n")
	}
	if len(v.kvs) > pageSize {
		hint := lipgloss.NewStyle().Foreground(theme.ColorMuted).
			Render("  " + strFmt(start+1, end, len(v.kvs)))
		sb.WriteString(hint)
	}
	_ = width // width unused — KV layout flexes to terminal width naturally.
	return sb.String()
}

// Details implements views.View — describe is full-width, no side pane.
func (v GenericDescribeView) Details(width, height int) string { return "" }

// IsDescribe lets the shell skip the right details pane when describe is up.
// Same predicate name as DescribeView so the shell can branch on either.
func (v GenericDescribeView) IsDescribe() bool { return true }
