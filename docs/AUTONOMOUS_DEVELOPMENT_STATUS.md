# Autonomous development status — 2026-07-30

## Summary

The first post-MVP product slice is complete. Ikemen Devtools is now a single Windows CLI binary with analysis, corpus, metadata, LSP, and MCP entry points:

```text
ikm check
ikm index
ikm corpus
ikm metadata
ikm lsp
ikm mcp
```

Built artifact: `dist/ikm.exe` (`0.1.0-alpha.1`, 4,374,528 bytes).

SHA-256: `9C7F9DAEC4B52E241ACAE12958BEFA4588B06985CB9B494BDE113B10462EAF8B`

## Architecture decision

One binary with subcommands is the default long-term shape.

- The Go packages remain the semantic core.
- CLI, LSP, MCP, future IDE adapters, and graph exporters are thin consumers.
- LSP and MCP share semantics, not transport framing.
- Agent access starts read-only and explicit. `ikm mcp` only preloads repeatable `--file` paths.
- Runtime claims require differential-oracle evidence.

This keeps deployment simple without coupling the semantic model to one editor or agent protocol.

## User stories

1. As a modder, I want one `ikm.exe` so I can analyze a character or start editor tooling without installing separate services.
2. As an editor, I want standard LSP document lifecycle behavior so diagnostics and symbols follow the current buffer.
3. As an AI agent, I want a protocol-correct, read-only MCP server with typed tools so queries are safe and machine-discoverable.

## Acceptance criteria

- [x] One compiled binary exposes `check`, `index`, `corpus`, `metadata`, `lsp`, and `mcp`.
- [x] LSP uses `Content-Length` framing.
- [x] LSP accepts `didOpen`, full-document `didChange`, and `didClose`.
- [x] MCP stdio uses newline-delimited JSON-RPC, not LSP framing.
- [x] MCP supports `server/discover`, legacy `initialize`, `ping`, `tools/list`, and `tools/call`.
- [x] MCP publishes JSON Schema inputs for all five semantic tools.
- [x] MCP can query files explicitly preloaded with repeatable `--file`.
- [x] JSON-RPC notifications produce no response.
- [x] Malformed JSON produces a parse-error response without a panic.
- [x] Existing package tests, race tests, vet, and formatting checks pass.

## What changed

- Added `ikm lsp` and `ikm mcp` command adapters.
- Added help as a successful command and documented all six entry points.
- Split MCP newline framing from LSP `Content-Length` framing.
- Added MCP version negotiation/discovery and real tool schemas.
- Added flat `{uri, line, character}` MCP tool arguments.
- Added LSP in-memory document lifecycle and URI/path aliases.
- Added protocol, CLI, transport, notification, schema, and preload tests.
- Updated README, CLI, LSP/MCP, safety, and evolution documentation.

## Verification evidence

Executed from `_upstream/ikemen-devtools`:

```text
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
gofmt -l .
go build -trimpath -ldflags "-X main.releaseVersion=0.1.0-alpha.1" -o dist/ikm.exe ./cmd/ikm
```

Results:

- Full unit suite: pass across every package.
- Race detector: pass across every package.
- Vet: pass with no findings.
- Formatting: every Go file formatted.
- Windows build: pass.

## Compiled-binary UAT

| Scenario | Real input | Expected | Result |
|---|---|---|---|
| Help | `ikm help` | Exit 0; lists LSP and MCP | Pass |
| Corpus | Distribution `data/select.def` | Valid JSON report | Pass; exit 1 reflects existing error-severity corpus diagnostics |
| Character check | `Uchiha Sasuke War.def` | Valid JSON report | Pass; exit 1 reflects existing error-severity character diagnostics |
| MCP discovery | `server/discover` | `2026-07-28`; newline transport | Pass |
| MCP semantic tool | Explicitly preload the real character DEF | Structured symbol result | Pass |
| MCP notification | `notifications/initialized` without id | Zero response bytes | Pass |
| Malformed MCP input | `{` | JSON-RPC parse error `-32700` | Pass |
| LSP initialize | Content-Length request | Content-Length response | Pass |

## Known boundaries

- This directory is not a Git checkout, so no commit, branch, or PR was created.
- The real distribution currently produces error-severity diagnostics. This slice preserves that behavior and confirms the reports are valid JSON; it does not hide or downgrade findings.
- `=/`, `stcommon`, duplicate-state precedence, and duplicate-command precedence still need authoritative runtime fixtures. Generic IKEMEN path-search code proves slash normalization and search ordering, but not those character-loader rules.
- MCP `server/discover` follows the 2026 protocol direction while legacy initialization remains available for existing clients.
- MCP has no workspace crawl, write tool, runtime control, rename, or patch application capability.

## Next milestone

Build a shared bounded workspace/session contract:

1. Add canonical `--root` and `--profile` selection.
2. Load deterministic workspace manifests instead of ad hoc file lists.
3. Expose MCP resources for files, diagnostics, symbols, and graph slices.
4. Reuse incremental snapshots across LSP and MCP requests.
5. Enforce file-count, byte, request-time, and graph-depth budgets.
6. Add runtime-oracle fixtures before changing compatibility precedence.

This is the smallest next step that turns the working binary into a dependable daily tool without prematurely adding mutation or runtime control.

## TL;DR

The single-binary alpha is real, protocol-correct, tested, and built. The next evolution is a shared bounded workspace model, then richer agent resources and IDE packaging.
