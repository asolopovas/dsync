# Quality score

Agent-legibility ledger. Update when quality materially changes.

| Area | Grade | State | Next |
| --- | --- | --- | --- |
| CLI wiring | A- | Combined flags, early validation, failed dispatch, and cancellation are tested. | Keep synthetic CLI smoke coverage current. |
| File sync | B+ | Safe arguments, failure propagation, atomic replacement, and exclusion checks against rsync. | Extend oracle cases when adding filter syntax. |
| DB orchestration | B+ | Pipeline ownership and error identity tested; private backup precedes reverse import. | Keep subprocess failure cases covered. |
| Replacement engine | B+ | Bounded PHP parser, SQL framing regressions, fuzz targets, and extended-insert benchmarks. | Largest-statement memory still scales with dump input. |
| Config | B+ | Operation-aware validation and private exclusive generation; unknown fields accepted. | Service/password expansion remains separate. |
| Verification | A- | Standard/race/fuzz, rsync oracle, Docker MariaDB, vulnerability scan, and cross builds. | Repeat benchmarks for future optimization work. |
| Docs | B+ | Map, architecture, plans, and references are compact. | Add link/freshness checks if drift appears. |

Debt: [`docs/tech-debt-tracker.md`](docs/tech-debt-tracker.md).
