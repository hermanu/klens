#!/usr/bin/env bash
# PostToolUse hook: format and lint Go files touched by Edit/Write/MultiEdit.
#
# - gofumpt -w on the file (silently skipped if gofumpt is not installed)
# - golangci-lint run --fast-only on the file's package
#
# On lint failure, exit 2 with stderr so the model picks up the diagnostics
# on its next turn. gofumpt is recommended; install with:
#   go install mvdan.cc/gofumpt@latest

set -uo pipefail

INPUT="$(cat)"
FILE_PATH="$(printf '%s' "$INPUT" | jq -r '.tool_input.file_path // empty')"

# Skip unless this is a real .go source file inside the project.
[[ -n "$FILE_PATH" && -f "$FILE_PATH" && "$FILE_PATH" == *.go ]] || exit 0
case "$FILE_PATH" in
  *vendor/*) exit 0 ;;
esac

ERR=""

# gofumpt formats more aggressively than gofmt; skip silently if missing.
if command -v gofumpt >/dev/null 2>&1; then
  if ! GOFUMPT_OUT="$(gofumpt -w "$FILE_PATH" 2>&1)"; then
    ERR+="gofumpt failed on $FILE_PATH:"$'\n'"$GOFUMPT_OUT"$'\n'
  fi
fi

# Lint scoped to the file's package keeps cold-cache latency bounded.
# --fast-only skips the heavier linters (revive, gocritic, exhaustive); they
# still run on `just check` / `just lint` and in CI.
if command -v golangci-lint >/dev/null 2>&1; then
  PKG_DIR="$(dirname "$FILE_PATH")"
  if ! LINT_OUT="$(golangci-lint run --fast-only "$PKG_DIR/..." 2>&1)"; then
    ERR+="golangci-lint issues in $PKG_DIR:"$'\n'"$LINT_OUT"$'\n'
  fi
fi

if [[ -n "$ERR" ]]; then
  printf '%s' "$ERR" >&2
  exit 2
fi
exit 0
