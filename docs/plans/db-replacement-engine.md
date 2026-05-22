# DB replacement engine plan

## Status

Draft. Migrated from `plan.td` so future agents can find it through the docs map.

## Goal

Refactor Dsync so WordPress database sync is fast, serialization-safe, and does not require WP-CLI by default.

## Constraints

- Preserve backward compatibility for existing configs.
- Keep reverse sync safe: backup remote before writing remote.
- Do not require WP-CLI for the default safe path.
- Keep raw replacement available for simple non-WordPress use cases.
- Add tests before replacing current behaviour.

## Tasks

- [ ] Add DB replacement engine config
  - [ ] Add `dbReplaceEngine` config option.
  - [ ] Supported values: `raw`, `wp-cli`, `go-serialized`, `none`.
  - [ ] Default to `go-serialized` for WordPress-style configs once stable.
  - [ ] Add `validateSerialized` boolean config option.
  - [ ] Add `dbReplaceFallback` config option with values `none` or `wp-cli`.
- [ ] Refactor DB sync to support streaming
  - [ ] Replace full SQL string dump/import flow with `io.Reader` / `io.Writer` pipeline.
  - [ ] Stream remote `mysqldump` stdout through transformer into local MySQL stdin.
  - [ ] Stream local dump through transformer into remote MySQL stdin for reverse sync.
  - [ ] Keep dump-to-file support without forcing full dump into memory.
- [ ] Improve dump command flags
  - [ ] Add `--single-transaction`.
  - [ ] Add `--quick`.
  - [ ] Add `--hex-blob`.
  - [ ] Add `--complete-insert` so column names are available.
  - [ ] Add `--default-character-set=utf8mb4`.
  - [ ] Verify compatibility with current remote and local MariaDB/MySQL versions.
- [ ] Build SQL stream transformer
  - [ ] Pass through non-INSERT SQL unchanged.
  - [ ] Detect `INSERT INTO ... VALUES ...;` statements.
  - [ ] Parse table name.
  - [ ] Parse optional complete-insert column list.
  - [ ] Parse multi-row `VALUES` lists.
  - [ ] Correctly handle quoted strings, escaped quotes, backslashes, NULL, numbers, and hex values.
  - [ ] Reconstruct transformed INSERT statements.
  - [ ] Add tests for multiline INSERT statements.
- [ ] Add column-aware replacement rules
  - [ ] Skip `guid` columns by default.
  - [ ] Support configurable `skipColumns`.
  - [ ] Support configurable `includeTables` / `skipTables` later if needed.
  - [ ] Ensure replacement still works when column list is unavailable, with clear warning or fallback.
- [ ] Add Go PHP serialization support
  - [ ] Evaluate `github.com/elliotchance/phpserialize`.
  - [ ] Evaluate `github.com/stlong5/phpserialize`.
  - [ ] Choose library based on WordPress sample compatibility and object handling.
  - [ ] Implement `isSerializedPHP(value string) bool`.
  - [ ] Implement unserialize -> recursive replace -> serialize.
  - [ ] Preserve PHP string byte lengths correctly.
  - [ ] Add fixtures for arrays, objects, bools, nulls, ints, floats, and UTF-8 strings.
- [ ] Support nested serialized strings
  - [ ] If a string value inside serialized data is itself serialized, process it recursively.
  - [ ] Replace inside string keys as well as string values.
  - [ ] Add recursion depth limit to avoid pathological data.
  - [ ] Add tests for nested serialized arrays and plugin-style option values.
- [ ] Handle SQL escaping safely
  - [ ] Implement SQL string unescape for mysqldump output.
  - [ ] Implement SQL string escape for re-emitting values.
  - [ ] Cover `\'`, `\\`, `\n`, `\r`, `\t`, `\0`, and escaped slashes.
  - [ ] Add tests using real WordPress option/meta samples.
- [ ] Add serialized validation without WP-CLI
  - [ ] Query text-like columns after import.
  - [ ] Detect values that look serialized but fail unserialization.
  - [ ] Report table, column, and primary key/id.
  - [ ] Make validation optional via `validateSerialized`.
  - [ ] Fail sync when validation finds newly corrupted serialized values.
- [ ] Keep WP-CLI engine as optional fallback
  - [ ] Keep any `wp search-replace` implementation behind `dbReplaceEngine: "wp-cli"`.
  - [ ] Avoid `--precise` by default for speed.
  - [ ] Add separate `dbReplacePrecise` flag if precise mode is needed.
  - [ ] Ensure users can set fallback to `none` to avoid WP-CLI completely.
- [ ] Add benchmarks
  - [ ] Benchmark raw replacement.
  - [ ] Benchmark WP-CLI replacement.
  - [ ] Benchmark Go serialized stream replacement.
  - [ ] Benchmark memory usage on large dumps.
  - [ ] Document results in README or docs.
- [ ] Add integration tests
  - [ ] Create small WordPress-like SQL dump fixture.
  - [ ] Include serialized options with changing string lengths.
  - [ ] Include `guid` values that must not change.
  - [ ] Include JSON, escaped URLs, and nested serialized data.
  - [ ] Verify transformed SQL imports successfully.
  - [ ] Verify no broken serialized values after import.
- [ ] Update docs
  - [ ] Document replacement engines and tradeoffs.
  - [ ] Document recommended WordPress config.
  - [ ] Document when to use `wp-cli` fallback.
  - [ ] Document validation behavior.
- [ ] Migration cleanup
  - [ ] Keep backward compatibility for existing configs.
  - [ ] Warn when using unsafe `raw` replacement with likely WordPress paths.
  - [ ] Add clear error messages for parser failures.
  - [ ] Ensure reverse sync uses the same safe engine with reversed replacements.

## Verification plan

- Unit tests for each parser and transformer layer.
- Fixtures for WordPress-like SQL with serialized PHP, JSON, escaped URLs, nested values, and `guid` rows.
- Reverse-sync tests that prove replacement inversion and remote backup order.
- Benchmarks for memory use and runtime against current raw replacement.

## Decision log

- Raw replacement remains documented as the current implementation and future compatibility mode.
- The safe default should be implemented only after fixture coverage proves serialization behaviour.
