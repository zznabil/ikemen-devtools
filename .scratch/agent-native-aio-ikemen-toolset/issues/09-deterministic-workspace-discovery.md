# 09 — Implement deterministic workspace discovery

**What to build:** Entry-point-driven discovery of active and orphaned IKEMEN ecosystem files with stable ordering, ignore rules, and bounded fallback inventory.

**Blocked by:** 08 — Enforce root containment and path sandboxing

**Status:** ready-for-agent

- [ ] Runtime config, motif, system, select, roster, stages, manifests, scripts, mods, and assets are traversed from evidence.
- [ ] Fallback inventory identifies orphaned packages without marking them active.
- [ ] `.ikmignore`, built-in output exclusions, includes, excludes, and file limits are deterministic.
- [ ] Every discovered file records why and from which entry point it was found.
- [ ] Repeated discovery produces byte-identical ordered records.
