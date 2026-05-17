package layout

// FilterChip represents an active filter shown above the table.
type FilterChip struct {
	Key    string
	Op     string // operator, e.g. "≥", ":" — defaults to ":"
	Value  string
	Strong bool // true = error-level chip (accent color)
}

// KeyHint is a key + label pair shown in the command bar.
type KeyHint struct {
	Key   string
	Label string
}

// KV is a key/value pair rendered in the details pane spec block.
type KV struct {
	Key   string
	Value string
	Warn  bool // render value in warn color (for restarts > 0, etc.)
}

// MetricSeries is one row in the details-pane "live · 60s" block: label,
// current value, and the last N samples (rendered as a sparkline).
type MetricSeries struct {
	Label   string
	Value   string // human-rendered current value, e.g. "142m" or "412M"
	Samples []float64
	// Color is the sparkline color; if empty the renderer picks an accent default.
	Color string
}

// LogLine is one entry in the details-pane log tail.
type LogLine struct {
	Time  string // pre-formatted, e.g. "14:08:21.412"
	Level string // INFO/WARN/ERROR/DEBUG
	Msg   string
}

// ContainerSummary holds the per-container data rendered in the CONTAINERS
// section of the details pane. Image is intentionally "—" when the pane is
// built from a list-level PodItem (which does not carry image info); a
// DescribePod fetch would populate it accurately.
type ContainerSummary struct {
	Name     string
	Image    string
	Status   string
	Restarts int32
}

// DetailsBlock is the data the details pane renders for the focused row.
// Pods populate Sparks + Containers; other resources usually only set Title + KVs.
type DetailsBlock struct {
	Title      string
	Subtitle   string // optional second line under the title (e.g. namespace chip + status)
	KVs        []KV
	Sparks     []MetricSeries
	LogTail    []LogLine
	Containers []ContainerSummary // rendered in the CONTAINERS section; ignored until Task 7
}

// TopBarConfig holds the data the top bar renders.
type TopBarConfig struct {
	Context    string
	Cluster    string
	User       string
	K8sVersion string
	Region     string
	KlensVer   string
	// BuildID is shown in the top-bar title after the version, e.g.
	// "◎ KLENS v0.3.0 · build a1b2c3d". Empty renders "build dev".
	BuildID string
	// Uptime is the cluster oldest-node age, rendered right-aligned in the
	// dense KV grid. Empty renders "—".
	Uptime string
	// NodesReady / NodesTotal drive the top bar's right-aligned `nodes 9/9`
	// counter. NodesTotal == 0 renders "—".
	NodesReady int
	NodesTotal int
	// CPUSamples is a 0..100 normalised series for the right-aligned cpu
	// sparkline. Empty renders "—" instead of a sparkline.
	CPUSamples []float64
	// CPUPercent is the latest cpu percent shown next to the sparkline. -1
	// renders "—" instead of a number.
	CPUPercent int
	// NavItems is the 8-entry resource list rendered as a horizontal strip in
	// the dashboard's row 2. The active entry carries the ▌ accent.
	// Nil/empty → strip is omitted.
	NavItems  []NavItem
	Namespace string // shown in vitals row, e.g. "ns default"
	Live      bool   // ● live indicator
	// PulseOn drives the animated ◉/◎ brand mark in both the panel title and
	// row 1 of the dashboard. Same value the caller passes to TopBarFoot, so
	// the mark blinks in lockstep with the watch dot.
	PulseOn bool
}
