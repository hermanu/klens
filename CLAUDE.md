# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`klens` is a keyboard-driven Kubernetes TUI (Bubble Tea + Lip Gloss) inspired by k9s. Two differentiating features:

- **Inline editing of secrets and configmaps** — values surfaced as plain text (no manual base64).
- **A bordered-panel shell** — every pane (top bar / table / details / command bar) is wrapped in its own `components.Panel` with notched titles and a 16-color ANSI palette. The top bar carries a 6-row block-shadow KLENS logo, a KV identity grid, and a 2-column resource nav grid; full-screen sub-views for logs and describe; centered overlays for `:` palette and `?` help.

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

### The bordered-panel shell

`app/app.go::View()` composes three vertically-stacked panels (each wrapped in `components.Panel`):

```
TopBar     (8 rows)              ← ◎ KLENS v · build (title)
                                   logo + ctx/region/k8s/uptime KV + 1-8 nav grid (body)
                                   ● watching (foot)
Mid row    (Table | Details)     ← table fills width minus details pane (when shown)
CommandBar (4 rows)              ← › / type to filter…   (body row 1)
                                   <↵> describe  <l> logs  <s> shell  <e> edit  </> filter  <?> help
```

Geometry constants live at the top of `app/app.go`: `topBarRowsWide` (8) drops to `topBarRowsNarrow` (3) below `topBarWideAt` (80 cols). The right Details pane drops below `minDetailsAt` (120 cols). The nav grid (3rd top-bar column) drops below `navGridAt` (110 cols) — set in `ui/layout/topbar.go`. Sub-views (logs, describe, generic_describe) take the full mid-row area. The `: palette` and `?` help overlays paint over the live frame via `ui/components/overlay.go` (`charmbracelet/x/ansi`-aware cell slicing) so the table stays visible behind them.

**Panel hard-clamps body to `Width × Height`** (see `ui/components/panel.go`) — lipgloss's `.Width()`/`.Height()` pad short content but DO NOT truncate over-tall or over-wide content. Panel splits body lines and trims each to `Width-2` cells × `Height-2` rows before rendering so a misbehaving body renderer can never push the frame off the alt-screen viewport.

### View contract (`ui/views/view.go`)

Every view implements:

