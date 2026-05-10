package views

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hermanu/klens/k8s"
	"github.com/hermanu/klens/k8s/resources"
	"github.com/hermanu/klens/port"
	"github.com/hermanu/klens/ui/components"
	"github.com/hermanu/klens/ui/layout"
	"github.com/hermanu/klens/ui/theme"
)

// sparkLen mirrors the design's SPARK_LEN — keep this many points per pod for
// the trend column and the live-metrics block.
const sparkLen = 24

// logTailLen caps the in-memory log tail so the details pane stays cheap to
// re-render. New lines push old ones out the front.
const logTailLen = 50

var podCols = []components.Column{
	{Header: "NAMESPACE", Width: 14},
	// NAME is Flex so any leftover horizontal width on wide terminals goes to
	// it — pod names are routinely long enough to truncate at 36, and there's
	// no point leaving a blank band on the right of the table while names get
	// chopped.
	{Header: "NAME", Width: 36, Flex: true},
	{Header: "READY", Width: 6, Align: components.AlignRight},
	{Header: "STATUS", Width: 18},
	{Header: "RST", Width: 4, Align: components.AlignRight},
	{Header: "CPU·m", Width: 7, Align: components.AlignRight},
	{Header: "MEM·MB", Width: 7, Align: components.AlignRight},
	{Header: "TREND", Width: 10},
	{Header: "IP", Width: 14},
	{Header: "NODE", Width: 18},
	{Header: "AGE", Width: 6, Align: components.AlignRight},
}

// podSeries is the per-pod ring buffer of CPU + memory samples used for the
// trend sparkline and the live-metrics block in the details pane.
type podSeries struct {
	cpu []float64
	mem []float64
}

// PodsView is the pod list view.
type PodsView struct {
	svc       port.PodService
	namespace string
	pods      []resources.PodItem
	samples   map[string]podSeries // key: ns/name
	logTail   []resources.LogLine
	table     components.Table
	filter    string
	err       error
}

func NewPodsView(svc port.PodService, namespace string) PodsView {
	return PodsView{
		svc:       svc,
		namespace: namespace,
		table:     components.NewTable(podCols, nil),
		samples:   make(map[string]podSeries),
	}
}

// podsListedMsg carries the result of an async ListPods back to the view, so
// the synchronous K8s API call doesn't block the Bubble Tea Update loop.
type podsListedMsg struct {
	pods []resources.PodItem
	err  error
}

func (v PodsView) Update(msg tea.Msg) (PodsView, tea.Cmd) {
	switch msg := msg.(type) {
	case k8s.PodsUpdatedMsg:
		// Run the List off the Update goroutine so the UI stays responsive
		// during slow informer ListAndWatch operations on large clusters.
		ns := v.namespace
		svc := v.svc
		return v, func() tea.Msg {
			pods, err := svc.ListPods(context.Background(), ns)
			return podsListedMsg{pods: pods, err: err}
		}

	case podsListedMsg:
		v.err = msg.err
		if msg.err == nil {
			v.pods = msg.pods
			v.table = v.table.SetRows(v.rows())
		}
		return v, nil

	case k8s.MetricsTickMsg:
		v = v.absorbMetrics(msg.Samples)
		// Refresh the table so the CPU/MEM/TREND columns reflect the new tick.
		v.table = v.table.SetRows(v.rows())
		return v, nil

	case k8s.LogLineMsg:
		// Only keep lines for the focused pod — the watcher already filters,
		// but this guards against late deliveries after a focus switch.
		if focus := v.SelectedPod(); focus == nil || msg.Line.Pod != focus.Name {
			return v, nil
		}
		v.logTail = append(v.logTail, msg.Line)
		if len(v.logTail) > logTailLen {
			v.logTail = v.logTail[len(v.logTail)-logTailLen:]
		}
		return v, nil

	case FilterMsg:
		v.filter = msg.Query
		v.table = v.table.SetRows(v.rows())
		return v, nil

	case NamespaceChangedMsg:
		v.namespace = msg.Namespace
		// Drop stale data — the follow-up PodsUpdatedMsg refetches.
		v.pods = nil
		v.logTail = nil
		v.table = v.table.SetRows(nil)
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			v.table = v.table.MoveDown()
			v.logTail = nil // focus changed; drop old logs
		case "k", "up":
			v.table = v.table.MoveUp()
			v.logTail = nil
		case "g":
			v.table = v.table.MoveTop()
			v.logTail = nil
		case "G":
			v.table = v.table.MoveBottom()
			v.logTail = nil
		case "l":
			// Switch to the dedicated full-screen logs view AND start the
			// stream over the last 30 min by default. LogsView lets the user
			// adjust the lookback range with digit shortcuts.
			if pod := v.SelectedPod(); pod != nil {
				ns, name := pod.Namespace, pod.Name
				title := "pod/" + name
				return v, tea.Batch(
					func() tea.Msg {
						return SwitchToLogsMsg{Namespace: ns, Pods: []string{name}, Title: title}
					},
					func() tea.Msg {
						return LogTailRequestMsg{Namespace: ns, Pods: []string{name}, SinceSeconds: 1800}
					},
				)
			}
		case "enter":
			// Open the full-screen describe view for the focused pod —
			// k9s-style detail dump (image, containers, env, conditions...).
			if pod := v.SelectedPod(); pod != nil {
				ns, name := pod.Namespace, pod.Name
				return v, func() tea.Msg {
					return SwitchToDescribeMsg{Namespace: ns, Pod: name}
				}
			}
		}
	}
	return v, nil
}

