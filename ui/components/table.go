package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// Column defines a table column's header and display width. Width is the
// minimum/baseline; Flex columns absorb any extra horizontal slack the table
// gets so the content stretches to the table's full assigned width instead
// of leaving an empty band on the right.
type Column struct {
	Header string
	Width  int
	Align  Align // default Left
	Flex   bool  // when true, the column grows to absorb leftover width
}

// Align is the cell alignment mode.
type Align int

// Alignment values for Align.
const (
	AlignLeft Align = iota
	AlignRight
)

// Row is a slice of string cells, one per column.
// Cells may contain pre-rendered ANSI styling (chips, pills, sparklines) — the
// table preserves it.
type Row []string

// Table is an immutable, value-type table component.
// All mutation methods return a new Table — safe to embed in Bubble Tea models.
type Table struct {
	cols     []Column
	rows     []Row
	selected int
	width    int // total terminal width, used for the divider
	height   int // available rows for the table (header + divider + body)
	offset   int // first visible row, advanced by selection-following scroll
}

// NewTable creates a Table with the given columns and initial rows.
func NewTable(cols []Column, rows []Row) Table {
	return Table{cols: cols, rows: rows}
}

// SetRows replaces the row data and clamps the selection to the new length.
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

// SetWidth sets the terminal width allocated to the table.
func (t Table) SetWidth(w int) Table { t.width = w; return t }

// SetHeight sets the number of terminal rows allocated to the table.
func (t Table) SetHeight(h int) Table { t.height = h; return t }

// SelectedIndex returns the zero-based index of the focused row.
func (t Table) SelectedIndex() int { return t.selected }

// RowCount returns the total number of rows in the table.
func (t Table) RowCount() int { return len(t.rows) }

// SelectedRow returns the currently focused Row, or nil if the table is empty.
func (t Table) SelectedRow() Row {
	if len(t.rows) == 0 {
		return nil
	}
	return t.rows[t.selected]
}

// MoveDown advances the selection by one row, stopping at the last row.
func (t Table) MoveDown() Table {
	if t.selected < len(t.rows)-1 {
		t.selected++
	}
	return t
}

// MoveUp moves the selection up by one row, stopping at the first row.
func (t Table) MoveUp() Table {
	if t.selected > 0 {
		t.selected--
	}
	return t
}

// MoveTop moves the selection to the first row.
func (t Table) MoveTop() Table { t.selected = 0; return t }

// MoveBottom moves the selection to the last row.
func (t Table) MoveBottom() Table {
	if len(t.rows) > 0 {
		t.selected = len(t.rows) - 1
	}
	return t
}

// View renders the table to a string, including the header row and selection cursor.
func (t Table) View() string {
	var sb strings.Builder

	colWidths := t.resolvedWidths()

	// Header row — left-padded by 2 cols so the headers align with row cells
	// (rows reserve col 0 for the `▌` cursor and col 1 for the `›` glyph).
	sb.WriteString("  ")
	for i, c := range t.cols {
		if i > 0 {
			sb.WriteString("  ")
		}
		s := theme.ColHeader.Width(colWidths[i])
		if c.Align == AlignRight {
			s = s.Align(lipgloss.Right)
		}
		sb.WriteString(s.Render(c.Header))
	}
	sb.WriteString("\n")
	sb.WriteString(theme.Divider(t.width))
	sb.WriteString("\n")

	// Compute viewport: pageSize is the body's row capacity. We reserve 3
	// rows for header + divider + status hint at the bottom; everything else
	// goes to data rows. On a 30-row terminal that's ~27 visible pods.
	pageSize := t.height - 3
	if pageSize < 1 {
		pageSize = 20 // default when caller hasn't set height yet
	}
	// Selection-following scroll: keep the cursor between [offset, offset+pageSize).
	offset := t.offset
	if t.selected < offset {
		offset = t.selected
	} else if t.selected >= offset+pageSize {
		offset = t.selected - pageSize + 1
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + pageSize
	if end > len(t.rows) {
		end = len(t.rows)
	}

	rendered := end - offset
	for i := offset; i < end; i++ {
		sel := i == t.selected
		sb.WriteString(t.renderRow(t.rows[i], sel, colWidths))
		sb.WriteString("\n")
	}
	// Pad blank rows so the table always consumes its full budgeted height.
	// Without this, short lists let the focus frame's bottom edge ride up
	// against the last row, and the position hint below would jump up/down
	// depending on how many rows are visible. The fixed height keeps both
	// the frame and the hint glued to a stable position.
	for i := rendered; i < pageSize; i++ {
		sb.WriteString("\n")
	}
	// Footer: position indicator, always rendered on the same row (just
	// below the padded body). Helpful when scrolling through hundreds of
	// rows so the user knows where they are.
	if len(t.rows) > 0 {
		hint := fmt.Sprintf("  %d–%d of %d", offset+1, end, len(t.rows))
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(hint))
	}
	return sb.String()
}

