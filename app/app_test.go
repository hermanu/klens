package app_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/manu/klens/app"
)

// TestModel_PaletteToggle verifies the command palette opens and closes.
// The test skips gracefully when no kubeconfig is available.
func TestModel_PaletteToggle(t *testing.T) {
	m, err := app.New()
	if err != nil {
		t.Skip("skipping: could not create model:", err)
	}

	if m.PaletteVisible() {
		t.Fatal("palette should be hidden on start")
	}

	// Open palette with ":"
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	nm, ok := next.(app.Model)
	if !ok {
		t.Fatal("Update did not return app.Model")
	}
	if !nm.PaletteVisible() {
		t.Error("want palette visible after ':'")
	}

	// Close with Esc
	next, _ = nm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	nm, ok = next.(app.Model)
	if !ok {
		t.Fatal("Update did not return app.Model")
	}
	if nm.PaletteVisible() {
		t.Error("want palette hidden after esc")
	}
}

// TestModel_QuitKey verifies q returns a Quit command.
func TestModel_QuitKey(t *testing.T) {
	m, err := app.New()
	if err != nil {
		t.Skip("skipping:", err)
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("want a tea.Cmd from q key, got nil")
	}
	// Execute the cmd to get the message — tea.Quit returns a QuitMsg.
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("want tea.QuitMsg, got %T", msg)
	}
}
