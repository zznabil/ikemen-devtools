# 47 — Semantic Rename and Fix Providers

**What to build:** Implement conservative, provider-based semantic rename and diagnostic-fix planning across supported IKEMEN text formats.

**Blocked by:** 46, 23, 41

**Status:** ready-for-agent

## User story

As an agent, I want safe rename and fix operations backed by resolved references so that I can automate maintenance without blind text replacement.

## Acceptance criteria

- [ ] Define a mutation-provider interface that returns patch plans, confidence, provenance, unresolved candidates, and explicit refusal reasons.
- [ ] Implement rename providers for the symbol kinds proven by the semantic model; unsupported kinds refuse cleanly.
- [ ] Implement fix providers only for diagnostics with deterministic, semantics-preserving remedies.
- [ ] Rename updates definitions and resolved references but never ambiguous or lexical-only matches by default.
- [ ] Case, quoting, separators, comments, and local formatting are preserved where the grammar permits.
- [ ] Cross-file asset/path renames distinguish reference edits from physical file moves; no file move occurs implicitly.
- [ ] Providers expose `prepare`, `plan`, and validation through the shared operation registry.
- [ ] Every plan reports skipped candidates and why they were skipped.
- [ ] Corpus fixtures cover same-name symbols in different scopes, case variants, comments, malformed files, and ambiguous references.

## UAT

1. Prepare a state/controller rename with references across two files.
2. Confirm only semantically resolved references are planned.
3. Include an ambiguous same-name token; confirm it is reported but not edited.
4. Apply the plan and rerun diagnostics; expect the targeted diagnostic removed with no new errors.

## Non-goals

- Arbitrary regex replacement.
- Automatic asset file moves.
- Heuristic edits without traceable semantic evidence.
