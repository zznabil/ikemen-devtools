# 20 — Add cache doctor and maintenance behavior

**What to build:** Readable diagnostics and safe maintenance operations for cache status, corruption, version skew, lock contention, size, rebuild, and cleanup.

**Blocked by:** 16 — Implement rebuildable cache lifecycle and locking

**Status:** ready-for-agent

- [ ] Doctor reports cache path, versions, snapshot, health, lock owner state, size, and rebuild reason.
- [ ] Rebuild and cleanup are explicit, root-contained, and never touch source files.
- [ ] Active locks and permission failures produce typed non-destructive errors.
- [ ] Corruption fixtures prove quarantine/recovery.
- [ ] Human and JSON output give the same actionable diagnosis.
