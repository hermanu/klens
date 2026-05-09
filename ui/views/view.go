package views

import (
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/layout"
)

// View is the contract every resource view satisfies so the root model can
// compose the modern shell (top bar / nav rail / filter chips / table /
// details / command bar) without each view re-implementing it.
//
// The root passes width/height per render call — views must not cache size,
// otherwise they lag tea.WindowSizeMsg by one frame.
type View interface {
	// Table renders just the table body for the central pane.
	Table(width, height int) string
	// Details renders the right-pane detail block for the focused row.
	// Non-pod views should call layout.DefaultDetails with their KVs.
	Details(width, height int) string
	// Chips returns the active filter chips shown above the table.
	Chips() []layout.FilterChip
	// KeyHints returns the per-view key hints shown in the bottom command bar.
	KeyHints() []layout.KeyHint
	// Title is the lower-case resource name, e.g. "pods".
	Title() string
	// Count returns (visible, total) — visible reflects current filtering.
	Count() (visible, total int)
}

// KeyMap is an optional interface views can implement to power the `?` help
// overlay. If a view doesn't implement it, the shell falls back to KeyHints().
//
// The full list (including "soon" entries that don't yet have a handler) lives
// here so the overlay can advertise the upcoming keymap honestly while
// KeyHints stays tight to what's actually wired today.
type KeyMap interface {
	KeyMap() []components.KeySpec
}