// SelectedPod resolves the table cursor back to a PodItem. Returns nil for
// empty tables.
func (v PodsView) SelectedPod() *resources.PodItem {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	// Match by namespace + name (column 0 + column 1 of the rendered row).
	// The cells contain ANSI codes from NSChip / cursor markers, so strip the
	// raw values from v.pods directly using the table index instead.
	idx := v.table.SelectedIndex()
	visible := v.visiblePods()
	if idx >= len(visible) {
		return nil
	}
	p := visible[idx]
	for i := range v.pods {
		if v.pods[i].Name == p.Name && v.pods[i].Namespace == p.Namespace {
			return &v.pods[i]
		}
	}
	return nil
}

// Title implements views.View.
func (v PodsView) Title() string { return "pods" }

// Count implements views.View.
func (v PodsView) Count() (visible, total int) {
	return len(v.visiblePods()), len(v.pods)
}

// Chips implements views.View. The namespace is shown prominently in the top
// bar already, so we don't repeat it here — only user-set chips (text filter)
// or non-default state goes in the strip.
func (v PodsView) Chips() []layout.FilterChip {
	chips := []layout.FilterChip{}
	if v.filter != "" {
		chips = append(chips, layout.FilterChip{Key: "/", Value: v.filter, Strong: true})
	}
	return chips
}

// KeyHints implements views.View — only the keys that actually do something
// today. shell / edit / port-fwd are deliberately omitted until they're
// implemented; showing aspirational hints just confused users.
func (v PodsView) KeyHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "↵", Label: "describe"},
		{Key: "l", Label: "logs"},
		{Key: "/", Label: "filter"},
	}
}

// Table implements views.View.
func (v PodsView) Table(width, height int) string {
	v.table = v.table.SetWidth(width).SetHeight(height)
	if v.err != nil {
		return "error: " + v.err.Error()
	}
	return v.table.View()
}

// Details implements views.View — pods-specific right pane with live metric
// sparklines and a SPEC block. The log tail is intentionally absent: `l` opens
// a dedicated full-screen logs view, so duplicating tail lines here added no
// information density and stole vertical space.
func (v PodsView) Details(width, height int) string {
	pod := v.SelectedPod()
	if pod == nil {
		return ""
	}
	key := pod.Namespace + "/" + pod.Name
	series := v.samples[key]
	cpuNow := lastF(series.cpu)
	memNow := lastF(series.mem)
	sparks := []layout.MetricSeries{
		{Label: "cpu", Value: fmt.Sprintf("%dm", int(cpuNow)), Samples: scaleSeries(series.cpu, scaleCPU)},
		{Label: "mem", Value: fmt.Sprintf("%dM", int(memNow)), Samples: scaleSeries(series.mem, scaleMem)},
	}
	return layout.DefaultDetails(width, height, layout.DetailsBlock{
		Title:    pod.Name,
		Subtitle: fmt.Sprintf("%s · %s · %s", pod.Namespace, pod.Status, fmtAge(pod.Age)),
		KVs: []layout.KV{
			{Key: "namespace", Value: pod.Namespace},
			{Key: "node", Value: pod.Node},
			{Key: "ip", Value: pod.IP},
			{Key: "ready", Value: pod.Ready},
			{Key: "restarts", Value: fmt.Sprintf("%d", pod.Restarts), Warn: pod.Restarts > 0},
		},
		Sparks: sparks,
	})
}

