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

# ─── develop ──────────────────────────────────────────────────────────────────

start:
    go run .

run *args:
    go run . {{args}}

help:
    go run . --help

version:
    go run . --version

# ─── quality ──────────────────────────────────────────────────────────────────

fmt:
    gofmt -w *.go

vet:
    go vet ./...

test:
    go test ./...

test-one name:
    go test -run {{name}}

integration-test:
    DSYNC_INTEGRATION=1 go test -run TestWordPressFixtureImportsIntoMariaDB -count=1

bench:
    go test -bench=Benchmark -benchmem ./...

# Pre-release gate. Compiles to a temp binary so checks do not dirty dist/.
check: fmt vet test build-check

[private]
build-check:
    @tmp="$(mktemp -d)"; trap 'rm -rf "${tmp}"' EXIT; go build -trimpath -ldflags="-s -w" -o "${tmp}/{{_bin}}" .

# ─── build / release ──────────────────────────────────────────────────────────

clean:
    rm -rf dist {{_release_dir}} releases/v*

build:
    mkdir -p dist
    go build -trimpath -ldflags="-s -w" -o dist/{{_bin}} .
    chmod +x dist/{{_bin}}

# Release flow: dev archives by default; --stable/--bump runs Go-module tag flow.
release *args:
    @stable=false; bump=""; version_override=""; output=""; push=true; dry_run=false; set -- {{args}}; \
    while [[ $# -gt 0 ]]; do \
        case "$1" in \
            --stable) stable=true; shift ;; \
            --bump) stable=true; if [[ $# -ge 2 && "$2" != --* ]]; then bump="$2"; shift 2; else bump=patch; shift; fi ;; \
            --bump=*) stable=true; bump="${1#--bump=}"; [[ -n "$bump" ]] || bump=patch; shift ;; \
            --version) [[ $# -ge 2 ]] || { echo 'release: --version requires a value' >&2; exit 2; }; version_override="$2"; shift 2 ;; \
            --version=*) version_override="${1#--version=}"; [[ -n "$version_override" ]] || { echo 'release: --version requires a value' >&2; exit 2; }; shift ;; \
            --output) [[ $# -ge 2 ]] || { echo 'release: --output requires a value' >&2; exit 2; }; output="$2"; shift 2 ;; \
            --output=*) output="${1#--output=}"; [[ -n "$output" ]] || { echo 'release: --output requires a value' >&2; exit 2; }; shift ;; \
            --no-push) push=false; shift ;; \
            --push) push=true; shift ;; \
            --dry-run) dry_run=true; shift ;; \
            -h|--help) just _release-help; exit 0 ;; \
            *) echo "release: unknown argument $1" >&2; just _release-help >&2; exit 2 ;; \
        esac; \
    done; \
    current="$(tr -d '[:space:]' < version)"; version="$current"; \
    if [[ "$stable" == true && -z "$bump" && -z "$version_override" ]]; then bump=patch; fi; \
    if [[ -n "$version_override" ]]; then version="$version_override"; [[ "$version" == v* ]] || version="v$version"; fi; \
    if [[ -n "$bump" ]]; then \
        base="${current#v}"; IFS=. read -r major minor patch <<<"$base"; \
        case "$bump" in \
            patch) version="v$major.$minor.$((patch + 1))" ;; \
            minor) version="v$major.$((minor + 1)).0" ;; \
            major) version="v$((major + 1)).0.0" ;; \
            v[0-9]*.[0-9]*.[0-9]*) version="$bump" ;; \
            [0-9]*.[0-9]*.[0-9]*) version="v$bump" ;; \
            *) echo "release: invalid bump $bump" >&2; exit 2 ;; \
        esac; \
    fi; \
    [[ -n "$output" ]] || { if [[ "$stable" == true ]]; then output="releases/$version"; else output="{{_release_dir}}"; fi; }; \
    if [[ "$dry_run" == true ]]; then echo "release: stable=$stable version=$version output=$output push=$push"; exit 0; fi; \
    if [[ "$stable" == true ]]; then \
        git diff --quiet || { echo 'release: tracked files changed; commit or stash before stable release' >&2; exit 1; }; \
        git diff --cached --quiet || { echo 'release: staged changes exist; commit or stash before stable release' >&2; exit 1; }; \
        ! git rev-parse -q --verify "refs/tags/$version" >/dev/null || { echo "release: tag exists: $version" >&2; exit 1; }; \
        printf '%s\n' "$version" > version; \
    fi; \
    just check; \
    just _release-build "$version" "$output"; \
    if [[ "$stable" != true ]]; then echo "release: dev artifacts are ready in $output"; exit 0; fi; \
    test "$(git diff --name-only)" = version || { echo 'release: unexpected tracked changes after build/check:' >&2; git diff --name-only >&2; exit 1; }; \
    git add version; \
    git commit -m "Release $version"; \
    git tag "$version"; \
    if git rev-parse -q --verify refs/tags/latest >/dev/null; then git tag -d latest >/dev/null; fi; \
    git tag latest; \
    if [[ "$push" == true ]]; then git push origin HEAD && git push origin "$version" && git push origin latest --force; fi; \
    echo "release: stable $version ready"

release-patch *args:
    @just release --bump patch {{args}}

release-minor *args:
    @just release --bump minor {{args}}

release-major *args:
    @just release --bump major {{args}}

[private]
_release-help:
    @echo 'usage:'
    @echo '  just release'
    @echo '  just release --stable'
    @echo '  just release --bump patch|minor|major|X.Y.Z'
    @echo '  just release --bump minor --no-push'
    @echo 'stable release runs check, build, version commit, semver tag, latest tag, and push'

[private]
_release-build version output_dir:
    @rm -rf "{{output_dir}}"
    @mkdir -p "{{output_dir}}"
    @just _release-target linux amd64 "{{version}}" "{{output_dir}}"
    @just _release-target linux arm64 "{{version}}" "{{output_dir}}"
    @just _release-target darwin amd64 "{{version}}" "{{output_dir}}"
    @just _release-target darwin arm64 "{{version}}" "{{output_dir}}"
    @(cd "{{output_dir}}" && sha256sum ./* > checksums.txt)
    @echo "release artifacts written to {{output_dir}}"

[private]
_release-target goos goarch version output_dir:
    @target="{{_bin}}_{{version}}_{{goos}}_{{goarch}}"; \
    work_dir="{{output_dir}}/${target}"; \
    binary="${work_dir}/{{_bin}}"; \
    mkdir -p "${work_dir}"; \
    echo "building {{goos}}/{{goarch}}"; \
    GOOS="{{goos}}" GOARCH="{{goarch}}" go build -trimpath -ldflags="-s -w" -o "${binary}" .; \
    chmod +x "${binary}"; \
    (cd "{{output_dir}}" && tar -czf "${target}.tar.gz" "${target}/{{_bin}}"); \
    rm -rf "${work_dir}"

# Backward-compatible tag flow. Prefer `just release --bump patch|minor|major`.
tag-push:
    @version="$(cat version)"; \
    git tag "${version}"; \
    git push origin "${version}"; \
    if git rev-parse latest >/dev/null 2>&1; then git tag -d latest; fi; \
    git tag latest; \
    git push origin latest --force
