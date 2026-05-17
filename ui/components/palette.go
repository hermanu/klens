package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/ui/theme"
)

// PaletteCmd is preserved as a name alias for backwards-compatibility with
// callers/tests that constructed palette entries directly. New code should
// use components.Command + DefaultCommands / FilterCommands from commands.go.
type PaletteCmd = Command

// Palette is an immutable command palette component (the modal surface).
// The inline ex-mode (`:`) doesn't use this — it renders its own one-line
// suggestion strip and shares only the underlying command list helpers.
//
// When the input is empty, the palette renders commands in grouped sections
// (RECENT / RESOURCES / ACTIONS / SYSTEM) so scanning is fast and intent
// is obvious. When the user types, sections collapse to a flat filtered list.
type Palette struct {
	cmds     []Command
	input    textinput.Model
	selected int
	// recent is a small ordered list of recently-run command names (most
	// recent first). Set via WithRecent before rendering. When non-empty and
	// the input is blank, the names surface as a RECENT section at the top
	// of the palette — even when they're duplicated in the categorized
	// sections below — so common shortcuts are reachable in one keystroke.
	recent []string
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

// WithRecent sets the most-recent-first list of command names. The palette
// renders matching entries as a RECENT section at the top when the input is
// blank. Empty/nil suppresses the section entirely. Returns the updated
// palette (value-type semantics).
func (p Palette) WithRecent(names []string) Palette {
	p.recent = names
	return p
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

// renderedItems returns the flat ordered list of commands the palette will
// display. Selection is an index into this slice, so the order here is the
// canonical "what's on screen" reference for navigation.
//
// When the input is blank, the order is RECENT (deduped by name) → resource
// jumps in DefaultCommands order → actions → system. When the input is
// non-blank, sections collapse to the bare Filtered() output so users
// searching for a specific command see a tight focused list.
func (p Palette) renderedItems() []Command {
	if strings.TrimSpace(p.input.Value()) != "" {
		return p.Filtered()
	}
	var out []Command
	seen := map[string]bool{}
	if len(p.recent) > 0 {
		for _, name := range p.recent {
			for _, c := range p.cmds {
				if c.Name == name && !seen[name] {
					out = append(out, c)
					seen[name] = true
					break
				}
			}
		}
	}
	for _, kind := range []CommandKind{KindResource, KindAction, KindSystem} {
		for _, c := range p.cmds {
			if c.Kind == kind {
				out = append(out, c)
			}
		}
	}
	return out
}

// Selected returns the currently highlighted command, or nil if the list is empty.
func (p Palette) Selected() *Command {
	items := p.renderedItems()
	if len(items) == 0 || p.selected >= len(items) {
		return nil
	}
	c := items[p.selected]
	return &c
}

// MoveDown moves the selection down one row.
func (p Palette) MoveDown() (Palette, tea.Cmd) {
	if items := p.renderedItems(); p.selected < len(items)-1 {
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

	items := p.renderedItems()
	showHeaders := strings.TrimSpace(p.input.Value()) == ""

	// Pre-compute per-item section labels so the View renders headers when
	// the section transitions. With showHeaders=false this stays unused.
	sections := make([]paletteSection, len(items))
	if showHeaders {
		recentCount := 0
		if len(p.recent) > 0 {
			for _, name := range p.recent {
				for _, c := range p.cmds {
					if c.Name == name {
						recentCount++
						_ = c
						break
					}
				}
			}
		}
		// Cap recentCount at items length to handle dedup edge cases.
		if recentCount > len(items) {
			recentCount = len(items)
		}
		for i := range items {
			if i < recentCount {
				sections[i] = sectionRecent
				continue
			}
			switch items[i].Kind {
			case KindResource:
				sections[i] = sectionResources
			case KindAction:
				sections[i] = sectionActions
			case KindSystem:
				sections[i] = sectionSystem
			}
		}
	}

	lastSection := paletteSection(-1)
	for i, c := range items {
		if showHeaders && sections[i] != lastSection {
			sb.WriteString(renderPaletteHeader(sections[i]))
			sb.WriteString("\n")
			lastSection = sections[i]
		}
		sb.WriteString(renderPaletteRow(c, i == p.selected))
		sb.WriteString("\n")
	}
	return sb.String()
}

// paletteSection enumerates the grouping labels rendered as headers above
// runs of commands sharing a Kind (or surfacing in RECENT).
type paletteSection int

const (
	sectionRecent paletteSection = iota
	sectionResources
	sectionActions
	sectionSystem
)

// renderPaletteHeader produces the muted-bold section label inserted above
// the first row of each section. Single-line, no border — readers know it's
// a header by the small-caps capitalization + dim color contrast vs the
// brighter command rows below it.
func renderPaletteHeader(s paletteSection) string {
	var label string
	switch s {
	case sectionRecent:
		label = "RECENT"
	case sectionResources:
		label = "RESOURCES"
	case sectionActions:
		label = "ACTIONS"
	case sectionSystem:
		label = "SYSTEM"
	}
	return lipgloss.NewStyle().Foreground(theme.ColorMuted2).Bold(true).Render("  " + label)
}

// renderPaletteRow renders one command line. Name in accent (bold on the
// selected row), description muted, alias in ok-green so the shortcut pops
// the same way the table breadcrumb's filter chip does.
func renderPaletteRow(c Command, selected bool) string {
	nameStyle := lipgloss.NewStyle().Foreground(theme.ColorFG).Width(18)
	descStyle := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	aliasStyle := lipgloss.NewStyle().Foreground(theme.ColorOk)

	if selected {
		nameStyle = lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true).Width(18)
		aliasStyle = aliasStyle.Bold(true)
		return lipgloss.NewStyle().Foreground(theme.ColorAccent).Render("▌ ") +
			nameStyle.Render(c.Name) + "  " + descStyle.Render(c.Desc) + "  " + aliasStyle.Render(c.Alias)
	}
	return "  " + nameStyle.Render(c.Name) + "  " + descStyle.Render(c.Desc) + "  " + aliasStyle.Render(c.Alias)
}
