# Sync and replacement design

## Config shape

Dsync reads `dsync-config.json` by default; `-c/--config` selects another file.

```json
{
  "sshHost": "user@example.com",
  "port": "22",
  "remote": { "host": "remote-db-host", "db": "remote_db" },
  "local": { "host": "localhost", "db": "local_db" },
  "dbReplaceEngine": "go-serialized",
  "validateSerialized": true,
  "skipColumns": ["guid"],
  "dbReplace": [{ "from": "https://example.com", "to": "http://example.test" }],
  "sync": [{ "remote": "/var/www/html/uploads", "local": "./uploads", "exclude": ["cache/", "*.log"] }]
}
```

`remote.host` and `local.host` exist in structs but are not used in command construction.

## Replacement order

Forward DB sync applies `dbReplace` in listed order: `from -> to`.

Reverse DB sync applies inverted rules in reverse list order: `to -> from`.

Order matters when outputs overlap later inputs. Use clean URL values such as `https://example.com`; engines also handle slash-escaped variants like `\/`.

## Engines

| Engine | Behavior | Use when |
| --- | --- | --- |
| `go-serialized` | Streams SQL, parses INSERT rows, skips `guid`/`skipColumns`, rewrites PHP serialized strings with corrected byte lengths. | Default for WordPress-style data. |
| `raw` | Whole-statement string replacement without SQL/serialization awareness. | Simple dumps where length changes cannot corrupt serialized data. |
| `none` | Streams unchanged. | Dump/import only. |

There is no fallback. Dsync uses exactly the selected engine. Empty configs default to `go-serialized` for WordPress-looking sync paths and `raw` otherwise.

## Limits

- Column skipping requires complete-insert column names; Dsync-generated dumps include them.
- DB credentials and service names are conventions in code, not config fields.
- Validation fails sync when transformed serialized values are invalid and `validateSerialized` is true.
