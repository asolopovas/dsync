# Sync and replacement

Dsync reads `dsync-config.json` by default; `-c/--config` selects another file.

## Environment settings

Config loads migrate literal endpoint and replacement values to `.env` beside the
config, using `DSYNC_` variables and `${VARIABLE}` JSON references. Existing `.env`
keys are never overwritten. A literal that differs from an existing `.env` value
for the same key is an error and changes neither file, so two configs sharing a
directory, or rows reordered after migration, cannot silently inherit another
endpoint. Unrelated entries, comments, unknown config fields, excludes, and
replacement flags are preserved. Values are always written quoted, so
numeric-looking strings such as `0123` keep their exact text. Exported
variables provide a fallback for references absent from `.env`. Expansion happens
on decoded string values so quotes and backslashes cannot corrupt JSON.

`dsync --check-config` performs migration and validates both files and DB settings
without starting a sync. Missing references fail before writing either file.
If the default JSON is absent, dsync loads `.env` directly. Migration saves a
`DSYNC_CONFIG` template for retaining non-endpoint options in env-only mode;
older `.env` files with individual `DSYNC_` endpoint variables also work.
`--all` skips file sync when no file paths are configured.
Keep `.env` out of Git; see the variable naming examples in [`../readme.md`](../readme.md).

## Minimal config

```json
{
  "sshHost": "user@example.com",
  "port": "22",
  "remote": { "host": "unused", "db": "remote_db" },
  "local": { "host": "unused", "db": "local_db" },
  "dbReplace": [{ "from": "https://example.com", "to": "http://example.test" }],
  "sync": [{ "remote": "/var/www/html/wp-content/uploads", "local": "./wp-content/uploads", "exclude": ["cache/", "*.log"], "replace": true }]
}
```

`remote.host` and `local.host` remain in structs but current commands do not use them.

## Remote database access

By default remote dumps, imports and reverse-sync backups run `mysqldump -uroot`
and `mysql -uroot`, which needs a root SSH login. Set `remote.wpPath` to the
remote WordPress root (`DSYNC_REMOTE_WPPATH` in `.env`) to run them through
`wp db export` and `wp db import` instead. WP-CLI reads the credentials from
`wp-config.php`, so an unprivileged per-site SSH user is enough. The reverse-sync
backup script runs under `sh`, independent of the remote login shell.

## Skipping sync paths

`--skip uploads` (or `DSYNC_SKIP=uploads`) omits `sync[]` entries whose local or
remote directory equals the value or ends with it as a whole path suffix, so
`uploads` matches `wp-content/uploads` but not `wp-content/my-uploads`. It only
affects file sync; WordPress detection and cache flushing use the full list.

## Replacement order

- Forward DB sync: `from -> to` in listed order.
- Reverse DB sync: `to -> from` in reverse list order.
- Forward file sync: when `sync[].replace` is true, Dsync applies `from -> to` to destination text files after rsync. The traversal honors rsync exclusions relative to each sync root, prunes excluded directories, and skips symlinks and nonregular files. Changes are atomic and preserve file permissions and modification times. Use this for generated WordPress CSS/JS that contains absolute remote URLs.
- Use clean URL/path values; engines also handle slash-escaped variants such as `\/`.

## WordPress object cache

For a forward database sync, Dsync derives local WordPress roots from `sync[].local` paths containing a `wp-content` directory. Duplicate roots are processed once. Before the remote dump starts, Dsync verifies that every root exists with `wp-load.php` and that WP-CLI is available.

After a successful local import, Dsync runs `wp --path=<root> --skip-plugins --skip-themes --quiet cache flush` for each root. This invalidates the site's WordPress object cache without flushing Redis globally. A failure is fatal and reports that the database was already imported. Reverse syncs, file-only syncs, and configurations without local WordPress paths do not run this stage.

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
- WP-CLI is a runtime dependency only for forward WordPress database syncs; there are no cache-related flags or config fields.
- A value counts as PHP serialized only when it starts with a complete type header such as `a:2:{`, `s:5:"` or `i:1;`; prose like `a: note` gets plain replacement. Surrounding whitespace is preserved.
- Invalid pre-existing serialized values pass through unchanged; Dsync avoids raw-editing data it cannot repair.
- Validation failures report table/row/column when a parsed transformed value becomes invalid. Disabling validation risks corrupted serialized data.

- PHP declared sizes must be nonnegative, fit an integer, and fit remaining input;
  array/object allocations grow from parsed data. Parsing and transformation use
  a depth limit of 20. Invalid nested serialized strings also remain unchanged.
- Dump parsing retains statement-based streaming: peak memory depends on the
  largest statement, including extended inserts. Custom client `DELIMITER`
  directives and stored-routine dumps are outside the supported format.
