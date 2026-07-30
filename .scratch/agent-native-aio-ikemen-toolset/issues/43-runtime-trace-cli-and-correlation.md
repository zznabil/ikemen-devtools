# 43 — Runtime Trace CLI and Correlation

**What to build:** Expose guarded loader runs and trace inspection through the CLI, correlated back to graph entities and source spans.

**Blocked by:** 42

**Status:** ready-for-agent

## User story

As an agent operator, I want a structured runtime trace command so that I can reconcile an IKEMEN load failure with the exact static path and source evidence.

## Acceptance criteria

- [ ] Add `ikm runtime check`, `ikm runtime trace`, and `ikm runtime explain` over the oracle service.
- [ ] Commands require explicit runtime enablement, executable allowlist, root, timeout, and disposable-output policy.
- [ ] JSON results include run identity, engine identity, command summary, timings, bounded output references, loader events, correlations, and inconclusive evidence.
- [ ] Correlation links runtime paths/messages to canonical files, entities, graph edges, diagnostics, and source spans where defensible.
- [ ] Human output prioritizes failure point, likely loading chain, conflicting static/runtime evidence, and next inspection command.
- [ ] Cancellation and timeout terminate the entire child process tree and finalize a valid partial trace.
- [ ] Logs and large outputs are stored as bounded artifacts rather than embedded unbounded in protocol results.
- [ ] CLI tests use the fake runtime; opt-in UAT covers the bundled Windows executable.

## UAT

1. Run a broken roster-to-character fixture with `runtime trace`.
2. Inspect the failing loader event and traverse its correlated source/graph evidence.
3. Force a timeout; confirm the process tree is gone and the result is typed `timed_out`.
4. Run with runtime disabled; confirm no executable starts.

## Non-goals

- MCP runtime tools in v1 by default.
- Live gameplay debugger.
- Modifying game configuration to make a run pass.
