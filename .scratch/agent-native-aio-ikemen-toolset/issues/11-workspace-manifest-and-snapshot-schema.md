# 11 — Define workspace manifest and snapshot identities

**What to build:** A canonical manifest and content-addressed snapshot schema covering files, entry points, relationships, diagnostics, profile/config, and truncation.

**Blocked by:** 09 — Implement deterministic workspace discovery; 10 — Classify files and inventory binary metadata

**Status:** ready-for-agent

- [ ] Snapshot identity excludes timestamps and absolute root strings.
- [ ] File, edge, diagnostic, candidate, budget, and aggregate records have versioned schemas.
- [ ] Relocating unchanged workspace content preserves relative identities.
- [ ] Partial scans produce distinct snapshots with explicit truncation reasons.
- [ ] Golden/property tests prove determinism across traversal and input ordering.
