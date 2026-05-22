# QUALITY SCORE

Agent-legibility ledger. Update when a change materially improves or weakens an area.

| Area | Grade | Current state | Next improvement |
| --- | --- | --- | --- |
| CLI wiring | B | Cobra flags are centralized in `root.go`; dispatch is simple. | Add command-level tests for flag combinations. |
| File sync | B | Path normalization is tested; rsync construction is small. | Add exclude-argument and reverse-direction tests. |
| DB orchestration | B- | `DBProvider` enables ordering tests; backup-before-write is covered. | Extract command arg builders with unit tests. |
| Replacement engine | B | Streaming, column-aware, PHP-serialization-safe path exists with fixtures and benchmarks. | Add more malformed-dump diagnostics and table-level options if needed. |
| Config | C+ | Schema is documented and small. | Validate required fields and dangerous empty values early. |
| Verification | B | `just check` is reliable; Docker integration exists. | Add Docker-free command-construction checks. |
| Docs | B+ | Harness-style map, source docs, plans, and references are compact. | Add a lightweight link/freshness check if drift appears. |

Debt lives in [`docs/exec-plans/tech-debt-tracker.md`](docs/exec-plans/tech-debt-tracker.md).
