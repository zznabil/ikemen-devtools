# 39 — Lua and ZSS Adapter Hardening

**What to build:** Harden Lua and ZSS adapters enough to contribute honest symbols, references, diagnostics, and distribution edges without claiming unsupported whole-language semantics.

**Blocked by:** 37

**Status:** ready-for-agent

## User story

As an agent, I want useful Lua/ZSS navigation with explicit confidence so that script-driven loading behavior is searchable without misleading certainty.

## Acceptance criteria

- [ ] Document the supported Lua and ZSS syntax/semantic subset and profile behavior.
- [ ] Produce stable document symbols, definitions, local references, includes/imports, string-path references, and syntax diagnostics for that subset.
- [ ] Recognize the distribution’s known loader/config access patterns through isolated, tested adapter rules.
- [ ] Dynamic/metaprogrammed references are emitted as inferred or unresolved candidates, never proven edges.
- [ ] Malformed or unsupported constructs preserve partial results and completeness metadata.
- [ ] Adapter crashes, recursion, and pathological files are bounded by shared budgets/cancellation.
- [ ] Symbols and edges use the common identity, provenance, ambiguity, and explanation model.
- [ ] Golden fixtures include real project snippets plus dynamic lookup, nested tables, long strings, comments, and incomplete code.

## UAT

1. Index the active startup and motif scripts.
2. Find known configuration/select-file references and inspect their confidence/evidence.
3. Query an intentionally dynamic lookup; expect an inferred/unresolved result rather than a false exact definition.
4. Introduce malformed Lua and confirm partial symbols plus diagnostics remain available.

## Non-goals

- Replacing a full Lua compiler/runtime.
- Executing project scripts.
- Claiming sound cross-module type inference.
