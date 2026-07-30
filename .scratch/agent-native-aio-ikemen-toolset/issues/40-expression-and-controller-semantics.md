# 40 — Expression and Controller Semantics

**What to build:** Deepen CNS/CMD expression and controller analysis into typed, scope-aware symbols and references suitable for reliable queries and future fixes.

**Blocked by:** 12

**Status:** ready-for-agent

## User story

As an agent, I want state, controller, trigger, variable, command, animation, sound, helper, and resource references resolved in context so that navigation reflects how IKEMEN code actually works.

## Acceptance criteria

- [ ] Define typed semantic entities and scope rules for supported CNS/CMD/AIR constructs and profiles.
- [ ] Resolve state definitions/calls, command declarations/uses, animation actions, sound groups/items, variables, helpers, targets, and supported controller parameters.
- [ ] Distinguish numeric coincidence from a semantic reference using syntactic role and controller context.
- [ ] Report dynamic expressions as bounded candidate sets or unresolved, with explanation and confidence.
- [ ] Validate controller parameter presence/types/ranges only where profile rules are authoritative.
- [ ] Incremental recomputation tracks dependencies between declarations, expressions, and consuming controllers.
- [ ] Hover/inspect facts are derived from the same semantic entities used by CLI/MCP/LSP.
- [ ] Corpus tests cover shadowing/scopes, constants, expressions, redirections, malformed controllers, dialect differences, and large state files.

## UAT

1. Resolve a command trigger, state transition, animation reference, and sound reference in a representative character.
2. Inspect each result and confirm role, scope, definition, evidence, and profile.
3. Query a computed state number; expect candidate/unresolved status rather than a fabricated exact target.
4. Change one declaration and confirm dependent references/diagnostics refresh.

## Non-goals

- Full runtime evaluation of arbitrary expressions.
- AI gameplay simulation.
- Enforcing undocumented engine behavior as errors.
