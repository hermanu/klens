package views

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/k8s"
	"github.com/hermanu/klens/k8s/resources"
	"github.com/hermanu/klens/ui/layout"
	"github.com/hermanu/klens/ui/theme"
)

// logBufferMax caps the in-memory log tail. Older lines roll off the front so
// memory stays bounded even on chatty pods.
const logBufferMax = 5000

// SwitchToLogsMsg asks the root model to focus the dedicated full-screen logs
// view. PodsView emits this on `l` / Enter alongside a LogTailRequestMsg.
type SwitchToLogsMsg struct {
	Namespace string
	Pod       string
}

// BackToPodsMsg asks the root model to return from the logs view back to pods.
type BackToPodsMsg struct{}

// rangePresets maps a digit shortcut → seconds of lookback. Index 0 is "all"
// (no SinceSeconds, falls back to the impl's tail-line cap). The default
// preset on first open is index 2 (30 min).
var rangePresets = []struct {
	key     string
	label   string
	seconds int64
}{
	{"0", "all", 0},
	{"1", "5m", 300},
	{"2", "30m", 1800},
	{"3", "1h", 3600},
	{"4", "6h", 21600},
	{"5", "24h", 86400},
}

// LogsView is the full-screen log tail. It owns its own scroll state so j/k
// can pause auto-follow and let the user inspect older lines.
type LogsView struct {
	namespace string
	pod       string
	lines     []resources.LogLine
	offset    int   // first visible line; -1 means follow tail
	follow    bool  // auto-scroll to newest when true
	since     int64 // seconds of lookback (0 = no since); also drives the chip
	filter    string
}

// NewLogsView returns an empty logs view. The root model swaps in a focused
// pod via WithFocus when the user presses `l` on the pods table.
func NewLogsView() LogsView {
	return LogsView{follow: true, since: 1800}
}

// WithFocus resets the view to a clean buffer for a new pod.
func (v LogsView) WithFocus(namespace, pod string) LogsView {
	v.namespace = namespace
	v.pod = pod
	v.lines = nil
	v.offset = -1
	v.follow = true
	if v.since == 0 {
		v.since = 1800
	}
	return v
}

func (v LogsView) Update(msg tea.Msg) (LogsView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.LogLineMsg:
		// Drop lines from other pods (e.g. lingering after a focus switch).
		if v.pod != "" && msg.Line.Pod != "" && msg.Line.Pod != v.pod {
			return v, nil
		}
		v.lines = append(v.lines, msg.Line)
		if len(v.lines) > logBufferMax {
			v.lines = v.lines[len(v.lines)-logBufferMax:]
		}
		return v, nil

	case FilterMsg:
		v.filter = msg.Query
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			v.offset++
			v.follow = false
		case "k", "up":
			if v.offset > 0 {
				v.offset--
			}
			v.follow = false
		case "g":
			v.offset = 0
			v.follow = false
		case "G":
			v.follow = true
		case "t":
			// Toggle live tail. Pausing keeps the buffer where it is so the
			// user can read calmly; resuming snaps back to the newest line.
			v.follow = !v.follow
			return v, nil
		case "c":
			v.lines = nil
			return v, nil
		case "esc":
			return v, func() tea.Msg { return BackToPodsMsg{} }
		}
		// Range shortcuts 0-5: change the lookback window. We restart the
		// stream with the new SinceSeconds and clear the existing buffer so
		// the visible content reflects the chosen window.
		for _, p := range rangePresets {
			if msg.String() == p.key {
				v.since = p.seconds
				v.lines = nil
				v.follow = true
				ns, pod, sec := v.namespace, v.pod, p.seconds
				return v, func() tea.Msg {
					return LogTailRequestMsg{Namespace: ns, Pods: []string{pod}, SinceSeconds: sec}
				}
			}
		}
	}
	return v, nil
}

// Title implements views.View.
func (v LogsView) Title() string { return "logs" }

// Count implements views.View — returns the number of buffered lines so the
// nav rail (and chip strip) can show "showing N lines".
func (v LogsView) Count() (visible, total int) {
	return len(v.visibleLines()), len(v.lines)
}

// Chips implements views.View.
func (v LogsView) Chips() []layout.FilterChip {
	chips := []layout.FilterChip{
		{Key: "ns", Value: fallbackOr(v.namespace)},
		{Key: "pod", Value: fallbackOr(v.pod)},
	}
	if v.follow {
		chips = append(chips, layout.FilterChip{Key: "tail", Value: "live", Strong: true})
	} else {
		chips = append(chips, layout.FilterChip{Key: "tail", Value: "paused"})
	}
	if v.filter != "" {
		chips = append(chips, layout.FilterChip{Key: "/", Value: v.filter, Strong: true})
	}
	return chips
}

