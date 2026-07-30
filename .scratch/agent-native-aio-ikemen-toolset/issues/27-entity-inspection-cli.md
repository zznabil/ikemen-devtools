# 27 — Ship workspace, character, stage, and file inspection

**What to build:** Compact structured summaries that let an agent understand an unfamiliar workspace or entity without assembling many low-level queries.

**Blocked by:** 19 — Build bounded query-store APIs; 11 — Define workspace manifest and snapshot identities

**Status:** ready-for-agent

- [ ] Workspace inspection reports entry points, active/orphaned entities, formats, health, and budgets.
- [ ] Character/stage inspection reports manifest, sources, assets, symbols, diagnostics, and relationships.
- [ ] File inspection reports kind, owner, declarations, references, diagnostics, and dependents.
- [ ] Summaries link to stable identities/resources rather than embedding unbounded data.
- [ ] Output is deterministic and paginates nested collections.
