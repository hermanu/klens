# klens

A fast, keyboard-driven Kubernetes TUI built for engineers who spend real time in their clusters. Inspired by k9s, rebuilt from scratch with a cleaner UI and first-class secret editing.

```
cluster › production  ns › default                              42/42  ● watching
────────────────────────────────────────────────────────────────────────────────
NAME                              READY    STATUS            RESTARTS   AGE
› api-gateway-7d9f4b8c6-xk2pq     2/2      Running           0          3d
  worker-5c8b9d7f4-mn3rs          1/1      Running           2          12h
  worker-5c8b9d7f4-qw7yt          1/1      Running           0          12h
✕ payment-svc-6b4f9c8d7-zp1lm     0/1      CrashLoopBackOff  14         47m
  redis-0                         1/1      Running           0          7d
────────────────────────────────────────────────────────────────────────────────
j/k navigate  enter detail  l logs  d delete  : command  ? help       pods
```

## Why klens?

k9s is great but has friction points: its UI is dense, secret values are read-only, and there is no easy way to edit configmap data in-place. **klens** fixes that:

- **Inline secret editor** — open any secret, edit values directly, save with `ctrl+s`. Values are decoded from base64 automatically; you work with plain text.
- **Configmap editor** — same experience for configmaps.
- **Live watching** — Kubernetes informers keep every view updated without polling.
- **Command palette** — press `:` to jump to any resource view instantly.
- **Dark, minimal UI** — designed from scratch with a coherent color palette. No visual noise.

## Features

| Resource | List | Edit | Delete |
|---|---|---|---|
| Pods | ✓ | — | ✓ |
| Deployments | ✓ | — | — |
| Services | ✓ | — | — |
| Secrets | ✓ | ✓ | ✓ |
| ConfigMaps | ✓ | ✓ | — |
| Namespaces | ✓ | — | — |
| Nodes | ✓ | — | — |
| PersistentVolumeClaims | ✓ | — | — |

## Installation

### Pre-built binary (recommended)

Download the binary for your platform from the [latest release](https://github.com/manu/klens/releases/latest):

```bash
# Linux (amd64)
curl -L https://github.com/manu/klens/releases/latest/download/klens_linux_amd64.tar.gz | tar xz
sudo mv klens /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/manu/klens/releases/latest/download/klens_darwin_arm64.tar.gz | tar xz
sudo mv klens /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/manu/klens/releases/latest/download/klens_darwin_amd64.tar.gz | tar xz
sudo mv klens /usr/local/bin/
```

### Go install

Requires Go 1.22+.

```bash
go install github.com/manu/klens@latest
```

### From source

```bash
git clone https://github.com/manu/klens
cd klens
go build -o klens .
sudo mv klens /usr/local/bin/
```

## Usage

```bash
# Use your current kubeconfig context
klens

# Specify a kubeconfig
klens --kubeconfig ~/.kube/staging.yaml

# Start in a specific namespace
klens --namespace production
```

## Keyboard shortcuts

### Global

| Key | Action |
|---|---|
| `:` | Open command palette |
| `q` / `ctrl+c` | Quit |
| `?` | Help |

### Navigation (all views)

| Key | Action |
|---|---|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `enter` | Open detail / editor |

### Command palette

| Key | Action |
|---|---|
| Type to filter | Fuzzy-match resource name or alias |
| `↑` / `↓` | Navigate results |
| `enter` | Switch to selected view |
| `esc` | Close |

**Aliases:** `:po` pods · `:dp` deployments · `:svc` services · `:sec` secrets · `:cm` configmaps · `:ns` namespaces · `:no` nodes · `:pvc` pvcs

### Secret / ConfigMap editor

| Key | Action |
|---|---|
| `tab` | Switch between key and value column |
| `↑` / `↓` | Move between rows |
| `ctrl+a` | Add a new key |
| `ctrl+d` | Delete selected key |
| `ctrl+h` | Toggle value visibility (hide/show) |
| `ctrl+s` | Save to cluster |
| `esc` | Cancel without saving |

## Configuration

klens reads `~/.klens/config.yaml` on startup. All fields are optional.

```yaml
# Path to kubeconfig (defaults to KUBECONFIG env var or ~/.kube/config)
kubeconfig: ~/.kube/config

# Default namespace (defaults to "default")
namespace: production

# Accent color for UI highlights (hex)
accent: "#e85a4f"
```

## Architecture

```
klens/
├── main.go               # entry point — wires app + watcher
├── app/                  # root tea.Model, view router, palette
├── config/               # config loading with defaults
├── port/                 # port interfaces (hexagonal architecture)
├── k8s/
│   ├── client.go         # kubeconfig loading, context listing
│   ├── watcher.go        # informer-based live updates → tea.Msg
│   └── resources/        # service structs implementing port interfaces
└── ui/
    ├── theme/            # Lip Gloss color tokens and base styles
    ├── layout/           # header bar, status bar
    ├── components/       # table, command palette, form editor
    └── views/            # one file per resource type
```

**Key design decisions:**

- **Hexagonal architecture** — `port/` defines interfaces; `k8s/resources/` implements them; `ui/views/` depends only on port interfaces. The UI has zero imports from `client-go`.
- **Immutable Bubble Tea models** — all view structs are value types. `Update` methods return new values, never mutate.
- **Informer-based watching** — `k8s/watcher.go` uses `SharedInformerFactory` to watch all resource types. Events are forwarded as `tea.Msg` values; each view re-fetches only its own data.
- **Secret safety** — `client-go` handles base64 encoding/decoding transparently. klens never touches raw base64; you always work with plain-text values.

## Development

```bash
# Run tests
go test ./...

# Lint
golangci-lint run ./...

# Build
go build .
```

Tests use `k8s.io/client-go/kubernetes/fake` — no real cluster needed to run them.

## Contributing

Issues and pull requests are welcome. For significant changes, open an issue first to discuss the approach.

**Adding a new resource type** requires four steps:
1. Define the item struct in `k8s/resources/types.go`
2. Add a service struct in `k8s/resources/<resource>.go` implementing the port interface
3. Add the port interface to `port/port.go` and the field to `port.Services`
4. Add the view in `ui/views/<resource>.go` following the existing pattern
5. Register the watcher in `k8s/watcher.go` and wire it into `app/app.go`

## License

MIT
