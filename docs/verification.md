# Verification

Use the smallest reliable check while iterating, then run the handoff gate before reporting done.

## Fast checks

```bash
go test -run TestApplyDBReplacements
go test -run TestSyncDB
go test -run TestEnsureTrailingSlash
```

## Handoff gate

```bash
gofmt -w *.go
go test ./...
go build -o ./dist/dsync .
go run . --help
go run . --version
```

`gofmt` should be a no-op unless Go files changed. The build writes `dist/dsync`, which is a generated binary.

## Manual checks

Run manual checks only when relevant and safe.

| Behaviour touched | Manual check |
| --- | --- |
| File sync command construction | Use a disposable local/remote path and run `go run . -c <safe config> -f`. |
| Forward DB sync | Use a disposable local DB and run `go run . -c <safe config> -d`. |
| Reverse DB sync | Confirm the target is disposable or backed up, then run `go run . -c <safe config> -d -r`. |
| Dump output | Run `go run . -c <safe config> -d --dump` and inspect `db.sql` or `db_reverse.sql`. |
| Completion command | Run `go run . completion` and inspect `~/.config/fish/completions/dsync.fish`. |

## Expected unit-test coverage

- Replacement order and escaped slash variants.
- Reverse replacement order.
- DB orchestration calls and backup-before-write order.
- Path normalization for rsync directory semantics.

## Handoff notes

State what ran and what did not run. If a manual check was skipped, say why: no safe remote, no Docker Compose DB, no SSH access, or not relevant.
