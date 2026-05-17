package app_test

import (
	"bytes"
	"regexp"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/hermanu/klens/app"
)

// uiAnsiRe matches CSI escape sequences so waitForOutput can substring-match
// against the visible glyph stream. The textinput cursor renders as a reverse-
// video escape inside the placeholder (e.g. `\x1b[7mr\x1b[0m\x1b[38;5;240mesource…`),
// which fragments substrings like "resource or command" — stripping escapes
// before the substring check makes assertions stable against cursor blink
// timing and any future style refactors.
var uiAnsiRe = regexp.MustCompile(`\x1b\[[\d;?]*[a-zA-Z]`)

// teatest drives the model through a real tea.Program so View() output
// reflects the same composition the user sees, with the rendered focus frame
// + bottom bar geometry. We only assert on substrings that are styled as a
// single unit (e.g. "KLENS", "deployments") because lipgloss may break ANSI
// segments around mid-token styling — substring spans across style
// boundaries are unstable.
//
// All tests skip gracefully when no kubeconfig is reachable, matching the
// existing app_test pattern. They use a 120x40 terminal — wide enough to
// fit the right details pane (minDetailsAt = 120).

const (
	uiTestWidth   = 120
	uiTestHeight  = 40
	uiTestTimeout = 3 * time.Second
)

// quitProgram nudges the program into a quit state so WaitFinished returns
// before its default timeout. ctrl+c is the only key the model treats as an
// unconditional emergency exit.
func quitProgram(tm *teatest.TestModel) {
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
}

// waitForOutput polls the program's output stream until the visible (ANSI-
// stripped) text contains `want`, or the deadline passes. teatest.WaitFor
// itself logs a useful failure with the captured output if the condition
// never holds.
func waitForOutput(t *testing.T, tm *teatest.TestModel, want string) {
	t.Helper()
	wantB := []byte(want)
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(uiAnsiRe.ReplaceAll(b, nil), wantB)
	}, teatest.WithDuration(uiTestTimeout))
}

func newTestUI(t *testing.T) *teatest.TestModel {
	t.Helper()
	m, err := app.New("", "")
	if err != nil {
		t.Skip("skipping:", err)
	}
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(uiTestWidth, uiTestHeight))
}

// TestUI_DefaultFrame verifies the bordered-panel shell composes top bar +
// mid row + bottom command bar on first paint. "KLENS" is the brand string
// rendered inside the top bar's notched title (◎ KLENS <ver> · build <id>),
// so its presence is stable proof the top panel rendered.
func TestUI_DefaultFrame(t *testing.T) {
	tm := newTestUI(t)
	// The panel title is `K·L·E·N·S` (middle-dot spaced) since the topbar
	// redesign. waitForOutput strips ANSI before matching so this substring
	// survives the foreground/bold/separator escapes the title emits.
	waitForOutput(t, tm, "K·L·E·N·S")
	quitProgram(tm)
	tm.WaitFinished(t, teatest.WithFinalTimeout(uiTestTimeout))
}

// TestUI_PaletteOpensWithCtrlP verifies the modal palette renders its prompt
// placeholder when ctrl+p is pressed. "resource or command..." is the
// textinput's placeholder set in components.NewPalette. Drives the
// keystroke before the first WaitFor so the reader doesn't get advanced
// past the relevant frame.
func TestUI_PaletteOpensWithCtrlP(t *testing.T) {
	tm := newTestUI(t)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlP})
	waitForOutput(t, tm, "resource or command")
	quitProgram(tm)
	tm.WaitFinished(t, teatest.WithFinalTimeout(uiTestTimeout))
}

// TestUI_CommandModeOnColon verifies that `:` enters the inline ex-mode and
// the suggestions strip surfaces command names (here "deployments" since
// it's in the default command list).
func TestUI_CommandModeOnColon(t *testing.T) {
	tm := newTestUI(t)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	// The suggestions strip lists "deployments" by name in the default
	// command set; "deployments" survives as a single styled token so a
	// substring check is stable.
	waitForOutput(t, tm, "deployments")
	quitProgram(tm)
	tm.WaitFinished(t, teatest.WithFinalTimeout(uiTestTimeout))
}

// TestUI_CommandModeUnknownFlash verifies the flash banner appears when
// the user runs an unknown command via the inline ex-mode. Drives all the
// keystrokes before the first WaitFor so the persistent reader sees the
// flash frame in its accumulated buffer (the banner is short-lived: a Tick
// clears it after flashTTL = 1.5s, so the assertion has to be patient but
// not too patient).
func TestUI_CommandModeUnknownFlash(t *testing.T) {
	tm := newTestUI(t)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	tm.Type("zzz")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	// "no command" is the prefix of the flash banner text, set by
	// updateCommandMode when ExactCommand returns nil and there are no
	// substring matches.
	waitForOutput(t, tm, "no command")
	quitProgram(tm)
	tm.WaitFinished(t, teatest.WithFinalTimeout(uiTestTimeout))
}

// (mnemonic absence is covered at the Update-state level in app_test.go;
// asserting a no-op on the rendered output is fragile because teatest's
// Output() reader can leave the post-event redraw out of the next WaitFor
// window when the view didn't actually change.)
