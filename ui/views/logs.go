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
// view. List views (pods, deployments, services, nodes) emit this on `l`
// alongside a LogTailRequestMsg.
//
// Pods carries one entry for a single-pod tail and N entries when the source
// is an owner (deployment/service/node). Title is the chip-strip caption — e.g.
// "pod/api-7d9-xyz", "deployment/foo", "node/ip-10-0-1-2" — so users see the
// scope of the tail at a glance regardless of pod count.
type SwitchToLogsMsg struct {
	Namespace string
	Pods      []string
	Title     string
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
//
// In multi-pod mode (deployment / service / node fan-out) podSet is non-empty
// and lines render with a colored `[pod-name]` prefix; in single-pod mode the
// prefix is suppressed since the namespace+pod chips already identify the source.
type LogsView struct {
	namespace string
	title     string              // chip caption — e.g. "deployment/foo", "pod/api-7d9-xyz"
	podSet    map[string]struct{} // empty for single-pod tails; populated when N>1
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

// WithFocus resets the view to a clean buffer for a new pod set. `pods` is the
// list of pods being tailed; `title` is the chip caption (e.g. "deployment/foo"
// or "pod/api-7d9-xyz"). For single-pod tails, podSet stays empty so the
// rendering skips the per-line `[pod-name]` prefix.
func (v LogsView) WithFocus(namespace string, pods []string, title string) LogsView {
	v.namespace = namespace
	v.title = title
	v.podSet = nil
	if len(pods) > 1 {
		v.podSet = make(map[string]struct{}, len(pods))
		for _, p := range pods {
			v.podSet[p] = struct{}{}
		}
	}
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
		// In multi-pod mode, accept any pod in the active set; lingering lines
		// from a previous focus that no longer match are dropped. In single-pod
		// mode (podSet empty) the watcher already filters per the active stream
		// so we accept everything.
		if len(v.podSet) > 0 && msg.Line.Pod != "" {
			if _, ok := v.podSet[msg.Line.Pod]; !ok {
				return v, nil
			}
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
		// the visible content reflects the chosen window. The pod set is
		// preserved across re-streams so deployment/service tails don't
		// collapse to a single pod when the window changes.
		for _, p := range rangePresets {
			if msg.String() == p.key {
				v.since = p.seconds
				v.lines = nil
				v.follow = true
				ns, pods, sec := v.namespace, v.activePods(), p.seconds
				return v, func() tea.Msg {
					return LogTailRequestMsg{Namespace: ns, Pods: pods, SinceSeconds: sec}
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

// Chips implements views.View. The scope chip's key is "pods" when fanning out
// across an owner (deployment/service/node) and "pod" for a single-pod tail —
// either way the value carries the user-facing title (`deployment/foo`,
// `pod/api-7d9-xyz`) so the chip strip identifies the source unambiguously.
func (v LogsView) Chips() []layout.FilterChip {
	scopeKey := "pod"
	if len(v.podSet) > 1 {
		scopeKey = "pods"
	}
	value := v.title
	if value == "" && len(v.podSet) == 1 {
		// Single-pod tail with no explicit title — fall back to the bare pod name.
		for p := range v.podSet {
			value = "pod/" + p
		}
	}
	chips := []layout.FilterChip{
		{Key: "ns", Value: fallbackOr(v.namespace)},
		{Key: scopeKey, Value: fallbackOr(value)},
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

// activePods returns the current pod set in slice form, used when restarting
// the stream after a range-preset change.
func (v LogsView) activePods() []string {
	if len(v.podSet) == 0 {
		return nil
	}
	out := make([]string, 0, len(v.podSet))
	for p := range v.podSet {
		out = append(out, p)
	}
	return out
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

// Table implements views.View — full-screen log body. The shell hides the
// right details pane while logs is active so the body gets the full width.
//
// Long messages soft-wrap onto multiple visual rows; pagination operates over
// rendered rows (not log entries), so a verbose multi-line stack trace doesn't
// silently push older entries off the visible window. Wrapped continuations
// are indented to align under the message column, not the timestamp — so the
// eye can pick which bytes belong to which log entry at a glance.
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
	multi := len(v.podSet) > 1

	// Flatten every log entry into rendered rows so wrap-aware pagination is
	// easy. Rows includes the continuation lines from any wrapped message.
	rows := make([]string, 0, len(visible))
	for _, l := range visible {
		rows = append(rows, strings.Split(formatLogRow(width, l, multi), "\n")...)
	}

	start := 0
	end := len(rows)
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
		if end > len(rows) {
			end = len(rows)
		}
	}

	var sb strings.Builder
	for i := start; i < end; i++ {
		sb.WriteString(rows[i])
		sb.WriteString("\n")
	}
	hint := lipgloss.NewStyle().
		Foreground(theme.ColorMuted).
		Render("  " + countHint(start+1, end, len(rows)))
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

func formatLogRow(width int, l resources.LogLine, multi bool) string {
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

	// Multi-pod tails prefix every line with [pod-name] in a stable per-pod
	// color so adjacent lines from different pods are visually separable.
	// Single-pod tails skip the prefix — the chip strip already identifies the
	// source and adding it per-line would be noise.
	podPrefix := ""
	podCols := 0
	if multi && l.Pod != "" {
		tag := lipgloss.NewStyle().
			Foreground(theme.NSColorAny(l.Pod)).
			Render("[" + l.Pod + "]")
		podPrefix = tag + " "
		podCols = lipgloss.Width(tag) + 1
	}

	// Reserve cols for timestamp + level + (optional pod prefix) + a trailing
	// space. The message column gets whatever remains; we soft-wrap into that
	// width so long stack traces stay readable instead of being truncated.
	prefixCols := lipgloss.Width(ts) + 1 + lipgloss.Width(lvl) + 1 + podCols
	msgWidth := width - prefixCols - 1
	if msgWidth < 20 {
		msgWidth = 20
	}

	msgStyle := lipgloss.NewStyle().Foreground(theme.ColorFG2)
	chunks := wrapMessage(l.Message, msgWidth)
	if len(chunks) == 0 {
		chunks = []string{""}
	}

	header := tsCol + " " + lvl + " " + podPrefix
	indent := strings.Repeat(" ", prefixCols)
	var sb strings.Builder
	for i, c := range chunks {
		if i == 0 {
			sb.WriteString(header + msgStyle.Render(c))
		} else {
			sb.WriteString(indent + msgStyle.Render(c))
		}
		if i < len(chunks)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// wrapMessage splits `s` into chunks ≤ width display cells. The split is on
// cell boundaries (no word-aware breaking) — a log message is plain text and
// programs frequently emit long unbroken paths/tokens, so cell-boundary
// wrapping is more predictable than greedy-word-wrap.
func wrapMessage(s string, width int) []string {
	if width < 1 {
		return []string{""}
	}
	if lipgloss.Width(s) <= width {
		return []string{s}
	}
	var out []string
	var line strings.Builder
	cells := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if cells+rw > width {
			out = append(out, line.String())
			line.Reset()
			cells = 0
		}
		line.WriteRune(r)
		cells += rw
	}
	if line.Len() > 0 {
		out = append(out, line.String())
	}
	return out
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
