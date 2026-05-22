# Technical debt

Track debt that should shape future agent work. Keep entries short and actionable.

## Active debt

| Debt | Impact | Preferred fix |
| --- | --- | --- |
| Raw DB replacement is not PHP-serialization safe. | WordPress options/meta can be corrupted when replacement lengths change. | Implement the planned serialization-safe stream transformer. |
| DB command args are embedded in provider methods. | Harder to test exact shell invocations without running commands. | Extract arg builders with unit tests. |
| Local DB service name and root password are hard-coded. | Dsync only fits one Docker Compose convention. | Add config fields or documented environment overrides with validation. |
| Config loading does not validate required fields. | Bad configs fail late inside shell commands. | Add explicit validation after JSON parsing. |
| Release tagging still force-updates `latest`. | Easy to surprise consumers watching the rolling tag. | Prefer semver tags for installs; keep `latest` documented as rolling. |

## Garbage-collection rule

Do not let debt hide in chat. If a shortcut is accepted, record it here or in an execution plan with an owner path and an exit condition.