// visiblePods returns the pods slice after applying v.filter through the
// shared matchesFields helper. Fields included: name, namespace, status,
// ready, node, IP — every stringy column the user sees in the table.
func (v PodsView) visiblePods() []resources.PodItem {
	if v.filter == "" {
		return v.pods
	}
	out := make([]resources.PodItem, 0, len(v.pods))
	for _, p := range v.pods {
		if matchesFields(v.filter, p.Name, p.Namespace, p.Status, p.Ready, p.Node, p.IP) {
			out = append(out, p)
		}
	}
	return out
}

func (v PodsView) rows() []components.Row {
	pods := v.visiblePods()
	rows := make([]components.Row, len(pods))
	for i, p := range pods {
		key := p.Namespace + "/" + p.Name
		s := v.samples[key]
		cpuCell := "—"
		memCell := "—"
		if v := lastF(s.cpu); v > 0 {
			cpuCell = fmt.Sprintf("%d", int(v))
		}
		if v := lastF(s.mem); v > 0 {
			memCell = fmt.Sprintf("%d", int(v))
		}
		spark := components.Sparkline(scaleSeries(s.cpu, scaleCPU), 10, statusDotColor(p.Status))
		rows[i] = components.Row{
			components.NSChip(p.Namespace),
			p.Name,
			p.Ready,
			components.StatusPill(p.Status),
			fmt.Sprintf("%d", p.Restarts),
			cpuCell,
			memCell,
			spark,
			fallbackOr(p.IP),
			fallbackOr(p.Node),
			fmtAge(p.Age),
		}
	}
	return rows
}

// absorbMetrics merges a metrics tick into the per-pod ring buffers and
// returns the new view. The map itself is shared (value-type semantics on the
// view but a reference inside) — Update only ever runs on one instance at a
// time so no concurrent-write hazard.
func (v PodsView) absorbMetrics(samples []resources.PodMetricSample) PodsView {
	if v.samples == nil {
		v.samples = make(map[string]podSeries, len(samples))
	}
	for _, s := range samples {
		key := s.Namespace + "/" + s.Name
		series := v.samples[key]
		series.cpu = appendCapped(series.cpu, float64(s.CPUm))
		series.mem = appendCapped(series.mem, float64(s.MemMB))
		v.samples[key] = series
	}
	return v
}

// statusDotColor returns the dot color for a status, used to tint the
// per-row trend sparkline.
func statusDotColor(phase string) lipgloss.Color {
	return theme.StatusStyleFor(phase).Dot
}

func appendCapped(buf []float64, x float64) []float64 {
	buf = append(buf, x)
	if len(buf) > sparkLen {
		buf = buf[len(buf)-sparkLen:]
	}
	return buf
}

func lastF(buf []float64) float64 {
	if len(buf) == 0 {
		return 0
	}
	return buf[len(buf)-1]
}

// scaleCPU normalises raw millicores to 0..100 for the sparkline. Anything
// above 1000m saturates the bar — a single pod hitting 1 vCPU is "full bar".
func scaleCPU(v float64) float64 {
	if v > 1000 {
		v = 1000
	}
	return v / 10.0
}

// scaleMem normalises raw MB to 0..100. 4 GiB saturates.
func scaleMem(v float64) float64 {
	if v > 4096 {
		v = 4096
	}
	return v / 40.96
}

func scaleSeries(buf []float64, fn func(float64) float64) []float64 {
	out := make([]float64, len(buf))
	for i, v := range buf {
		out[i] = fn(v)
	}
	return out
}

// fallbackOr returns s if non-blank, otherwise an em-dash placeholder. Used
// across pod and log views for missing IP/node/pod fields.
func fallbackOr(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
