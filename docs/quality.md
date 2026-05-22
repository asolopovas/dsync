# Quality ledger

This file tracks where the project is easy or hard for agents to change safely.

| Area | Grade | Current state | Next improvement |
| --- | --- | --- | --- |
| CLI wiring | B | Cobra flags are centralized in `root.go`; behaviour dispatch is simple. | Add command-level tests for flag combinations. |
| File sync | B | `rsync` construction is small and path normalization is tested. | Add tests for exclude argument construction and reverse source/destination order. |
| DB orchestration | B- | `DBProvider` enables tests; backup-before-write is covered. | Split command construction from execution so shell args can be unit-tested. |
| Replacement engine | C | Covers raw and slash-escaped replacements. | Implement serialization-safe engine from [`plans/db-replacement-engine.md`](plans/db-replacement-engine.md). |
| Config | C+ | JSON schema is small and documented. | Add validation for required fields and dangerous empty values. |
| Verification | B- | Unit tests and build are straightforward. | Add a safe integration fixture for SQL transformation and Docker-free command checks. |
| Docs | B | Compact map exists and links to source-of-truth docs. | Add a lightweight doc freshness check if docs start drifting. |

## Rule

When a change improves or weakens an area, update this ledger in the same change.
