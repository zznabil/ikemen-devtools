# 16 — Implement rebuildable cache lifecycle and locking

**What to build:** A disposable workspace cache that opens, validates, locks, quarantines/rebuilds, and preserves the last committed snapshot.

**Blocked by:** 15 — Adopt supported Go and embedded SQLite; 11 — Define workspace manifest and snapshot identities

**Status:** ready-for-agent

- [ ] Cache identity records schema, tool, IR, profile/config, root, and snapshot versions.
- [ ] Incompatible/corrupt caches are quarantined and rebuilt without modifying source files.
- [ ] One writer lock and concurrent committed-snapshot readers behave consistently across platforms.
- [ ] Cancellation or process failure cannot replace the last complete cache.
- [ ] `--cache off` provides behaviorally equivalent ephemeral operation.
