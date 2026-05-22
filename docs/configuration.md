# Configuration

Dsync reads JSON config from `dsync-config.json` by default. Use `-c` or `--config` to select another file.

Generate a starter file:

```bash
dsync --gen
```

## Schema

```json
{
  "sshHost": "user@example.com",
  "port": "22",
  "remote": {
    "host": "remote-db-host",
    "db": "remote_db"
  },
  "local": {
    "host": "localhost",
    "db": "local_db"
  },
  "dbReplaceEngine": "go-serialized",
  "validateSerialized": true,
  "skipColumns": ["guid"],
  "dbReplace": [
    {
      "from": "https://example.com",
      "to": "http://example.test"
    }
  ],
  "sync": [
    {
      "remote": "/var/www/html/wp-content/uploads",
      "local": "./wp-content/uploads",
      "exclude": ["cache/", "*.log"]
    }
  ]
}
```

## Fields

| Field | Meaning |
| --- | --- |
| `sshHost` | SSH target passed to `ssh` and used as the remote side of `rsync`. Usually `user@host`. |
| `port` | SSH port as a string. |
| `remote.db` | Remote database name used by `mysqldump` and `mysql`. |
| `local.db` | Local database name used inside the Docker Compose `mariadb` service. |
| `dbReplace` | Ordered string replacements applied by the selected replacement engine. |
| `dbReplaceEngine` | Replacement engine: `go-serialized`, `raw`, or `none`. Empty configs use `go-serialized` for WordPress-looking sync paths and `raw` otherwise. |
| `validateSerialized` | Re-parse transformed serialized PHP values during transformation and fail if they are invalid. |
| `skipColumns` | Columns skipped by column-aware engines. `guid` is always skipped by default. |
| `sync` | List of directory pairs to sync. |
| `sync[].exclude` | Patterns passed to rsync as `--exclude=<pattern>`. |

`remote.host` and `local.host` exist in the struct but are not used by current command construction.

## Replacement rules

Forward sync applies `dbReplace` in listed order:

```text
from -> to
```

Reverse sync applies the same rules inverted and in reverse list order:

```text
to -> from
```

This order matters when one replacement output becomes another replacement input, such as domain plus protocol changes.

All engines that replace text also try slash-escaped URL variants such as `\/`, `\\/`, and deeper escaped forms. Prefer clean config values like `https://example.com`; do not pre-escape unless a test proves you need it.

## Replacement engines

| Engine | Behaviour | Use when |
| --- | --- | --- |
| `go-serialized` | Streams SQL, parses `INSERT ... VALUES` rows, skips `guid`/`skipColumns`, rewrites PHP serialized arrays/objects/scalars with corrected string byte lengths, and passes non-INSERT SQL through unchanged. | Recommended for WordPress-style databases. |
| `raw` | Applies the historic whole-statement string replacement without SQL or serialization awareness. Dsync warns when this is explicitly used with WordPress-looking paths. | Simple non-WordPress dumps where replacement lengths cannot corrupt serialized data. |
| `none` | Streams the dump unchanged. | You only need dump/import without replacements. |

`go-serialized` still transforms strings when the dump lacks a complete-insert column list, but it can only skip `guid` when column names are present. Dsync now dumps with `--complete-insert` so normal Dsync-generated dumps include column names.

## Current limits

- Serialized validation happens while streaming transformed dump values. The Docker integration test also imports the transformed fixture and re-reads serialized values.
- Dsync uses the selected replacement engine only; there is no fallback engine.
- DB credentials and service names are currently conventions in code, not config fields.

See [`plans/README.md`](plans/README.md) before changing replacement engines.
