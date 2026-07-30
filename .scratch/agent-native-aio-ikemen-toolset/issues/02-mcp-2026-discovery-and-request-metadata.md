# 02 — Implement MCP 2026 discovery and request metadata

**What to build:** Modern stateless MCP version negotiation with current `server/discover`, per-request metadata, capability, cache, and server identity response shapes.

**Blocked by:** 01 — Make JSON-RPC core conformant

**Status:** ready-for-agent

- [ ] `server/discover` follows the `2026-07-28` shape, including server identity under protocol metadata.
- [ ] Every modern request validates required protocol/client metadata and the negotiated version.
- [ ] Unsupported versions return the specified typed error without connection state.
- [ ] Discovery advertises only capabilities actually available under current server authority.
- [ ] Official discovery examples and negative cases are wire-tested over stdio.
