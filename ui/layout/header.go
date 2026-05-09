package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/manu/klens/ui/theme"
)

// FilterChip represents an active filter shown in the header.
type FilterChip struct {
	Key    string
	Op     string   // operator, e.g. "≥", ":" — defaults to ":"
	Value  string
	Strong bool     // true = error-level chip (accent color)
}

// HeaderConfig holds all data needed to render the context bar.
type HeaderConfig struct {
	Cluster   string
	Namespace string
	Filters   []FilterChip
	Count     int
	Total     int
	Watching  bool // true = live tailing dot, false = paused
}

// Header renders the top context bar.
// Design: matches modern.jsx ContextBar component.
func Header(width int, cfg HeaderConfig) string {
	// Breadcrumb: cluster › namespace
	cluster := cfg.Cluster
	if cluster == "" {
		cluster = "—"
	}
	ns := cfg.Namespace
	if ns == "" {
		ns = "all"
	}
	crumb := theme.Faint.Render("cluster") + " " +
		theme.Faint.Render("›") + " " +
		theme.Base.Render(cluster) + "  " +
		theme.Faint.Render("ns") + " " +
		theme.Faint.Render("›") + " " +
		theme.Base.Render(ns)

	// Filter chips
	var chips []string
	for _, f := range cfg.Filters {
		op := f.Op
		if op == "" {
			op = ":"
		}
		text := fmt.Sprintf("%s %s %s ×", f.Key, op, f.Value)
		if f.Strong {
			chips = append(chips, theme.ChipStrong.Render(text))
		} else {
			chips = append(chips, theme.Chip.Render(text))
		}
	}
	chips = append(chips, theme.Faint.Render("+ filter"))
	chipsStr := strings.Join(chips, " ")

	// Counter + watch indicator
	counter := theme.Base.Render(fmt.Sprintf("%d", cfg.Count)) +
		theme.Faint.Render(fmt.Sprintf("/%d", cfg.Total))
	var indicator string
	if cfg.Watching {
		indicator = theme.Accent.Render("● ") + theme.Base.Render("watching")
	} else {
		indicator = theme.Faint.Render("○ paused")
	}

	left := crumb + "  " + chipsStr
	right := counter + "  " + indicator

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return theme.Panel.Width(width).Padding(0, 1).Render(line)
}
