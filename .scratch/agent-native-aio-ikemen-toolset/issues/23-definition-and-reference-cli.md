# 23 — Ship definition and reference CLI

**What to build:** Definition and reference queries by source position or stable symbol/reference identity with exact, ambiguous, unresolved, and dynamic outcomes.

**Blocked by:** 22 — Ship symbol search CLI

**Status:** ready-for-agent

- [ ] Path/line/column and identity inputs share one semantic resolver.
- [ ] Exact results include target and provenance; ambiguity includes ordered candidates.
- [ ] Unresolved/dynamic results explain why no exact answer exists.
- [ ] Reference inclusion of declarations is explicit.
- [ ] CLI and existing LSP behavior agree on shared fixtures.
