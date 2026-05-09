package components_test

import (
	"testing"

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
