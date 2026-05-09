package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hermanu/klens/ui/theme"
)

type formRow struct {
	key   textinput.Model
	value textinput.Model
	hide  bool
}

// Form is an immutable key/value form editor for secrets and configmaps.
// All mutation methods return a new Form — safe for Bubble Tea models.
type Form struct {
	rows     []formRow
	selected int // which row has focus
	col      int // 0=key column, 1=value column
	dirty    bool
}

// NewForm creates a Form pre-populated from a decoded secret Data map.
func NewForm(data map[string][]byte) Form {
	rows := make([]formRow, 0, len(data))
	for k, v := range data {
		rows = append(rows, newRow(k, string(v)))
	}
	f := Form{rows: rows}
	f.focusCurrent()
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

func (f Form) RowCount() int { return len(f.rows) }
func (f Form) IsDirty() bool { return f.dirty }
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

// AddRow appends a new row with the given key/value.
func (f Form) AddRow(k, v string) Form {
	f.rows = append(f.rows, newRow(k, v))
	f.dirty = true
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
	f.dirty = true
	f.focusCurrent()
	return f
}

// Data exports all non-empty key rows as a map[string][]byte for saving.
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

// Update handles keyboard input, returning an updated Form and optional Cmd.
func (f Form) Update(msg tea.Msg) (Form, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			f.col = (f.col + 1) % 2
			f.focusCurrent()
			return f, nil
		case "shift+tab":
			f.col = (f.col + 1) % 2
			f.focusCurrent()
			return f, nil
		case "down", "ctrl+n":
			if f.selected < len(f.rows)-1 {
				f.selected++
				f.col = 0
				f.focusCurrent()
			}
			return f, nil
		case "up", "ctrl+p":
			if f.selected > 0 {
				f.selected--
				f.col = 0
				f.focusCurrent()
			}
			return f, nil
		case "ctrl+h":
			if f.selected < len(f.rows) {
				f.rows[f.selected].hide = !f.rows[f.selected].hide
			}
			return f, nil
		case "ctrl+d":
			return f.DeleteSelected(), nil
		}
	}
	// Forward to the focused textinput
	var cmd tea.Cmd
	if len(f.rows) > 0 {
		if f.col == 0 {
			f.rows[f.selected].key, cmd = f.rows[f.selected].key.Update(msg)
		} else {
			f.rows[f.selected].value, cmd = f.rows[f.selected].value.Update(msg)
		}
		f.dirty = true
	}
	return f, cmd
}

// focusCurrent focuses the active textinput and blurs all others.
// NOTE: mutates in place — only called on copies (value receiver pattern).
func (f *Form) focusCurrent() {
	for i := range f.rows {
		f.rows[i].key.Blur()
		f.rows[i].value.Blur()
	}
	if len(f.rows) == 0 {
		return
	}
	if f.col == 0 {
		f.rows[f.selected].key.Focus()
	} else {
		f.rows[f.selected].value.Focus()
	}
}

// View renders the form editor.
func (f Form) View() string {
	var sb strings.Builder

	// Column headers
	sb.WriteString(
		theme.ColHeader.Width(26).Render("KEY") + "  " +
			theme.ColHeader.Width(42).Render("VALUE") + "\n",
	)
	sb.WriteString(theme.Divider(72) + "\n")

	for i, r := range f.rows {
		sel := i == f.selected

		valStr := r.value.View()
		if r.hide && !sel {
			valStr = theme.Dim.Render("••••••••••••••")
		}

		line := r.key.View() + "  " + valStr
		if sel {
			line = theme.Selected.Render(line)
		}
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(
		theme.Faint.Render("[ + Add key ]") + "  " +
			theme.Accent.Render("[ Save ]") + "  " +
			theme.Dim.Render("[ Cancel ]") + "\n",
	)
	sb.WriteString("\n")
	sb.WriteString(theme.Faint.Render(
		"<tab> col  <↑↓> row  <ctrl+a> add  <ctrl+d> del  <ctrl+h> hide  <ctrl+s> save  <esc> cancel",
	))
	return sb.String()
}
