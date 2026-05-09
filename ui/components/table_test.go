package components_test

import (
	"testing"

	"github.com/hermanu/klens/ui/components"
)

func TestTable_Navigation(t *testing.T) {
	cols := []components.Column{
		{Header: "NAME", Width: 20},
		{Header: "STATUS", Width: 10},
	}
	rows := []components.Row{
		{"pod-1", "Running"},
		{"pod-2", "Pending"},
		{"pod-3", "Error"},
	}
	tbl := components.NewTable(cols, rows)

	if tbl.SelectedIndex() != 0 {
		t.Errorf("want index 0, got %d", tbl.SelectedIndex())
	}
	tbl = tbl.MoveDown()
	if tbl.SelectedIndex() != 1 {
		t.Errorf("want index 1 after MoveDown, got %d", tbl.SelectedIndex())
	}
	tbl = tbl.MoveUp()
	if tbl.SelectedIndex() != 0 {
		t.Errorf("want index 0 after MoveUp, got %d", tbl.SelectedIndex())
	}
}

func TestTable_Boundaries(t *testing.T) {
	cols := []components.Column{{Header: "NAME", Width: 20}}
	rows := []components.Row{{"a"}, {"b"}}
	tbl := components.NewTable(cols, rows)

	// MoveUp at top should stay at 0
	tbl = tbl.MoveUp()
	if tbl.SelectedIndex() != 0 {
		t.Errorf("want 0 at top boundary, got %d", tbl.SelectedIndex())
	}

	// MoveBottom then MoveDown should stay at last
	tbl = tbl.MoveBottom()
	tbl = tbl.MoveDown()
	if tbl.SelectedIndex() != 1 {
		t.Errorf("want 1 at bottom boundary, got %d", tbl.SelectedIndex())
	}
}

func TestTable_SetRows(t *testing.T) {
	cols := []components.Column{{Header: "NAME", Width: 20}}
	tbl := components.NewTable(cols, nil)
	tbl = tbl.SetRows([]components.Row{{"pod-1"}, {"pod-2"}})
	if tbl.RowCount() != 2 {
		t.Errorf("want 2, got %d", tbl.RowCount())
	}
}

func TestTable_SelectedRow(t *testing.T) {
	cols := []components.Column{{Header: "NAME", Width: 20}}
	rows := []components.Row{{"alpha"}, {"beta"}}
	tbl := components.NewTable(cols, rows)
	tbl = tbl.MoveDown()
	row := tbl.SelectedRow()
	if row == nil || row[0] != "beta" {
		t.Errorf("want beta, got %v", row)
	}
}

func TestTable_EmptyRows(t *testing.T) {
	cols := []components.Column{{Header: "NAME", Width: 20}}
	tbl := components.NewTable(cols, nil)
	if tbl.SelectedRow() != nil {
		t.Error("want nil SelectedRow for empty table")
	}
}
