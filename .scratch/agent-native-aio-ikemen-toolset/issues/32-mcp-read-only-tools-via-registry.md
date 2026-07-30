# 32 — MCP Read-Only Tools via Registry

**What to build:** Replace transport-specific MCP query implementations with adapters generated from the canonical operation registry.

**Blocked by:** 30, 29, 05

**Status:** ready-for-agent

## User story

As an MCP agent, I want the same diagnostics, search, reference, graph, inspect, and export behavior as the CLI so that transport choice does not change semantics.

## Acceptance criteria

- [ ] Expose registry-backed tools for workspace status, diagnostics, symbol search, definition, references, lexical search, graph, impact, edge explanation, inspection, and bounded export.
- [ ] Tool names are stable, namespaced, deterministic in `tools/list`, and mapped one-to-one to canonical operations.
- [ ] Input decoding, defaults, validation, execution, and result projection reuse shared code.
- [ ] Read-only tool discovery works with writes and runtime disabled.
- [ ] Every result identifies schema version, operation, snapshot, profile, completeness, truncation, and provenance.
- [ ] Unsupported operation/profile combinations return typed tool errors rather than panics or empty success.
- [ ] Existing legacy tool names are retained only through a documented compatibility alias layer.
- [ ] Parity tests compare normalized CLI and MCP results for identical inputs.

## UAT

1. Run the same symbol, reference, and impact queries through CLI JSON and MCP.
2. Normalize transport envelopes and confirm canonical data equality.
3. Request an unsupported query for the selected profile; expect the same error code/details through both adapters.
4. Confirm `tools/list` order is stable across restarts.

## Non-goals

- Mutation tools.
- Runtime execution tools.
- A second MCP-only semantic model.
