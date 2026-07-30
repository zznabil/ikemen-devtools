# 24 — Ship bounded lexical search CLI

**What to build:** Fast FTS/text search across indexed authored files with deterministic ranking and clear lexical classification.

**Blocked by:** 19 — Build bounded query-store APIs

**Status:** ready-for-agent

- [ ] Search supports text, phrase, path, extension/kind, and package filters.
- [ ] Results include snippet, span, file kind, score, snapshot, and `lexical` classification.
- [ ] Ranking ties and snippets are deterministic.
- [ ] Query syntax, row, byte, and time limits prevent abuse.
- [ ] Search hits never appear as semantic graph edges without independent resolution.