// KeyHints implements views.View.
func (v LogsView) KeyHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "j/k", Label: "scroll"},
		{Key: "G", Label: "tail"},
		{Key: "c", Label: "clear"},
		{Key: "/", Label: "filter"},
		{Key: "esc", Label: "back"},
	}
}

// Table implements views.View — but here it's the full-screen log body. The
// shell hides the right-hand details pane when this view is active so the
// logs get the entire content width.
func (v LogsView) Table(width, height int) string {
	visible := v.visibleLines()
	if len(visible) == 0 {
		hint := "waiting for log lines… (press G to follow tail, esc to go back)"
		return lipgloss.NewStyle().Foreground(theme.ColorMuted).Padding(1, 2).Render(hint)
	}

	pageSize := height - 1 // leave one row for the position hint
	if pageSize < 1 {
		pageSize = 20
	}
	// In follow mode we always pin to the newest lines.
	start := 0
	end := len(visible)
	if v.follow || v.offset < 0 {
		if end > pageSize {
			start = end - pageSize
		}
	} else {
		start = v.offset
		if start > end-1 {
			start = end - 1
		}
		if start < 0 {
			start = 0
		}
		end = start + pageSize
		if end > len(visible) {
			end = len(visible)
		}
	}

	var sb strings.Builder
	for i := start; i < end; i++ {
		sb.WriteString(formatLogRow(width, visible[i]))
		sb.WriteString("\n")
	}
	hint := lipgloss.NewStyle().
		Foreground(theme.ColorMuted).
		Render("  " + countHint(start+1, end, len(visible)))
	sb.WriteString(hint)
	return sb.String()
}

// Details implements views.View — the logs view takes the full width, so the
// details pane is empty (and the shell hides it for this view).
func (v LogsView) Details(width, height int) string { return "" }

// IsLogs reports whether this view is currently the full-screen logs view.
// The shell uses this to skip rendering the right details pane.
func (v LogsView) IsLogs() bool { return true }

func (v LogsView) visibleLines() []resources.LogLine {
	if v.filter == "" {
		return v.lines
	}
	q := strings.ToLower(v.filter)
	out := make([]resources.LogLine, 0, len(v.lines))
	for _, l := range v.lines {
		if strings.Contains(strings.ToLower(l.Message), q) ||
			strings.Contains(strings.ToLower(l.Level), q) {
			out = append(out, l)
		}
	}
	return out
}

func formatLogRow(width int, l resources.LogLine) string {
	ts := l.Time.Format("15:04:05.000")
	tsCol := lipgloss.NewStyle().Foreground(theme.ColorMuted2).Render(ts)
	level := l.Level
	if level == "" {
		level = "·"
	}
	levelStyle := lipgloss.NewStyle().Foreground(theme.ColorMid).Width(6)
	switch strings.ToUpper(level) {
	case "ERROR":
		levelStyle = levelStyle.Foreground(theme.ColorError)
	case "WARN", "WARNING":
		levelStyle = levelStyle.Foreground(theme.ColorWarn)
	case "INFO":
		levelStyle = levelStyle.Foreground(theme.ColorAccent)
	case "DEBUG":
		levelStyle = levelStyle.Foreground(theme.ColorMid)
	}
	lvl := levelStyle.Render(level)
	// Reserve a few cols for timestamp + level + spaces.
	prefixWidth := lipgloss.Width(ts) + 1 + lipgloss.Width(lvl) + 1
	msgWidth := width - prefixWidth - 2
	if msgWidth < 10 {
		msgWidth = 10
	}
	msg := l.Message
	if lipgloss.Width(msg) > msgWidth {
		msg = truncateRunes(msg, msgWidth)
	}
	msgCol := lipgloss.NewStyle().Foreground(theme.ColorFG2).Render(msg)
	return tsCol + " " + lvl + " " + msgCol
}

func countHint(start, end, total int) string {
	if total == 0 {
		return ""
	}
	return strFmt(start, end, total)
}

// strFmt is a tiny helper kept inline to avoid pulling fmt for one Sprintf.
func strFmt(a, b, c int) string {
	return itoa(a) + "–" + itoa(b) + " of " + itoa(c)
}

// truncateRunes clamps a plain (no-ANSI) string to at most maxWidth display
// columns. The logs view emits log message text, which is plaintext.
func truncateRunes(s string, maxWidth int) string {
	if maxWidth < 1 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > maxWidth {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
