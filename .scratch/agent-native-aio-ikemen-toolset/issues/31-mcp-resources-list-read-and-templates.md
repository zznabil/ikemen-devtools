# 31 — MCP Resources List, Read, and Templates

**What to build:** Publish workspace metadata and source-backed entities through safe, paginated MCP resources and resource templates.

**Blocked by:** 30, 19

**Status:** ready-for-agent

## User story

As an MCP agent, I want stable resource URIs for files, snapshots, diagnostics, symbols, and entities so that I can retrieve context without inventing tool calls.

## Acceptance criteria

- [ ] Implement `resources/list`, `resources/read`, and `resources/templates/list` for the active MCP protocol.
- [ ] Define and document versioned custom `ikm://` URIs using opaque IDs or safely encoded root-relative paths.
- [ ] At minimum expose workspace summary, snapshot, source file, diagnostic collection, symbol, and semantic entity resources.
- [ ] Lists are deterministic, paginated, bounded, and return valid continuation cursors.
- [ ] Reads enforce root containment, size budgets, content type, encoding, and snapshot identity.
- [ ] Missing/malformed/escaped resource URIs return the protocol’s invalid-params error without leaking host paths.
- [ ] Resources carry useful names, descriptions, MIME types, and cache/version metadata.
- [ ] Binary assets return bounded metadata by default, not full multi-gigabyte content.
- [ ] Protocol and golden tests cover list, read, templates, pagination, stale snapshots, and malicious URIs.

## UAT

1. List workspace resources and page through them deterministically.
2. Read one source file and one symbol/entity resource.
3. Request a large SFF/SND; expect bounded metadata rather than the entire payload.
4. Attempt traversal in a URI; expect `-32602` and no external content.

## Non-goals

- Resource subscriptions.
- Arbitrary filesystem access.
- Mirroring every source file into one unbounded response.
