# LSP and MCP onboarding

The same `ikm` binary exposes both protocol servers. They share one semantic engine but deliberately use different stdio transports:

- `ikm lsp` uses LSP `Content-Length` framing.
- `ikm mcp` uses one JSON-RPC message per line.

Protocol stdout is clean. Operational errors go to stderr.

## LSP

Start the server:

```text
ikm lsp
```

The server advertises full-document text synchronization plus diagnostics, document symbols, hover, definition, and references. It accepts `textDocument/didOpen`, full-document `textDocument/didChange`, and `textDocument/didClose`; notifications receive no JSON-RPC response.

The package API remains available:

```go
server := lsp.NewServer()
ctx := context.Background()
_ = server.SetDocument(ctx, "hero.cmd", []byte("[Command]\nname = \"jump\"\n"))
_ = server.Serve(ctx, os.Stdin, os.Stdout)
```

Positions are zero-based LSP positions. `SetDocument` replaces an in-memory parsed document and never writes to disk.

## MCP

Start the server and explicitly preload queryable files:

```text
ikm mcp --file chars/hero/hero.def --file chars/hero/hero.cmd
```

MCP stdio is newline-delimited JSON-RPC. Do not send LSP `Content-Length` headers. The server supports:

- Modern discovery through `server/discover`, including the `2026-07-28` protocol.
- Legacy initialization for `2025-11-25`, `2025-06-18`, and `2024-11-05`.
- `ping`, `tools/list`, and `tools/call`.

`tools/list` returns five read-only tools with JSON Schema input contracts:

- `document_diagnostics` and `document_symbols`: `{ "uri": "..." }`
- `hover`, `definition`, and `references`: `{ "uri": "...", "line": 0, "character": 0 }`

The package API remains available for hosts that own document loading:

```go
server := mcp.NewServerWithVersion("1.0.0")
ctx := context.Background()
_ = server.SetDocument(ctx, "hero.cmd", source)
_ = server.Serve(ctx, os.Stdin, os.Stdout)
```

There are no write tools, runtime controls, rename operations, or patch application paths. The CLI reads only files explicitly named with `--file`; the server never discovers or walks a workspace. Unknown or malformed requests return JSON-RPC errors rather than panicking. Treat results as bounded static analysis, not proof of runtime behavior.
