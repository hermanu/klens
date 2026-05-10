package app_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hermanu/klens/app"
	"github.com/hermanu/klens/ui/views"
)

// TestModel_PaletteToggle verifies the modal palette opens with ctrl+p and
// closes with esc. The test skips gracefully when no kubeconfig is available.
func TestModel_PaletteToggle(t *testing.T) {
	m, err := app.New()
	if err != nil {
		t.Skip("skipping: could not create model:", err)
	}

	if m.PaletteVisible() {
		t.Fatal("palette should be hidden on start")
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	nm, ok := next.(app.Model)
	if !ok {
		t.Fatal("Update did not return app.Model")
	}
	if !nm.PaletteVisible() {
		t.Error("want palette visible after ctrl+p")
	}

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
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("want tea.QuitMsg, got %T", msg)
	}
}

// TestModel_ColonEntersCommandMode verifies that `:` opens the inline ex-mode
// (separate from the modal palette which now lives on ctrl+p).
func TestModel_ColonEntersCommandMode(t *testing.T) {
	m, err := app.New()
	if err != nil {
		t.Skip("skipping:", err)
	}
	if m.CommandModeActive() {
		t.Fatal("command mode should be inactive on start")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	nm, ok := next.(app.Model)
	if !ok {
		t.Fatal("Update did not return app.Model")
	}
	if !nm.CommandModeActive() {
		t.Error("want command mode active after ':'")
	}
	if nm.PaletteVisible() {
		t.Error("modal palette must not open on ':'")
	}
}

// TestModel_CommandModeUnknownFlashes verifies that pressing Enter in inline
// ex-mode without a matching command surfaces a flash error rather than
// silently dismissing.
func TestModel_CommandModeUnknownFlashes(t *testing.T) {
	m, err := app.New()
	if err != nil {
		t.Skip("skipping:", err)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	m, _ = next.(app.Model)
	// Type a clearly-bogus command. Each rune is a separate KeyMsg so the
	// textinput sees them the same way the user types.
	for _, r := range "zzz" {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m, _ = next.(app.Model)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = next.(app.Model)

	if m.CommandModeActive() {
		t.Error("command mode should close after Enter, even on no-match")
	}
	if m.FlashError() == "" {
		t.Error("want flash error after Enter on unknown command")
	}
}

// TestModel_PerViewFilterPersistsAcrossLogs verifies the regression fix: a
// filter set on pods is preserved when the user opens logs and esc-pops back.
func TestModel_PerViewFilterPersistsAcrossLogs(t *testing.T) {
	m, err := app.New()
	if err != nil {
		t.Skip("skipping:", err)
	}
	// Reset to pods regardless of the persisted LastView in config.yaml.
	// ctrl+p opens the palette with "pods" pre-selected; Enter runs it.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m, _ = next.(app.Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = next.(app.Model)

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m, _ = next.(app.Model)
	for _, r := range "foo" {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m, _ = next.(app.Model)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = next.(app.Model)

	// Simulate the logs view being entered via the SwitchToLogsMsg the
	// PodsView would emit on `l`. Then esc-pop back via BackToPodsMsg.
	next, _ = m.Update(views.SwitchToLogsMsg{Namespace: "default", Pods: []string{"x"}, Title: "pod/x"})
	m, _ = next.(app.Model)
	next, _ = m.Update(views.BackToPodsMsg{})
	m, _ = next.(app.Model)

	// The pods view's filter must still be "foo" — the fix removed the
	// silent FilterMsg{""} broadcast that used to wipe it on history pop.
	if got := m.PodsFilter(); got != "foo" {
		t.Errorf("pods filter lost across logs round-trip: want %q, got %q", "foo", got)
	}
}
