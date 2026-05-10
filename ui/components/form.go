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

// FormMode is the editor's input state. Splitting "navigation" from
// "field editing" avoids the ambiguity that made the old tab-toggling
// model confusing — only one mode accepts text at any time.
//
// ModeNav is the vim "normal" equivalent: j/k navigate, h/l switch
// columns, i/a enter edit, : opens the ex-prompt for save/quit.
// ModeCommand hosts the ":" prompt itself; the textinput in
// f.commandInput captures the ex line until Enter or Esc.
type FormMode int

const (
	ModeNav FormMode = iota
	ModeValueEdit
	ModeKeyEdit
	ModeConfirmDiscard
	ModeConfirmSave
	ModeCommand
)

// FormSaveRequestedMsg is emitted when the user confirms a save (second
// ^s in ModeConfirmSave). The hosting view is responsible for picking
// this up, calling Form.Data(), and persisting via its service.
type FormSaveRequestedMsg struct{}

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

	mode FormMode

	// original is the baseline used for dirty/diff calculations.
	// Set by NewForm and refreshable via WithOriginal (after a save).
	original map[string][]byte

	// name is the resource label rendered in the status strip.
	name string

	// Cached diff counts populated when entering ModeConfirmSave so the
	// confirm screen renders without recomputing on every keypress.
	diffAdded   int
	diffRemoved int
	diffChanged int

	// pending stores a single-character prefix waiting for completion
	// (e.g. "d" before "dd" to delete, "g" before "gg" to jump-top).
	// Cleared on any key that doesn't complete the sequence.
	pending string

	// commandInput backs the inline `:` ex-mode prompt — same idea as
	// vim's command-line. Active only while mode == ModeCommand.
	commandInput textinput.Model

	width int
}

// FormQuitRequestedMsg is emitted when the user runs `:q` on a clean
// form (or `:q!` regardless). The hosting view treats it like an Esc:
// pop back to the parent view.
type FormQuitRequestedMsg struct{}

