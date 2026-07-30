# 19 — Build bounded query-store APIs

**What to build:** Snapshot-aware repository queries for diagnostics, symbols, references, lexical search, files, entities, and graph edges with stable pagination.

**Blocked by:** 17 — Persist complete snapshots transactionally

**Status:** ready-for-agent

- [ ] Every query requires or resolves one committed snapshot.
- [ ] Sorting and opaque cursor behavior are stable across calls.
- [ ] Limits, timeouts, cancellation, and maximum visited rows are enforced.
- [ ] Lexical results are never returned as exact semantic results.
- [ ] In-memory and persistent backends satisfy one contract test suite.
