package components

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// FormMode is the editor's input state. Three modes — that's the whole
// state machine.
//
//	ModeNav         — j/k navigate, ↵ to edit a row, esc to exit (or
//	                  open ModeConfirmExit when the form is dirty).
//	ModeEdit        — the selected row's value field is a textinput; the
//	                  form captures every keystroke until esc commits and
//	                  drops back to ModeNav. The value lands in the form's
//	                  in-memory data; the API write happens on confirm.
//	ModeConfirmExit — single inline bar at the bottom: `s` save & exit,
//	                  `d` discard & exit, `esc` cancel back to ModeNav.
type FormMode int

const (
	ModeNav FormMode = iota
	ModeEdit
	ModeConfirmExit
)

// FormSaveRequestedMsg is emitted when the user picks `s` in
// ModeConfirmExit. The host view persists via its service then closes
// the form on the SecretSavedMsg / ConfigMapSavedMsg success path.
type FormSaveRequestedMsg struct{}

// FormQuitRequestedMsg is emitted when the user picks `d` in
// ModeConfirmExit (or hits esc on a clean form). The host view drops
// the form without saving.
type FormQuitRequestedMsg struct{}

type formRow struct {
	key   textinput.Model
	value textinput.Model
	hide  bool
}

// Form is an immutable key/value editor for secrets and configmaps.
// All mutation methods return a new Form — safe for Bubble Tea models.
type Form struct {
	rows     []formRow
	selected int
	mode     FormMode

	// original is the baseline used for dirty/diff calculations.
	// Set by NewForm and refreshable via WithOriginal (after a save).
	original map[string][]byte

	name string

	// pending stashes a single key for the `dd` two-stroke. Cleared on
	// any key that doesn't complete the sequence.
	pending string

	width int
}

// NewForm creates a Form pre-populated from a decoded secret/configmap
// Data map. The same map is also stashed as the diff baseline.
func NewForm(data map[string][]byte) Form {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	rows := make([]formRow, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, newRow(k, string(data[k])))
	}
	f := Form{
		rows:     rows,
		mode:     ModeNav,
		original: copyData(data),
	}
	f.blurAll()
	return f
}

// WithOriginal swaps the diff baseline. Callers use this after a
// successful save so subsequent edits compare against the new state.
func (f Form) WithOriginal(data map[string][]byte) Form {
	f.original = copyData(data)
	return f
}

// WithName attaches a label rendered in the status strip ("secret: foo").
func (f Form) WithName(name string) Form {
	f.name = name
	return f
}

// SetWidth sets the form's content width. Hosts call this from their
// Table() method so the form fills the focus frame's full width instead
// of the legacy 72-col default.
func (f Form) SetWidth(w int) Form {
	f.width = w
	return f
}

func newRow(k, v string) formRow {
	ki := textinput.New()
	ki.SetValue(k)
	ki.Width = 24
	vi := textinput.New()
	vi.SetValue(v)
	vi.Width = 64
	return formRow{key: ki, value: vi}
}

func copyData(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		b := make([]byte, len(v))
		copy(b, v)
		out[k] = b
	}
	return out
}

// RowCount returns the number of rows currently in the form.
func (f Form) RowCount() int { return len(f.rows) }

// Mode exposes the form's current input state.
func (f Form) Mode() FormMode { return f.mode }

// IsDirty reports whether the in-memory data differs from the baseline
// captured by NewForm/WithOriginal. Computed (not flagged) so reverting
// a typo back to its original value correctly reports clean.
func (f Form) IsDirty() bool {
	cur := f.Data()
	if len(cur) != len(f.original) {
		return true
	}
	for k, v := range cur {
		ov, ok := f.original[k]
		if !ok || !bytes.Equal(ov, v) {
			return true
		}
	}
	return false
}

// IsHidden reports whether row i is rendered as ••••.
func (f Form) IsHidden(i int) bool {
	if i < 0 || i >= len(f.rows) {
		return false
	}
	return f.rows[i].hide
}

// AddRow appends a new row with the given key/value and lands on it
// in edit mode so the user can fill it in immediately.
func (f Form) AddRow(k, v string) Form {
	f.rows = append(f.rows, newRow(k, v))
	f.selected = len(f.rows) - 1
	f.mode = ModeEdit
	f.blurAll()
	f.rows[f.selected].value.Focus()
	return f
}

// DeleteSelected removes the currently selected row.
func (f Form) DeleteSelected() Form {
	if len(f.rows) == 0 {
		return f
	}
	f.rows = append(f.rows[:f.selected], f.rows[f.selected+1:]...)
	if f.selected >= len(f.rows) && f.selected > 0 {
		f.selected--
	}
	f.mode = ModeNav
	f.blurAll()
	return f
}

