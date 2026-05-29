# Dsync Refactor Plan

## Status

Completed for the Go-only replacement path. Dsync now derives the replacement engine from minimal config and has no WP-CLI fallback path.

## Goal

Refactor Dsync so WordPress database sync is fast, serialization-safe, and does not require WP-CLI by default.

## Tasks

- [x] Add automatic DB replacement engine selection
  - [x] Support internal engines: `raw`, `go-serialized`, `none`.
  - [x] Default to `none` when there are no replacements.
  - [x] Default to `go-serialized` for WordPress-style configs with replacements.
  - [x] Keep legacy `dbReplaceEngine` and `validateSerialized` overrides for existing configs.
  - [x] Do not add a WP-CLI fallback engine.

- [x] Refactor DB sync to support streaming
  - [x] Replace full SQL string dump/import flow with `io.Reader` / `io.Writer` pipeline.
  - [x] Stream remote `mysqldump` stdout through transformer into local MySQL stdin.
  - [x] Stream local dump through transformer into remote MySQL stdin for reverse sync.
  - [x] Keep dump-to-file support without forcing full dump into memory by teeing the transformed stream.

- [x] Improve dump command flags
  - [x] Add `--single-transaction`.
  - [x] Add `--quick`.
  - [x] Add `--hex-blob`.
  - [x] Add `--complete-insert` so column names are available.
  - [x] Add `--default-character-set=utf8mb4`.
  - [x] Verify import compatibility with Docker `mariadb:lts` integration coverage. Real remote compatibility remains an operational preflight because it depends on the target host.

- [x] Build SQL stream transformer
  - [x] Pass through non-INSERT SQL unchanged.
  - [x] Detect `INSERT INTO ... VALUES ...;` statements.
  - [x] Parse table name for clearer parser errors.
  - [x] Parse optional complete-insert column list.
  - [x] Parse multi-row `VALUES` lists.
  - [x] Correctly handle quoted strings, escaped quotes, backslashes, NULL, numbers, and hex values.
  - [x] Reconstruct transformed INSERT statements.
  - [x] Add tests for multiline INSERT statements.

- [x] Add column-aware replacement rules
  - [x] Skip `guid` columns by default.
  - [x] Keep legacy configurable `skipColumns` while skipping `guid` by default.
  - [x] Defer `includeTables` / `skipTables`; not needed for the current engine.
  - [x] Ensure replacement still works when column list is unavailable and document that column skipping requires column names.

- [x] Add Go PHP serialization support
  - [x] Evaluated library options and chose an in-repo parser to preserve object payloads without adding a dependency.
  - [x] Implement `isSerializedPHP(value string) bool`.
  - [x] Implement unserialize -> recursive replace -> serialize.
  - [x] Preserve PHP string byte lengths correctly.
  - [x] Add fixtures/tests for arrays, objects, bools, nulls, ints, floats, and UTF-8 strings.

- [x] Support nested serialized strings
  - [x] If a string value inside serialized data is itself serialized, process it recursively.
  - [x] Replace inside string keys as well as string values.
  - [x] Add recursion depth limit to avoid pathological data.
  - [x] Add tests for nested serialized arrays and plugin-style option values.

- [x] Handle SQL escaping safely
  - [x] Implement SQL string unescape for mysqldump output.
  - [x] Implement SQL string escape for re-emitting values.
  - [x] Cover `\'`, `\\`, `\n`, `\r`, `\t`, `\0`, and slash-escaped URL values.
  - [x] Add WordPress-like option/meta fixture coverage.

- [x] Add serialized validation without WP-CLI
  - [x] Detect values that look serialized but fail unserialization during stream transformation.
  - [x] Report table, row, and column for transformation failures.
  - [x] Enable validation by default and keep `validateSerialized` as a legacy opt-out.
  - [x] Fail sync when transformed serialized values are invalid.
  - [x] Verify transformed values import into MariaDB in the Docker-backed integration test.

- [x] Do not implement WP-CLI replacement
  - [x] Use the Go serialized engine as the WordPress-safe method.
  - [x] Do not implement fallback; dsync derives or uses the legacy configured engine.

- [x] Add benchmarks
  - [x] Benchmark raw replacement.
  - [x] Benchmark raw stream replacement.
  - [x] Benchmark Go serialized stream replacement.
  - [x] Benchmark memory usage on large dumps with `b.ReportAllocs`.
  - [x] Document benchmark snapshot in README.

- [x] Add integration tests
  - [x] Create small WordPress-like SQL dump fixture.
  - [x] Include serialized options with changing string lengths.
  - [x] Include `guid` values that must not change.
  - [x] Include JSON, escaped URLs, and nested serialized data.
  - [x] Verify transformed SQL imports successfully into Docker `mariadb:lts` when `DSYNC_INTEGRATION=1`.
  - [x] Verify no broken serialized values after import.

- [x] Update docs
  - [x] Document replacement engines and tradeoffs.
  - [x] Document recommended WordPress config.
  - [x] Document that Dsync does not fallback between replacement engines.
  - [x] Document validation behavior.
  - [x] Document release commands and benchmark snapshot.

- [x] Migration cleanup
  - [x] Keep backward compatibility for existing configs while generated configs omit derived DB options.
  - [x] Warn when using unsafe `raw` replacement with likely WordPress paths.
  - [x] Add clear error messages for parser failures.
  - [x] Ensure reverse sync uses the same safe engine with reversed replacements.
