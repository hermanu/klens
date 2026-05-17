---
name: add-resource
description: Scaffold a new Kubernetes resource type across types.go, port.go, k8s/resources/, k8s/watcher.go, app/app.go, and ui/views/ following the 7-step recipe documented in CLAUDE.md. User-only — invoke manually via /add-resource because it writes ~6 files.
disable-model-invocation: true
---

# add-resource

Use when adding support for a new Kubernetes resource type to klens. Codifies the 7-step recipe documented in the project's `CLAUDE.md` and points at canonical reference files in the repo so the patterns stay in sync without duplicating templates here.

The two highest-value first invocations are `Events` and `CronJobs` — both already have informers registered in `k8s/watcher.go` (and emit `EventsUpdatedMsg{}` / `CronJobsUpdatedMsg{}`) but lack dedicated views.

## Inputs

Collect these before starting:

- **`<Resource>`** — PascalCase Go type stem (e.g. `Event`, `CronJob`, `Ingress`).
- **`<resource>`** — lowercase singular for field/var names (e.g. `event`, `cronJob`, `ingress`).
- **`<resources>`** — lowercase plural for the canonical view name string (e.g. `events`, `cronjobs`, `ingresses`).
- **`<mnemonic>`** — 2-4 char palette alias (e.g. `ev`, `cj`, `ing`).
- **Group / Version path** — where the resource lives in client-go's informer factory, e.g. `Core().V1().Events()`, `Batch().V1().CronJobs()`, `Networking().V1().Ingresses()`.
- **Namespaced vs cluster-scoped** — namespaced resources implement `GetNamespace()` returning the actual namespace; cluster-scoped resources return `""`.

## Reference files (copy & adapt these)

Pick the one closest to the resource you're adding and mirror its shape rather than starting from scratch:

| Use case | Reference |
|---|---|
| **Simplest resource** (no metrics, no detail fetch) | `k8s/resources/pvcs.go` (62 lines) + `k8s/resources/pvcs_test.go` (97 lines) |
| **Cluster-scoped** (`GetNamespace()` returns `""`) | `k8s/resources/nodes.go`, `k8s/resources/namespaces.go` |
| **Resource with detail fetch + mutation** | `k8s/resources/secrets.go` (`GetSecret`, `UpdateSecret`, `DeleteSecret`) |
| **View with drill-down to pods** | `ui/views/deployments.go` (emits `SwitchToLogsMsg`) |
| **View with KV-only Details panel** | `ui/views/pvcs.go` (uses `layout.DefaultDetails`) |

Open the closest reference and copy column sets, filter fields, and helpers — don't reinvent them.

## The recipe

### Step 1 — `k8s/resources/types.go`

Add a `<Resource>Item` struct that implements the `Resource` interface (`GetName`, `GetNamespace`, `GetAge`).

- For **namespaced** resources: mirror `PVCItem` (`types.go` lines 195-214).
- For **cluster-scoped** resources: mirror `NodeItem` (lines 170-193) — `GetNamespace()` returns `""`.

Every exported field must have a doc comment explaining *why* the field exists or what constraint it carries — restating the name fails `revive/exported`.

### Step 2 — `k8s/resources/<resource>.go`

Define `<Resource>Svc` with a `kubernetes.Interface` field, a `New<Resource>Svc` constructor, and `List<Resources>(ctx, namespace) ([]<Resource>Item, error)`.

Pattern: `k8s/resources/pvcs.go` is the cleanest 62-line template. Adapt the API path:

```go
list, err := s.client.<Group>().<Resources>(namespace).List(ctx, metav1.ListOptions{})
```

For cluster-scoped resources, drop the `(namespace)` segment.

### Step 3 — `port/port.go`

Add the interface:

```go
// <Resource>Service is the port for <resource> operations.
type <Resource>Service interface {
    List<Resources>(ctx context.Context, namespace string) ([]resources.<Resource>Item, error)
}
```

Then add a field to `Services`:

```go
<Resources> <Resource>Service
```

### Step 4 — `app/app.go` (the multi-touchpoint step — sync ALL of these)

CLAUDE.md warns: *"Adding a new resource means adding to viewKind, the field set, the constructor, currentView, reloadCmd, View, and paletteNameToView — keep these in sync."*

1. Append `view<Resources>` to the `viewKind` enum.
2. Add canonical name constant: `viewName<Resources> = "<resources>"`.
3. Add `<resources>View views.<Resources>View` field on `Model`.
4. In `New(...)`, construct the view and assign it: `m.<resources>View = views.New<Resources>View(services.<Resources>, namespace)`.
5. Wire concrete service in `buildServices(...)`: `services.<Resources> = resources.New<Resource>Svc(client.Kube)`.
6. Add `case view<Resources>: return m.<resources>View` to `currentView()`.
7. Add `case view<Resources>: return m.<resources>View.Reload()` (or equivalent) to `reloadCmd()`.
8. Add `case view<Resources>:` branch to the `View()` switch (panel rendering).
9. Add `case viewName<Resources>: return view<Resources>` to `paletteNameToView()`.
10. Optionally append `{Name: viewName<Resources>, Desc: "list <resources>", Alias: ":<mnemonic>"}` in `ui/components/commands.go::DefaultCommands()`.
11. Optionally extend the nav grid in `app.New(...)`'s `NavItems` and the mnemonic-routing block in `updateGlobal()` for a number-key shortcut.