// resolvedWidths returns one width per column after distributing any leftover
// table width across the Flex columns. Without this, fixed-width columns
// summed to ~140 cols leave a trailing gap on terminals 200+ cols wide; with
// it, the flex column (typically NAME) absorbs the slack and the table fills
// edge to edge.
func (t Table) resolvedWidths() []int {
	out := make([]int, len(t.cols))
	totalFixed := 0
	flexCount := 0
	for i, c := range t.cols {
		out[i] = c.Width
		totalFixed += c.Width
		if c.Flex {
			flexCount++
		}
	}
	if flexCount == 0 || t.width <= 0 {
		return out
	}
	// Account for the cursor column (2 cols) + 2-col separators between
	// columns, which the row renderer adds on top of the column widths.
	const cursorCols = 2
	separators := (len(t.cols) - 1) * 2
	consumed := cursorCols + separators + totalFixed
	slack := t.width - consumed - 1 // -1 for a tiny right-edge breathing room
	if slack <= 0 {
		return out
	}
	share := slack / flexCount
	rem := slack - share*flexCount
	for i, c := range t.cols {
		if c.Flex {
			out[i] += share
			if rem > 0 {
				out[i]++
				rem--
			}
		}
	}
	return out
}

// truncateCell clamps a cell to at most maxWidth display columns. ANSI escape
// sequences are preserved verbatim and don't count toward the budget; printable
// runes do, using lipgloss-aware width measurement (multi-byte / wide chars).
func truncateCell(s string, maxWidth int, hasANSI bool) string {
	if maxWidth < 1 {
		return ""
	}
	if !hasANSI {
		// Fast path — plain string. Use rune-by-rune width counting so we
		// don't slice mid-rune.
		if lipgloss.Width(s) <= maxWidth {
			return s
		}
		var b strings.Builder
		w := 0
		for _, r := range s {
			rw := lipgloss.Width(string(r))
			if w+rw > maxWidth {
				break
			}
			b.WriteRune(r)
			w += rw
		}
		return b.String()
	}
	// ANSI path — copy escape sequences verbatim, count visible runes.
	var b strings.Builder
	w := 0
	inEsc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEsc = true
			b.WriteRune(r)
		case inEsc:
			b.WriteRune(r)
			if (r >= '@' && r <= '~') && r != '[' {
				inEsc = false
			}
		default:
			rw := lipgloss.Width(string(r))
			if w+rw > maxWidth {
				// Emit a reset so trailing styles don't bleed into next cell.
				b.WriteString("\x1b[0m")
				return b.String()
			}
			b.WriteRune(r)
			w += rw
		}
	}
	return b.String()
}

// renderRow renders one row. The selected row gets an accent left bar (`▌`),
// a `›` cursor on the first cell, and a bold accent foreground on every
// plain-text cell — three independent visual cues so the focus is obvious.
func (t Table) renderRow(row Row, sel bool, colWidths []int) string {
	var sb strings.Builder

	// Left bar (1 col): accent for selected, transparent otherwise.
	if sel {
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("▌"))
	} else {
		sb.WriteString(" ")
	}

	for j, c := range t.cols {
		if j > 0 {
			sb.WriteString("  ")
		}
		val := ""
		if j < len(row) {
			val = row[j]
		}
		// Selection cursor on the first cell of the selected row.
		switch {
		case sel && j == 0:
			val = "› " + val
		case j == 0:
			val = "  " + val
		}

		w := colWidths[j]
		// First column absorbs the cursor prefix without changing column width.
		// Lipgloss handles ANSI when measuring width, so we don't subtract.

		hasANSI := strings.ContainsRune(val, '\x1b')
		// Truncate before width-styling: lipgloss.Width(w) wraps long lines
		// to a second row, which destroys the table's column alignment.
		val = truncateCell(val, w, hasANSI)
		style := lipgloss.NewStyle().Width(w)
		if c.Align == AlignRight {
			style = style.Align(lipgloss.Right)
		}
		switch {
		case sel:
			// Selected row: bold + accent foreground for any plain-text cell
			// so the cursor row is unmistakable. ANSI cells (chips, status
			// pills, sparklines) keep their own colors but still get bolded.
			style = style.Bold(true)
			if !hasANSI {
				style = style.Foreground(theme.ColorAccent)
			}
		default:
			if !hasANSI {
				style = style.Foreground(theme.ColorFG2)
			}
		}
		sb.WriteString(style.Render(val))
	}
	return sb.String()
}
