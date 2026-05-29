# Technical debt tracker

Keep entries actionable.

| Debt | Impact | Fix |
| --- | --- | --- |
| DB command args live inside provider methods. | Shell invocation tests are harder. | Extract arg builders with unit tests. |
| Local DB service and root password are hard-coded. | Only one Compose convention fits. | Add config/env overrides with validation. |
| Config loading lacks required-field validation. | Bad configs fail late in shell commands. | Validate after JSON parse. |
| Release force-updates `latest`. | Watchers of `latest` can be surprised. | Treat semver tags as stable; document `latest` as rolling. |
| Docs have no link/freshness check. | Drift is caught by humans. | Add a lightweight docs CI check when drift appears. |

If a shortcut is accepted, record it here or in an execution plan with an exit condition.
