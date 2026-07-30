# 37 — Screenpack, Select, and Stage Relationship Graph

**What to build:** Model the distribution-level loading chain from config and motif through roster entries, characters, stages, and common assets.

**Blocked by:** 11, 25

**Status:** ready-for-agent

## User story

As an agent, I want to trace how the active IKEMEN distribution reaches a character or stage so that I can answer “why is this loaded?” with evidence.

## Acceptance criteria

- [ ] Add typed nodes/edges for config-to-motif, motif-to-select, select sections/rows, character entries, stage entries, options, and common resources.
- [ ] Preserve roster order, separator/empty/random rows, path spelling, options, and source spans.
- [ ] Resolve normalized paths without losing their authored representation.
- [ ] Represent missing, ambiguous, conditional, and profile-dependent targets explicitly.
- [ ] Graph queries explain each edge with source file, span, syntax, resolution rule, and confidence/provenance.
- [ ] Incremental invalidation updates only affected distribution relationships.
- [ ] Fixtures cover Windows separators, implicit extensions, duplicate display entries, alternates/backups, malformed sections, and missing files.
- [ ] CLI graph/impact/inspect operations expose the new relationships without special transport logic.

## UAT

1. Starting at `save/config.json`, traverse to the active motif, `select.def`, a chosen roster row, its character DEF, and one stage.
2. Confirm every hop includes source evidence and authored path text.
3. Break one roster path and rescan; expect a missing-target edge and diagnostic without losing the row.
4. Edit only `select.def`; confirm unrelated character semantic snapshots remain reusable.

## Non-goals

- Simulating every Lua runtime branch.
- Reordering the roster.
- Editing distribution files.
