# 25 — Ship dependency graph CLI

**What to build:** Dependency, dependent, and bounded path queries over typed file, symbol, roster, stage, configuration, asset, and runtime-evidence edges.

**Blocked by:** 19 — Build bounded query-store APIs; 11 — Define workspace manifest and snapshot identities

**Status:** ready-for-agent

- [ ] Every edge includes stable ID, kind, endpoints, source span, resolver, classification, evidence, and snapshot.
- [ ] Exact, ambiguous, unresolved, dynamic, lexical, and runtime-observed edges remain separable.
- [ ] Depth, node, edge, time, and output budgets are enforced.
- [ ] Path queries are deterministic under equal alternatives.
- [ ] Synthetic cycles and large graphs cannot hang or explode memory.
