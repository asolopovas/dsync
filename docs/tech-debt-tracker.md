# Technical debt tracker

Keep entries actionable.

| Debt | Impact | Fix |
| --- | --- | --- |
| Statement-based SQL transformation retains a full extended INSERT in memory. | Peak memory follows the largest statement. | Consider row streaming only after profiling realistic dumps. |
| Local DB service and root password are hard-coded. | Only one Compose convention fits. | Add config/env overrides with validation. |
| Release force-updates `latest`. | Watchers of `latest` can be surprised. | Treat semver tags as stable; document `latest` as rolling. |
| Docs have no link/freshness check. | Drift is caught by humans. | Add a lightweight docs CI check when drift appears. |

If a shortcut is accepted, record it here or in an execution plan with an exit condition.
