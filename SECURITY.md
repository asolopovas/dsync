# Security

Dsync touches remote servers, DB dumps, and private configs. Treat generated data as sensitive.

## Never commit

- `dsync-config.json`
- `configs/**`
- `db.sql`, `db_reverse.sql`
- `db/**`
- `dist/dsync`
- `releases/**`

Configs may reveal hosts, paths, DB names, and replacement targets. Dumps may contain user data.

## Destructive actions

- Reverse sync writes local state to remote.
- Reverse DB sync must create a remote backup before import.
- Do not probe remote systems unless the target is task-relevant and safe.
- Keep command errors verbose enough to diagnose access and dependency failures.

## Command safety

- Preserve context-aware external commands.
- Keep shell construction explicit and testable.
- Names and output must include direction and target for remote writes.
