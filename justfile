set shell := ["bash", "-euo", "pipefail", "-c"]
set dotenv-load := false

_bin := "dsync"
_dist_dir := "dist"
_dist_bin := _dist_dir / _bin
_release_dir := "releases/dev"
_tmp_dir := "tmp"
_test_packages := "./..."

export CGO_ENABLED := env_var_or_default("CGO_ENABLED", "0")

default:
    @just --list --unsorted

# ─── setup ────────────────────────────────────────────────────────────────────

setup:
    go mod download

[private]
tidy:
    go mod tidy

# ─── develop ──────────────────────────────────────────────────────────────────

# Run the CLI from source. Example: `just run --help`.
run *args:
    go run . {{args}}

[private]
start *args:
    @just run {{args}}

[private]
help:
    go run . --help

[private]
version:
    go run . --version

[private]
run-built *args: build
    ./{{_dist_bin}} {{args}}

# ─── quality ──────────────────────────────────────────────────────────────────

# Pre-release gate: all jobs by default, or selected jobs by name.
check *jobs:
    @if [[ -z "{{jobs}}" ]]; then \
        just fmt; \
        just vet; \
        just test; \
        just build-check; \
    else \
        for job in {{jobs}}; do \
            case "${job}" in \
                fmt|vet|test|test-one|test-race|integration-test|bench|coverage|build|build-check) just "${job}" ;; \
                *) echo "check: unknown job: ${job}" >&2; echo 'known jobs: fmt vet test test-race integration-test bench coverage build' >&2; exit 2 ;; \
            esac; \
        done; \
    fi

[private]
fmt:
    gofmt -w $(git ls-files '*.go')

[private]
vet:
    go vet {{_test_packages}}

[private]
test *args:
    go test {{_test_packages}} {{args}}

[private]
test-one pattern *args:
    go test {{_test_packages}} -run '{{pattern}}' {{args}}

[private]
test-race *args:
    go test -race {{_test_packages}} {{args}}

[private]
integration-test *args:
    DSYNC_INTEGRATION=1 go test {{_test_packages}} -run TestWordPressFixtureImportsIntoMariaDB -count=1 {{args}}

[private]
bench pattern="Benchmark" *args:
    go test {{_test_packages}} -bench='{{pattern}}' -benchmem {{args}}

[private]
coverage:
    mkdir -p {{_tmp_dir}}
    go test {{_test_packages}} -coverprofile={{_tmp_dir}}/coverage.out
    go tool cover -func={{_tmp_dir}}/coverage.out

[private]
coverage-html: coverage
    go tool cover -html={{_tmp_dir}}/coverage.out

[private]
ci: check

[private]
build-check:
    @tmp="$(mktemp -d)"; trap 'rm -rf "${tmp}"' EXIT; go build -trimpath -ldflags="-s -w" -o "${tmp}/{{_bin}}" .

# ─── build / release ──────────────────────────────────────────────────────────

# Build ./dist/dsync.
build:
    @just _go-build "{{_dist_bin}}"

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

[private]
clean:
    rm -rf {{_dist_dir}} {{_tmp_dir}} {{_release_dir}} releases/v*

# Install the current default-branch dev build to /usr/local/bin/dsync.
install dest="/usr/local/bin/dsync":
    @dest="{{dest}}"; dir="$(dirname "${dest}")"; \
    default_branch="$(git remote show origin 2>/dev/null | awk '/HEAD branch/ {print $NF}' || true)"; \
    current_branch="$(git branch --show-current 2>/dev/null || true)"; \
    if [[ -n "${default_branch}" && -n "${current_branch}" && "${current_branch}" != "${default_branch}" ]]; then \
        echo "install: expected default branch ${default_branch}, currently on ${current_branch}" >&2; \
        exit 1; \
    fi; \
    tmp="$(mktemp -d)"; trap 'rm -rf "${tmp}"' EXIT; \
    go build -trimpath -ldflags="-s -w" -o "${tmp}/{{_bin}}" .; \
    if [[ -w "${dir}" ]]; then \
        install -m 0755 "${tmp}/{{_bin}}" "${dest}"; \
    else \
        sudo install -m 0755 "${tmp}/{{_bin}}" "${dest}"; \
    fi; \
    installed_version="$("${dest}" --version)"; \
    echo "installed ${installed_version} to ${dest}"

[private]
_go-build output:
    mkdir -p "$(dirname "{{output}}")"
    go build -trimpath -ldflags="-s -w" -o "{{output}}" .
    chmod +x "{{output}}"

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

[private]
release-patch *args:
    @just release --bump patch {{args}}

[private]
release-minor *args:
    @just release --bump minor {{args}}

[private]
release-major *args:
    @just release --bump major {{args}}

[private]
tag-push:
    @version="$(cat version)"; \
    git tag "${version}"; \
    git push origin "${version}"; \
    if git rev-parse latest >/dev/null 2>&1; then git tag -d latest; fi; \
    git tag latest; \
    git push origin latest --force
