package views

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/k8s/resources"
	"github.com/hermanu/klens/port"
	"github.com/hermanu/klens/ui/layout"
	"github.com/hermanu/klens/ui/theme"
)

// SwitchToDescribeMsg asks the root model to focus the dedicated full-screen
// describe view. PodsView emits this on Enter alongside a DescribeFetchMsg.
type SwitchToDescribeMsg struct {
	Namespace string
	Pod       string
}

// describeFetchedMsg carries the result of an async DescribePod call back to
// the view. Hidden because only DescribeView consumes it.
type describeFetchedMsg struct {
	desc resources.PodDescription
	err  error
}

// DescribeView renders the full spec+status of a focused pod (k9s-style
// describe). Built once per Enter on a pod row.
type DescribeView struct {
	svc       port.PodService
	namespace string
	pod       string
	desc      resources.PodDescription
	err       error
	loading   bool
	offset    int // first visible line for j/k scroll
}

// NewDescribeView returns an empty describe view. The root model swaps in a
// focused pod via WithFocus when the user presses Enter on the pods table.
func NewDescribeView(svc port.PodService) DescribeView {
	return DescribeView{svc: svc}
}

// WithFocus resets the view to a clean state for a new pod and triggers the
// async fetch — the returned cmd resolves to a describeFetchedMsg.
func (v DescribeView) WithFocus(namespace, pod string) (DescribeView, tea.Cmd) {
	v.namespace = namespace
	v.pod = pod
	v.desc = resources.PodDescription{}
	v.err = nil
	v.loading = true
	v.offset = 0
	svc := v.svc
	return v, func() tea.Msg {
		desc, err := svc.DescribePod(context.Background(), namespace, pod)
		return describeFetchedMsg{desc: desc, err: err}
	}
}

// Update routes tea.Msg through the pod describe view, handling fetch results
// and scroll/navigation keys.
func (v DescribeView) Update(msg tea.Msg) (DescribeView, tea.Cmd) {
	switch msg := msg.(type) {
	case describeFetchedMsg:
		// Drop late-arriving results for a previously-focused pod.
		if msg.desc.Name != "" && v.pod != "" && msg.desc.Name != v.pod {
			return v, nil
		}
		v.loading = false
		v.desc = msg.desc
		v.err = msg.err
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", keyDown:
			v.offset++
		case "k", "up":
			if v.offset > 0 {
				v.offset--
			}
		case "g":
			v.offset = 0
		case keyEsc:
			return v, func() tea.Msg { return BackToPodsMsg{} }
		}
	}
	return v, nil
}

// Title implements views.View.
func (v DescribeView) Title() string { return "describe" }

// Count implements views.View — total = number of body lines, visible = same
// after clamping to the viewport.
func (v DescribeView) Count() (visible, total int) {
	n := len(v.bodyLines())
	return n, n
}

// Chips implements views.View.
func (v DescribeView) Chips() []layout.FilterChip {
	return []layout.FilterChip{
		{Key: "ns", Value: fallbackOr(v.namespace)},
		{Key: "pod", Value: fallbackOr(v.pod)},
	}
}

// KeyHints implements views.View.
func (v DescribeView) KeyHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "j/k", Label: "scroll"},
		{Key: keyEsc, Label: "back"},
	}
}

// Table implements views.View — but here it's the full-screen describe body.
// The shell hides the right details pane when this view is active so the
// describe content gets the entire content width.
func (v DescribeView) Table(width, height int) string {
	if v.loading {
		return lipgloss.NewStyle().
			Foreground(theme.ColorMuted).
			Padding(1, 2).
			Render("loading describe…")
	}
	if v.err != nil {
		return lipgloss.NewStyle().
			Foreground(theme.ColorError).
			Padding(1, 2).
			Render("error: " + v.err.Error())
	}

	lines := v.bodyLines()
	pageSize := height - 1
	if pageSize < 1 {
		pageSize = 20
	}
	start := v.offset
	if start > len(lines)-1 {
		start = max0(len(lines) - 1)
	}
	end := start + pageSize
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[start:end]

	var sb strings.Builder
	for _, line := range visible {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if len(lines) > pageSize {
		hint := fmt.Sprintf("  %d–%d of %d", start+1, end, len(lines))
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(hint))
	}
	return sb.String()
}

// Details implements views.View — describe view takes full width, no side pane.
func (v DescribeView) Details(width, height int) string { return "" }

