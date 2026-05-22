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
go run .                 # run CLI from source
make start               # Makefile alias for go run .
go test ./...            # full unit suite
go test -run TestName    # focused test
go vet ./...             # static checks
go build -o ./dist/dsync .
make build               # build and chmod ./dist/dsync
just check               # fmt, vet, tests, temp compile check
just release             # cross-compile archives into releases/dev/
go test -bench=Benchmark -benchmem ./...
DSYNC_INTEGRATION=1 go test -run TestWordPressFixtureImportsIntoMariaDB -count=1
make install-local       # install to $GOBIN/dsync
```

`make test` currently runs the CLI with `./dsync-config.json`; it is not the unit test suite. Use `go test ./...` for verification.

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
