package views

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/k8s"
	"github.com/hermanu/klens/k8s/resources"
	"github.com/hermanu/klens/ui/layout"
	"github.com/hermanu/klens/ui/theme"
)

// logBufferMax caps the in-memory log tail. The view shows the most
// recent ~50 lines on entry (see resources.fallbackTailLines) and lets
// the user scroll back through up to logBufferMax-1 older lines before
// they roll off the front. 200 is roughly four screens of scrollback —
// enough to read context without keeping a chatty pod's noise pinned
// in memory forever.
const logBufferMax = 200

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
	// wrap toggles soft-wrap of long messages. Off by default: long
	// lines clip at the right edge, which keeps row alignment stable
	// for grep-eyeballing structured logs. `w` flips it.
	wrap bool
}

// NewLogsView returns an empty logs view. The root model swaps in a focused
// pod via WithFocus when the user presses `l` on the pods table.
//
// Default since=0 is k9s-style "tail-only" — the impl falls back to
// fallbackTailLines so quiet pods still show their last N lines on entry.
// A 30-min default would silently produce empty screens for any pod that
// hadn't logged recently, which is the failure mode users hit most.
func NewLogsView() LogsView {
	return LogsView{follow: true, since: 0}
}

// WithFocus resets the view to a clean buffer for a new pod set. `pods` is the
// list of pods being tailed; `title` is the chip caption (e.g. "deployment/foo"
// or "pod/api-7d9-xyz"). For single-pod tails, podSet stays empty so the
// rendering skips the per-line `[pod-name]` prefix.
//
// Logs is the one view that resets its filter on entry: a stale filter
// applied to a fresh stream almost always hides the line the user actually
// opened logs to see. Other views preserve their per-view filter across
// drill-downs because the underlying resource list is stable.
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
	v.filter = ""
	v.offset = -1
	v.follow = true
	// Don't auto-promote since=0 to a 30-min window any more — that's
	// what made fresh entries silently empty for quiet pods. since=0
	// means "use the impl's tail-line cap", which always shows
	// something.
	return v
}

// Update routes tea.Msg through the logs view, handling scroll, search, and
// lookback-window changes.
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
		case "j", keyDown:
			// In follow mode j is a no-op — the viewport is already pinned to
			// the newest line, and dropping out of follow on every j press
			// jumped the viewport to the top (offset=1), making logs feel
			// like they "wrapped" when in fact follow had just been disabled.
			if v.follow {
				return v, nil
			}
			v.offset++
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
		case "t", " ":
			// Toggle live tail (space is an alias for t). Pausing keeps the
			// buffer where it is so the user can read calmly; resuming
			// snaps back to the newest line.
			v.follow = !v.follow
			return v, nil
		case "m":
			// Drop a marker line — renders as a ──── separator below. Useful
			// when watching a fast stream and you want to bookmark "from
			// here, watch for repro". The marker carries a timestamp so the
			// user can see how long ago they set it.
			v.lines = append(v.lines, resources.LogLine{
				Time:     time.Now(),
				IsMarker: true,
			})
			if len(v.lines) > logBufferMax {
				v.lines = v.lines[len(v.lines)-logBufferMax:]
			}
			return v, nil
		case "c":
			v.lines = nil
			return v, nil
		case "w":
			// Toggle wrap. Off by default keeps the timestamp+level
			// columns aligned so the user's eye can scan; on is for
			// reading multi-line stack traces or long JSON blobs.
			v.wrap = !v.wrap
			return v, nil
		case keyEsc:
			return v, func() tea.Msg { return BackToPodsMsg{} }
		}
		// Range shortcuts 0-5: change the lookback window. We restart the
		// stream with the new SinceSeconds and clear the existing buffer so
		// the visible content reflects the chosen window. The pod set is
		// preserved across re-streams so deployment/service tails don't
		// collapse to a single pod when the window changes.
		for _, p := range rangePresets {
			if msg.String() != p.key {
				continue
			}
			v.since = p.seconds
			v.lines = nil
			v.follow = true
			ns, pods, sec := v.namespace, v.activePods(), p.seconds
			return v, func() tea.Msg {
				return LogTailRequestMsg{Namespace: ns, Pods: pods, SinceSeconds: sec}
			}
		}
	}
	return v, nil
}

// Title implements views.View.
func (v LogsView) Title() string { return labelLogs }

// Filter implements views.Filterable. Logs filters lines (not rows), but the
// shell treats Filterable uniformly — the bottom command-bar mirrors this
// value when LogsView is focused.
func (v LogsView) Filter() string { return v.filter }

// Count implements views.View. Returns 0,0 so the panel title chip
// disappears — "lines" isn't a meaningful count to anchor the user on,
// and the in-body view scrolls live; a numeric counter just rotted next
// to the actual content.
func (v LogsView) Count() (visible, total int) {
	return 0, 0
}

