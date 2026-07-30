# 17 — Persist complete snapshots transactionally

**What to build:** Transactional persistence for workspace files, symbols, references, diagnostics, edges, candidates, aggregates, and snapshot metadata.

**Blocked by:** 16 — Implement rebuildable cache lifecycle and locking

**Status:** ready-for-agent

- [ ] A snapshot becomes visible only after every related row commits.
- [ ] Existing repository identities and schema concepts are reused or migrated explicitly.
- [ ] Canonical insertion order and uniqueness constraints prevent duplicate semantic rows.
- [ ] Failed/cancelled writes leave the prior snapshot queryable.
- [ ] Round-trip tests compare persisted results with the in-memory canonical manifest.
