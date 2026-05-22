# Plans

Plans are first-class repository artifacts. Use them when a task spans multiple turns, changes architecture, or needs decisions preserved for future agents.

## Index

| Plan | Status | Purpose |
| --- | --- | --- |
| [`db-replacement-engine.md`](db-replacement-engine.md) | Draft | Make DB replacement streaming and serialization-safe. |

## Plan rules

- Keep small tasks in the prompt or handoff note.
- Create a plan when progress, decisions, or partial completion must survive context loss.
- Include goal, constraints, task list, verification, and decision log.
- Update the plan as work lands. Do not leave completed work marked open.
- Prefer checked-in plans over loose scratch files.
