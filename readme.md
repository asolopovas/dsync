# Dsync

Go CLI for syncing web project files and MySQL/MariaDB databases between remote and local environments.

- Files: `rsync` over ssh.
- DB: streamed dump/import with ordered replacements.
- WordPress: serialized PHP strings are length-repaired; `guid` is skipped by default.
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
```

Reverse DB sync backs up remote before import.

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
