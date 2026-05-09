package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// Column defines a table column's header and fixed display width.
type Column struct {
	Header string
	Width  int
}

// Row is a slice of string cells, one per column.
type Row []string

// Table is an immutable, value-type table component.
// All mutation methods return a new Table — safe to embed in Bubble Tea models.
type Table struct {
	cols     []Column
	rows     []Row
	selected int
	width    int // total terminal width, used for the divider
}

func NewTable(cols []Column, rows []Row) Table {
	return Table{cols: cols, rows: rows}
}

func (t Table) SetRows(rows []Row) Table {
	t.rows = rows
	if t.selected >= len(rows) && len(rows) > 0 {
		t.selected = len(rows) - 1
	}
	if len(rows) == 0 {
		t.selected = 0
	}
	return t
}

func (t Table) SetWidth(w int) Table { t.width = w; return t }

func (t Table) SelectedIndex() int { return t.selected }
func (t Table) RowCount() int      { return len(t.rows) }

func (t Table) SelectedRow() Row {
	if len(t.rows) == 0 {
		return nil
	}
	return t.rows[t.selected]
}

func (t Table) MoveDown() Table {
	if t.selected < len(t.rows)-1 {
		t.selected++
	}
	return t
}

func (t Table) MoveUp() Table {
	if t.selected > 0 {
		t.selected--
	}
	return t
}

func (t Table) MoveTop() Table { t.selected = 0; return t }

func (t Table) MoveBottom() Table {
	if len(t.rows) > 0 {
		t.selected = len(t.rows) - 1
	}
	return t
}

func (t Table) View() string {
	var sb strings.Builder

	// Header row
	for i, c := range t.cols {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(theme.ColHeader.Width(c.Width).Render(c.Header))
	}
	sb.WriteString("\n")
	sb.WriteString(theme.Divider(t.width))
	sb.WriteString("\n")

	// Data rows
	for i, row := range t.rows {
		sel := i == t.selected
		for j, c := range t.cols {
			if j > 0 {
				sb.WriteString("  ")
			}
			val := ""
			if j < len(row) {
				val = row[j]
			}
			var style lipgloss.Style
			if sel {
				style = lipgloss.NewStyle().
					Background(theme.ColorSel).
					Foreground(theme.ColorFG).
					Width(c.Width)
			} else {
				style = lipgloss.NewStyle().
					Foreground(theme.ColorMid).
					Width(c.Width)
			}
			sb.WriteString(style.Render(val))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
