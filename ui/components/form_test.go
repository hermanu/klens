package components_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hermanu/klens/ui/components"
)

func TestForm_NewFromData(t *testing.T) {
	f := components.NewForm(map[string][]byte{
		"API_KEY": []byte("secret"),
		"DB_URL":  []byte("postgres://"),
	})
	if f.RowCount() != 2 {
		t.Errorf("want 2 rows, got %d", f.RowCount())
	}
}

func TestForm_AddRow(t *testing.T) {
	f := components.NewForm(map[string][]byte{"KEY": []byte("val")})
	f = f.AddRow("NEW_KEY", "new-value")
	if f.RowCount() != 2 {
		t.Errorf("want 2 rows after AddRow, got %d", f.RowCount())
	}
}

func TestForm_DeleteSelected(t *testing.T) {
	f := components.NewForm(map[string][]byte{
		"A": []byte("1"),
		"B": []byte("2"),
	})
	before := f.RowCount()
	f = f.DeleteSelected()
	if f.RowCount() != before-1 {
		t.Errorf("want %d rows after delete, got %d", before-1, f.RowCount())
	}
}

func TestForm_DeleteLast(t *testing.T) {
	f := components.NewForm(map[string][]byte{"ONLY": []byte("one")})
	f = f.DeleteSelected()
	// deleting the last row should leave 0 rows without panicking
	if f.RowCount() != 0 {
		t.Errorf("want 0 rows, got %d", f.RowCount())
	}
}

func TestForm_Data(t *testing.T) {
	f := components.NewForm(map[string][]byte{
		"KEY": []byte("val"),
	})
	data := f.Data()
	if string(data["KEY"]) != "val" {
		t.Errorf("want val, got %s", data["KEY"])
	}
}

func TestForm_IsDirty(t *testing.T) {
	f := components.NewForm(map[string][]byte{"K": []byte("v")})
	if f.IsDirty() {
		t.Error("fresh form should not be dirty")
	}
	f = f.AddRow("K2", "v2")
	if !f.IsDirty() {
		t.Error("form should be dirty after AddRow")
	}
}

func TestForm_HideToggle(t *testing.T) {
	f := components.NewForm(map[string][]byte{"SECRET": []byte("password")})
	if f.IsHidden(0) {
		t.Error("row should not be hidden by default")
	}
	f = f.ToggleHide(0)
	if !f.IsHidden(0) {
		t.Error("row should be hidden after ToggleHide")
	}
}

// keyMsg builds a tea.KeyMsg matching the textinput parser. For ASCII
// characters we use KeyRunes; for named keys we use the corresponding
// tea.KeyType.
func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+a":
		return tea.KeyMsg{Type: tea.KeyCtrlA}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+h":
		return tea.KeyMsg{Type: tea.KeyCtrlH}
	}
	// Single rune fallback.
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// sendKeys feeds a sequence of keys through Update and returns the
// final form plus a slice of any commands produced (one per keystroke).
func sendKeys(t *testing.T, f components.Form, keys ...string) (components.Form, []tea.Cmd) {
	t.Helper()
	cmds := make([]tea.Cmd, 0, len(keys))
	for _, k := range keys {
		var cmd tea.Cmd
		f, cmd = f.Update(keyMsg(k))
		cmds = append(cmds, cmd)
	}
	return f, cmds
}

func TestForm_DirtyAfterEditValue(t *testing.T) {
	f := components.NewForm(map[string][]byte{"DATABASE_URL": []byte("postgres://old")})
	if f.IsDirty() {
		t.Fatal("fresh form should not be dirty")
	}
	// Enter ModeValueEdit, type "x".
	f, _ = sendKeys(t, f, "right", "x")
	if f.Mode() != components.ModeValueEdit {
		t.Errorf("want ModeValueEdit, got %v", f.Mode())
	}
	if !f.IsDirty() {
		t.Error("form should be dirty after typing in value editor")
	}
}

