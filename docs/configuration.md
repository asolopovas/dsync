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
| `dbReplace` | Ordered raw string replacements applied to SQL dumps. |
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

The replacement engine also tries slash-escaped URL variants such as `\/`, `\\/`, and deeper escaped forms. Prefer clean config values like `https://example.com`; do not pre-escape unless a test proves you need it.

## Current limits

- Replacement is raw string replacement, not PHP-serialization aware.
- WordPress serialized option values can be corrupted when replacement lengths change.
- `guid` columns are not skipped.
- DB credentials and service names are currently conventions in code, not config fields.

See [`plans/README.md`](plans/README.md) before changing replacement engines.
