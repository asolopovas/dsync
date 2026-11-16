# Dsync

Go CLI for syncing web project files and MySQL/MariaDB databases between remote and local environments.

- Files: `rsync` over ssh.
- DB: streamed dump/import with ordered replacements.
- WordPress: serialized PHP strings are length-repaired, `guid` is skipped by default, and forward DB syncs finish with a site-scoped object-cache flush.
- Reverse sync: local -> remote, with mandatory remote DB backup.

## Install

```bash
go install github.com/asolopovas/dsync@latest
```

From this repo:

```bash
just install          # dev build to current dsync path
just install --stable # latest semver tag
```

## Config

Generate a starter config:

```bash
dsync --gen
```

Minimal shape:

```json
{
  "sshHost": "user@example.com",
  "port": "22",
  "remote": { "host": "unused", "db": "remote_db" },
  "local": { "host": "unused", "db": "local_db" },
  "dbReplace": [{ "from": "https://example.com", "to": "http://example.test" }],
  "sync": [{ "remote": "/var/www/html/wp-content/uploads", "local": "./wp-content/uploads", "exclude": ["cache/"] }]
}
```

When the default `dsync-config.json` is absent, dsync loads settings directly
from the project `.env`. An explicitly selected missing custom config remains an
error. `dsync -a` syncs the database when no file paths are configured.

On the first sync, dsync moves literal connection settings, sync directories, and
replacement values into `.env` beside the selected config. It rewrites the JSON
with `${DSYNC_...}` references and adds only missing variables, preserving existing
`.env` values and unrelated content. A literal that conflicts with an existing
`.env` value is an error, not a silent substitution. Later runs resolve these references from
`.env`; exported environment variables are a fallback for missing keys. No shell
commands are evaluated. Missing references fail before syncing.

Run migration and validation without syncing:

```bash
dsync --check-config
```

Variables include `DSYNC_SSH_HOST`, `DSYNC_PORT`, `DSYNC_REMOTE_DB`,
`DSYNC_LOCAL_DB`, `DSYNC_SYNC_0_REMOTE`, `DSYNC_SYNC_0_LOCAL`,
`DSYNC_DBREPLACE_0_FROM`, and `DSYNC_DBREPLACE_0_TO`. Array indexes start at zero.
Migration also adds `DSYNC_CONFIG`, a JSON template retaining exclusions,
replacement flags, and optional settings for use after removing the JSON file.
Existing `DSYNC_CONFIG` values are preserved; edit this template in `.env` when
changing those options in env-only mode.
You can use existing project variables explicitly, for example
`"local": { "db": "${DB_DATABASE}" }`.

Keep `.env` ignored by Git. Migration writes changed files atomically with mode
`0600` and refuses to replace symlinks. Configs containing only variable references
can be versioned after reviewing their remaining options for private values.
Previously committed values remain in Git history.

Keep real configs private. See [`docs/sync-and-replacement.md`](docs/sync-and-replacement.md).

## Use

```bash
dsync -a                 # files + DB, remote -> local
dsync -f                 # files only
dsync -d                 # DB only
dsync -a -r              # files + DB, local -> remote
dsync -d --dump          # import local DB and write db.sql
dsync -d -r --dump       # import remote DB and write db_reverse.sql
dsync -c configs/site.json -a
dsync -a --skip uploads  # leave out sync paths ending in "uploads"
```

`--skip` takes directory names or path suffixes (repeatable or comma-separated)
and omits matching `sync[]` entries from file sync in either direction; a value
matching nothing is an error. `DSYNC_SKIP` sets a machine-wide default, for
example on a small server that should never download media, and ignores values
that match nothing. The flag replaces the variable. Database sync, replacement
engine selection and the WordPress cache flush still use every configured path.

Reverse DB sync backs up remote before import.

Forward WordPress DB sync requires WP-CLI on `PATH`. Dsync discovers local WordPress roots from configured `sync[].local` `wp-content` paths, checks them before import, and flushes each site's object cache afterward. Cache failure is fatal but does not roll back the completed database import.

## Develop

```bash
just setup
just run -- --help
just check
just check integration-test
just install
```

Private E2E smoke:

```bash
just install
cd /home/andrius/www/avianese.test/wp-content/themes/avianese-theme
dsync -a
```

## Release

```bash
just release
just release --bump patch|minor|major
```

## License

MIT

## Supported toolchain

Build with Go 1.27.0 or newer (verified with Go 1.27.1). macOS binaries require
macOS 13 or newer, following the [Go 1.27 platform requirements](https://go.dev/doc/go1.27).