// IsDescribe lets the shell skip the right details pane when describe is up.
func (v DescribeView) IsDescribe() bool { return true }

// bodyLines builds the full describe text as one line per pre-styled string.
// Sections are separated by blank lines for readability.
func (v DescribeView) bodyLines() []string {
	d := v.desc
	if d.Name == "" {
		return []string{lipgloss.NewStyle().Foreground(theme.ColorMuted).Render("no data")}
	}

	section := func(label string) string {
		return lipgloss.NewStyle().
			Foreground(theme.ColorAccent).
			Bold(true).
			Render(label)
	}
	kv := func(k, v string) string {
		return lipgloss.NewStyle().Foreground(theme.ColorMuted).Width(16).Render(k) +
			lipgloss.NewStyle().Foreground(theme.ColorFG).Render(v)
	}

	var out []string
	out = append(out, section("metadata"))
	out = append(
		out,
		kv("name", d.Name),
		kv("namespace", d.Namespace),
		kv("node", fallbackOr(d.Node)),
		kv("pod ip", fallbackOr(d.IP)),
		kv("host ip", fallbackOr(d.HostIP)),
		kv("service acct", fallbackOr(d.ServiceAccount)),
		kv("qos class", fallbackOr(d.QoSClass)),
		kv("restart policy", fallbackOr(d.RestartPolicy)),
		kv(kvAge, fmtAge(d.Age)),
		kv("phase", d.Phase),
	)

	if len(d.Conditions) > 0 {
		out = append(out, "", section("conditions"))
		for _, c := range d.Conditions {
			out = append(out, "  "+lipgloss.NewStyle().Foreground(theme.ColorFG2).Render(c))
		}
	}

	if len(d.Labels) > 0 {
		out = append(out, "", section("labels"))
		out = append(out, kvLines(d.Labels)...)
	}
	if len(d.Annotations) > 0 {
		out = append(out, "", section("annotations"))
		out = append(out, kvLines(d.Annotations)...)
	}

	out = append(out, "", section(fmt.Sprintf("containers (%d)", len(d.Containers))))
	for _, c := range d.Containers {
		out = append(out, containerLines(c)...)
	}
	if len(d.InitContainers) > 0 {
		out = append(out, "", section(fmt.Sprintf("init containers (%d)", len(d.InitContainers))))
		for _, c := range d.InitContainers {
			out = append(out, containerLines(c)...)
		}
	}
	return out
}

// containerLines renders one container's spec/status as multiple lines.
func containerLines(c resources.ContainerInfo) []string {
	header := lipgloss.NewStyle().
		Foreground(theme.ColorFG).
		Bold(true).
		Render("  ▸ " + c.Name)
	stateColor := theme.ColorMid
	if strings.HasPrefix(c.State, "Running") {
		stateColor = theme.ColorOk
	} else if strings.HasPrefix(c.State, "Waiting") || strings.HasPrefix(c.State, "Terminated") {
		stateColor = theme.ColorWarn
	}
	if !c.Ready && c.State != "" {
		stateColor = theme.ColorError
	}
	state := lipgloss.NewStyle().Foreground(stateColor).Render(c.State)
	out := []string{header + "  " + state}

	innerKV := func(k, v string) string {
		return "      " +
			lipgloss.NewStyle().Foreground(theme.ColorMuted).Width(12).Render(k) +
			lipgloss.NewStyle().Foreground(theme.ColorFG).Render(v)
	}
	out = append(out, innerKV("image", c.Image))
	if c.Ports != "" {
		out = append(out, innerKV("ports", c.Ports))
	}
	if c.CPU != "" {
		out = append(out, innerKV("cpu", c.CPU))
	}
	if c.Memory != "" {
		out = append(out, innerKV("memory", c.Memory))
	}
	if len(c.Command) > 0 {
		out = append(out, innerKV("command", strings.Join(c.Command, " ")))
	}
	if len(c.Args) > 0 {
		out = append(out, innerKV("args", strings.Join(c.Args, " ")))
	}
	return out
}

// kvLines sorts a map into lines for the describe view.
func kvLines(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out,
			"  "+
				lipgloss.NewStyle().Foreground(theme.ColorMuted).Render(k+"=")+
				lipgloss.NewStyle().Foreground(theme.ColorFG2).Render(m[k]))
	}
	return out
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
