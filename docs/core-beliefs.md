# Core beliefs

Dsync adapts an agent harness to a small CLI: humans supply intent and judgment; agents execute inside repo-owned constraints.

## Beliefs

- The repo is the source of truth. Hidden knowledge must become docs, tests, code, schemas, or plans.
- `AGENTS.md` is a map, not a manual.
- Keep instructions short and linked.
- Enforce invariants mechanically with tests, tasks, structure, or lints.
- Make destructive direction visible in names, output, and docs.
- Record accepted shortcuts in [`tech-debt-tracker.md`](tech-debt-tracker.md).

## Agent loop

1. Read `AGENTS.md`.
2. Open task-relevant docs.
3. Inspect code before editing.
4. Plan the smallest coherent change.
5. Implement.
6. Run focused checks, then `just check` when code changed.
7. Drive the CLI/E2E path when behavior changed.
8. Update docs, plans, tests, or debt so the lesson persists.

## Taste invariants

- Eschew obfuscation; elucidate intent.
- Use direct words: remote, local, dump, replacement, backup.
- Keep command construction visible and testable.
- Prefer one well-tested helper over repeated near-copies.
