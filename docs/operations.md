# Operations

Dsync can overwrite files and databases. Treat each run as a deployment action.

## Forward sync: remote to local

Sync files and DB:

```bash
dsync -c ./configs/site.json -a
```

Files only:

```bash
dsync -c ./configs/site.json -f
```

DB only:

```bash
dsync -c ./configs/site.json -d
```

Forward DB flow copies the remote database into the local database after replacements.

## Reverse sync: local to remote

```bash
dsync -c ./configs/site.json -a -r
dsync -c ./configs/site.json -d -r
```

Reverse DB flow writes to the remote database. It first creates a timestamped remote backup with `mysqldump -uroot <db> > <db>_backup_<timestamp>.sql`.

Before reverse sync, confirm:

- `sshHost` points to the intended server.
- The target database is disposable or backed up.
- Replacement rules reverse cleanly.
- File paths do not point at production-only uploads or user data by mistake.

## Dump files

Forward DB dump:

```bash
dsync -c ./configs/site.json -d --dump
```

Writes `db.sql` after replacements.

Reverse DB dump:

```bash
dsync -c ./configs/site.json -d -r --dump
```

Writes `db_reverse.sql` after reversed replacements.

Do not commit dump files. They may contain private data.

## Generated and local artefacts

| Path | Meaning | Commit? |
| --- | --- | --- |
| `dist/dsync` | Built CLI binary. | No. |
| `db.sql` | Forward dump output. | No. |
| `db_reverse.sql` | Reverse dump output. | No. |
| `dsync-config.json` | Local config, often secret. | No. |
| `configs/**` | Local config directory. | No. |
| `db/**` | Local database data. | No. |

## Failure handling

- Read the returned command output first; command errors include stderr/stdout where the current code captures it.
- For file sync failures, verify ssh access, rsync installation, port, path existence, and excludes.
- For local DB failures, verify Docker is running, the compose file path is right, and the service name is `mariadb`.
- For remote DB failures, verify `mysqldump`/`mysql` availability and root access on the remote host.
