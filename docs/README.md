# Documentation map

This directory holds durable project knowledge. `AGENTS.md` is only the entry point.

## Files

| File | Purpose |
| --- | --- |
| [`agent-principles.md`](agent-principles.md) | Agent-first operating rules, feedback loop, taste invariants. |
| [`architecture.md`](architecture.md) | Code layout, boundaries, external tools, sync data flow. |
| [`dev-loop.md`](dev-loop.md) | Setup, common commands, local iteration habits. |
| [`verification.md`](verification.md) | Tests, builds, manual checks, handoff contract. |
| [`configuration.md`](configuration.md) | `dsync-config.json` schema and replacement behaviour. |
| [`operations.md`](operations.md) | Safe use of forward sync, reverse sync, dumps, and generated artefacts. |
| [`quality.md`](quality.md) | Agent-legibility quality ledger by area. |
| [`technical-debt.md`](technical-debt.md) | Known debt and preferred fixes. |
| [`plans/README.md`](plans/README.md) | Where multi-turn plans live. |

## Freshness rule

When code behaviour changes, update the matching document in the same change. Prefer small edits over long prose. If a rule is only temporary, put it in a plan rather than in a permanent guide.

## Harness rule

If an agent repeats a mistake, do not rely on memory. Add a doc rule, a test, a command, or a clearer boundary so the next run can inspect and verify it.

## Writing rule

Be plain. Explain the sharp edge, the command, and the reason. Avoid clever wording when a direct sentence will do.
