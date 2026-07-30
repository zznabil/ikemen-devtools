# 45 — Patch Plan and Diff Artifact

**What to build:** Add a shared, serializable patch-plan operation that turns semantic edits into a deterministic preview before any file is changed.

**Blocked by:** 41, 04

**Status:** ready-for-agent

## User story

As an agent, I want every proposed mutation represented as a reviewable plan so that I can verify intent, evidence, and blast radius before applying it.

## Acceptance criteria

- [ ] Add a versioned `PatchPlan` schema with operation ID, workspace identity, profile, input snapshot, file preconditions, ordered edits, explanations, warnings, and expected postconditions.
- [ ] Each edit carries a root-relative canonical path, byte span, original hash, replacement text, and semantic reason.
- [ ] Plans are deterministically ordered and serialize identically for identical inputs.
- [ ] The shared operation registry exposes plan generation; CLI and future MCP adapters do not reimplement it.
- [ ] `ikm patch plan` supports human output and the standard JSON envelope.
- [ ] `ikm patch diff` emits a unified diff without modifying the workspace.
- [ ] Stale hashes, overlapping edits, invalid UTF-8 targets, and paths outside the root are rejected with typed diagnostics.
- [ ] Plans contain enough provenance to explain which symbol/reference/diagnostic caused each edit.
- [ ] Unit and golden tests cover stable serialization, diff output, overlap rejection, and stale-input rejection.

## UAT

1. Generate a plan that renames one controller reference in two CNS files.
2. Confirm both files appear in deterministic order with hashes, reasons, and a readable unified diff.
3. Confirm workspace file hashes are unchanged after `plan` and `diff`.
4. Change one source file and rerun the saved plan; expect a typed stale-plan failure and no writes.

## Non-goals

- Applying edits.
- Inventing semantic rename rules.
- Interactive diff UI.
