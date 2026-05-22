# SECURITY

Dsync handles remote servers, database dumps, and local configs. Treat generated data as sensitive.

## Secrets and private artifacts

Never commit:

- `dsync-config.json`
- `configs/**`
- `db.sql`
- `db_reverse.sql`
- `db/**`
- `dist/dsync`
- `releases/**`

Configs can contain hosts, paths, and database names. Dumps can contain user data.

## Destructive actions

- Reverse sync writes local data to remote. Use only with an intended target.
- Reverse DB sync must create a remote backup before import.
- Do not run manual remote checks unless the target is safe and task-relevant.
- Keep command errors verbose enough to diagnose access and dependency failures.

## Command safety

- Preserve context-aware external commands.
- Keep shell construction explicit and testable.
- Avoid hiding remote writes behind helper names that do not mention direction or target.
