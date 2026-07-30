# Ikemen Devtools

Agent-first static analysis and indexing for MUGEN/Ikemen authored files. The product ships as one Go binary, `ikm`, while keeping reusable Go packages for hosts that need in-process integration.

## Quick start

From this directory:

```text
go run ./cmd/ikm check path/to/character.def
go run ./cmd/ikm check --json path/to/character.def
go run ./cmd/ikm index --output index.sql path/to/character.def
go run ./cmd/ikm corpus --json --output roster.json data/select.def
go run ./cmd/ikm metadata --timestamp 2026-07-30T12:00:00Z --file README.md
```

`check` and `index` load a root `.def` and its `[Files]` text sources. `index` emits executable SQL with ordinary and FTS5 tables; it does not open SQLite. `corpus` audits a `select.def`; `metadata` emits deterministic release metadata. `lsp` and `mcp` start protocol servers over stdio. See [CLI onboarding](docs/CLI.md) for flags and exit behavior.

## Product slices

- **Core:** tolerant DEF/CNS/CMD/ST parsing, source spans, semantic resolution, deterministic diagnostics, and SQL export.
- **Compatibility:** explicit strict/portable and distribution profiles; corpus manifests can expose path and source-resolution gaps.
- **Protocols:** read-only LSP and MCP JSON-RPC servers for diagnostics, symbols, hover, definition, and references. Both are available through the `ikm` binary and as Go APIs.
- **Persistence and operations:** migration-aware repository API (schema 2), IR identity contract `0.2.0`, and canonical/Ed25519-capable release metadata.

## Supported dialect and known limits

The parser covers a tolerant INI-like subset of `.def`, `.cns`, `.st`, and `.cmd`. The workspace follows `[Files]` keys `cmd`, `cns`, `st`, `stN`, and `stcommon`. Expressions, ZSS, AIR/SFF/SND semantics, motif/screenpack relationships, and Lua-aware analysis are not complete. Dynamic or ambiguous targets are not guessed into exact references.

There is no real runtime driver by default. LSP documents arrive from the editor. MCP only reads files explicitly named with repeatable `--file`; neither protocol server writes files. SQLite persistence requires the host to inject/register a `database/sql` driver; the module intentionally does not bundle one. See [corpus and profiles](docs/COMPATIBILITY.md), [LSP/MCP](docs/LSP_MCP.md), and [safety boundaries](docs/SAFETY.md).

## Upgrade checklist

1. Keep the original MVP flow (`check`/`index`) as the baseline and record its selected profile.
2. Run `corpus --json` against the active `select.def` and review unresolved or non-exact results.
3. Adopt the distribution profile where literal `=` directories and authored empty state sections require it.
4. If using persistence, back up the database and let `repository.Open` migrate schema 1 to schema 2; never open a newer schema with an older binary.
5. Treat IR contract `0.2.0`, semantic identities, and protocol DTOs as versioned contracts.
6. Keep LSP/MCP integrations read-only and in-memory; add filesystem roots, bounds, and authorization in the host wrapper.

Further details: [migration/versioning](docs/MIGRATION.md), [release metadata](docs/RELEASE.md), and the [MVP-to-product roadmap](docs/MVP_TO_PRODUCT_EVOLUTION.md).
