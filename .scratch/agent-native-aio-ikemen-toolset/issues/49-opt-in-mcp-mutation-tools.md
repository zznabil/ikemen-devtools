# 49 — Opt-In MCP Mutation Tools

**What to build:** Add MCP mutation tools as a disabled-by-default adapter over the validated patch operations.

**Blocked by:** 36, 48

**Status:** ready-for-agent

## User story

As an AI agent using MCP, I want the same safe mutation workflow as the CLI so that tool transport does not weaken review, authorization, or rollback guarantees.

## Acceptance criteria

- [ ] MCP mutation tools are absent from `tools/list` unless the server starts with an explicit write capability flag.
- [ ] Enabling writes does not enable runtime execution; capabilities remain independent.
- [ ] Expose prepare/plan/diff/apply/recover operations through the shared registry with schemas derived from the canonical contracts.
- [ ] Apply requires a short-lived plan token bound to server instance, workspace identity, plan hash, and current snapshot.
- [ ] Tool descriptions state that preview does not write and apply is transactional.
- [ ] Structured results include changed files, transaction ID, postcondition status, warnings, and recovery guidance.
- [ ] Tool errors use `isError` plus typed structured details and do not leak absolute host paths unless explicitly requested by policy.
- [ ] Cancellation before commit writes nothing; cancellation during commit completes or rolls back before responding.
- [ ] MCP tests prove mutation results match CLI results for the same operation.

## UAT

1. Start MCP without write authorization and list tools; confirm mutation tools are absent.
2. Start with write authorization, prepare and diff a rename, then apply its token.
3. Reuse the token; expect a typed invalid/consumed-token failure.
4. Change a target after preparation; expect no writes and stale-token details.

## Non-goals

- Unreviewed one-call rename-and-apply.
- Remote authentication or multi-tenant authorization.
- Runtime process execution.
