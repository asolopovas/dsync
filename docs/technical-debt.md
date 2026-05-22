# Technical debt

Track debt that should shape future agent work. Keep entries short and actionable.

## Active debt

| Debt | Impact | Preferred fix |
| --- | --- | --- |
| Raw DB replacement is not PHP-serialization safe. | WordPress options/meta can be corrupted when replacement lengths change. | Implement the planned serialization-safe stream transformer. |
| DB command args are embedded in provider methods. | Harder to test exact shell invocations without running commands. | Extract arg builders with unit tests. |
| Local DB service name and root password are hard-coded. | Dsync only fits one Docker Compose convention. | Add config fields or documented environment overrides with validation. |
| Config loading does not validate required fields. | Bad configs fail late inside shell commands. | Add explicit validation after JSON parsing. |
| `make test` is not the Go test suite. | Agents may run the wrong check. | Rename it or add a real `go test ./...` target. |
| Release tagging is manual and force-updates `latest`. | Easy to tag the wrong commit. | Add preflight checks for clean tree, passing tests, and version format. |

## Garbage-collection rule

Do not let debt hide in chat. If a shortcut is accepted, record it here or in an execution plan with an owner path and an exit condition.
