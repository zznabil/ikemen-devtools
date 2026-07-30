# 03 — Preserve legacy MCP compatibility

**What to build:** A bounded legacy initialization adapter for older MCP hosts without contaminating the modern stateless operation core.

**Blocked by:** 01 — Make JSON-RPC core conformant; 02 — Implement MCP 2026 discovery and request metadata

**Status:** ready-for-agent

- [ ] Legacy initialization negotiates supported `2025-11-25`, `2025-06-18`, and `2024-11-05` versions.
- [ ] Modern clients can probe discovery first and older clients can fall back cleanly.
- [ ] Version-specific response differences are isolated in adapters.
- [ ] Unsupported/malformed legacy handshakes fail deterministically.
- [ ] Compatibility tests cover each retained version and the future removal seam.
