# Frontend

Dsync has no web frontend. Its UI is terminal output.

## CLI presentation rules

- Show direction before work starts: remote -> local or local -> remote.
- Name the stage: files, DB dump, backup, transform, import.
- During DB streaming, show dump bytes read and transformed bytes sent.
- Include captured stderr/stdout in command errors.
- Keep safety-critical details visible; pterm decoration must not hide them.
