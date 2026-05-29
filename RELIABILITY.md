# Reliability

Use `just` tasks for repo workflows. Record commands and results in handoffs/PRs.

## Agent loop

```text
inspect repo -> plan -> implement -> run focused checks -> run app/CLI -> inspect output/logs -> self-review -> rerun checks
```

Escalate only for product judgment, risk, ambiguity, or unsafe credentials/targets.

## Local checks

```bash
just setup
just run -- --help
just test
just test-one TestName
just vet
just check
just check test-race
just check integration-test   # Docker-backed MariaDB import
just bench
```

`just check` runs gofmt, vet, tests, and a temp compile check without dirtying `dist/`.

| Change | Minimum check |
| --- | --- |
| Replacement engine | `just test-one TestTransformSQLDump` |
| DB orchestration | `just test-one TestSyncDB` |
| DB replacements | `just test-one TestApplyDBReplacements` |
| Rsync paths | `just test-one TestEnsureTrailingSlash` |
| DB import semantics | `just check integration-test` when Docker is available |
| Handoff | `just check` |

## Quick E2E smoke

This is destructive in the configured direction. Confirm the private config target first.

```bash
just install
cd /home/andrius/www/avianese.test/wp-content/themes/avianese-theme
dsync -a
```

Use the same pattern for any site directory containing a private `dsync-config.json`.

## User commands

```bash
dsync -c ./configs/site.json -a          # files + DB, remote -> local
dsync -c ./configs/site.json -f          # files only
dsync -c ./configs/site.json -d          # DB only
dsync -c ./configs/site.json -a -r       # local -> remote
dsync -c ./configs/site.json -d -r       # DB local -> remote, with backup
dsync -c ./configs/site.json -d --dump   # import + db.sql
dsync -c ./configs/site.json -d -r --dump # import remote + db_reverse.sql
```

Before reverse sync, confirm host, DB, file paths, and replacement reversibility.

## Failure triage

- File sync: ssh, rsync on both hosts, port, paths, excludes.
- Local DB: Docker, compose file, `mariadb` service, root password `secret`.
- Remote DB: `mysqldump`/`mysql`, root access, non-interactive auth.
- Serialized replacement: inspect reported table/row/column; use `raw` only when corruption risk is accepted.

## Release

```bash
just release
just release --bump patch|minor|major
just release --dry-run --bump patch
```

Do not manually split the release flow.
