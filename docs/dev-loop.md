# Development loop

## First setup

```bash
go version
go mod download
go test ./...
go run . --help
```

Use Go 1.23 or newer unless `go.mod` changes.

## Common commands

```bash
just start               # run CLI from source
just run -- --help       # run CLI with arbitrary args
just test                # full unit suite
just test-one TestName   # focused test
just vet                 # static checks
just build               # build and chmod ./dist/dsync
just check               # fmt, vet, tests, temp compile check
just release             # checked dev release archives into releases/dev/
just release --bump patch|minor|major  # stable Go-module release tag flow
just bench               # benchmarks
just integration-test    # Docker-backed serialized DB import test
```

## Release flow

`just release` runs the release gate and builds archives plus `checksums.txt` into `releases/dev/`.

Stable releases use Go module tags; pushing `vX.Y.Z` is the registry step that makes `go install github.com/asolopovas/dsync@latest` resolve the new version. The release task is the release procedure: it must run the preflight, version update, `just check`, archive build, release commit, semver tag, rolling `latest` tag, and push. Do not manually run those steps as separate commands when releasing.

```bash
just release --stable          # patch bump, commit, tag, push
just release --bump patch      # same as --stable
just release --bump minor
just release --bump major
just release --bump 1.2.3
just release --bump patch --no-push  # local commit/tags only
just release-patch            # convenience wrapper
just release-minor
just release-major
```

Use `just release --dry-run --bump minor` to inspect the computed version without changing files.

## Local config

Real configs are ignored by git. Keep private values in `dsync-config.json`, `configs/`, or another ignored path.

Generate a starter config:

```bash
go run . --gen
```

Run with a custom config:

```bash
go run . -c ./configs/site.json -a
go run . -c ./configs/site.json -f
go run . -c ./configs/site.json -d
```

## Local database compose file

By default local DB commands use:

```bash
$HOME/www/dev/docker-compose.yml
```

Override it for another project:

```bash
DSYNC_COMPOSE_FILE=/path/to/docker-compose.yml go run . -c ./configs/site.json -d
```

The local service name is currently expected to be `mariadb`, and root password is expected to be `secret`.

## Iteration habits

- Use focused tests for replacement and orchestration changes (`go test -run TestTransformSQLDump`, `go test -run TestSyncDB`).
- Add tests before changing sync ordering or replacement order.
- Keep external-command errors readable; include captured output when possible.
- Do not run reverse sync against a real remote unless the task requires it and the target is safe.
