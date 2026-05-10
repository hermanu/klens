package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hermanu/klens/ui/theme"
)

// PaletteCmd is preserved as a name alias for backwards-compatibility with
// callers/tests that constructed palette entries directly. New code should
// use components.Command + DefaultCommands / FilterCommands from commands.go.
type PaletteCmd = Command

// Palette is an immutable command palette component (the modal surface).
// The inline ex-mode (`:`) doesn't use this — it renders its own one-line
// suggestion strip and shares only the underlying command list helpers.
type Palette struct {
	cmds     []Command
	input    textinput.Model
	selected int
}

// NewPalette creates a Palette. Pass nil for cmds to use DefaultCommands.
func NewPalette(cmds []Command) Palette {
	if cmds == nil {
		cmds = DefaultCommands()
	}
	ti := textinput.New()
	ti.Focus()
	// Drop the textinput's default "> " prompt — palette.View() draws its
	// own accent "›" prompt, otherwise the line reads as a double-prompt.
	ti.Prompt = ""
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
func (p Palette) Filtered() []Command {
	return FilterCommands(p.cmds, p.input.Value())
}

// Selected returns the currently highlighted command, or nil if the list is empty.
func (p Palette) Selected() *Command {
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
	if msg, ok := msg.(tea.KeyMsg); ok {
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

	// Input line — accent "›" prompt matches the bottom command bar.
	sb.WriteString(theme.Accent.Render("›") + " " + p.input.View() + "\n")
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
