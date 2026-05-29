# Design

Dsync favors inspectable sync over hidden automation.

## Decisions

- Stay single-package until seams become painful.
- Cobra wires commands; sync behavior sits behind small functions.
- `DBProvider` is the test seam for DB orchestration.
- Dumps stream; avoid whole-dump memory use.
- Reverse DB sync backs up remote before import.
- File endpoints are directories; trailing slash normalization is required.

## Change rules

- Test sync order, replacement order, and backup behavior before changing them.
- Prefer explicit argv builders over opaque shell strings.
- Promote repeated review feedback into docs, tests, `just` tasks, or lints.