// Chips implements views.View. The scope chip's key is "pods" when fanning out
// across an owner (deployment/service/node) and "pod" for a single-pod tail —
// either way the value carries the user-facing title (`deployment/foo`,
// `pod/api-7d9-xyz`) so the chip strip identifies the source unambiguously.
func (v LogsView) Chips() []layout.FilterChip {
	scopeKey := "pod"
	if len(v.podSet) > 1 {
		scopeKey = labelPods
	}
	value := v.title
	if value == "" && len(v.podSet) == 1 {
		// Single-pod tail with no explicit title — fall back to the bare pod name.
		for p := range v.podSet {
			value = "pod/" + p
		}
	}
	// Namespace is shown in the top bar already; the chip strip carries
	// only logs-specific context (the focused workload + tail state +
	// optional /filter), so the user's eye doesn't read the same value
	// twice on the same screen.
	chips := []layout.FilterChip{
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

// KeyHints implements views.View. The wrap label flips so the user can
// see the toggle's current state without opening the help overlay. The
// space/pause hint also flips so the user can tell if scroll is live or
// paused at a glance.
func (v LogsView) KeyHints() []layout.KeyHint {
	wrapLabel := "wrap"
	if v.wrap {
		wrapLabel = "no-wrap"
	}
	pauseLabel := "pause"
	if !v.follow {
		pauseLabel = "resume"
	}
	return []layout.KeyHint{
		{Key: "j/k", Label: "scroll"},
		{Key: "G", Label: "tail"},
		{Key: "␣", Label: pauseLabel},
		{Key: "m", Label: "mark"},
		{Key: "w", Label: wrapLabel},
		{Key: "/", Label: labelFilter},
		{Key: keyEsc, Label: "back"},
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

	pageSize := height
	if pageSize < 1 {
		pageSize = 20
	}
	multi := len(v.podSet) > 1

	// Flatten every log entry into rendered rows so wrap-aware pagination is
	// easy. Rows includes the continuation lines from any wrapped message.
	// Pass the filter through so matching substrings get highlighted in the
	// message body — the filter already gates v.visibleLines, but the eye
	// still needs to find the hit inside a long line.
	rows := make([]string, 0, len(visible))
	for _, l := range visible {
		rows = append(rows, strings.Split(formatLogRow(width, l, multi, v.filter, v.wrap), "\n")...)
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

	// Join body lines without a trailing newline so the panel's height
	// clamp sees exactly (end-start) rows. The in-body count hint
	// ("1-38 of 200") was dropped here too — the panel foot already
	// surfaces the line total, so duplicating it stole a row of logs.
	out := make([]string, end-start)
	for i := start; i < end; i++ {
		out[i-start] = rows[i]
	}
	return strings.Join(out, "\n")
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

func formatLogRow(width int, l resources.LogLine, multi bool, filter string, wrap bool) string {
	// Markers render as a horizontal separator with a timestamp anchor —
	// inserted by the `m` key so users can bookmark a moment in a busy
	// stream. The accent color makes them pop against the muted log rows.
	if l.IsMarker {
		ts := l.Time.Format("15:04:05")
		label := "  ── " + ts + " "
		fill := width - lipgloss.Width(label)
		if fill < 1 {
			fill = 1
		}
		return lipgloss.NewStyle().Foreground(theme.ColorAccent).
			Render(label + strings.Repeat("─", fill))
	}
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
	// In wrap mode the message soft-wraps onto continuation rows so
	// long stack traces stay fully readable. In no-wrap mode (default)
	// we collapse to a single visual row clipped at msgWidth — keeps
	// timestamp+level columns aligned for grep-style scanning. The
	// horizontal `…` ellipsis tells the reader the line was clipped.
	var chunks []string
	if wrap {
		chunks = wrapMessage(l.Message, msgWidth)
	} else {
		single := strings.ReplaceAll(l.Message, "\n", " ")
		if lipgloss.Width(single) > msgWidth {
			single = clipDisplay(single, msgWidth-1) + "…"
		}
		chunks = []string{single}
	}
	if len(chunks) == 0 {
		chunks = []string{""}
	}

	header := tsCol + " " + lvl + " " + podPrefix
	indent := strings.Repeat(" ", prefixCols)
	var sb strings.Builder
	for i, c := range chunks {
		// Pure highlightMatch would leave the unmatched runs as plain
		// text, breaking the muted body color. mixHighlight applies
		// msgStyle to the unmatched runs and matchHighlight to the
		// hits so both colors coexist on the same line.
		var body string
		if filter == "" {
			body = msgStyle.Render(c)
		} else {
			body = mixHighlight(c, filter, msgStyle)
		}
		if i == 0 {
			sb.WriteString(header + body)
		} else {
			sb.WriteString(indent + body)
		}
		if i < len(chunks)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// mixHighlight renders a log-message chunk with two interleaved styles: the
// muted body color for non-matched text and matchHighlight for filter hits.
// We can't use highlightMatch directly here because that returns plain text
// for the unmatched runs, which would clash with the muted msgStyle the rest
// of the row uses.
func mixHighlight(text, filter string, body lipgloss.Style) string {
	if filter == "" {
		return body.Render(text)
	}
	q := strings.ToLower(strings.TrimSpace(filter))
	if q == "" {
		return body.Render(text)
	}
	lower := strings.ToLower(text)
	var sb strings.Builder
	i := 0
	for i < len(text) {
		idx := strings.Index(lower[i:], q)
		if idx < 0 {
			sb.WriteString(body.Render(text[i:]))
			break
		}
		abs := i + idx
		if abs > i {
			sb.WriteString(body.Render(text[i:abs]))
		}
		sb.WriteString(matchHighlight.Render(text[abs : abs+len(q)]))
		i = abs + len(q)
	}
	return sb.String()
}

// clipDisplay returns the longest prefix of s whose display width fits in
// `width` cells. Used by formatLogRow's no-wrap path to truncate long
// messages without splitting wide runes mid-character.
func clipDisplay(s string, width int) string {
	if width <= 0 {
		return ""
	}
	var b strings.Builder
	cells := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if cells+rw > width {
			break
		}
		b.WriteRune(r)
		cells += rw
	}
	return b.String()
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
