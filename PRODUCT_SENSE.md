# Product sense

Dsync makes local/remote web-development syncs fast, predictable, and recoverable.

## Principles

- Default to safe local workflows.
- Make reverse sync explicit because it writes remote state.
- Preserve WordPress serialized data during URL/path replacement.
- Keep config small enough to audit before running.
- Prefer clear terminal output over clever automation.

## Trust requirements

- Direction is visible before data moves.
- Users can trace behavior from config plus flags.
- Failures identify missing tools, access, or unsafe data.
- Configs and dumps remain private.
