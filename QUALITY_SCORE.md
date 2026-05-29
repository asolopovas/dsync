# Quality score

Agent-legibility ledger. Update when quality materially changes.

| Area | Grade | State | Next |
| --- | --- | --- | --- |
| CLI wiring | B | Flags are centralized in `root.go`; dispatch is simple. | Add flag-combination tests. |
| File sync | B | Path normalization is tested; rsync construction is small. | Add exclude and reverse-direction tests. |
| DB orchestration | B- | `DBProvider` covers ordering and backup-before-write. | Extract command arg builders with tests. |
| Replacement engine | B | Streaming, column-aware, serialized-safe path has fixtures and benchmarks. | Improve malformed-dump diagnostics. |
| Config | C+ | Schema is small and documented. | Validate required fields early. |
| Verification | B | `just check`, race, and Docker integration paths exist. | Add Docker-free command-construction checks. |
| Docs | B+ | Map, architecture, plans, and references are compact. | Add link/freshness checks if drift appears. |

Debt: [`docs/tech-debt-tracker.md`](docs/tech-debt-tracker.md).
