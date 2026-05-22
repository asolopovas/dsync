# Architecture

Dsync is a single-package Go CLI. It syncs directory trees with `rsync` and streams MySQL/MariaDB dumps through shell commands.

## Source layout

| Path | Responsibility |
| --- | --- |
| `main.go` | Process entry point; calls `Execute()`. |
| `root.go` | Cobra command tree, flags, version output, config loading, operation dispatch, fish completion generation. |
| `config.go` | JSON config structs, config loading, default config generation. |
| `sync.go` | File sync presentation and `rsync` invocation. |
| `db.go` | DB sync orchestration, real DB command provider, dump/import streaming, local DB creation. |
| `db_transform.go` | SQL dump transformation, raw replacements, column-aware INSERT parsing, PHP serialization-safe replacement. |
| `*_test.go` | Unit coverage for replacement behaviour, DB orchestration order, and path helpers. |
| `version` | Embedded CLI version string. |
| `Makefile` | Local build, install, and release tag helpers. |

## Runtime dependencies

- `ssh` reaches the remote host from `sshHost` and `port`.
- `rsync` must exist locally and remotely for file sync.
- Remote DB commands assume `mysqldump` and `mysql` with root access and no password prompt.
- Local DB commands use `docker compose exec -T mariadb ...`.
- Local MariaDB root password is currently `secret`.
- Local compose file defaults to `$HOME/www/dev/docker-compose.yml`; override with `DSYNC_COMPOSE_FILE`.

## Data flow

### Files, remote to local

`SyncFiles` normalizes both endpoints with trailing slashes, builds `rsync -azr -e "ssh -p <port>" --info=progress2`, adds each `--exclude`, and copies `sshHost:remote/` to `local/`.

### Files, local to remote

The same path and exclude rules apply, but source and destination are reversed.

### Database, remote to local

1. `DumpRemote` starts remote `mysqldump -uroot` over ssh and returns stdout as a stream.
2. The stream passes through the configured replacement engine.
3. `WriteLocal` creates the local database/user if needed, then imports the transformed stream through Docker Compose.
4. `--dump` tees the transformed stream to `db.sql` while importing.

### Database, local to remote

1. `DumpLocal` streams a local `mariadb-dump` when available, otherwise `mysqldump`, from the Docker Compose `mariadb` service.
2. Replacements are inverted and applied in reverse list order.
3. `BackupRemote` writes a timestamped SQL backup on the remote host before any remote write.
4. `WriteRemote` imports the transformed stream through ssh.
5. `--dump` tees the transformed stream to `db_reverse.sql` while importing.

## Boundaries

- CLI flags should stay thin. Put behaviour in functions that tests can call.
- `DBProvider` is the seam between DB orchestration and real shell commands.
- External command construction must remain explicit. Hidden shell pipelines make errors harder to inspect.
- Replacement engines are selected by config. `raw` preserves the old whole-dump string replacement behaviour. `go-serialized` parses INSERT statements, skips configured columns such as `guid`, and rewrites PHP serialized strings with corrected byte lengths.
