# 41 — Provenance, Ambiguity, and Explanation Unification

**What to build:** Make every semantic answer classify what is proven, inferred, lexical, ambiguous, unresolved, or unavailable and explain how it was derived.

**Blocked by:** 38, 39, 40

**Status:** ready-for-agent

## User story

As an agent, I want calibrated evidence on every answer so that I know what is safe to act on and what needs inspection.

## Acceptance criteria

- [ ] Define shared provenance classes, confidence semantics, completeness states, ambiguity groups, and refusal reasons.
- [ ] Every definition/reference/edge/impact/diagnostic/metadata result carries source evidence and derivation rule.
- [ ] Exact, inferred, lexical-only, ambiguous, unresolved, and unsupported results are structurally distinct.
- [ ] Explanation chains are bounded, deterministic, cycle-safe, and reference stable entity/edge IDs.
- [ ] Query filters can include/exclude provenance classes and set a minimum actionability policy.
- [ ] CLI, MCP, LSP, exports, and patch planning render/project the same underlying classification.
- [ ] No adapter upgrades confidence or drops ambiguity when translating a result.
- [ ] Golden tests cover conflicting definitions, dynamic strings, missing targets, profile differences, lexical fallbacks, and incomplete parses.

## UAT

1. Query one exact state reference, one inferred Lua path, one ambiguous asset, and one unresolved expression.
2. Confirm their classifications and explanations are visibly different and evidence-backed.
3. Filter to action-safe results; confirm ambiguous/inferred items are excluded.
4. Compare CLI and MCP results for identical classifications and evidence IDs.

## Non-goals

- A universal numeric probability score.
- Hiding incomplete analysis behind a success flag.
- LLM-generated explanations.
