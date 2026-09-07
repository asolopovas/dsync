# CLI sync workflows

## Setup

- `dsync --gen` exclusively creates `dsync-config.json` with mode `0600`; an existing file or symlink is an error.
- `-c/--config` selects a config.
- Real configs stay ignored/private.

## Commands

| Command | Behavior |
| --- | --- |
| `dsync -a` | files + DB, remote -> local; flush local WordPress object caches after import |
| `dsync -f` | files only, remote -> local |
| `dsync -d` | DB only, remote -> local; flush local WordPress object caches after import |
| `dsync -d --dump` | DB import + transformed `db.sql`, then local WordPress object-cache flush |
| `dsync -a -r` | files + DB, local -> remote |
| `dsync -d -r` | DB local -> remote after backup |
| `dsync -d -r --dump` | remote DB import + transformed `db_reverse.sql` |

## Acceptance

- Direction is visible before work starts.
- Forward WordPress DB sync preflights every deduplicated local site root and WP-CLI before starting the remote dump.
- Forward WordPress DB sync finishes by running `wp --path=<root> --skip-plugins --skip-themes --quiet cache flush` once per detected site.
- Cache invalidation is skipped for file-only, reverse, and non-WordPress syncs.
- Reverse DB sync backs up remote before import.
- Replacement order follows [`sync-and-replacement.md`](sync-and-replacement.md).
- Errors include enough command output to diagnose tools or access.
- A cache-flush failure exits nonzero and makes clear that the local database import already succeeded.

## Validation and failure behavior

SSH host and numeric port are checked before sync. File operations require
nonempty directory endpoints; DB operations require both database names.
Unknown JSON fields remain accepted, and combined sync flags still work.
Unknown replacement engines and empty effective replacement sources are errors.
Positional arguments are rejected; `--dump` requires `--db` or `--all`.

Rsync failure stops replacements, remaining paths, and DB dispatch. Interrupts
cancel the active operation. Local dump files have mode `0600`; existing regular
dumps are replaced, and symlink/nonregular destinations are rejected. Reverse
backups use private, random `dsync-backup-<random>.sql` names in the remote working
directory, and backup failure prevents import.
