# AGENTS.md

Stack: Go 1.23 · Cobra CLI · pterm terminal UI · rsync/ssh · Docker Compose + MariaDB/MySQL.

This file is the agent table of contents, not the project manual. Keep durable knowledge in `docs/` and link it here. If a task uncovers a missing rule, stale workflow, or repeated mistake, update the relevant doc in the same change.

## Start here

| Need | Source of truth |
| --- | --- |
| Documentation map, ownership, freshness | [`docs/README.md`](docs/README.md) |
| Agent operating principles and taste invariants | [`docs/agent-principles.md`](docs/agent-principles.md) |
| Repository layout, boundaries, data flow | [`docs/architecture.md`](docs/architecture.md) |
| Local development loop and commands | [`docs/dev-loop.md`](docs/dev-loop.md) |
| Verification matrix and handoff checks | [`docs/verification.md`](docs/verification.md) |
| Config file schema and replacement rules | [`docs/configuration.md`](docs/configuration.md) |
| Sync operations, safety, generated files | [`docs/operations.md`](docs/operations.md) |
| Current quality and debt ledger | [`docs/quality.md`](docs/quality.md), [`docs/technical-debt.md`](docs/technical-debt.md) |
| Multi-turn execution plans | [`docs/plans/README.md`](docs/plans/README.md) |

## Commands

```bash
go run .                 # run dsync from the current directory
just start               # same local run path as go run .
go test ./...            # unit test suite
go test -run TestName    # focused test while iterating
go build -o ./dist/dsync .
just build               # build ./dist/dsync and chmod it executable
just check               # fmt, vet, tests, temp compile check
just release             # checked dev release archives into releases/dev/
just release --bump patch|minor|major  # stable Go-module release tag flow
go test -bench=Benchmark -benchmem ./...
DSYNC_INTEGRATION=1 go test -run TestWordPressFixtureImportsIntoMariaDB -count=1
just tag-push            # tag from ./version and force-update latest
```

Run `just check` before handing off code changes. Run `just integration-test` when DB streaming/serialization import behaviour changes and Docker is available. Run manual rsync, ssh, Docker, or database checks only when the task touches those behaviours and a safe target is available.

## Non-negotiable invariants

- Prefer clear names and direct control flow. Eschew obfuscation; elucidate intent.
- Repository-local docs are the system of record; if feedback matters later, capture it in `docs/` or a test.
- Task-first workflow is mandatory: when a `just` task exists, run that task instead of manually running its underlying commands.
- Treat `dsync-config.json` and all real configs as local secrets. `.gitignore` excludes JSON and `configs/**`.
- Keep CLI wiring in `root.go`; keep config parsing in `config.go`; keep file sync in `sync.go`; keep DB sync and replacement logic in `db.go` until a deliberate refactor moves it.
- Forward DB sync is remote dump -> replacements in listed order -> local import.
- Reverse DB sync is local dump -> reversed replacements in reverse order -> remote backup -> remote import.
- Never remove the remote backup step before reverse remote writes.
- `DBProvider` exists so DB orchestration can be tested without real ssh, Docker, or MySQL.
- Preserve context-aware external commands and return stderr/stdout in errors where useful.
- `SyncPath` endpoints are directory semantics; `ensureTrailingSlash` is part of the rsync contract.
- Do not commit generated dumps, private configs, or local database directories.
- Stable releases must be done with `just release --bump ...`; do not manually run separate check, build, commit, tag, or push commands for a release.
