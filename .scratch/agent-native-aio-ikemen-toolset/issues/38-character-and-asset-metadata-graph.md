# 38 — Character and Asset Metadata Graph

**What to build:** Extend the relationship graph through character manifests and bounded binary asset metadata.

**Blocked by:** 37, 10

**Status:** ready-for-agent

## User story

As an agent, I want a character’s DEF, CNS/CMD/AIR files, palettes, sprites, sounds, and dependencies connected so that I can inspect packaging and breakage without opening giant binaries.

## Acceptance criteria

- [ ] Parse character `[Files]`, palette, arcade, and relevant dependency fields into typed entities with spans.
- [ ] Resolve CNS/CMD/AIR/SFF/SND/common and palette relationships relative to the character package.
- [ ] Extract only bounded, documented header/metadata fields from supported binary assets; never load full large assets by default.
- [ ] Record asset size, identity/hash policy, format/version, readable metadata, and parse completeness.
- [ ] Model missing, duplicate, case-mismatched, escaped, unsupported, and corrupt asset references explicitly.
- [ ] Provide reverse edges from files/assets to referring characters and manifest fields.
- [ ] Cache binary metadata by file identity and invalidate it only when the asset changes.
- [ ] Fixtures cover alternate DEF layouts, repeated slots, omitted optional assets, corrupt headers, huge sparse files, and shared resources.

## UAT

1. Inspect a representative character and list every referenced source/binary asset with source evidence.
2. Query reverse references for its SFF and confirm the manifest field is reported.
3. Rename an asset in a fixture without updating the DEF; expect a missing dependency.
4. Scan a multi-gigabyte asset and confirm metadata stays within the configured header and memory budget.

## Non-goals

- Rendering sprites or playing sounds.
- Full SFF/SND extraction.
- Automatic package repair.
