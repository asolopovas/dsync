# DESIGN

Dsync favors explicit orchestration over hidden automation. Sync operations are easy to inspect, test, and abort.

## Decisions

- Single-package CLI until seams become painful enough to justify packages.
- Cobra owns command wiring; sync behavior lives behind small functions.
- `DBProvider` is the DB orchestration seam for tests and future command-provider refactors.
- DB dumps stream through transformers; avoid whole-dump memory use where possible.
- Reverse DB sync must back up remote before remote import.
- File sync treats endpoints as directories; trailing slash normalization is required.

## Change guidance

- Add tests before changing sync order, replacement order, or backup behavior.
- Prefer explicit arg builders over opaque shell strings when refactoring commands.
- Promote repeated review comments into docs, tests, or `just` tasks.
