# AGENTS.md

Stack: Go 1.23 · Cobra · pterm · rsync/ssh · Docker Compose + MariaDB/MySQL.

This file is a map. Open only the docs needed for the task.

## Source map

| Need | Read |
| --- | --- |
| Architecture and boundaries | [`ARCHITECTURE.md`](ARCHITECTURE.md) |
| Agent loop, taste, cleanup | [`docs/core-beliefs.md`](docs/core-beliefs.md) |
| Product behavior | [`PRODUCT_SENSE.md`](PRODUCT_SENSE.md), [`docs/cli-sync-workflows.md`](docs/cli-sync-workflows.md) |
| Config and DB replacement | [`docs/sync-and-replacement.md`](docs/sync-and-replacement.md), [`docs/config-reference-llms.txt`](docs/config-reference-llms.txt) |
| Commands, checks, E2E smoke | [`RELIABILITY.md`](RELIABILITY.md), [`docs/local-commands-llms.txt`](docs/local-commands-llms.txt) |
| Secrets and destructive actions | [`SECURITY.md`](SECURITY.md) |
| Quality and debt | [`QUALITY_SCORE.md`](QUALITY_SCORE.md), [`docs/tech-debt-tracker.md`](docs/tech-debt-tracker.md) |
| Multi-turn work | [`PLANS.md`](PLANS.md) |

## Commands

```bash
just run -- --help
just check
just check test-race
just check integration-test
just install
just release
just release --bump patch|minor|major
```

Quick private E2E smoke: `just install`, then run `dsync -a` inside a site directory that owns a private `dsync-config.json` (see [`RELIABILITY.md`](RELIABILITY.md)).

## Invariants

- Keep behavior explicit; prefer clear names over clever helpers.
- Treat `dsync-config.json`, `configs/**`, dumps, and local DB data as secrets.
- Keep CLI wiring in `root.go`, config in `config.go`, file sync in `sync.go`, DB orchestration in `db.go`, dump transformation in `db_transform.go` unless a plan says otherwise.
- Forward DB: remote dump -> replacements in listed order -> local import.
- Reverse DB: local dump -> reversed replacements in reverse order -> remote backup -> remote import.
- Never remove the reverse-sync remote backup step.
- Preserve `DBProvider`; it is the DB orchestration test seam.
- Preserve context-aware external commands and useful stderr/stdout in errors.
- `SyncPath` endpoints are directories; `ensureTrailingSlash` is part of the rsync contract.
- If a mistake should not repeat, encode it in docs, tests, commands, or structure.
