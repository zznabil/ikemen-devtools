# 44 — Compatibility Promotion Gate

**What to build:** Define an evidence-based process for promoting dialect/profile rules from guessed or inferred behavior to supported compatibility guarantees.

**Blocked by:** 42, 43

**Status:** ready-for-agent

## User story

As a maintainer, I want compatibility rules promoted only after static and runtime evidence agree so that strict diagnostics do not encode folklore as fact.

## Acceptance criteria

- [ ] Define lifecycle states for rules: experimental, inferred, runtime-observed, fixture-confirmed, and supported.
- [ ] Each rule records engine/profile scope, source references, fixtures, oracle runs, counterexamples, confidence class, and owner/version history.
- [ ] Promotion requires deterministic static tests plus the documented runtime evidence threshold.
- [ ] Contradictions automatically block promotion and remain visible in doctor/export evidence.
- [ ] Strict diagnostics and safe mutation providers consume only rule states allowed by policy.
- [ ] A machine-readable compatibility matrix is generated from rule data, not manually duplicated prose.
- [ ] Regression tests run all supported fixtures and flag engine-version behavior changes.
- [ ] Demotion/deprecation is explicit, versioned, and never silently changes historical evidence.

## UAT

1. Take an experimental path-resolution rule through static fixtures and runtime oracle confirmation.
2. Promote it and confirm strict diagnostics now use it for the matching profile.
3. Add a counterexample; confirm promotion is blocked or the supported rule regresses visibly.
4. Generate the compatibility matrix and trace each supported claim to evidence.

## Non-goals

- Claiming exhaustive compatibility with every MUGEN fork.
- Automatic promotion from a single log.
- Hiding unresolved engine differences.
