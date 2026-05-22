# CLI sync workflows

## Setup

- `dsync --gen` creates a starter `dsync-config.json`.
- `-c/--config` selects a config file.
- Real configs should live in ignored paths such as `configs/`.

## Forward workflows

| Command | Expected behavior |
| --- | --- |
| `dsync -a` | Sync files and DB from remote to local. |
| `dsync -f` | Sync files from remote to local. |
| `dsync -d` | Sync DB from remote to local. |
| `dsync -d --dump` | Import DB and write transformed `db.sql`. |

## Reverse workflows

| Command | Expected behavior |
| --- | --- |
| `dsync -a -r` | Sync files and DB from local to remote. |
| `dsync -d -r` | Sync DB from local to remote after remote backup. |
| `dsync -d -r --dump` | Import remote DB and write transformed `db_reverse.sql`. |

## Acceptance criteria

- Direction is clear before a destructive action.
- Reverse DB sync always backs up remote first.
- Replacement order matches [`../design-docs/sync-and-replacement.md`](../design-docs/sync-and-replacement.md).
- Errors include enough command output to identify missing access or tools.
