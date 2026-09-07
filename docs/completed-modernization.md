# Modernization and reliability

## Goal and scope
Upgrade to Go 1.27 (validate with 1.27.1), current compatible dependencies,
and improve cancellation, pipeline ownership, command construction, validation,
file privacy, exclusion handling, and SQL/PHP transformation safety/performance.
Keep DBProvider, single-package ownership, valid configuration compatibility,
sequential replacements, and reverse backup-before-import. No private sync,
installation, publishing, or release.

## Acceptance criteria
Regression tests for failures, cleanup, error identity, quoting, exclusions,
private files, malformed SQL/PHP, and cancellation; fuzz parser boundaries;
compare repeated realistic benchmarks; run standard/race/integration checks,
vulnerability scanning, synthetic CLI smoke and four platform builds.

## Progress
- Inspected repository, confirmed clean baseline and installed Go 1.27.1.
- Implemented and verified CLI, pipeline, quoting, file-safety, parser, and
  performance changes. Updated architecture, workflow, quality, and debt docs.
- Confirmed requested dependency versions through the Go module proxy and
  toolchain platform requirements in https://go.dev/doc/go1.27.

## Decisions
- Retain encoding/json compatibility, including accepting unknown fields.
- Keep statement-based streaming and destination-wide file replacement.

## Validation
All checks passed with Go 1.27.1 on Linux/amd64:

- `just check` (format, vet, unit tests, temporary compile).
- `just check test-race integration-test`: race tests and disposable MariaDB
  fixture import, including quoted account/database setup and dump arguments.
- `govulncheck ./...`: no vulnerabilities found; `go mod verify` passed.
- PHP fuzzing: 20 seconds, 1,384,057 executions. SQL fuzzing: 21 seconds,
  1,147,976 executions, followed by 16 seconds / 1,029,156 executions after
  tightening INSERT header validation. No failures.
- Local rsync oracle tests cover anchored/directory/nested patterns, `*`, `?`,
  `**`, `***`, POSIX/negated classes, escaped characters, include and reset rules.
- Synthetic CLI help/version/config/invalid-argument smoke passed. A subprocess
  SIGINT smoke confirmed both CLI failure and reaping of the fake rsync process.
- Linux/macOS amd64/arm64 builds passed, with CGO disabled.
- No private-site sync, installation, publishing, or release was performed.

### Repeated benchmarks

Six samples before and after, using `go test -run '^$' -bench . -benchmem -count=6`
and `golang.org/x/perf/cmd/benchstat`. Both runs used Go 1.27.1 on an AMD Ryzen 7
5800X3D. Extended insert fixtures were added before measuring the baseline.
The final run includes stricter SQL framing/header checks and bounded PHP parsing.

| Benchmark | Before MiB/s | After MiB/s | Before allocations/op | After allocations/op |
| --- | ---: | ---: | ---: | ---: |
| Standalone raw replacement | 323.0 | 324.2 | 12 | 13 |
| Raw dump stream | 54.87 | 87.73 | 17,013 | 7,015 |
| Serialized dump | 19.25 | 34.03 | 110,010 | 44,016 |
| Large serialized dump | 26.16 | 41.45 | 560,500 | 310,000 |
| Extended insert, matches | 15.11 | 25.19 | 432,000 | 130,100 |
| Extended insert, no matches | 22.98 | 26.96 | 221,300 | 120,100 |

All streaming throughput improvements were significant (p=0.002, n=6).
Standalone raw throughput was unchanged statistically (p=0.699); its additional
allocation holds the compiled replacement list. Serialized stream allocations
fell 60%, allocated bytes fell 44%, and throughput increased 77%. Extended
inserts with matches improved throughput 67% with 70% fewer allocations.
The implementation retains statement streaming; no larger parser redesign was
needed to obtain these improvements.

## Follow-up debt
Service/password configuration and release-process changes remain out of scope.