// NewForm creates a Form pre-populated from a decoded secret/configmap
// Data map. The same map is also stashed as the diff baseline.
func NewForm(data map[string][]byte) Form {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys) // stable order — map iteration is not.

	rows := make([]formRow, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, newRow(k, string(data[k])))
	}
	ci := textinput.New()
	ci.Prompt = ""
	ci.Placeholder = "w · q · wq · q!"
	ci.CharLimit = 16
	f := Form{
		rows:         rows,
		mode:         ModeNav,
		original:     copyData(data),
		commandInput: ci,
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
// Optional — empty name renders as "edit".
func (f Form) WithName(name string) Form {
	f.name = name
	return f
}

// SetWidth lets the host view tell the form how much horizontal space
// it has. The form uses this to size the expanded value editor.
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
	vi.Width = 40
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

// Mode exposes the form's current input state. Hosts can use it to
// know whether to swallow keys (e.g. esc) before forwarding.
func (f Form) Mode() FormMode { return f.mode }

// DiffCounts returns (added, removed, changed) counts cached when the
// form entered ModeConfirmSave. Outside that mode the counts may be
// zero or stale — call only when Mode() == ModeConfirmSave.
func (f Form) DiffCounts() (added, removed, changed int) {
	return f.diffAdded, f.diffRemoved, f.diffChanged
}

// IsDirty reports whether the current data differs from the baseline
// captured by NewForm/WithOriginal. Computed (not flagged) so that
// "edit then revert to original" correctly resets to clean.
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

// ToggleHide toggles the hidden state of row i.
func (f Form) ToggleHide(i int) Form {
	if i < 0 || i >= len(f.rows) {
		return f
	}
	f.rows[i].hide = !f.rows[i].hide
	return f
}

// AddRow appends a new row with the given key/value. Kept for
// backwards-compat with the existing view callers.
func (f Form) AddRow(k, v string) Form {
	f.rows = append(f.rows, newRow(k, v))
	f.selected = len(f.rows) - 1
	f.mode = ModeKeyEdit
	f.blurAll()
	f.rows[f.selected].key.Focus()
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

// Update handles input. The mode-machine swallows global keys (esc,
// vim normal-mode bindings, the ":" ex prompt) and only forwards
// keystrokes to the underlying textinput when in ModeKeyEdit/ModeValueEdit
// (insert mode) or ModeCommand (the ex prompt).
func (f Form) Update(msg tea.Msg) (Form, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		// Non-key messages (e.g. cursor blink) still need to reach the
		// active textinput so the cursor animates.
		return f.forwardToActive(msg)
	}

	switch f.mode {
	case ModeConfirmDiscard:
		return f.updateConfirmDiscard(km)
	case ModeConfirmSave:
		return f.updateConfirmSave(km)
	case ModeKeyEdit, ModeValueEdit:
		return f.updateEditing(km)
	case ModeCommand:
		return f.updateCommand(km)
	default:
		return f.updateNav(km)
	}
}

func (f Form) updateNav(km tea.KeyMsg) (Form, tea.Cmd) {
	key := km.String()

	// Multi-key sequences (vim "dd", "gg"). The previous key is stashed
	// in f.pending; any key that doesn't complete a known sequence
	// clears the prefix and is processed normally below.
	if f.pending != "" {
		prev := f.pending
		f.pending = ""
		switch prev + key {
		case "dd":
			return f.DeleteSelected(), nil
		case "gg":
			f.selected = 0
			return f, nil
		}
		// fall through: treat the new key as a fresh keystroke
	}

	switch key {
	// Movement — arrows + vim hjkl. h/l switch between key/value
	// columns by entering the corresponding edit mode (vim's `i` for
	// "insert here"). j/k change rows. Capital G jumps to the bottom.
	case "up", "k", "ctrl+p":
		if f.selected > 0 {
			f.selected--
		}
		return f, nil
	case "down", "j", "ctrl+n":
		if f.selected < len(f.rows)-1 {
			f.selected++
		}
		return f, nil
	case "G":
		if len(f.rows) > 0 {
			f.selected = len(f.rows) - 1
		}
		return f, nil
	case "g":
		// Single 'g' arms the gg sequence — second 'g' jumps to top.
		f.pending = "g"
		return f, nil

	// Column focus — h/l mirror left/right. Tab is a vim-foreign but
	// long-standing alias for "switch to key column".
	case "right", "l":
		if len(f.rows) > 0 {
			f.mode = ModeValueEdit
			f.blurAll()
			f.rows[f.selected].value.Focus()
		}
		return f, nil
	case "left", "h", "tab":
		if len(f.rows) > 0 {
			f.mode = ModeKeyEdit
			f.blurAll()
			f.rows[f.selected].key.Focus()
		}
		return f, nil

	// Insert mode — vim's i/a both drop into the value field. enter
	// stays as a friendly synonym so non-vim users aren't stranded.
	case "i", "a", "enter":
		if len(f.rows) > 0 {
			f.mode = ModeValueEdit
			f.blurAll()
			f.rows[f.selected].value.Focus()
		}
		return f, nil
	case "I":
		// vim "I" jumps to the start of the line — here, edit the key.
		if len(f.rows) > 0 {
			f.mode = ModeKeyEdit
			f.blurAll()
			f.rows[f.selected].key.Focus()
		}
		return f, nil

	// Row mutation — vim's `o` opens a new line below; we don't have a
	// notion of "above" since the rows are sorted, so `O` aliases the
	// same behaviour. ctrl+a kept as the muscle-memory shortcut.
	case "o", "O", "ctrl+a":
		return f.AddRow("", ""), nil
	case "d":
		// First half of "dd" — arm the prefix so the next 'd' deletes.
		f.pending = "d"
		return f, nil
	case "ctrl+d":
		return f.DeleteSelected(), nil

	// Hide toggle — capital H frees lowercase h for navigation. ctrl+h
	// stays as a fallback but works less reliably across terminals
	// that swallow it as backspace.
	case "H", "ctrl+h":
		if len(f.rows) > 0 {
			f.rows[f.selected].hide = !f.rows[f.selected].hide
		}
		return f, nil

	// Save — ctrl+s remains the single-keystroke save with diff
	// preview. The vim-equivalent is `:w`, handled in updateCommand.
	case "ctrl+s":
		if f.IsDirty() {
			f.computeDiff()
			f.mode = ModeConfirmSave
		}
		return f, nil

	// Ex-mode entry. Empty input + Esc cancels back to nav.
	case ":":
		f.mode = ModeCommand
		f.commandInput.SetValue("")
		f.commandInput.Focus()
		return f, nil

	case "esc":
		if f.IsDirty() {
			f.mode = ModeConfirmDiscard
		}
		return f, nil
	}
	return f, nil
}

func (f Form) updateEditing(km tea.KeyMsg) (Form, tea.Cmd) {
	switch km.String() {
	case "esc":
		f.mode = ModeNav
		f.blurAll()
		return f, nil
	case "ctrl+s":
		// Same as nav — let users save without bouncing back first.
		f.mode = ModeNav
		f.blurAll()
		if f.IsDirty() {
			f.computeDiff()
			f.mode = ModeConfirmSave
		}
		return f, nil
	}
	// Forward to the active textinput.
	var cmd tea.Cmd
	if len(f.rows) > 0 {
		if f.mode == ModeKeyEdit {
			f.rows[f.selected].key, cmd = f.rows[f.selected].key.Update(km)
		} else {
			f.rows[f.selected].value, cmd = f.rows[f.selected].value.Update(km)
		}
	}
	return f, cmd
}

func (f Form) updateConfirmDiscard(km tea.KeyMsg) (Form, tea.Cmd) {
	switch km.String() {
	case "y", "Y":
		// Restore original data — rebuild rows from baseline.
		restored := NewForm(f.original).WithOriginal(f.original).WithName(f.name).SetWidth(f.width)
		return restored, nil
	case "n", "N", "esc":
		f.mode = ModeNav
		return f, nil
	}
	return f, nil
}

func (f Form) updateConfirmSave(km tea.KeyMsg) (Form, tea.Cmd) {
	switch km.String() {
	case "ctrl+s", "y", "Y", "enter":
		// Second confirm commits. We accept enter and y/Y too so the
		// modal feels natural to non-vim users; ctrl+s is preserved
		// for the muscle-memory "save again" gesture.
		f.mode = ModeNav
		return f, func() tea.Msg { return FormSaveRequestedMsg{} }
	case "esc", "n", "N":
		f.mode = ModeNav
		return f, nil
	}
	return f, nil
}

// updateCommand drives the inline ":" ex-prompt. Recognised commands:
//
//	:w   — show diff preview + arm save (same as ctrl+s).
//	:q   — quit. Refuses with a no-op when dirty (vim does the same);
//	       use :q! to discard.
//	:wq  — save, then quit on the FormSaveRequestedMsg success path.
//	:q!  — discard pending edits and quit.
//
// Anything else exits the prompt silently. Esc cancels.
func (f Form) updateCommand(km tea.KeyMsg) (Form, tea.Cmd) {
	switch km.String() {
	case "esc":
		f.mode = ModeNav
		f.commandInput.SetValue("")
		f.commandInput.Blur()
		return f, nil
	case "enter":
		raw := strings.TrimSpace(f.commandInput.Value())
		f.commandInput.SetValue("")
		f.commandInput.Blur()
		switch raw {
		case "w":
			f.mode = ModeNav
			if f.IsDirty() {
				f.computeDiff()
				f.mode = ModeConfirmSave
			}
			return f, nil
		case "wq":
			// Combined save+quit: enter the diff confirm; the host view
			// can listen for FormSaveRequestedMsg and pop back after a
			// successful save. Equivalent to :w then :q in two strokes.
			f.mode = ModeNav
			if !f.IsDirty() {
				return f, func() tea.Msg { return FormQuitRequestedMsg{} }
			}
			f.computeDiff()
			f.mode = ModeConfirmSave
			return f, nil
		case "q":
			if f.IsDirty() {
				f.mode = ModeConfirmDiscard
				return f, nil
			}
			f.mode = ModeNav
			return f, func() tea.Msg { return FormQuitRequestedMsg{} }
		case "q!":
			// Force-quit — discard the buffer and notify the host.
			f.mode = ModeNav
			return f, func() tea.Msg { return FormQuitRequestedMsg{} }
		}
		// Unknown command — silently bail back to nav. We don't echo
		// an error since the form's chrome is tight; the user just
		// sees the prompt clear and can try again.
		f.mode = ModeNav
		return f, nil
	}
	var cmd tea.Cmd
	f.commandInput, cmd = f.commandInput.Update(km)
	return f, cmd
}

// forwardToActive sends a non-key message to whichever textinput is
// active, so cursor blinks etc. keep flowing in edit modes.
func (f Form) forwardToActive(msg tea.Msg) (Form, tea.Cmd) {
	if len(f.rows) == 0 {
		return f, nil
	}
	var cmd tea.Cmd
	switch f.mode {
	case ModeKeyEdit:
		f.rows[f.selected].key, cmd = f.rows[f.selected].key.Update(msg)
	case ModeValueEdit:
		f.rows[f.selected].value, cmd = f.rows[f.selected].value.Update(msg)
	}
	return f, cmd
}

func (f *Form) blurAll() {
	for i := range f.rows {
		f.rows[i].key.Blur()
		f.rows[i].value.Blur()
	}
}

// computeDiff fills the cached diff counters by comparing the current
// data with the baseline. Counts are cached so re-renders during the
// confirm screen don't re-walk the maps.
func (f *Form) computeDiff() {
	cur := f.Data()
	added, removed, changed := 0, 0, 0
	for k, v := range cur {
		ov, ok := f.original[k]
		if !ok {
			added++
			continue
		}
		if !bytes.Equal(ov, v) {
			changed++
		}
	}
	for k := range f.original {
		if _, ok := cur[k]; !ok {
			removed++
		}
	}
	f.diffAdded = added
	f.diffRemoved = removed
	f.diffChanged = changed
}

// View renders the entire editor body, mode-aware.
func (f Form) View() string {
	var sb strings.Builder

	// Top strip — either the discard confirm or the status line.
	switch f.mode {
	case ModeConfirmDiscard:
		sb.WriteString(lipgloss.NewStyle().
			Foreground(theme.ColorWarn).
			Render("discard changes? y / n"))
		sb.WriteString("\n")
	default:
		sb.WriteString(f.renderStatusStrip())
		sb.WriteString("\n")
	}
	sb.WriteString(theme.Divider(f.dividerWidth()) + "\n")

	// Rows.
	for i, r := range f.rows {
		if i == f.selected && f.mode != ModeConfirmSave {
			sb.WriteString(f.renderFocusedRow(r))
		} else {
			sb.WriteString(f.renderCompactRow(r))
		}
	}

	// Save confirm banner replaces buttons when active.
	if f.mode == ModeConfirmSave {
		sb.WriteString("\n")
		sb.WriteString(f.renderSaveConfirm())
		return sb.String()
	}

	// Inline `:` ex-prompt replaces the button row while active —
	// keeps the form's vertical footprint stable across modes.
	if f.mode == ModeCommand {
		sb.WriteString("\n")
		sb.WriteString(f.renderCommandPrompt())
		return sb.String()
	}

	sb.WriteString("\n")
	sb.WriteString(f.renderButtons())
	return sb.String()
}

// renderCommandPrompt draws the inline ":" ex-mode line. Same accent
// prompt + textinput pattern as the app-level ex-mode, scoped to the
// form's width.
func (f Form) renderCommandPrompt() string {
	prompt := lipgloss.NewStyle().
		Foreground(theme.ColorAccent).
		Bold(true).
		Render(":") + " "
	hint := theme.Faint.Render("  ⏎ run · ⎋ cancel  (w · q · wq · q!)")
	return prompt + f.commandInput.View() + hint
}

func (f Form) renderStatusStrip() string {
	name := f.name
	if name == "" {
		name = "edit"
	}
	parts := []string{
		theme.Faint.Render("secret:") + " " +
			lipgloss.NewStyle().Foreground(theme.ColorFG).Render(name),
		theme.Dim.Render(fmt.Sprintf("%d keys", len(f.rows))),
	}
	if f.IsDirty() {
		parts = append(parts,
			lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("DIRTY"))
	}
	return strings.Join(parts, " · ")
}

func (f Form) renderFocusedRow(r formRow) string {
	innerW := f.dividerWidth()
	if innerW < 20 {
		innerW = 20
	}
	header := theme.Faint.Render("─── KEY ") +
		theme.Faint.Render(strings.Repeat("─", maxInt(0, innerW-9)))

	keyLine := " " + r.key.View()
	valHdr := theme.Dim.Render(" value:")
	// Give the value field the full inner width while editing.
	v := r.value
	v.Width = maxInt(10, innerW-2)
	valLine := " " + v.View()
	if r.hide && f.mode != ModeValueEdit {
		valLine = " " + theme.Dim.Render("••••••••••••••")
	}
	return header + "\n" + keyLine + "\n" + valHdr + "\n" + valLine + "\n"
}

func (f Form) renderCompactRow(r formRow) string {
	keyW := 24
	keyText := r.key.Value()
	if keyText == "" {
		keyText = theme.Faint.Render("(empty)")
	} else {
		keyText = lipgloss.NewStyle().Foreground(theme.ColorFG).Render(padRight(keyText, keyW))
	}
	valStr := r.value.Value()
	if r.hide {
		valStr = "••••"
	}
	maxVal := f.dividerWidth() - keyW - 2
	if maxVal < 4 {
		maxVal = 4
	}
	if len(valStr) > maxVal {
		valStr = valStr[:maxVal-1] + "…"
	}
	return keyText + "  " + theme.Dim.Render(valStr) + "\n"
}

func (f Form) renderSaveConfirm() string {
	added := lipgloss.NewStyle().Foreground(theme.ColorOk).
		Render(fmt.Sprintf("+added: %d", f.diffAdded))
	removed := lipgloss.NewStyle().Foreground(theme.ColorError).
		Render(fmt.Sprintf("-removed: %d", f.diffRemoved))
	changed := lipgloss.NewStyle().Foreground(theme.ColorWarn).
		Render(fmt.Sprintf("~changed: %d", f.diffChanged))
	hint := theme.Faint.Render("^s again to save · esc to cancel")
	return added + "\n" + removed + "\n" + changed + "\n" + hint
}

// chip renders a single accent-bordered key chip matching the bottom
// command bar's KeyChip styling. `accent` flips the foreground from
// muted to ColorAccent — used to highlight the save chip when dirty.
func chip(label, key string, accent bool) string {
	keyFG := theme.ColorMuted
	if accent {
		keyFG = theme.ColorAccent
	}
	keyStyled := lipgloss.NewStyle().
		Foreground(keyFG).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(keyFG).
		Padding(0, 1).
		Render(label + " " + key)
	return keyStyled
}

func (f Form) renderButtons() string {
	dirty := f.IsDirty()
	// Lead with vim verbs (i / : / dd / o) since those are the
	// canonical bindings now; ctrl+ chips kept as smaller faint
	// reminders so muscle memory across sessions still reads.
	chips := []string{
		chip("Edit", "i", false),
		chip("Save", ":w", dirty),
		chip("Quit", ":q", false),
		chip("Add", "o", false),
		chip("Del", "dd", false),
		chip("Hide", "H", false),
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, chips...)
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
