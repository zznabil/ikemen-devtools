# CLI onboarding

Build or run from `_upstream/ikemen-devtools`:

```text
go run ./cmd/ikm check path/to/character.def
go run ./cmd/ikm check --json path/to/character.def
go run ./cmd/ikm index path/to/character.def > index.sql
go run ./cmd/ikm index --output index.sql path/to/character.def
go run ./cmd/ikm corpus data/select.def
go run ./cmd/ikm corpus --json --output roster.json data/select.def
go run ./cmd/ikm metadata --timestamp 2026-07-30T12:00:00Z --file README.md --file go.mod
go run ./cmd/ikm lsp
go run ./cmd/ikm mcp --file path/to/hero.def --file path/to/hero.cmd
```

`check` loads the root `.def`, follows `[Files]` entries, resolves semantics, and prints human diagnostics by default. `--json` emits the report JSON. `index` exports deterministic SQL (ordinary tables plus FTS5); without `--output` it writes SQL to stdout. `corpus` builds a manifest from a `select.def`; `--json` changes the representation and `--output` writes it atomically. `metadata` emits canonical release metadata and requires an explicit RFC3339 timestamp; repeat `--file` for every hashed file.

`lsp` serves LSP 3.18-compatible JSON-RPC over stdio using `Content-Length` framing. Editors supply and update documents with `didOpen`, full-document `didChange`, and `didClose`.

`mcp` serves MCP over newline-delimited stdio JSON-RPC. Repeat `--file` to explicitly preload every document agents may query. The server supports legacy `initialize`, modern `server/discover`, `ping`, `tools/list`, and `tools/call`. Stdout contains protocol messages only.

Exit status is `0` when no error-severity diagnostics are produced and `1` for errors, usage failures, or output failures. The CLI currently uses the distribution compatibility profile; there is no profile flag yet. Output paths may not alias a loaded source file.

The CLI is intentionally static: it does not import or start the Ikemen runtime. See [compatibility and corpus](COMPATIBILITY.md) for the files and profile behavior currently covered, and [LSP/MCP](LSP_MCP.md) for protocol contracts.
