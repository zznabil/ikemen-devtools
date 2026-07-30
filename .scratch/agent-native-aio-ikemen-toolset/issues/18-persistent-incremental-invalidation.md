# 18 — Persist incremental invalidation

**What to build:** Hash- and dependency-based incremental refresh that updates only affected documents, semantic results, graph edges, and aggregates.

**Blocked by:** 12 — Build the incremental workspace session; 17 — Persist complete snapshots transactionally

**Status:** ready-for-agent

- [ ] No-change scans avoid reparsing unchanged dependencies.
- [ ] Changed, added, deleted, renamed, and previously unreadable files invalidate correct dependents.
- [ ] Profile/config/adapter changes trigger the documented rebuild scope.
- [ ] Incremental and full rebuild outputs are byte-equivalent.
- [ ] Warm reference-workspace rescan meets the performance objective.
