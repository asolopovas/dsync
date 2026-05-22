# RELIABILITY

Use `just` tasks for repository workflows. State what ran in handoff notes.

## Development loop

```bash
just setup
just start
just run -- --help
just test
just test-one TestName
just vet
just check
just bench
just integration-test
```

`just check` runs gofmt, vet, tests, and a temp compile check without dirtying `dist/`.

## Focused checks

| Area touched | Check |
| --- | --- |
| Replacement engine | `just test-one TestTransformSQLDump` |
| DB orchestration | `just test-one TestSyncDB` |
| DB replacements | `just test-one TestApplyDBReplacements` |
| Rsync paths | `just test-one TestEnsureTrailingSlash` |
| Full code handoff | `just check` |
| Docker DB import | `just integration-test` when Docker is available and relevant |

## Operations

Forward sync:

```bash
dsync -c ./configs/site.json -a   # files + DB, remote -> local
dsync -c ./configs/site.json -f   # files only
dsync -c ./configs/site.json -d   # DB only
```

Reverse sync:

```bash
dsync -c ./configs/site.json -a -r
dsync -c ./configs/site.json -d -r
```

Dump transformed DB stream:

```bash
dsync -c ./configs/site.json -d --dump      # writes db.sql
dsync -c ./configs/site.json -d -r --dump   # writes db_reverse.sql
```

Before reverse sync, confirm target host, database, replacement reversibility, and file paths.

## Failure handling

- File sync: verify ssh, rsync on both hosts, port, path existence, excludes.
- Local DB: verify Docker, compose file, `mariadb` service, root password convention.
- Remote DB: verify `mysqldump`/`mysql` and root access on remote host.
- Serialized replacement: inspect reported table/row/column; use `raw` only when corruption risk is acceptable.

## Release reliability

Use the task-owned flow only:

```bash
just release
just release --bump patch|minor|major
just release --dry-run --bump patch
```

Do not manually split stable release steps into separate check/build/commit/tag/push commands.
