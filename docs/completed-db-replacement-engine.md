# DB replacement engine plan

Status: completed for the Go-only replacement path.

## Goal

Make database replacement streaming, WordPress-safe, and independent of WP-CLI.

## Shipped decisions

- Internal engines: `go-serialized`, `raw`, `none`.
- Minimal configs omit engine selection; Dsync derives `none` for empty replacements, `go-serialized` for WordPress-looking paths, and `raw` otherwise.
- DB sync streams dumps through `io.Reader`/`io.Writer` transformers.
- Dsync dumps with `--single-transaction`, `--quick`, `--hex-blob`, `--complete-insert`, and `--default-character-set=utf8mb4`.
- Column-aware replacement skips `guid` by default; legacy configs can add more skipped columns with `skipColumns`.
- In-repo PHP serialization support rewrites strings with corrected byte lengths, including nested serialized strings.
- Serialized validation is enabled by default; legacy configs can disable it with `validateSerialized: false`.

## Verification kept

- Unit tests for SQL parsing, replacement order, reverse replacement, serialized values, and escaping.
- WordPress-like fixture with serialized data, JSON, escaped URLs, and `guid` rows.
- Docker-backed MariaDB import test gated by `DSYNC_INTEGRATION=1`.
- Benchmarks for raw and Go serialized transformers.

## Follow-up debt

See [`tech-debt-tracker.md`](tech-debt-tracker.md) for remaining command/config improvements.
