# Agent operating principles

These rules adapt harness-engineering practice to this repository.

## Core beliefs

- `AGENTS.md` is a map, not a manual.
- Versioned repository docs are the system of record. Chat, memory, and local habits do not count until captured here.
- Context is scarce. Start compact, then link to the precise deeper doc.
- Prefer enforceable invariants over style lectures.
- When an agent repeats a mistake, fix the harness: add a test, a doc rule, a command, or a clearer boundary.
- Keep the codebase legible to the next agent run. Clear structure beats clever shortcuts.

## Practical loop

1. Read `AGENTS.md`.
2. Open only the docs relevant to the task.
3. Inspect the code before editing.
4. Make the smallest coherent change.
5. Run the matching checks from [`verification.md`](verification.md).
6. Update docs or plans when behaviour, workflow, or known debt changes.

## Taste invariants

- Eschew obfuscation; elucidate intent.
- Use direct names for data flow: remote, local, dump, replacement, backup.
- Keep shell command construction visible and testable.
- Keep destructive operations obvious in names, docs, and output.
- Prefer one well-tested helper over repeated near-copies.
