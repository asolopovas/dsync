# Technical debt tracker

Keep entries short and actionable.

| Debt | Impact | Preferred fix |
| --- | --- | --- |
| DB command args are embedded in provider methods. | Exact shell invocation tests are harder. | Extract arg builders with unit tests. |
| Local DB service name and root password are hard-coded. | Dsync fits one Docker Compose convention. | Add config fields or documented environment overrides with validation. |
| Config loading does not validate required fields. | Bad configs fail late inside shell commands. | Add explicit validation after JSON parsing. |
| Release tagging force-updates `latest`. | Consumers watching `latest` can be surprised. | Keep semver tags as install path; document `latest` as rolling. |

## Garbage-collection rule

If a shortcut is accepted, record it here or in an execution plan with an exit condition.
