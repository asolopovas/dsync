# PRODUCT SENSE

Dsync exists to make local/remote web-development syncs fast, predictable, and recoverable.

## Product principles

- Default to safe local development workflows.
- Make reverse sync intentionally explicit because it can overwrite remote state.
- Preserve WordPress data integrity; serialized PHP values must not be corrupted by URL replacement.
- Keep config small enough to audit before running.
- Prefer clear terminal output over clever automation.

## User trust requirements

- A user can tell which direction data will move before it moves.
- A user can dry-run mentally from config and command flags.
- A failed shell command points to the missing dependency or access issue.
- Generated dumps and configs are treated as private data.
