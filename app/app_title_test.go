package app

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// titleAnsiRe matches any CSI escape so substring assertions work against the
// rendered (ANSI-styled) title without false positives from `[` in escape codes.
var titleAnsiRe = regexp.MustCompile(`\x1b\[[\d;]*[a-zA-Z]`)

func plainTitle(s string) string { return titleAnsiRe.ReplaceAllString(s, "") }

// TestMain forces TrueColor so lipgloss emits color codes that match the
// strip pattern. Without this, `go test` runs without a TTY and lipgloss
// skips color, but stripping a no-op is harmless.
func TestMain(m *testing.M) {
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	m.Run()
}

func TestTablePanelTitle_SubviewFallsThroughToBareTitle(t *testing.T) {
	// Sub-views (logs, describe, genericDescribe) call with total=0 and no scope.
	got := plainTitle(tablePanelTitle("logs", "default", "", 0, 0, ""))
	if got != "LOGS" {
		t.Errorf("subview should render bare title; got %q", got)
	}
	if strings.Contains(got, "ns:") {
		t.Errorf("subview must NOT render the ns: chip; got %q", got)
	}
}

func TestTablePanelTitle_ListViewRendersBreadcrumb(t *testing.T) {
	cases := []struct {
		name      string
		namespace string
		filter    string
		visible   int
		total     int
		scope     string
		want      string
	}{
		{
			name:    "no filter, empty namespace renders ns:all",
			total:   54,
			visible: 54,
			want:    "PODS · ns:all [54]",
		},
		{
			name:      "namespace set, no filter, all visible",
			namespace: "default",
			total:     54,
			visible:   54,
			want:      "PODS · ns:default [54]",
		},
		{
			name:      "filter active",
			namespace: "default",
			filter:    "foo",
			total:     54,
			visible:   4,
			want:      "PODS · ns:default · /foo [4/54]",
		},
		{
			name:      "filter + scope both present",
			namespace: "default",
			filter:    "foo",
			scope:     "deployment/api",
			total:     54,
			visible:   3,
			want:      "PODS · ns:default · /foo · scope: deployment/api [3/54]",
		},
		{
			name:    "scope only (filter empty), zero total — used when drill returns nothing",
			scope:   "deployment/api",
			total:   0,
			visible: 0,
			want:    "PODS · ns:all · scope: deployment/api",
		},
		{
			name:    "filter shows visible/total when different",
			filter:  "api",
			total:   54,
			visible: 7,
			want:    "PODS · ns:all · /api [7/54]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := plainTitle(tablePanelTitle("pods", tc.namespace, tc.filter, tc.visible, tc.total, tc.scope))
			if got != tc.want {
				t.Errorf("\nwant: %q\ngot:  %q", tc.want, got)
			}
		})
	}
}

func TestCmdPanelTitle(t *testing.T) {
	cases := []struct {
		name          string
		commandMode   bool
		filterFocused bool
		want          string
	}{
		{name: "default → NAV", want: "NAV"},
		{name: "filter focused → FILTER", filterFocused: true, want: "FILTER"},
		{name: "command mode → :EX", commandMode: true, want: ":EX"},
		// commandMode takes precedence over filterFocused — entering `:` from
		// a focused filter doesn't keep the FILTER label; we're now in ex-mode.
		{name: "commandMode wins over filterFocused", commandMode: true, filterFocused: true, want: ":EX"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{commandMode: tc.commandMode, filterFocused: tc.filterFocused}
			got := plainTitle(cmdPanelTitle(m))
			if got != tc.want {
				t.Errorf("\nwant: %q\ngot:  %q", tc.want, got)
			}
		})
	}
}
