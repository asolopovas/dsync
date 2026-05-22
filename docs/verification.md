# Verification

Use the smallest reliable `just` task while iterating, then run the handoff gate before reporting done. If a task exists, run the task instead of manually running the underlying commands.

## Fast checks

```bash
just test-one TestApplyDBReplacements
just test-one TestTransformSQLDump
just test-one TestSyncDB
just test-one TestEnsureTrailingSlash
```

## Handoff gate

```bash
just check
just integration-test  # when Docker is available
just help
just version
```

`just check` runs formatting, vet, tests, and a temporary compile check without dirtying `dist/`.

## Manual checks

Run manual checks only when relevant and safe.

| Behaviour touched | Manual check |
| --- | --- |
| File sync command construction | Use a disposable local/remote path and run `go run . -c <safe config> -f`. |
| Forward DB sync | Use a disposable local DB and run `go run . -c <safe config> -d`. |
| Reverse DB sync | Confirm the target is disposable or backed up, then run `go run . -c <safe config> -d -r`. |
| Dump output | Run `go run . -c <safe config> -d --dump` and inspect `db.sql` or `db_reverse.sql`. |
| Release packaging | Run `just release` and inspect `releases/dev/checksums.txt`. For stable flow, use only `just release --bump patch|minor|major`; the task owns check, build, commit, tags, and push. Use `just release --dry-run --bump patch` only to preview. |
| Serialized DB import | Run `DSYNC_INTEGRATION=1 go test -run TestWordPressFixtureImportsIntoMariaDB -count=1` with Docker available. |
| Completion command | Run `go run . completion` and inspect `~/.config/fish/completions/dsync.fish`. |

## Expected unit-test coverage

- Replacement order and escaped slash variants.
- Reverse replacement order.
- DB orchestration calls and backup-before-write order.
- SQL stream transformation for multiline INSERTs, column skipping, SQL escaping, PHP serialized values, and WordPress-like fixtures.
- Path normalization for rsync directory semantics.

## Handoff notes

State what ran and what did not run. If a manual check was skipped, say why: no safe remote, no Docker Compose DB, no SSH access, or not relevant.
