package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/manu/klens/ui/theme"
)

// PaletteCmd is a single command in the palette.
type PaletteCmd struct {
	Name  string // e.g. "pods"
	Desc  string // e.g. "list pods"
	Alias string // e.g. ":po"
}

// defaultCmds are the built-in resource navigation commands.
var defaultCmds = []PaletteCmd{
	{Name: "pods",        Desc: "list pods",                     Alias: ":po"},
	{Name: "deployments", Desc: "list deployments",              Alias: ":dp"},
	{Name: "services",    Desc: "list services",                 Alias: ":svc"},
	{Name: "secrets",     Desc: "list secrets",                  Alias: ":sec"},
	{Name: "configmaps",  Desc: "list configmaps",               Alias: ":cm"},
	{Name: "namespaces",  Desc: "list namespaces",               Alias: ":ns"},
	{Name: "nodes",       Desc: "list nodes",                    Alias: ":no"},
	{Name: "pvcs",        Desc: "list persistent volume claims", Alias: ":pvc"},
	{Name: "quit",        Desc: "exit klens",                    Alias: ":q"},
}

// Palette is an immutable command palette component.
type Palette struct {
	cmds     []PaletteCmd
	input    textinput.Model
	selected int
}

// NewPalette creates a Palette. Pass nil for cmds to use the default resource list.
func NewPalette(cmds []PaletteCmd) Palette {
	if cmds == nil {
		cmds = defaultCmds
	}
	ti := textinput.New()
	ti.Focus()
	ti.Placeholder = "resource or command..."
	ti.CharLimit = 64
	return Palette{cmds: cmds, input: ti}
}

// SetInput sets the filter text directly (used in tests and keyboard shortcuts).
func (p Palette) SetInput(s string) Palette {
	p.input.SetValue(s)
	p.selected = 0
	return p
}

// Filtered returns commands matching the current input.
func (p Palette) Filtered() []PaletteCmd {
	q := strings.ToLower(strings.TrimSpace(p.input.Value()))
	if q == "" {
		return p.cmds
	}
	var out []PaletteCmd
	for _, c := range p.cmds {
		if strings.Contains(c.Name, q) || strings.Contains(c.Alias, q) {
			out = append(out, c)
		}
	}
	return out
}

// Selected returns the currently highlighted command, or nil if the list is empty.
func (p Palette) Selected() *PaletteCmd {
	f := p.Filtered()
	if len(f) == 0 || p.selected >= len(f) {
		return nil
	}
	c := f[p.selected]
	return &c
}

// MoveDown moves the selection down one row.
func (p Palette) MoveDown() (Palette, tea.Cmd) {
	if f := p.Filtered(); p.selected < len(f)-1 {
		p.selected++
	}
	return p, nil
}

// MoveUp moves the selection up one row.
func (p Palette) MoveUp() (Palette, tea.Cmd) {
	if p.selected > 0 {
		p.selected--
	}
	return p, nil
}

// Update handles tea.KeyMsg for input and navigation. Returns updated Palette + Cmd.
func (p Palette) Update(msg tea.Msg) (Palette, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+n", "down":
			return p.MoveDown()
		case "ctrl+p", "up":
			return p.MoveUp()
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.selected = 0 // reset selection on input change
	return p, cmd
}

// View renders the palette content (not the overlay — the parent renders the overlay).
func (p Palette) View(width int) string {
	var sb strings.Builder

	// Input line
	sb.WriteString(theme.Accent.Render(":") + " " + p.input.View() + "\n")
	sb.WriteString(theme.Divider(width) + "\n")

	// Command list
	for i, c := range p.Filtered() {
		sel := i == p.selected
		var line string
		if sel {
			border := theme.Accent.Render("│ ")
			name := theme.Base.Width(18).Render(c.Name)
			desc := theme.Dim.Render(c.Desc)
			alias := theme.Faint.Render(c.Alias)
			line = border + name + "  " + desc + "  " + alias
		} else {
			name := theme.Mid.Width(18).Render(c.Name)
			desc := theme.Dim.Render(c.Desc)
			alias := theme.Faint.Render(c.Alias)
			line = "  " + name + "  " + desc + "  " + alias
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}
