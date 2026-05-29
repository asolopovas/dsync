# Sync and replacement design

## Config shape

Dsync reads `dsync-config.json` by default; `-c/--config` selects another file.

```json
{
  "sshHost": "user@example.com",
  "port": "22",
  "remote": { "host": "remote-db-host", "db": "remote_db" },
  "local": { "host": "localhost", "db": "local_db" },
  "dbReplace": [{ "from": "https://example.com", "to": "http://example.test" }],
  "sync": [{ "remote": "/var/www/html/wp-content/uploads", "local": "./wp-content/uploads", "exclude": ["cache/", "*.log"] }]
}
```

`remote.host` and `local.host` exist in structs but are not used in command construction.

## Replacement order

Forward DB sync applies `dbReplace` in listed order: `from -> to`.

Reverse DB sync applies inverted rules in reverse list order: `to -> from`.

Order matters when outputs overlap later inputs. Use clean URL values such as `https://example.com`; engines also handle slash-escaped variants like `\/`.

## Engines

Dsync derives replacement behavior from the minimal config:

| Engine | Behavior | Selected when |
| --- | --- | --- |
| `go-serialized` | Streams SQL, parses INSERT rows, skips `guid` plus any legacy `skipColumns`, rewrites PHP serialized strings with corrected byte lengths, and validates transformed serialized values by default. | `dbReplace` has rules and sync paths look WordPress-like. |
| `raw` | Whole-statement string replacement without SQL/serialization awareness. | `dbReplace` has rules and paths do not look WordPress-like. |
| `none` | Streams unchanged. | `dbReplace` is empty. |

Legacy configs may still override `dbReplaceEngine`, `validateSerialized`, or `skipColumns`, but generated and documented configs omit them.

## Limits

- Column skipping requires complete-insert column names; Dsync-generated dumps include them.
- DB credentials and service names are conventions in code, not config fields.
- Validation fails sync when transformed serialized values are invalid; legacy configs can set `validateSerialized` false to disable it.
