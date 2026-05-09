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
	Value   string  // human-rendered current value, e.g. "142m" or "412M"
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

// DetailsBlock is the data the details pane renders for the focused row.
// Pods populate Sparks + LogTail; other resources usually only set Title + KVs.
type DetailsBlock struct {
	Title    string
	Subtitle string // optional second line under the title (e.g. namespace chip + status)
	KVs      []KV
	Sparks   []MetricSeries
	LogTail  []LogLine
}

// ClusterMeta is the small footer block at the bottom of the nav rail.
type ClusterMeta struct {
	NodesReady int
	NodesTotal int
	CPUPercent int  // 0..100
	MemPercent int  // 0..100
	Pods       int
	PodsCap    int
}

// NavItem is a single resource entry in the nav rail.
type NavItem struct {
	Key      string // matches viewKind by string, e.g. "pods"
	Label    string // display label, e.g. "Pods"
	Mnemonic string // single key, e.g. "1"
	Count    int
}

// TopBarConfig holds the data the top bar renders.
type TopBarConfig struct {
	Context    string
	Cluster    string
	User       string
	K8sVersion string
	Region     string
	KlensVer   string
	Namespace  string // shown in the breadcrumb, e.g. "ns:all"
	Resource   string // shown in the breadcrumb, e.g. "pods"
	Live       bool   // ● live indicator
	// VisibleCount/TotalCount are the canonical filtered/total counts that
	// row 2 anchors at the same column on every render. When equal, the bar
	// shows "· N"; when different, "· V of N" with V in accent.
	VisibleCount int
	TotalCount   int
	// Totals is the legacy aggregate counter set. Row 2 no longer renders it
	// (counters were redundant with VisibleCount/TotalCount), but other call
	// sites may still consult it — leave the field in place.
	Totals Totals
}

// Totals are the right-aligned counter chips: pods, deployments, services, events.
type Totals struct {
	Pods        int
	Deployments int
	Services    int
	Events      int
}
