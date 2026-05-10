package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// ContextPicker renders a centered modal listing available kubeconfig
// contexts. Used at startup when no current-context is set (or `klens` was
// pointed at a kubeconfig with multiple contexts and no default), so the user
// gets a list-pick UX instead of a cryptic "no current-context" error.
//
// The component is purely a renderer — it doesn't own its own selection state
// because the shell already knows how to wire arrow-key + enter routing
// against a numeric index. Pass `selected` from the model.
func ContextPicker(width, height int, contexts []string, selected int, errMsg string) string {
	title := lipgloss.NewStyle().
		Foreground(theme.ColorAccent).
		Bold(true).
		Render("select cluster")

	subtitle := theme.Faint.Render("kubeconfig contexts found — pick one to continue")

	var sb strings.Builder
	sb.WriteString(title + "\n")
	sb.WriteString(subtitle + "\n\n")

	if len(contexts) == 0 {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(theme.ColorWarn).
			Render("no contexts in kubeconfig — set $KUBECONFIG or run `aws eks update-kubeconfig …`"))
	} else {
		for i, c := range contexts {
			row := renderContextRow(c, i == selected)
			sb.WriteString(row + "\n")
		}
	}
	if errMsg != "" {
		sb.WriteString("\n" + lipgloss.NewStyle().Foreground(theme.ColorError).Render(errMsg))
	}
	sb.WriteString("\n")
	sb.WriteString(theme.Faint.Render("↑↓ pick · ⏎ select · esc quit"))

	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorAccent).
		Padding(1, 2).
		Render(sb.String())

	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		body,
	)
}

func renderContextRow(name string, active bool) string {
	if active {
		cursor := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render("▌")
		text := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Render(name)
		return cursor + " " + text
	}
	return "  " + lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(name)
}
