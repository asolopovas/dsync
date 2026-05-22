# Core beliefs

Harness-engineering adaptation for Dsync.

## Beliefs

- Humans steer; agents execute. The repo must contain enough intent for safe agent work.
- `AGENTS.md` is a table of contents, not an encyclopedia.
- Context is scarce. Start with maps; link to exact deeper docs.
- Repository-local, versioned docs are the system of record.
- Enforce invariants with tests, tasks, and structure. Do not rely on memory.
- Optimize for future-agent legibility: direct names, explicit data flow, boring dependencies.
- Garbage-collect debt continuously; record accepted shortcuts in [`../exec-plans/tech-debt-tracker.md`](../exec-plans/tech-debt-tracker.md).

## Operating loop

1. Read `AGENTS.md`.
2. Open only task-relevant docs.
3. Inspect code before editing.
4. Make the smallest coherent change.
5. Run the matching `just` task from [`../../RELIABILITY.md`](../../RELIABILITY.md).
6. Update docs or execution plans when behavior, workflow, or debt changes.

## Taste invariants

- Eschew obfuscation; elucidate intent.
- Use direct sync names: remote, local, dump, replacement, backup.
- Keep destructive operations obvious in names, output, and docs.
- Keep shell command construction visible and testable.
- Prefer one well-tested helper over repeated near-copies.