// Data exports all non-empty key rows as a map[string][]byte.
func (f Form) Data() map[string][]byte {
	out := make(map[string][]byte, len(f.rows))
	for _, r := range f.rows {
		k := strings.TrimSpace(r.key.Value())
		if k != "" {
			out[k] = []byte(r.value.Value())
		}
	}
	return out
}

// Update routes input by mode. Non-key messages reach the active
// textinput so cursor blinks keep flowing.
func (f Form) Update(msg tea.Msg) (Form, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return f.forwardToActive(msg)
	}
	switch f.mode {
	case ModeEdit:
		return f.updateEdit(km)
	case ModeConfirmExit:
		return f.updateConfirmExit(km)
	default:
		return f.updateNav(km)
	}
}

func (f Form) updateNav(km tea.KeyMsg) (Form, tea.Cmd) {
	key := km.String()

	// Two-stroke `dd` for delete. Any non-completing key clears the
	// prefix and is processed normally below.
	if f.pending == "d" {
		f.pending = ""
		if key == "d" {
			return f.DeleteSelected(), nil
		}
	}

	switch key {
	case "up", "k":
		if f.selected > 0 {
			f.selected--
		}
		return f, nil
	case "down", "j":
		if f.selected < len(f.rows)-1 {
			f.selected++
		}
		return f, nil
	case "enter":
		// ↵ on the selected row drops into the value editor. Tab/right
		// stay as no-ops here so single-key access is the canonical
		// path.
		if len(f.rows) > 0 {
			f.mode = ModeEdit
			f.blurAll()
			f.rows[f.selected].value.Focus()
		}
		return f, nil
	case "o":
		return f.AddRow("", ""), nil
	case "d":
		f.pending = "d"
		return f, nil
	case "H":
		if len(f.rows) > 0 {
			f.rows[f.selected].hide = !f.rows[f.selected].hide
		}
		return f, nil
	case "esc":
		// Esc on a clean form exits without ceremony. Dirty forms
		// open the confirm-exit bar so the user picks save / discard
		// / cancel deliberately.
		if !f.IsDirty() {
			return f, func() tea.Msg { return FormQuitRequestedMsg{} }
		}
		f.mode = ModeConfirmExit
		return f, nil
	}
	return f, nil
}

func (f Form) updateEdit(km tea.KeyMsg) (Form, tea.Cmd) {
	if km.String() == "esc" {
		// Commit the field by simply leaving edit mode — the textinput
		// already holds the typed value. Back to nav; the form's
		// IsDirty check picks up the change.
		f.mode = ModeNav
		f.blurAll()
		return f, nil
	}
	// Forward to the value textinput.
	if len(f.rows) == 0 {
		return f, nil
	}
	var cmd tea.Cmd
	f.rows[f.selected].value, cmd = f.rows[f.selected].value.Update(km)
	return f, cmd
}

func (f Form) updateConfirmExit(km tea.KeyMsg) (Form, tea.Cmd) {
	switch km.String() {
	case "s", "S", "y", "Y", "enter":
		// Save & exit — the host view will receive FormSaveRequestedMsg,
		// run the API call, and close the form on the success path.
		f.mode = ModeNav
		return f, func() tea.Msg { return FormSaveRequestedMsg{} }
	case "d", "D":
		// Discard & exit. We don't restore form state because the
		// form is about to be torn down anyway.
		f.mode = ModeNav
		return f, func() tea.Msg { return FormQuitRequestedMsg{} }
	case "esc", "n", "N":
		f.mode = ModeNav
		return f, nil
	}
	return f, nil
}

func (f Form) forwardToActive(msg tea.Msg) (Form, tea.Cmd) {
	if len(f.rows) == 0 || f.mode != ModeEdit {
		return f, nil
	}
	var cmd tea.Cmd
	f.rows[f.selected].value, cmd = f.rows[f.selected].value.Update(msg)
	return f, cmd
}

func (f *Form) blurAll() {
	for i := range f.rows {
		f.rows[i].key.Blur()
		f.rows[i].value.Blur()
	}
}

// View renders the editor body, mode-aware.
func (f Form) View() string {
	var sb strings.Builder

	sb.WriteString(f.renderStatusStrip())
	sb.WriteString("\n")
	sb.WriteString(theme.Divider(f.dividerWidth()) + "\n")

	for i, r := range f.rows {
		switch {
		case i == f.selected && f.mode == ModeEdit:
			sb.WriteString(f.renderEditingRow(r))
		case i == f.selected:
			sb.WriteString(f.renderSelectedRow(r))
		default:
			sb.WriteString(f.renderCompactRow(r))
		}
	}

	if f.mode == ModeConfirmExit {
		sb.WriteString("\n")
		sb.WriteString(f.renderConfirmExit())
	}
	return sb.String()
}

