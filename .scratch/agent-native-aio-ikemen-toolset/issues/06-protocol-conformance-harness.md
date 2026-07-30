# 06 — Add the protocol conformance harness

**What to build:** A reusable compiled-binary conformance suite for JSON-RPC, modern/legacy MCP, schema validation, transport cleanliness, and cancellation.

**Blocked by:** 01 — Make JSON-RPC core conformant; 02 — Implement MCP 2026 discovery and request metadata; 03 — Preserve legacy MCP compatibility; 05 — Build the shared capability registry

**Status:** ready-for-agent

- [ ] Scenarios exercise real stdin/stdout rather than only in-process handlers.
- [ ] Stdout pollution, response-shape, ID, batch, notification, and version failures are detected.
- [ ] Advertised input/output schemas validate representative results.
- [ ] Malformed and oversized messages cannot panic or allocate without bound.
- [ ] The harness runs in CI on Windows, Linux, and macOS.
