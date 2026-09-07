# Architecture

Dsync is a single-package Go CLI. It syncs directories with `rsync` and streams MySQL/MariaDB dumps through replacement engines.

## Code map

| Path | Owns |
| --- | --- |
| `main.go` | process entry |
| `root.go` | Cobra flags, config load, dispatch, completion |
| `config.go` | JSON config and generation |
| `sync.go` | file sync UI and `rsync` invocation |
| `db.go` | DB orchestration, providers, dump/import streaming |
| `db_transform.go` | SQL parsing, replacements, PHP serialization repair |
| `*_test.go` | unit, orchestration, fixture, integration coverage |
| `justfile` | local/CI/release workflow |

## Runtime dependencies

- Local: `ssh`, `rsync`, Go, Docker Compose with `mariadb` service for DB import.
- Local WordPress forward DB syncs: WP-CLI (`wp`) and a valid WordPress root derived from each `sync[].local` path containing `wp-content`.
- Remote: `rsync`, `mysqldump`, `mysql`, root DB access without interactive prompts.
- Compose file: `$HOME/www/dev/docker-compose.yml`, override with `DSYNC_COMPOSE_FILE`.
- Local MariaDB root password convention: `secret`.

## Data flows

Files:

- Forward: `sshHost:remote/ -> local/`.
- Reverse: `local/ -> sshHost:remote/`.
- `SyncFiles` enforces directory trailing slashes and passes excludes to `rsync -azr`.

DB forward:

1. WordPress-looking local sync paths are resolved to deduplicated site roots, then each root and WP-CLI are checked before the database is replaced.
2. Remote `mysqldump -uroot` streams over ssh.
3. Transformer applies configured replacements while progress counts read/sent bytes.
4. Local Docker MariaDB creates DB/user if needed and imports.
5. For WordPress sites, WP-CLI flushes each site's object cache after a successful import with plugins and themes skipped.
6. `--dump` tees transformed SQL to `db.sql`.

DB reverse:

1. Local dump streams from `mariadb-dump` or `mysqldump`.
2. Replacement list is inverted and applied in reverse order.
3. Remote backup `<db>_backup_<timestamp>.sql` is created before import.
4. Remote `mysql` imports over ssh.
5. `--dump` tees transformed SQL to `db_reverse.sql`.

## Boundaries

- Flags stay thin; behavior lives in testable functions.
- `DBProvider` separates orchestration from shell commands, including WordPress cache preflight and invalidation.
- Command construction stays explicit and inspectable.
- Minimal config selects engines: no replacements -> `none`; WordPress-looking paths -> `go-serialized`; otherwise `raw`.
- `go-serialized` repairs PHP serialized lengths, preserves `r`/`R` references, validates by default, and skips `guid` when column names exist.
- Dsync dumps use `--complete-insert` and `--extended-insert` so replacements remain column-aware while database imports avoid one statement per row.
- Cache invalidation is local and site-scoped; Dsync never flushes Redis globally.
