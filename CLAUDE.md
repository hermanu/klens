# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`klens` is a keyboard-driven Kubernetes TUI (Bubble Tea + Lip Gloss) inspired by k9s. Its differentiating feature is **inline editing of secrets and configmaps** with values surfaced as plain text (no manual base64). Module path: `github.com/hermanu/klens`. Go **1.26.3** is the required toolchain (per `go.mod`); the README still says 1.22+ — trust `go.mod`.

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

Tests use `k8s.io/client-go/kubernetes/fake`; **no real cluster is needed**, and tests must not require one.

## Architecture

### Hexagonal split

The dependency direction is strict and load-bearing — keep it that way:

```
ui/views  ──depends on──▶  port  ◀──implements──  k8s/resources
```

- `port/port.go` defines one interface per resource (`PodService`, `SecretService`, …) and a `Services` struct that bundles them.
- `k8s/resources/*.go` implements those interfaces against `kubernetes.Interface` (real or fake).
- `ui/views/*.go` accepts only the port interface it needs in its constructor (`NewSecretsView(svc port.SecretService, …)`). **Views must not import `client-go`.** If you find yourself reaching for `k8s.io/...` from a view, the missing operation belongs in the port and resource layer.
- `app/app.go` is the only place that wires concrete services into views (`buildServices`).

### Bubble Tea model

- All view structs are **value types**. `Update` returns a new value; never mutate the receiver. The root `Model.Update` follows the same rule and uses a generic helper `updateView[V]` because Go generics can't express the per-view method set.
- The root model owns every view as a field and routes messages with a `viewKind` enum. Adding a new resource means adding to `viewKind`, the field set, the constructor, `routeToCurrentView`, `reloadCmd`, `View`, and `paletteNameToView` — keep these in sync.
- `app.Model.Init()` and `reloadCmd()` kick views by sending the corresponding `*UpdatedMsg`; views re-fetch on receipt. There is no shared cache — each view fetches its own data.

### Watcher → tea.Msg pipeline

`k8s/watcher.go` wraps a `SharedInformerFactory` (30s resync). `Start()` registers an informer per resource and forwards Add/Update/Delete events as typed `tea.Msg` values (`PodsUpdatedMsg{}`, etc.) via `program.Send`. Views listen for their own message type in `Update` and call their service to refetch. To support a new resource the watcher must register its informer and emit a new `*UpdatedMsg`.

### Secret/ConfigMap editing

`client-go` already decodes `Secret.Data` to raw bytes — **never base64-encode/decode manually**. `SecretSvc.GetSecret` populates `SecretItem.Data`; `ListSecrets` deliberately leaves it empty for performance. The form component (`ui/components/form.go`) edits the decoded map and `UpdateSecret` writes it back via a Get-then-Update (preserves other fields like `Type` and annotations).

### Adding a new resource type

1. Add the item struct to `k8s/resources/types.go` (must implement `Resource`).
2. Add `<Resource>Svc` in `k8s/resources/<resource>.go`.
3. Add the interface to `port/port.go` and a field to `port.Services`.
4. Wire it in `app.buildServices` and add a view field + constructor + routing in `app/app.go`.
5. Register an informer + `*UpdatedMsg` in `k8s/watcher.go`.
6. Add `ui/views/<resource>.go` following an existing view (pods is the simplest list-only example; secrets covers the editor case).

## Conventions

- `port.SvcService` is named that way intentionally to avoid collision with the generic word "service" — don't rename.
- Comments in this codebase explain **why** (constraints, invariants like "Data intentionally omitted"); don't add WHAT-comments.
- `golangci-lint` runs `misspell`, `unconvert`, `unparam` on top of the standard set; `unparam` is disabled for `_test.go`.
- Releases are driven by GoReleaser (`.goreleaser.yml`); `main.version/commit/date` are populated via `-ldflags` at build time, so `go build` locally yields `version="dev"` — that's expected.

## Gotchas

- **`client` may be nil.** `app.New` logs a warning and continues when no kubeconfig is reachable; `main.go` skips the watcher in that case. Any new view/service code must tolerate `m.client == nil` rather than panic.
- **Override precedence.** `app.New(kubeconfigOverride, namespaceOverride)` lets non-empty CLI flags shadow the values from `~/.klens/config.yaml`. Empty strings fall through to the config. The CLI in `main.go` is the only caller — keep flag parsing there, not inside the package.
- **Resync interval is 30s.** `NewWatcher` hardcodes a 30s informer resync. Tests that depend on watcher cadence should either fake the informer or accept this floor.