func (f Form) renderStatusStrip() string {
	name := f.name
	if name == "" {
		name = "edit"
	}
	parts := []string{
		theme.Faint.Render("editing:") + " " +
			lipgloss.NewStyle().Foreground(theme.ColorFG).Render(name),
		theme.Dim.Render(fmt.Sprintf("%d keys", len(f.rows))),
	}
	if f.IsDirty() {
		parts = append(parts,
			lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render("● dirty"))
	}
	return strings.Join(parts, " · ")
}

const (
	rowKeyWidth    = 28
	rowGutterWidth = 4
)

// renderCompactRow renders a non-selected row as a single line.
func (f Form) renderCompactRow(r formRow) string {
	keyText := r.key.Value()
	if keyText == "" {
		keyText = theme.Faint.Render("(empty)")
	} else {
		keyText = lipgloss.NewStyle().Foreground(theme.ColorFG).Render(padRight(keyText, rowKeyWidth))
	}
	valStr := r.value.Value()
	if r.hide {
		valStr = "••••"
	}
	maxVal := f.dividerWidth() - rowKeyWidth - rowGutterWidth - 2
	if maxVal < 4 {
		maxVal = 4
	}
	if len(valStr) > maxVal {
		valStr = valStr[:maxVal-1] + "…"
	}
	return strings.Repeat(" ", rowGutterWidth) + keyText + "  " + theme.Dim.Render(valStr) + "\n"
}

// renderSelectedRow renders the cursor row in nav mode: cursor + key on
// line 1, full soft-wrapped value on the lines below. Long values cap
// at selectedValueMaxLines so a 40-line cert doesn't dominate the form.
func (f Form) renderSelectedRow(r formRow) string {
	cursor := lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("▌")
	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)

	keyText := r.key.Value()
	if keyText == "" {
		keyText = "(empty)"
	}
	line1 := cursor + " " + accent.Render("› "+keyText)

	valStr := r.value.Value()
	if r.hide {
		valStr = "•••• (hidden — H to reveal)"
	}
	innerW := f.dividerWidth() - rowGutterWidth
	if innerW < 20 {
		innerW = 20
	}
	wrapped := lipgloss.NewStyle().Width(innerW).Render(valStr)
	lines := strings.Split(wrapped, "\n")
	truncated := false
	if len(lines) > selectedValueMaxLines {
		lines = lines[:selectedValueMaxLines]
		truncated = true
	}

	var sb strings.Builder
	sb.WriteString(line1 + "\n")
	for _, line := range lines {
		sb.WriteString(cursor + "   " + accent.Render(line) + "\n")
	}
	if truncated {
		sb.WriteString(cursor + "   " +
			theme.Faint.Render("… (truncated — ↵ to edit and see all)") + "\n")
	}
	return sb.String()
}

const selectedValueMaxLines = 5

// renderEditingRow shows the cursor row while the user is in edit
// mode. Layout matches renderSelectedRow but the value line is a
// textinput sized to the form's full inner width.
func (f Form) renderEditingRow(r formRow) string {
	cursor := lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("▌")
	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)

	keyText := r.key.Value()
	if keyText == "" {
		keyText = "(empty)"
	}
	line1 := cursor + " " + accent.Render("› "+keyText)

	innerW := f.dividerWidth() - rowGutterWidth
	if innerW < 20 {
		innerW = 20
	}
	v := r.value
	v.Width = innerW
	line2 := cursor + "   " + v.View()

	return line1 + "\n" + line2 + "\n"
}

// renderConfirmExit is the inline bar shown when the user tries to
// leave a dirty form. Three crisp options, no overlay.
func (f Form) renderConfirmExit() string {
	prompt := lipgloss.NewStyle().
		Foreground(theme.ColorWarn).
		Bold(true).
		Render("● unsaved changes")

	opts := []string{
		theme.KeyChip.Render("s") + theme.Dim.Render(" save & exit"),
		theme.KeyChip.Render("d") + theme.Dim.Render(" discard"),
		theme.KeyChip.Render("esc") + theme.Dim.Render(" cancel"),
	}
	return prompt + "  " + strings.Join(opts, "   ")
}

func (f Form) dividerWidth() int {
	if f.width > 0 {
		return f.width
	}
	return 72
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}
