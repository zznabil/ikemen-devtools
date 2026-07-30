# 22 — Ship symbol search CLI

**What to build:** Stable symbol search by name, kind, identity, file, character, stage, and workspace scope.

**Blocked by:** 19 — Build bounded query-store APIs

**Status:** ready-for-agent

- [ ] Exact identity lookup and name search are distinct modes.
- [ ] Results include semantic key, occurrence identity, kind, path, span, owner, and snapshot.
- [ ] Case/profile behavior and fuzzy ranking are explicit and deterministic.
- [ ] Pagination and budgets are enforced.
- [ ] Identity stability tests survive comments, relocation, and unrelated document changes.
