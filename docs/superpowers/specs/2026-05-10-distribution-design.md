# Distribution: Homebrew + deb/rpm

**Date:** 2026-05-10
**Status:** Approved

## Goal

Make `klens` easy to install on macOS via Homebrew and on Linux via native `.deb`/`.rpm` packages, without adding infrastructure to maintain beyond a tap repository.

## Scope

- Homebrew tap for macOS (and Linux Homebrew users)
- `.deb` and `.rpm` packages attached as GitHub release assets
- No hosted apt/yum repository — users download packages manually from GitHub Releases
- No AUR, no Snap, no install script

## Approach

Extend the existing GoReleaser v2 config with two stanzas (`brews:` and `nfpms:`). The existing release workflow (triggered on `v*` tags) and CI pipeline are unchanged except for one new env var passed to the GoReleaser action.

## Components

### 1. Tap repository: `hermanu/homebrew-klens`

A new public GitHub repo named exactly `hermanu/homebrew-klens`. The `homebrew-` prefix is required — it signals to Homebrew that this is a tap. The repo starts empty; GoReleaser pushes the generated formula to `Formula/klens.rb` on every release.

User install flow:
```
brew tap hermanu/klens
brew install klens
```

### 2. GitHub PAT secret

GoReleaser needs push access to `hermanu/homebrew-klens`, which is outside the scope of the default `GITHUB_TOKEN`. A fine-grained PAT (or classic PAT with `repo` scope) scoped to `hermanu/homebrew-klens` is created manually by the user and stored as a repository secret named `HOMEBREW_TAP_GITHUB_TOKEN` in `hermanu/klens`.

### 3. GoReleaser: `brews:` stanza

Added to `.goreleaser.yml`. Points to the tap repo, sets formula metadata (description, homepage, license), and references the macOS `tar.gz` assets already being built. GoReleaser computes the checksums and writes them into the formula automatically.

Relevant fields:
- `repository.owner`: `hermanu`
- `repository.name`: `homebrew-klens`
- `repository.token`: `{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}`
- `homepage`: `https://github.com/hermanu/klens`
- `description`: short one-line description of klens
- `license`: to match the LICENSE file in the repo

### 4. GoReleaser: `nfpms:` stanza

Added to `.goreleaser.yml`. Produces `.deb` (for `apt`) and `.rpm` (for `yum`/`dnf`) from the Linux binaries already being compiled. Packages are attached as additional release assets.

Required metadata:
- `maintainer`: name + email
- `description`: multi-line package description
- `homepage`
- `license`
- `formats: [deb, rpm]`

User install flow (Debian/Ubuntu):
```
wget https://github.com/hermanu/klens/releases/latest/download/klens_linux_amd64.deb
sudo dpkg -i klens_linux_amd64.deb
```

User install flow (Fedora/RHEL):
```
wget https://github.com/hermanu/klens/releases/latest/download/klens_linux_amd64.rpm
sudo rpm -i klens_linux_amd64.rpm
```

### 5. Release workflow: env var

One addition to `.github/workflows/release.yml` — pass the new secret to the GoReleaser action step:

```yaml
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
```

### 6. Release footer

The `release.footer` in `.goreleaser.yml` is updated to document all install paths:

```
**Homebrew (macOS/Linux):**
brew tap hermanu/klens && brew install klens

**Debian/Ubuntu:**
wget .../klens_linux_amd64.deb && sudo dpkg -i klens_linux_amd64.deb

**Fedora/RHEL:**
wget .../klens_linux_amd64.rpm && sudo rpm -i klens_linux_amd64.rpm

**Binary (all platforms):** download from assets above.

**Go install:**
go install github.com/hermanu/klens@{{.Tag}}
```

## Implementation order

1. Create `hermanu/homebrew-klens` repo (public, empty)
2. Create PAT and add as `HOMEBREW_TAP_GITHUB_TOKEN` secret in `hermanu/klens`
3. Add `nfpms:` stanza to `.goreleaser.yml`
4. Add `brews:` stanza to `.goreleaser.yml`
5. Pass `HOMEBREW_TAP_GITHUB_TOKEN` env var in `release.yml`
6. Update `release.footer` in `.goreleaser.yml`
7. Validate with `just release-dry` (snapshot build)

## Out of scope

- AUR (requires GoReleaser Pro for automation, or manual PKGBUILD maintenance)
- Snap/Flatpak
- Hosted apt/yum repository
- Scoop (Windows)
- Install script (`curl | sh`)
