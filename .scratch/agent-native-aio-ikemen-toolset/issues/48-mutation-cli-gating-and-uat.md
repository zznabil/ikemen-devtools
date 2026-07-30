# 48 — Mutation CLI Gating and UAT

**What to build:** Expose mutation workflows through a deliberately gated CLI with preview-first ergonomics, machine-readable prompts, and end-to-end safety tests.

**Blocked by:** 46, 47

**Status:** ready-for-agent

## User story

As an agent operator, I want write commands to be unmistakable and scriptable so that automation never applies a patch through an accidental default.

## Acceptance criteria

- [ ] Add `ikm rename prepare`, `ikm fix prepare`, `ikm patch plan`, `ikm patch diff`, `ikm patch apply`, and `ikm patch recover`.
- [ ] Read-only preparation and preview commands need no write opt-in.
- [ ] Apply requires both an explicit subcommand and `--allow-write`; missing authorization returns the documented policy exit code.
- [ ] Non-interactive mode never prompts and emits a complete typed remediation message.
- [ ] Human interactive confirmation, when enabled, names the root, file count, edit count, and rollback behavior.
- [ ] `--json` uses the common envelope and never mixes logs into stdout.
- [ ] Help and examples show the safe plan-review-apply sequence first.
- [ ] End-to-end tests exercise success, refusal, stale plan, interrupted transaction recovery, and malformed plan input.

## UAT

1. Run a rename preparation and diff in a disposable fixture.
2. Run apply without `--allow-write`; expect a policy refusal and unchanged hashes.
3. Run apply with the flag; expect an atomic change and structured result.
4. Pipe JSON mode into a parser and confirm stderr logging does not corrupt stdout.

## Non-goals

- MCP write exposure.
- Automatic Git commits.
- Destructive cleanup outside transaction artifacts.
