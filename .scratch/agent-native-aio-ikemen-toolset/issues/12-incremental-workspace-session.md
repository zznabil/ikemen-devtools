# 12 — Build the incremental workspace session

**What to build:** A shared in-memory session that scans, parses, resolves, snapshots, cancels superseded work, and invalidates only affected dependents.

**Blocked by:** 11 — Define workspace manifest and snapshot identities

**Status:** ready-for-agent

- [ ] Existing coordinator/document snapshot behavior is preserved behind the workspace session.
- [ ] Reads flow through root authority and configured budgets.
- [ ] File changes invalidate dependent roots and semantic results deterministically.
- [ ] Cancellation leaves the last complete snapshot available.
- [ ] Concurrent read queries observe one committed immutable snapshot.
