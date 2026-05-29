# CLI sync workflows

## Setup

- `dsync --gen` creates `dsync-config.json`.
- `-c/--config` selects a config.
- Real configs stay ignored/private.

## Commands

| Command | Behavior |
| --- | --- |
| `dsync -a` | files + DB, remote -> local |
| `dsync -f` | files only, remote -> local |
| `dsync -d` | DB only, remote -> local |
| `dsync -d --dump` | DB import + transformed `db.sql` |
| `dsync -a -r` | files + DB, local -> remote |
| `dsync -d -r` | DB local -> remote after backup |
| `dsync -d -r --dump` | remote DB import + transformed `db_reverse.sql` |

## Acceptance

- Direction is visible before work starts.
- Reverse DB sync backs up remote before import.
- Replacement order follows [`sync-and-replacement.md`](sync-and-replacement.md).
- Errors include enough command output to diagnose tools or access.
