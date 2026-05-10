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
	if f.RowCount() != 0 {
		t.Errorf("want 0 rows, got %d", f.RowCount())
	}
}

func TestForm_Data(t *testing.T) {
	f := components.NewForm(map[string][]byte{
		"K1": []byte("v1"),
		"K2": []byte("v2"),
	})
	d := f.Data()
	if string(d["K1"]) != "v1" || string(d["K2"]) != "v2" {
		t.Errorf("Data() round-trip failed: %v", d)
	}
}

func TestForm_IsDirty(t *testing.T) {
	f := components.NewForm(map[string][]byte{"K": []byte("v")})
	if f.IsDirty() {
		t.Error("freshly built form should be clean")
	}
	f = f.AddRow("NEW", "val")
	if !f.IsDirty() {
		t.Error("form should be dirty after AddRow")
	}
}

// keyMsg builds a tea.KeyMsg matching the textinput parser.
func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

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

func msgFromCmd(c tea.Cmd) tea.Msg {
	if c == nil {
		return nil
	}
	return c()
}

// TestForm_EnterEntersEdit verifies ↵ on a row drops into ModeEdit on
// the value field.
func TestForm_EnterEntersEdit(t *testing.T) {
	f := components.NewForm(map[string][]byte{"K": []byte("v")})
	f, _ = sendKeys(t, f, "enter")
	if f.Mode() != components.ModeEdit {
		t.Errorf("want ModeEdit after enter, got %v", f.Mode())
	}
}

// TestForm_EditEscReturnsNav verifies esc in edit mode commits the
// field (textinput retains its value) and returns to nav.
func TestForm_EditEscReturnsNav(t *testing.T) {
	f := components.NewForm(map[string][]byte{"K": []byte("v")})
	f, _ = sendKeys(t, f, "enter", "x", "esc")
	if f.Mode() != components.ModeNav {
		t.Errorf("want ModeNav after esc-from-edit, got %v", f.Mode())
	}
	if !f.IsDirty() {
		t.Error("form should be dirty after typing in edit mode")
	}
}

// TestForm_EscOnCleanQuits verifies esc on a clean form emits
// FormQuitRequestedMsg straight away (no confirm dialog).
func TestForm_EscOnCleanQuits(t *testing.T) {
	f := components.NewForm(map[string][]byte{"K": []byte("v")})
	_, cmds := sendKeys(t, f, "esc")
	got := msgFromCmd(cmds[len(cmds)-1])
	if _, ok := got.(components.FormQuitRequestedMsg); !ok {
		t.Errorf("want FormQuitRequestedMsg from esc on clean form, got %T", got)
	}
}

// TestForm_EscOnDirtyOpensConfirmExit verifies esc on a dirty form
// opens the confirm-exit bar instead of quitting.
func TestForm_EscOnDirtyOpensConfirmExit(t *testing.T) {
	f := components.NewForm(map[string][]byte{"K": []byte("v")})
	f, _ = sendKeys(t, f, "enter", "x", "esc") // dirty + back to nav
	f, _ = sendKeys(t, f, "esc")
	if f.Mode() != components.ModeConfirmExit {
		t.Errorf("want ModeConfirmExit, got %v", f.Mode())
	}
}

// TestForm_ConfirmExitSaveEmits verifies `s` in ModeConfirmExit emits
// FormSaveRequestedMsg so the host view can persist via its service.
func TestForm_ConfirmExitSaveEmits(t *testing.T) {
	f := components.NewForm(map[string][]byte{"K": []byte("v")})
	f, _ = sendKeys(t, f, "enter", "x", "esc", "esc")
	if f.Mode() != components.ModeConfirmExit {
		t.Fatalf("setup: want ModeConfirmExit, got %v", f.Mode())
	}
	_, cmds := sendKeys(t, f, "s")
	got := msgFromCmd(cmds[len(cmds)-1])
	if _, ok := got.(components.FormSaveRequestedMsg); !ok {
		t.Errorf("want FormSaveRequestedMsg from `s`, got %T", got)
	}
}

// TestForm_ConfirmExitDiscardEmits verifies `d` emits FormQuitRequestedMsg.
func TestForm_ConfirmExitDiscardEmits(t *testing.T) {
	f := components.NewForm(map[string][]byte{"K": []byte("v")})
	f, _ = sendKeys(t, f, "enter", "x", "esc", "esc")
	_, cmds := sendKeys(t, f, "d")
	got := msgFromCmd(cmds[len(cmds)-1])
	if _, ok := got.(components.FormQuitRequestedMsg); !ok {
		t.Errorf("want FormQuitRequestedMsg from `d`, got %T", got)
	}
}

// TestForm_ConfirmExitCancelReturnsNav verifies esc in the confirm bar
// drops back to nav so the user can keep editing.
func TestForm_ConfirmExitCancelReturnsNav(t *testing.T) {
	f := components.NewForm(map[string][]byte{"K": []byte("v")})
	f, _ = sendKeys(t, f, "enter", "x", "esc", "esc")
	f, _ = sendKeys(t, f, "esc")
	if f.Mode() != components.ModeNav {
		t.Errorf("want ModeNav after cancel, got %v", f.Mode())
	}
}

// TestForm_DDDeletes verifies the `dd` two-stroke removes the current row.
func TestForm_DDDeletes(t *testing.T) {
	f := components.NewForm(map[string][]byte{
		"A": []byte("1"),
		"B": []byte("2"),
	})
	before := f.RowCount()
	f, _ = sendKeys(t, f, "d", "d")
	if f.RowCount() != before-1 {
		t.Errorf("want %d rows after dd, got %d", before-1, f.RowCount())
	}
}

// TestForm_OAddsRow verifies `o` appends and lands on the new row in edit.
func TestForm_OAddsRow(t *testing.T) {
	f := components.NewForm(map[string][]byte{"K": []byte("v")})
	before := f.RowCount()
	f, _ = sendKeys(t, f, "o")
	if f.RowCount() != before+1 {
		t.Errorf("want %d rows after o, got %d", before+1, f.RowCount())
	}
	if f.Mode() != components.ModeEdit {
		t.Errorf("want ModeEdit after o, got %v", f.Mode())
	}
}
