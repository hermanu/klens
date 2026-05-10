# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`klens` is a keyboard-driven Kubernetes TUI (Bubble Tea + Lip Gloss) inspired by k9s. Two differentiating features:

- **Inline editing of secrets and configmaps** — values surfaced as plain text (no manual base64).
- **A modern composed shell** — top bar + filter chips + table + right details + bottom command bar, with full-screen sub-views for logs and describe and a centered overlay for `:` palette and `?` help.

Module path: `github.com/hermanu/klens`. Go **1.26.3** is the required toolchain (per `go.mod`); the README still says 1.22+ — trust `go.mod`.

## Commands

The `justfile` is the canonical task runner. Common entries:

- `just build` — `go build -o klens .`
- `just run -- --namespace production` — run from source
- `just test` / `just test-race` / `just test-v`
- `just lint` — `golangci-lint run ./...`
- `just check` — test + vet + lint (run before committing)
- `just release-dry` — `goreleaser release --snapshot --clean`
- `just release v0.2.0` — tag + push (CI publishes)

Run a single test:

```
go test ./k8s/resources -run TestSecretSvc_UpdateSecret -v
```

Tests use `k8s.io/client-go/kubernetes/fake` and `k8s.io/metrics/pkg/client/clientset/versioned/fake`; **no real cluster is needed**, and tests must not require one.

## Architecture

### Hexagonal split

The dependency direction is strict and load-bearing — keep it that way:

```
ui/views  ──depends on──▶  port  ◀──implements──  k8s/resources
```

- `port/port.go` defines one interface per resource (`PodService`, `SecretService`, `LogService`, `MetricsService`, …) and a `Services` struct that bundles them.
- `k8s/resources/*.go` implements those interfaces against `kubernetes.Interface` (real or fake) and `metrics.Interface` (real or fake).
- `ui/views/*.go` accepts only the port interface(s) it needs. **Views must not import `k8s.io/...`.** If you find yourself reaching for `client-go` from a view, the missing operation belongs in the port and resource layer.
- `app/app.go::buildServices` is the only place that wires concrete services into views.

A regression check: `grep -rn "k8s.io/" ui/views/` must return nothing.

### The modern shell

`app/app.go::View()` composes:

```
TopBar  (1 row + divider)        ← identity · ns chip · resource V/T · KLENS · : palette · ● live
FilterChips  (1 row)             ← user-applied filter pills
Table       (table | Details)    ← table fills the width minus the right Details pane
CommandBar  (1 row)              ← › / type to filter…              [↵ describe] [l logs] [?]
```

The right Details pane drops below `minDetailsAt` (120 cols). Sub-views (logs, describe, generic_describe) take the full content area. The `: palette` and `?` help overlays paint over the live frame via `ui/components/overlay.go` (`charmbracelet/x/ansi`-aware cell slicing) so the table stays visible behind them.

### View contract (`ui/views/view.go`)

Every view implements:

```go
type View interface {
    Table(width, height int) string
    Details(width, height int) string
    Chips() []layout.FilterChip
    KeyHints() []layout.KeyHint    // shown in the bottom command bar
    Title() string                 // also used as the resource label in the top bar
    Count() (visible, total int)   // drives `pods 4/56` in the top bar
}

type KeyMap interface {
    KeyMap() []components.KeySpec  // optional; powers the `?` help overlay (full keymap incl. `Soon`)
}
```

`Update` stays per-concrete-type so the existing generic `updateView[V]` keeps compiling. Views are **value types** — `Update` returns a new value; never mutate the receiver. All existing list views populate filter state via `FilterMsg{Query string}` and re-filter via `matchesFields(filter, fields...)` from `ui/views/helpers.go`.

### Sub-views and drill-down

The shell routes specialized sub-views via messages from the active list view:

- `SwitchToDescribeMsg{Namespace, Pod}` — full-screen pod describe (containers, env, conditions, …).
- `SwitchToLogsMsg{Namespace, Pods []string, Title string}` — full-screen logs view. `Pods` carries one entry for a single-pod tail and N entries when the source is an owner (deployment/service/node). `Title` is the chip caption (e.g. `deployment/foo`).
- `SwitchToGenericDescribeMsg{Title, KVs}` — full-screen KV describe for non-pod resources.
- `DrillToPodsMsg{Filter}` — Enter on a deployment/service/node filters pods by that owner.
- `NamespaceSelectedMsg{Namespace}` — Enter on a namespace switches the scope.

The root model maintains a navigation history stack — drill-downs push, `esc` pops. Mnemonic 1-8 always clears the stack so we don't ricochet.

### Watcher → tea.Msg pipeline

`k8s/watcher.go` wraps a `SharedInformerFactory` (30s resync). Each event is forwarded as a typed `tea.Msg` (`PodsUpdatedMsg{}`, `MetricsTickMsg{Samples}`, `LogLineMsg{Line}`, `PulseTickMsg{}`) via `program.Send`. Per-resource events are debounced with a 500ms `time.AfterFunc` so a busy informer doesn't spam the model. Multi-pod log streaming goes through `Watcher.StartPodLogTails(ns, []pods, sinceSeconds)` which fans out via `LogService.StreamPodLogsMulti`.

To support a new resource the watcher must register its informer and emit a new `*UpdatedMsg`.

### Async list calls

Views' `ListX` calls run off the Update goroutine via a `tea.Cmd` returning a `*ListedMsg`. Synchronous listing on large clusters used to wedge the UI for 20–30s; the async pattern is now mandatory for new list operations.

### Bubble Tea routing

The root `app.Model` owns every view as a field and routes messages with a `viewKind` enum. Adding a new resource means adding to `viewKind`, the field set, the constructor, `currentView`, `reloadCmd`, `View`, and `paletteNameToView` — keep these in sync. Same applies to sub-views (`viewLogs`, `viewDescribe`, `viewGenericDescribe`).

