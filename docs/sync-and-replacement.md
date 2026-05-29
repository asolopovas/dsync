# Sync and replacement

Dsync reads `dsync-config.json` by default; `-c/--config` selects another file.

## Minimal config

```json
{
  "sshHost": "user@example.com",
  "port": "22",
  "remote": { "host": "unused", "db": "remote_db" },
  "local": { "host": "unused", "db": "local_db" },
  "dbReplace": [{ "from": "https://example.com", "to": "http://example.test" }],
  "sync": [{ "remote": "/var/www/html/wp-content/uploads", "local": "./wp-content/uploads", "exclude": ["cache/", "*.log"] }]
}
```

`remote.host` and `local.host` remain in structs but current commands do not use them.

## Replacement order

- Forward DB sync: `from -> to` in listed order.
- Reverse DB sync: `to -> from` in reverse list order.
- Use clean URL/path values; engines also handle slash-escaped variants such as `\/`.

## Engines

| Engine | Selected when | Behavior |
| --- | --- | --- |
| `none` | no replacements | stream unchanged |
| `go-serialized` | replacements + WordPress-looking sync paths | parse INSERT rows, skip `guid`, repair PHP serialized strings, validate by default |
| `raw` | replacements + non-WordPress paths | whole-statement string replacement |

Legacy configs may override `dbReplaceEngine`, `validateSerialized`, or `skipColumns`; new configs should not.

## Limits

- Column skipping needs complete INSERT column names; Dsync dumps include them.
- DB credentials and service names are code conventions, not config fields.
- Invalid pre-existing serialized values pass through unchanged; Dsync avoids raw-editing data it cannot repair.
- Validation failures report table/row/column when a parsed transformed value becomes invalid. Disabling validation risks corrupted serialized data.
