package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// FilterChips renders the filter strip directly above the table body.
// Layout: chip chip chip chip ........................................ ● watch
//
// The filtered/total count used to live here ("matched 4/12"); it has been
// promoted to the top bar's scope row (TopBarConfig.VisibleCount/TotalCount)
// so it sits at a stable screen column across every view. This strip now
// shows ONLY the active filter pills plus the live-watch dot at the right
// edge.
//
// `count` and `total` are accepted to keep the existing call signature stable;
// they're intentionally unused here. We keep the parameters rather than break
// callers — the count is rendered by TopBar.
func FilterChips(width int, chips []FilterChip, count, total int) string {
	_ = count
	_ = total
	if width < 1 {
		width = 1
	}

	parts := make([]string, 0, len(chips))
	for _, c := range chips {
		op := c.Op
		if op == "" {
			op = ":"
		}
		// "key : value ×" — × is the dismiss affordance.
		text := fmt.Sprintf("%s %s %s ×", c.Key, op, c.Value)
		if c.Strong {
			parts = append(parts, theme.ChipStrong.Render(text))
		} else {
			parts = append(parts, theme.Chip.Render(text))
		}
	}
	left := strings.Join(parts, " ")

	watch := lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("●") + " " +
		lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("watch")

	inner := width - 2 // padding 0,1 from Panel
	if inner < 1 {
		inner = 1
	}
	gap := inner - lipgloss.Width(left) - lipgloss.Width(watch)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + watch
	return theme.Panel.Width(width).Padding(0, 1).Render(line)
}