`SetLogTailStarter` uses a `*func(...)` pointer slot so the watcher's start function survives the Bubble Tea program copy (the model is value-passed; the function pointer is a shared mutable cell).

### Cluster picker on startup

When kubeconfig has no current-context (or fails to load) but at least one context is parseable, `app.New` populates `availableContexts` and sets `showContextPicker = true`. The picker is rendered by `ui/components/contextpicker.go` and replaces the entire frame until the user picks one (↑↓⏎) or quits (esc). On selection the model re-runs `attachClient(...)` to wire every per-cluster view + service field; no restart required.

### Secret/ConfigMap editing

`client-go` already decodes `Secret.Data` to raw bytes — **never base64-encode/decode manually**. `SecretSvc.GetSecret` populates `SecretItem.Data`; `ListSecrets` deliberately leaves it empty for performance. `UpdateSecret` writes back via Get-then-Update so other fields (`Type`, annotations) survive.

The form component (`ui/components/form.go`) is a mode-separated state machine: `ModeNav`, `ModeValueEdit`, `ModeKeyEdit`, `ModeConfirmDiscard`, `ModeConfirmSave`. `^s` in dirty state shows a 3-line `+added −removed ~changed` diff preview; second `^s` emits `FormSaveRequestedMsg{}`. `esc` while dirty pops a `discard? y/n` confirm.

### Layout & components

- `ui/layout/topbar.go` — single content row + divider.
- `ui/layout/filterchips.go` — user-applied filter pills only; the `V/T` count moved to the top bar's identity strip.
- `ui/layout/details.go` — `DefaultDetails(width, height, DetailsBlock{Title, Subtitle, KVs, Sparks})`. Pods passes `Sparks`; non-pod views pass only `Title + KVs` from a per-view `focusKVs()` method. The right pane intentionally **does not** render a log tail — `l` opens the dedicated full-screen logs view, so duplicating tail lines here added no information.
- `ui/components/table.go` — `Column.Flex bool`. Flex columns absorb leftover horizontal width so the table fills edge to edge instead of leaving a blank band on the right.
- `ui/components/overlay.go` — cell-aware ANSI overlay used by palette + help so they paint over the live frame instead of replacing it.

## Conventions

- `port.SvcService` is named that way intentionally to avoid collision with the generic word "service" — don't rename.
- All exported symbols **must have a doc comment** — `golangci-lint` enforces this via `revive/exported`. The doc comment should explain **why** the symbol exists or any non-obvious constraint; don't restate what the name already says.
- Inline comments explain **why** (constraints, invariants like "Data intentionally omitted"); don't add WHAT-comments.
- Two-tone palette: muted everywhere inactive, accent (with bold) on the active item. The active resource indicator in the top bar (`pods 4/56`) follows the same rule — V is bold accent, `/T` is muted.
- KeyHints honesty: only advertise keys the view's `Update` actually handles. `KeyMap()` carries the full keymap (including `Soon: true` items) for the `?` overlay.
- `golangci-lint` enforces the full linter set defined in `.golangci.yml` (see that file for rationale comments on each linter). Run `just lint` before committing.
- Releases are driven by GoReleaser (`.goreleaser.yml`); `main.version/commit/date` are populated via `-ldflags` at build time, so `go build` locally yields `version="dev"` — that's expected.

## Adding a new resource type

1. Add the item struct to `k8s/resources/types.go` (must implement `Resource`).
2. Add `<Resource>Svc` in `k8s/resources/<resource>.go`.
3. Add the interface to `port/port.go` and a field to `port.Services`.
4. Wire it in `app.buildServices` and add a view field + constructor + routing in `app/app.go` (`viewKind` enum, `currentView`, `reloadCmd`, `paletteNameToView`).
5. Register an informer + `*UpdatedMsg` in `k8s/watcher.go`.
6. Add `ui/views/<resource>.go` implementing the `View` interface (`Table`, `Details`, `Chips`, `KeyHints`, `Title`, `Count`). For non-pod views, return `layout.DefaultDetails(...)` from `Details` driven by a per-view `focusKVs()` method that maps the focused row to a `[]layout.KV`. Implement `KeyMap()` to surface the full keymap (including `Soon`) in the `?` overlay.
7. Run async `List` via `tea.Cmd` returning `*ListedMsg` — synchronous list calls block the Update loop on large clusters.

## Gotchas

- **`client` may be nil.** `app.New` logs a warning and continues when no kubeconfig is reachable, then either hands control to the cluster picker (if contexts are parseable) or stays in a no-cluster state. `main.go` skips the watcher in either case. Any new view/service code must tolerate `m.client == nil` rather than panic.
- **Override precedence.** `app.New(kubeconfigOverride, namespaceOverride)` lets non-empty CLI flags shadow the values from `~/.klens/config.yaml`. Empty strings fall through to the config. The CLI in `main.go` is the only caller — keep flag parsing there, not inside the package.
- **Resync interval is 30s.** `NewWatcher` hardcodes a 30s informer resync. Tests that depend on watcher cadence should either fake the informer or accept this floor.
- **klog must be silenced.** klog's reflector traces leak to stderr and corrupt the alt-screen. `main.go` sets `klog.SetLogger(logr.Discard())`, `klog.SetOutput(io.Discard)`, and the flag-based suppressors. Don't remove these.
- **Lipgloss has no native overlay.** Use `components.Overlay` for any modal that needs to paint over a live frame; `lipgloss.Place` blanks the background.
- **Persistence.** `~/.klens/config.yaml` carries `Namespace`, `LastView`, and `LogsSinceSeconds`. `app.persistState()` writes after every meaningful state change; failures are swallowed.
