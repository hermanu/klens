<div align="center">

```
 █  █ █      █▀▀ █▄  █ █▀▀
 █▄▀  █      █▀▀ █ ▀▄█ ▀▀▄
 █ █  █▄▄▄▄  █▄▄ █   █ ▄▄▀
```

**A fast, keyboard-driven Kubernetes TUI for engineers who live in their clusters.**

[![CI](https://github.com/hermanu/klens/actions/workflows/ci.yml/badge.svg)](https://github.com/hermanu/klens/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/hermanu/klens?sort=semver)](https://github.com/hermanu/klens/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/hermanu/klens.svg)](https://pkg.go.dev/github.com/hermanu/klens)
[![Go Report Card](https://goreportcard.com/badge/github.com/hermanu/klens)](https://goreportcard.com/report/github.com/hermanu/klens)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

</div>

Inspired by k9s, rebuilt from scratch with a bordered-panel shell, first-class secret editing, and a 16-color ANSI palette that renders identically on every terminal.

```
┌─ ◎ KLENS 0.3.0 · build a1b2c3d ──────────────────────────────────────────────────────────┐
│ █▄▀ █   █▀▀ █▄ █ █▀  ctx prod-eu     k8s v1.30    region us-east-1                       │
│ █ █ █▄▄ █▄▄ █ ▀█ ▄█  user alice      uptime 62d 14h                                      │
└─────────────────────────────────────────────────────────────────── ● watching ───────────┘
┌─ RESOURCES ────────┐┌─ PODS [5] ─────────────────────────────────┐┌─ FOCUS ──────────────┐
│ ▌ 1  pods          ││  NAMESPACE  NAME                  READY    ││ api-gateway-7d9f4b8…  │
│   2  deployments   ││  payments   api-gateway-7d…       2/2  ●R  ││ payments · Running    │
│   3  services      ││▸ payments   worker-5c8b9-mn3rs    1/1  ●R  ││                       │
│   4  nodes         ││  payments   worker-5c8b9-qw7yt    1/1  ●R  ││ METRICS · last 60s    │
│   5  configmaps    ││  payments   payment-svc-zp1lm     0/1  ●E  ││ ────────────────────  │
│   6  secrets       ││  payments   redis-0               1/1  ●R  ││ cpu  ▁▂▃▅▄     147m   │
│   7  namespaces    ││                                            ││ mem  ▃▃▃▄▃     312Mi  │
│   8  pvcs          ││                                            ││ net↓ ▁▂▃▃▄     58KB/s │
│                    ││                                            ││ net↑ ▁▂▂▃▃     32KB/s │
│  CLUSTER           ││                                            ││                       │
│  ──────────────    ││                                            ││ CONTAINERS            │
│  nodes     9/9     ││                                            ││ ────────────────────  │
│  pods      54      ││                                            ││ ▸ api-gateway  ●R rst 0│
│  cpu  ▃▅▇▆ 62%     ││                                            ││   image  ghcr.io/…    │
│  mem  ▄▅▆▆ 78%     ││                                            ││                       │
└──── [ ] cycle ─────┘└────────────────────────────────── 2 / 5 ───┘└───────────────────────┘
┌─ COMMAND ────────────────────────────────────────────────────────────────────────────────┐
│ › / type to filter…                                                                      │
│ <↵> describe  <l> logs  <s> shell  <e> edit  </> filter  <?> help                        │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```
(stylised — see [docs/screenshots/](docs/screenshots/) for real captures)

## Why klens?

k9s is great but has friction points: its UI is dense, secret values are read-only, and there is no easy way to edit configmap data in-place. **klens** fixes that:

- **Inline secret editor** — open any secret with `↵`, edit values directly, save with `esc → s`. Values are decoded from base64 automatically; you work with plain text.
- **ConfigMap editor** — same vim-style editor, same flow.
- **Live watching** — Kubernetes informers keep every view updated without polling. The `● watch` dot on the chip strip flips on the moment the watcher is wired.
- **Two ways to navigate** — `ctrl+p` opens a modal palette (browse-by-list); `:` opens an inline ex-mode prompt with type-ahead. Both dispatch to the same command set, so `:po`, `:dp`, `:svc`, `:sec`, `:cm`, `:ns`, `:no`, `:pvc`, `:ctx`, `:q` work in either surface.
- **Drill-down with history** — `↵` on a deployment / service / node row narrows the pods view to that workload. `esc` pops back. Drill scope is shown as a chip on the filter strip so it's never invisible state.
- **Modern composed shell** — top bar with cluster identity + active resource + watch state, filter chips for the user's narrowing, the table fills the available width, a focused-row details pane carries live CPU/MEM sparklines, and a single-row command bar at the bottom advertises the keys the active view actually handles.
- **Full-screen sub-views** — `l` opens a dedicated logs view (multi-pod fan-out, scroll, soft-wrap, lookback presets); `↵` on a pod opens a k9s-style describe dump. `esc` returns.
- **Cluster picker on startup** — if no kubeconfig context is current but contexts are parseable, klens lets you pick one with `↑/↓/↵`. No restart needed when you switch clusters mid-session via `:ctx`.

## Features

| Resource | List | Edit | Live metrics | Drill-down |
|---|---|---|---|---|
| Pods | ✓ | — | CPU/MEM | — |
| Deployments | ✓ | — | — | → pods |
| Services | ✓ | — | — | → pods |
| Secrets | ✓ | ✓ | — | — |
| ConfigMaps | ✓ | ✓ | — | — |
| Namespaces | ✓ | — | — | → pods (scope) |
| Nodes | ✓ | — | — | → pods |
| PersistentVolumeClaims | ✓ | — | — | — |

Logs and Describe are always-on full-screen sub-views, available from any pod-bearing list.

## Installation

### Pre-built binary (recommended)

Download the binary for your platform from the [latest release](https://github.com/hermanu/klens/releases/latest):

```bash
# Linux (amd64)
curl -L https://github.com/hermanu/klens/releases/latest/download/klens_linux_amd64.tar.gz | tar xz
sudo mv klens /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/hermanu/klens/releases/latest/download/klens_darwin_arm64.tar.gz | tar xz
sudo mv klens /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/hermanu/klens/releases/latest/download/klens_darwin_amd64.tar.gz | tar xz
sudo mv klens /usr/local/bin/
```

### Go install

Requires Go 1.26+ (matches `go.mod`).

```bash
go install github.com/hermanu/klens@latest
```

### From source

```bash
git clone https://github.com/hermanu/klens
cd klens
just build      # or: go build -o klens .
sudo mv klens /usr/local/bin/
```

## Usage

```bash
# Use your current kubeconfig context
klens

# Specify a kubeconfig (overrides config file and KUBECONFIG env var)
klens --kubeconfig ~/.kube/staging.yaml

# Start in a specific namespace
klens --namespace production

# Print version and exit
klens --version
```

## Keyboard shortcuts

### Global

| Key | Action |
|---|---|
| `ctrl+p` | Open the modal command palette (browse-by-list) |
| `:` | Open inline ex-mode (vim-style prompt with type-ahead) |
| `/` | Focus the filter input on the active view |
| `esc` | Exit filter focus → pop the navigation history → let the view handle it |
| `?` | Help overlay (full keymap for the active view) |
| `q` | Quit |
| `ctrl+c` | Quit (second `ctrl+c` or 5s force-exits) |

### Navigation (list views)

| Key | Action |
|---|---|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `↵` | Open detail / editor / drill-down (view-dependent) |
| `l` | Open the full-screen logs view (when the focused row owns pods) |

### Command palette / ex-mode

| Key | Action |
|---|---|
| Type to filter | Fuzzy-match resource name or alias |
| `↑` / `↓` | Navigate results |
| `↵` | Run the selected command |
| `esc` | Close |

**Aliases:** `:po` pods · `:dp` deployments · `:svc` services · `:sec` secrets · `:cm` configmaps · `:ns` namespaces · `:no` nodes · `:pvc` pvcs · `:ctx` switch cluster · `:q` quit

### Logs view

| Key | Action |
|---|---|
| `j` / `k` | Scroll line-by-line (pauses live tail) |
| `g` / `G` | Jump to top / resume tail at bottom |
| `t` | Toggle live tail |
| `w` | Toggle soft-wrap for long messages |
| `c` | Clear the buffer |
| `0` … `5` | Lookback window: all · 5m · 30m · 1h · 6h · 24h |
| `/` | Filter lines |
| `esc` | Back to the previous view |

### Secret / ConfigMap editor

The form is a vim-style state machine — list rows in nav mode, drop into a single value field with `↵`, leave with `esc`, and `esc` on a dirty form opens a save/discard/cancel prompt.

| Mode | Key | Action |
|---|---|---|
| Nav | `j` / `k` | Move between rows |
| Nav | `↵` | Edit the focused value |
| Nav | `o` | Add a new row |
| Nav | `dd` | Delete the focused row (two-stroke) |
| Nav | `H` | Toggle hide/show for the focused value |
| Nav | `esc` | Exit (clean) or open the confirm prompt (dirty) |
| Edit | `esc` | Commit the field and return to nav |
| Confirm | `s` / `y` / `↵` | Save and exit |
| Confirm | `d` | Discard and exit |
| Confirm | `n` / `esc` | Cancel and stay in the form |

## Configuration

klens reads `~/.klens/config.yaml` on startup. All fields are optional; the file is also written automatically to remember your last namespace, last view, and last logs lookback so klens reopens where you left it.

```yaml
# Path to kubeconfig (defaults to KUBECONFIG env var or ~/.kube/config)
kubeconfig: ~/.kube/config

# Default namespace (empty = all namespaces)
namespace: payments

# Resource view to reopen on startup
# (pods, deployments, services, secrets, configmaps, namespaces, nodes, pvcs)
last_view: pods

# Lookback for the logs view, in seconds (0 = tail-only)
logs_since_seconds: 1800

# Accent color for UI highlights (hex)
accent: "#e85a4f"
```

## Architecture

```
klens/
├── main.go               # entry point — wires app + watcher, owns klog suppression
├── app/                  # root tea.Model, view router, history stack, command dispatch
├── config/               # config loading + auto-persisted state (namespace, last view, ...)
├── port/                 # one interface per resource (hexagonal architecture)
├── k8s/
│   ├── client.go         # kubeconfig loading, context listing, cluster picker source
│   ├── watcher.go        # SharedInformerFactory → typed tea.Msg events (debounced)
│   └── resources/        # service structs implementing port interfaces (real + fake friendly)
└── ui/
    ├── theme/            # Lip Gloss color tokens, namespace-chip palette, base styles
    ├── layout/           # topbar, filterchips, details pane, command bar
    ├── components/       # table, palette, form, overlay, context picker, sparkline, help
    └── views/            # one file per resource + logs/describe/generic_describe sub-views
```

**Key design decisions:**

- **Hexagonal architecture.** `port/` defines interfaces; `k8s/resources/` implements them; `ui/views/` depends only on port interfaces. The UI has zero imports from `client-go` — `grep -rn "k8s.io/" ui/views/` must return nothing.
- **Immutable Bubble Tea models.** All view structs are value types. `Update` methods return a new value; receivers are never mutated.
- **Informer-based watching.** `k8s/watcher.go` runs a `SharedInformerFactory` (30s resync) and forwards each event as a typed `tea.Msg` (`PodsUpdatedMsg`, `MetricsTickMsg`, `LogLineMsg`, …) via `program.Send`. Per-resource events are debounced 500ms so a busy informer can't spam the model.
- **Composed shell with overlays.** `app.View()` stacks top bar → filter chips → table+details → command bar. Sub-views (logs, describe) take over the body; `:` palette and `?` help paint over the live frame using a cell-aware ANSI overlay (`ui/components/overlay.go`) so the table stays visible behind them — Lip Gloss's native `Place` would blank it.
- **Async list calls.** Synchronous `client-go` listing on large clusters used to wedge the UI for 20–30s; every list now runs off the Update loop via a `tea.Cmd` returning a typed `*ListedMsg`.
- **Secret safety.** `client-go` already handles base64 transparently. klens never touches raw base64 — `SecretSvc.UpdateSecret` does Get-then-Update so other fields (`Type`, annotations) survive the round-trip.

## Development

```bash
# The justfile is the canonical task runner
just check          # test + vet + lint
just test           # go test ./...
just test-race      # -race
just lint           # golangci-lint run ./...
just build          # go build -o klens .
just run -- --namespace production
just release-dry    # goreleaser snapshot
```

Tests use `k8s.io/client-go/kubernetes/fake` and `metrics/...fake` — **no real cluster is needed**, and tests must not require one.

The linter config (`.golangci.yml`) runs `misspell`, `unconvert`, `unparam`, `durationcheck`, `exhaustive`, `nilerr`, `reassign`, `goconst`, `gocritic`, `prealloc`, `revive` (with `exported` to enforce doc comments), `usetesting`, and `gofumpt`. All exported symbols carry doc comments so the module renders cleanly on [pkg.go.dev](https://pkg.go.dev/github.com/hermanu/klens).

## Contributing

Issues and pull requests are welcome. For significant changes, open an issue first to discuss the approach.

**Adding a new resource type** is a 7-step wire-up that touches the port, the resource layer, the watcher, and the shell:

1. Add the item struct in `k8s/resources/types.go` (must implement `Resource`).
2. Add `<Resource>Svc` in `k8s/resources/<resource>.go`, implementing the port interface.
3. Add the interface to `port/port.go` and the field to `port.Services`.
4. Wire it in `app.buildServices` and add a view field + constructor + routing in `app/app.go` (`viewKind` enum, `currentView`, `reloadCmd`, `paletteNameToView`).
5. Register an informer + `*UpdatedMsg` in `k8s/watcher.go`.
6. Add `ui/views/<resource>.go` implementing the `View` interface (`Table`, `Details`, `Chips`, `KeyHints`, `Title`, `Count`). Implement `KeyMap()` for the `?` overlay.
7. Run async `List` via `tea.Cmd` returning `*ListedMsg` — synchronous list calls block the Update loop on large clusters.

See [`CLAUDE.md`](CLAUDE.md) for the complete architecture brief, including the view contract, sub-view message shapes, and the sub-view drill-down protocol.

## License

MIT
