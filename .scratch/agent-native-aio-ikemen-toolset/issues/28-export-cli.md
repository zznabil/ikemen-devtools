# 28 — Ship canonical export CLI

**What to build:** Snapshot-aware JSONL, SCIP, and SQL export commands with atomic output, schemas, provenance, and bounded selection.

**Blocked by:** 19 — Build bounded query-store APIs

**Status:** ready-for-agent

- [ ] Existing SQL and SCIP exporters are wired through the shared operation contract.
- [ ] JSONL streams manifest/file/symbol/reference/diagnostic/edge records in deterministic order.
- [ ] Export scope, snapshot, profile, classification, and truncation are recorded.
- [ ] Output cannot alias source/cache inputs and is written atomically.
- [ ] Golden exports are byte-stable and schema-valid.
