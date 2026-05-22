# AGENTS.md

Stack: Go 1.23 · Cobra CLI · pterm terminal UI · rsync/ssh · Docker Compose + MariaDB/MySQL.

This file is a map, not a manual. Repository-local docs are the system of record; keep them compact and update them with behavior changes.

## Start here

| Need | Source of truth |
| --- | --- |
| Architecture and code boundaries | [`ARCHITECTURE.md`](ARCHITECTURE.md) |
| Agent-first beliefs and taste | [`docs/design-docs/core-beliefs.md`](docs/design-docs/core-beliefs.md) |
| Product/workflow intent | [`PRODUCT_SENSE.md`](PRODUCT_SENSE.md), [`docs/product-specs/index.md`](docs/product-specs/index.md) |
| Config and replacement design | [`docs/design-docs/sync-and-replacement.md`](docs/design-docs/sync-and-replacement.md), [`docs/references/config-reference-llms.txt`](docs/references/config-reference-llms.txt) |
| Verification and operations | [`RELIABILITY.md`](RELIABILITY.md) |
| Secrets and destructive actions | [`SECURITY.md`](SECURITY.md) |
| Quality and debt | [`QUALITY_SCORE.md`](QUALITY_SCORE.md), [`docs/exec-plans/tech-debt-tracker.md`](docs/exec-plans/tech-debt-tracker.md) |
| Multi-turn plans | [`PLANS.md`](PLANS.md) |

## Commands

```bash
just run --help          # run CLI from source with arbitrary args
just check               # fmt, vet, tests, temp compile check
just check test-race     # selected check job
just check integration-test  # Docker-backed DB import test
just build               # build ./dist/dsync
just release             # checked dev archives into releases/dev/
just release --bump patch|minor|major  # stable release flow
```

Use `just` tasks for project workflows; `.vscode/tasks.json` mirrors the common tasks for editor users. Run `just check` before handoff for code changes. Run `just integration-test` when DB import/serialization behavior changes and Docker is available.

## Non-negotiable invariants

- Eschew obfuscation; elucidate intent.
- Treat `dsync-config.json`, `configs/**`, dumps, and local DB data as secrets.
- Keep CLI wiring in `root.go`, config loading in `config.go`, file sync in `sync.go`, DB orchestration in `db.go`, and dump transformation in `db_transform.go` unless a plan says otherwise.
- Forward DB sync: remote dump -> replacements in listed order -> local import.
- Reverse DB sync: local dump -> reversed replacements in reverse order -> remote backup -> remote import.
- Never remove the reverse-sync remote backup step.
- Preserve `DBProvider`; it is the test seam for DB orchestration.
- Preserve context-aware external commands and useful stderr/stdout in errors.
- `SyncPath` endpoints are directories; `ensureTrailingSlash` is part of the rsync contract.
- If a mistake should not repeat, encode it in docs, tests, commands, or structure.
