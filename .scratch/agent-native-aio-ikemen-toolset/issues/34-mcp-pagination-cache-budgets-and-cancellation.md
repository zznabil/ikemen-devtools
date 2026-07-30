# 34 — MCP Pagination, Cache, Budgets, and Cancellation

**What to build:** Enforce bounded, resumable, cancellable MCP reads using opaque cursors and snapshot-aware caching.

**Blocked by:** 31, 32, 33

**Status:** ready-for-agent

## User story

As an MCP agent, I want predictable result limits and continuation so that large IKEMEN projects cannot overflow my context window or hang the server.

## Acceptance criteria

- [ ] All list/search/graph/export operations honor default and maximum item, byte, depth, node, and duration budgets.
- [ ] Truncated results say why they stopped and provide an opaque continuation cursor when continuation is valid.
- [ ] Cursors are bound to operation, normalized inputs, root, profile, schema version, and snapshot; tampering or staleness returns a typed error.
- [ ] Cached results are keyed by the same identities and never cross workspace or profile boundaries.
- [ ] Request cancellation propagates through parsing, repository queries, graph traversal, and serialization.
- [ ] Timeout and cancellation responses distinguish partial safe results from no-result failures.
- [ ] Server-side work stops promptly after cancellation and does not leak goroutines or locks.
- [ ] Tests cover exact page boundaries, cursor replay/tampering, snapshot changes, output byte limits, timeouts, and cancellations.

## UAT

1. Query more symbols than the default page size and follow cursors to exhaustion.
2. Confirm concatenated pages equal the bounded CLI export order with no duplicates.
3. Edit the workspace and reuse the old cursor; expect a stale-cursor response.
4. Cancel a large graph request and confirm quick termination plus a subsequent healthy request.

## Non-goals

- Infinite server-side cursor storage.
- Unbounded “return all” modes.
- Persisting client session state across server restarts.