func TestForm_EscWhenDirtyOpensConfirm(t *testing.T) {
	f := components.NewForm(map[string][]byte{"K": []byte("v")})
	// Edit value to make it dirty.
	f, _ = sendKeys(t, f, "right", "x", "esc")
	// Back in ModeNav, still dirty.
	if !f.IsDirty() {
		t.Fatal("expected dirty after edit")
	}
	if f.Mode() != components.ModeNav {
		t.Fatalf("want ModeNav after esc-from-edit, got %v", f.Mode())
	}
	// Now esc again — should open discard confirm.
	f, _ = sendKeys(t, f, "esc")
	if f.Mode() != components.ModeConfirmDiscard {
		t.Errorf("want ModeConfirmDiscard, got %v", f.Mode())
	}
}

func TestForm_DiscardYReturnsClean(t *testing.T) {
	f := components.NewForm(map[string][]byte{"K": []byte("v")})
	f, _ = sendKeys(t, f, "right", "x", "esc", "esc")
	if f.Mode() != components.ModeConfirmDiscard {
		t.Fatalf("setup: want ModeConfirmDiscard, got %v", f.Mode())
	}
	f, _ = sendKeys(t, f, "y")
	if f.IsDirty() {
		t.Error("form should be clean after discard")
	}
	if f.Mode() != components.ModeNav {
		t.Errorf("want ModeNav after discard, got %v", f.Mode())
	}
	if got := string(f.Data()["K"]); got != "v" {
		t.Errorf("want original value v, got %q", got)
	}
}

// fakeSaveCmd is a sentinel used to identify our own emitted message.
func msgFromCmd(c tea.Cmd) tea.Msg {
	if c == nil {
		return nil
	}
	return c()
}

func TestForm_SaveTwoStep(t *testing.T) {
	f := components.NewForm(map[string][]byte{"K": []byte("v")})
	// Edit, ^s -> opens save confirm.
	f, cmds := sendKeys(t, f, "right", "x", "esc", "ctrl+s")
	if f.Mode() != components.ModeConfirmSave {
		t.Fatalf("want ModeConfirmSave after first ^s, got %v", f.Mode())
	}
	// No FormSaveRequestedMsg should have been emitted yet.
	for i, c := range cmds {
		if _, ok := msgFromCmd(c).(components.FormSaveRequestedMsg); ok {
			t.Fatalf("save msg leaked from keystroke %d (before second ^s)", i)
		}
	}
	// Second ^s commits.
	_, cmds = sendKeys(t, f, "ctrl+s")
	if len(cmds) != 1 {
		t.Fatalf("want one cmd from second ^s, got %d", len(cmds))
	}
	msg := msgFromCmd(cmds[0])
	if _, ok := msg.(components.FormSaveRequestedMsg); !ok {
		t.Errorf("want FormSaveRequestedMsg, got %T (%v)", msg, msg)
	}
}

func TestForm_DiffCounts(t *testing.T) {
	// Original {a:1, b:2}.
	f := components.NewForm(map[string][]byte{
		"a": []byte("1"),
		"b": []byte("2"),
	})
	// Mutate to {a:1, b:9, c:3}: change b, add c.
	// b is at row index 1 (NewForm sorts keys), so down to b, edit value to 9.
	// Currently selected=0 (a). down→b. right→ModeValueEdit. Type 9 — but
	// the existing value is "2"; typing 9 appends → "29". Use backspace first.
	f, _ = sendKeys(t, f, "down", "right")
	// Send a backspace then "9".
	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	f, _ = f.Update(keyMsg("9"))
	// Back to nav, add a row.
	f, _ = sendKeys(t, f, "esc", "ctrl+a")
	// In ModeKeyEdit on the new row — type "c".
	f, _ = f.Update(keyMsg("c"))
	// Tab/right to value, type "3".
	f, _ = sendKeys(t, f, "esc", "right")
	f, _ = f.Update(keyMsg("3"))
	// Trigger save confirm to populate diff counts.
	f, _ = sendKeys(t, f, "esc", "ctrl+s")
	if f.Mode() != components.ModeConfirmSave {
		t.Fatalf("want ModeConfirmSave, got %v", f.Mode())
	}
	a, r, c := f.DiffCounts()
	if a != 1 {
		t.Errorf("added: want 1, got %d", a)
	}
	if r != 0 {
		t.Errorf("removed: want 0, got %d", r)
	}
	if c != 1 {
		t.Errorf("changed: want 1, got %d", c)
	}
}