```go
type View interface {
    Table(width, height int) string
    Details(width, height int) string
    Chips() []layout.FilterChip    // vestigial — filter chips were removed from the shell, but the method stays on the interface; new views can return nil
    KeyHints() []layout.KeyHint    // shown in the bottom command bar's hint row
    Title() string                 // becomes the table panel title (uppercased), e.g. PODS
    Count() (visible, total int)   // drives the [N] (or [V/T]) chip on the table panel title; return 0,0 to suppress
}

type KeyMap interface {
    KeyMap() []components.KeySpec  // optional; powers the `?` help overlay (full keymap incl. `Soon`)
}

type Cursored interface {
    CursorIndex() int              // optional; 1-indexed row position. Drives the "15 / 54" foot on the table panel
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

- `ui/components/panel.go` — `Panel(PanelConfig{Width, Height, Title, Foot, Active, Body})`. Wraps a pre-rendered body in a `lipgloss.NormalBorder()` rectangle. Title is overlaid onto the top border at col 2; Foot onto the bottom border right-aligned. Active=true swaps `theme.ColorBorder` → `theme.ColorAccent`. Body is hard-clamped to `Width-2 × Height-2`.
- `ui/layout/topbar.go` — `TopBar(width, cfg) string` returns the body (no border — caller wraps via Panel). Wide path: 6-row block-shadow `KlensLogo` + 6-row KV column + optional 2-col × 4-row resource nav grid. Narrow path (`width < topBarWideAt`): single-row identity strip. `TopBarTitle(cfg)` and `TopBarFoot(pulseOn, live)` produce the title/foot strings.
- `ui/layout/navstrip.go` — `NavStrip(width, items)` returns a single-row horizontal mnemonic list. **Currently unused** by `app/app.go::View()` — the nav lives in the top bar's 3rd column via `cfg.NavItems`. Kept as an alternate placement option.
- `ui/layout/details.go` — `DefaultDetails(width, height, DetailsBlock{Title, Subtitle, KVs, Sparks, Containers})`. Renders 4 sections: header (title + subtitle) / KVs / METRICS (sparklines) / CONTAINERS. Each section is suppressed when its data is empty. Pods view populates Sparks + Containers; non-pod views typically only set Title + KVs.
- `ui/layout/commandbar.go` — `CommandBar(width, inputView, hints)` returns 2 body rows: prompt + input, then the `<key> label` hint row.
- `ui/components/table.go` — `Column.Flex bool`. Flex columns absorb leftover horizontal width so the table fills edge to edge. Total body = `pageSize` rows (header + divider + data rows, no in-body counter); cursor position lives in the panel foot via `views.Cursored`.
- `ui/components/overlay.go` — cell-aware ANSI overlay used by palette + help so they paint over the live frame instead of replacing it.

## Conventions

- `port.SvcService` is named that way intentionally to avoid collision with the generic word "service" — don't rename.
- All exported symbols **must have a doc comment** — `golangci-lint` enforces this via `revive/exported`. The doc comment should explain **why** the symbol exists or any non-obvious constraint; don't restate what the name already says.
- Inline comments explain **why** (constraints, invariants like "Data intentionally omitted"); don't add WHAT-comments.
- Two-tone palette: muted everywhere inactive, accent (with bold) on the active item. The active resource indicator is the `▌ N label` cell in the top bar's nav grid AND the uppercased table panel title (e.g. `PODS [54]`).
- KeyHints honesty: only advertise keys the view's `Update` actually handles. `KeyMap()` carries the full keymap (including `Soon: true` items) for the `?` overlay.
- Mnemonics 1-8 + `[`/`]` cycle are gated by `isTopLevelList()` in `updateGlobal()` so sub-views (logs, describe) keep their own digit handling (e.g. logs view's 0-5 lookback presets).
- The 16-color ANSI palette is the source of truth — never inline hex literals at call sites. The palette renders identically on terminals without truecolor and lipgloss falls back cleanly on legacy terminals.
- `golangci-lint` enforces the full linter set defined in `.golangci.yml` (see that file for rationale comments on each linter). Run `just lint` before committing.
- Releases are driven by GoReleaser (`.goreleaser.yml`); `main.version/commit/date` are populated via `-ldflags` at build time, so `go build` locally yields `version="dev"` — that's expected.

## Adding a new resource type

1. Add the item struct to `k8s/resources/types.go` (must implement `Resource`).
2. Add `<Resource>Svc` in `k8s/resources/<resource>.go`.
3. Add the interface to `port/port.go` and a field to `port.Services`.
4. Wire it in `app.buildServices` and add a view field + constructor + routing in `app/app.go` (`viewKind` enum, `currentView`, `reloadCmd`, `paletteNameToView`).
5. Register an informer + `*UpdatedMsg` in `k8s/watcher.go`.
6. Add `ui/views/<resource>.go` implementing the `View` interface (`Table`, `Details`, `Chips`, `KeyHints`, `Title`, `Count`). For non-pod views, return `layout.DefaultDetails(...)` from `Details` driven by a per-view `focusKVs()` method that maps the focused row to a `[]layout.KV`. Implement `KeyMap()` to surface the full keymap (including `Soon`) in the `?` overlay. Implement `Cursored.CursorIndex()` so the table panel foot shows the 1-indexed row position.
7. Run async `List` via `tea.Cmd` returning `*ListedMsg` — synchronous list calls block the Update loop on large clusters.

## Gotchas

- **`client` may be nil.** `app.New` logs a warning and continues when no kubeconfig is reachable, then either hands control to the cluster picker (if contexts are parseable) or stays in a no-cluster state. `main.go` skips the watcher in either case. Any new view/service code must tolerate `m.client == nil` rather than panic.
- **Override precedence.** `app.New(kubeconfigOverride, namespaceOverride)` lets non-empty CLI flags shadow the values from `~/.klens/config.yaml`. Empty strings fall through to the config. The CLI in `main.go` is the only caller — keep flag parsing there, not inside the package.
- **Resync interval is 30s.** `NewWatcher` hardcodes a 30s informer resync. Tests that depend on watcher cadence should either fake the informer or accept this floor.
- **klog must be silenced.** klog's reflector traces leak to stderr and corrupt the alt-screen. `main.go` sets `klog.SetLogger(logr.Discard())`, `klog.SetOutput(io.Discard)`, and the flag-based suppressors. Don't remove these.
- **Lipgloss has no native overlay.** Use `components.Overlay` for any modal that needs to paint over a live frame; `lipgloss.Place` blanks the background.
- **Persistence.** `~/.klens/config.yaml` carries `Namespace`, `LastView`, and `LogsSinceSeconds`. `app.persistState()` writes after every meaningful state change; failures are swallowed. `app.New` also TOLERATES a malformed config file (logs to stderr and falls back to defaults) — stale fields from older releases used to crash startup.
- **EKS identity duplication.** `aws eks update-kubeconfig` writes the cluster ARN to all three of context/cluster/user. `ui/layout/topbar.go::kvColumn` collapses identical rows to a single `ctx <basename>` entry instead of rendering the ARN three times. `trimClusterIdent` does the basename trim.
- **Logs view bookmarks.** Press `m` in the logs view to insert a marker line (rendered as a `── HH:MM:SS ─────` separator). `space` and `t` toggle live tail; in follow mode, `j` is a no-op (used to silently disable follow, jumping the viewport to the top).
