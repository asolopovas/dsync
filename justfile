set shell := ["bash", "-euo", "pipefail", "-c"]
set dotenv-load := false

_bin := "dsync"
_release_dir := "releases/dev"

export CGO_ENABLED := env_var_or_default("CGO_ENABLED", "0")

default:
    @just --list --unsorted

# ─── setup ────────────────────────────────────────────────────────────────────

setup:
    go mod download

# ─── quality ──────────────────────────────────────────────────────────────────

fmt:
    gofmt -w *.go

vet:
    go vet ./...

test:
    go test ./...

# Pre-release gate. Compiles to a temp binary so checks do not dirty dist/.
check: fmt vet test build-check

[private]
build-check:
    tmp="$$(mktemp -d)"; trap 'rm -rf "$${tmp}"' EXIT; go build -trimpath -ldflags="-s -w" -o "$${tmp}/{{_bin}}" .

# ─── build / release ──────────────────────────────────────────────────────────

clean:
    rm -rf dist {{_release_dir}}

build:
    mkdir -p dist
    go build -trimpath -ldflags="-s -w" -o dist/{{_bin}} .
    chmod +x dist/{{_bin}}

# Cross-compile release archives into releases/dev/ by default.
release *args: check
    ./scripts/build-release.sh {{args}}

# Backward-compatible tag flow. Prefer `just release` before publishing tags.
tag-push:
    version="$$(cat version)"; \
    git tag "$${version}"; \
    git push origin "$${version}"; \
    if git rev-parse latest >/dev/null 2>&1; then git tag -d latest; fi; \
    git tag latest; \
    git push origin latest --force
