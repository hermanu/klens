# Contributing to klens

Thanks for your interest. Issues and pull requests are both welcome. For non-trivial changes, please open an issue first so we can align on the approach before you write code.

## Development setup

Requires Go 1.26 (see `go.mod`) and [`just`](https://github.com/casey/just).

```bash
git clone https://github.com/hermanu/klens
cd klens
just check    # runs test + vet + lint
just run      # builds and runs against your current kubeconfig
```

Tests use `k8s.io/client-go/kubernetes/fake`, so no real cluster is needed to run `just test`.

## Common commands

| Command | What it does |
|---|---|
| `just build` | Build the `klens` binary |
| `just test` | Run all tests |
| `just test-race` | Run tests with the race detector |
| `just lint` | Run `golangci-lint` |
| `just vet` | Run `go vet` |
| `just check` | Test + vet + lint (run before opening a PR) |
| `just tidy` | `go mod tidy` |

## Branch and commit conventions

- Branch from `master`. Suggested prefixes: `feat/`, `fix/`, `docs/`, `chore/`, `refactor/`.
- Commits follow [Conventional Commits](https://www.conventionalcommits.org/). The release notes generator filters by these prefixes (`feat:`, `fix:`, `docs:`, `chore:`, `ci:` — see `.goreleaser.yml`).
- PRs are squash-merged; the PR title becomes the squash commit message, so it must follow Conventional Commits format.

## Pull request checklist

- [ ] `just check` passes locally
- [ ] New or modified behavior is covered by a test
- [ ] User-facing changes are reflected in the README
- [ ] The PR title follows Conventional Commits

## Adding a new resource type

Adding a new Kubernetes resource type to klens requires updating six places in lockstep:

1. Define the item struct in `k8s/resources/types.go` (must implement the `Resource` interface).
2. Add a service in `k8s/resources/<resource>.go` that implements the matching port interface.
3. Add the port interface to `port/port.go` and a field on `port.Services`.
4. Wire the service in `app.buildServices` and add a view field, constructor, and routing in `app/app.go` (`viewKind`, `routeToCurrentView`, `reloadCmd`, `View`, `paletteNameToView`).
5. Register the informer and a `*UpdatedMsg` in `k8s/watcher.go`.
6. Add the view in `ui/views/<resource>.go` following an existing view (pods is the simplest list-only example; secrets covers the editor case).

## Architecture invariants

A few rules that keep the codebase navigable:

- **Hexagonal direction**: `ui/views` may only depend on `port`, never on `k8s.io/client-go`. If a view needs a new operation, add it to the port and resource layer first.
- **Bubble Tea models are value types**: `Update` returns a new value; never mutate the receiver.
- **No manual base64**: `client-go` already decodes `Secret.Data` to raw bytes. Work with `map[string][]byte` directly.

## Reporting bugs

Please use the issue template and include:

- Output of `klens --help` and `kubectl version`
- Steps to reproduce
- Expected vs. actual behavior

For security-sensitive issues, see [SECURITY.md](SECURITY.md) instead of filing a public issue.