### Step 5 — `k8s/watcher.go`

Define the message:

```go
// <Resources>UpdatedMsg signals that the <resources> resource changed; see PodsUpdatedMsg.
type <Resources>UpdatedMsg struct{}
```

Register the informer in `Watcher.Start()`:

```go
w.register(w.factory.<Group>().<Version>().<Resources>().Informer(), <Resources>UpdatedMsg{})
```

(Skip this step if the informer is already registered — `EventsUpdatedMsg` and `CronJobsUpdatedMsg` already exist; you only need to wire them through `app.go::Update` to the new view's reload.)

### Step 6 — `ui/views/<resource>.go`

Implement the full `View` interface (see `ui/views/view.go` for the contract):

- `Table(width, height int) string`
- `Details(width, height int) string` — for non-pod views, return `layout.DefaultDetails(width, height, ...)` driven by a per-view `focusKVs()` mapping the focused row to `[]layout.KV`
- `Chips() []layout.FilterChip` — return `nil` (vestigial; CLAUDE.md notes the method stays on the interface)
- `KeyHints() []layout.KeyHint`
- `Title() string` — uppercased (e.g. `EVENTS`)
- `Count() (visible, total int)`

Optional interfaces — implement all three for parity with existing views:

- `KeyMap() []components.KeySpec` — drives the `?` overlay
- `CursorIndex() int` — 1-indexed focused row; drives the panel foot
- `Filter() string` — current filter string; the shell restores it on view switch

Pattern: `ui/views/pvcs.go` (255 lines) is the closest reference for a no-metrics, no-drill-down resource.

**Critical invariant**: `Update` must be a **value-receiver** method that returns a new view value. Never mutate the receiver. CLAUDE.md: *"Views are value types — Update returns a new value; never mutate the receiver."*

### Step 7 — Async list

Wrap every `svc.List<Resources>(ctx, ns)` call in a `tea.Cmd`. From CLAUDE.md: *"Synchronous listing on large clusters used to wedge the UI for 20–30s; the async pattern is now mandatory for new list operations."*

Pattern:

```go
func (v <Resources>View) reload() tea.Cmd {
    svc := v.svc
    ns := v.namespace
    return func() tea.Msg {
        items, err := svc.List<Resources>(context.Background(), ns)
        if err != nil {
            return ListErrorMsg{Err: err}
        }
        return <Resources>ListedMsg{Items: items}
    }
}
```

In `Update`, fire `reload()` on initial load and on the corresponding `<Resources>UpdatedMsg`.

### Step 8 — Tests (CLAUDE.md doesn't list this, but the codebase culture demands it)

Add `k8s/resources/<resource>_test.go` using `k8s.io/client-go/kubernetes/fake.NewSimpleClientset(...)`. Every existing resource has a `*_test.go` with at least a happy-path test and one edge case. CLAUDE.md is explicit: *"tests must not require [a real cluster]."*

Pattern: `k8s/resources/pvcs_test.go` (97 lines, two test cases).

## Verification checklist

After implementation:

```bash
# Hexagonal regression check — must return nothing:
grep -rn "k8s.io/" ui/views/

# Full check (test + vet + lint):
just check

# Manual confirmation in the TUI:
just run            # then navigate via :<resources> or :<mnemonic>
```

If `just check` fails:
- `revive/exported` failures → add doc comments to new exported symbols
- `exhaustive` failures → add new `viewKind` value to switches that lack a `default:` case (most don't, because CLAUDE.md says: *"`default:` case satisfies the check"*)
- `goconst` failures → resource name string occurs ≥7 times; promote to a constant (this is why `viewNamePods`, `viewNameDeployments`, etc. exist in `app/app.go`)

## Notes

- `Events` and `CronJobs` informers already exist; for those, skip Step 5 entirely and just wire the existing `*UpdatedMsg` through `app.go::Update` to call the new view's reload.
- If your resource is cluster-scoped (no namespace concept — e.g. ClusterRoles), the `namespace` parameter on `List<Resources>` should be ignored by the service implementation, and `GetNamespace()` on the item must return `""`. The shell's "all namespaces" toggle is irrelevant for these.
- Doc comments are mandatory on every exported symbol — the project's lint config enforces this and CI will fail without them.
- The `klens-hex-reviewer` subagent will catch most invariant violations after implementation. Invoke it before opening a PR.
