# ARCHITECTURE.md

Dsync is a single-package Go CLI. It syncs directory trees with `rsync` and streams MySQL/MariaDB dumps through replacement engines.

## Source map

| Path | Responsibility |
| --- | --- |
| `main.go` | Process entry; calls `Execute()`. |
| `root.go` | Cobra commands, flags, version, config loading, operation dispatch, fish completion. |
| `config.go` | JSON config structs, loading, default config generation. |
| `sync.go` | File sync presentation and `rsync` invocation. |
| `db.go` | DB orchestration, command provider, dump/import streaming, local DB creation. |
| `db_transform.go` | SQL stream transformation, replacements, INSERT parsing, PHP serialization-safe rewriting. |
| `*_test.go` | Unit and integration coverage for orchestration, replacement, paths, fixtures. |
| `justfile` | Development, verification, benchmark, build, and release tasks. |

## External dependencies

- `ssh` reaches `sshHost` using `port`.
- `rsync` must exist locally and remotely for file sync.
- Remote DB commands assume `mysqldump`/`mysql` root access without an interactive password prompt.
- Local DB commands use `docker compose exec -T mariadb ...`.
- Default local compose file: `$HOME/www/dev/docker-compose.yml`; override with `DSYNC_COMPOSE_FILE`.
- Local MariaDB root password convention: `secret`.

## Data flows

### Files

Forward: `sshHost:remote/ -> local/`. Reverse: `local/ -> sshHost:remote/`.

`SyncFiles` normalizes both endpoints with trailing slashes, builds `rsync -azr -e "ssh -p <port>" --info=progress2`, and appends each `--exclude`.

### Database forward: remote to local

1. `DumpRemote` streams remote `mysqldump -uroot` over ssh.
2. The configured replacement engine transforms the stream while CLI progress reports bytes read from the dump and bytes sent to import.
3. `WriteLocal` creates local DB/user if needed, then imports via Docker Compose.
4. `--dump` tees the transformed stream to `db.sql` while importing.

### Database reverse: local to remote

1. `DumpLocal` streams local `mariadb-dump` when available, else `mysqldump`.
2. Replacements are inverted and applied in reverse list order.
3. `BackupRemote` writes `<db>_backup_<timestamp>.sql` before any remote write.
4. `WriteRemote` imports the transformed stream over ssh.
5. `--dump` tees the transformed stream to `db_reverse.sql`.

## Boundaries

- Keep flags thin; put behavior in testable functions.
- `DBProvider` separates orchestration from real shell commands.
- Keep command construction explicit. Hidden shell pipelines are harder to inspect.
- Replacement engines are selected by config: `go-serialized`, `raw`, or `none`; there is no fallback.
- `go-serialized` repairs PHP serialized string lengths and preserves PHP `r`/`R` references. Invalid or unsupported serialized values are left unchanged unless `validateSerialized` is enabled.
- Column-aware replacement can skip `guid` only when dump statements include column names. Dsync uses `--complete-insert` for generated dumps.
- Generated dumps also use `--skip-extended-insert` so the transformer emits row-sized statements instead of holding very large multi-row INSERTs before import progress appears.
