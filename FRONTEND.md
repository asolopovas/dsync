# FRONTEND

Dsync has no web frontend. Its user interface is a CLI with terminal output.

## CLI presentation rules

- Make destructive direction visible: remote -> local or local -> remote.
- Show which operation is running: files, database, dump, backup, import.
- Keep errors actionable; include captured stderr/stdout when available.
- Do not hide safety-critical details behind decorative terminal UI.
- Completion generation belongs in `root.go` with other CLI wiring.
