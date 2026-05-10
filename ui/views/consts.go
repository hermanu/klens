package views

// Column headers shared across all list views.
const (
	colName = "NAME"
	colAge  = "AGE"
)

// Key names used in msg.String() switch cases and KeyHint Key fields.
const (
	keyDown  = "down"
	keyEnter = "enter"
	keyEsc   = "esc"
)

// Key hint labels used in KeyHints() return values.
const (
	labelEdit   = "edit"
	labelFilter = "filter"
	labelLogs   = "logs"
	labelPods   = "pods"
	labelYAML   = "yaml"
)

// kvAge is the details-pane key for the resource age row.
const kvAge = "age"
