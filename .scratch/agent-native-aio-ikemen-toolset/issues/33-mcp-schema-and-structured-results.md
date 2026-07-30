# 33 — MCP Input/Output Schema and Structured Results

**What to build:** Give every MCP tool complete JSON Schema 2020-12 contracts and machine-first structured results with compatible text summaries.

**Blocked by:** 32

**Status:** ready-for-agent

## User story

As an MCP agent, I want accurate schemas and structured results so that I can call tools and validate responses without parsing prose.

## Acceptance criteria

- [ ] Generate each tool’s `inputSchema` and `outputSchema` from versioned canonical contract definitions.
- [ ] Schemas declare required fields, enums, bounds, cursor types, mutually exclusive inputs, and additional-property policy.
- [ ] Results use `structuredContent` matching `outputSchema` plus a short text representation for compatible clients.
- [ ] Tool execution failures use `isError` and structured typed error details; JSON-RPC errors remain reserved for protocol/request failures.
- [ ] No successful response contains both incompatible result shapes or schema-invalid data.
- [ ] Schema IDs and versions are published in the schema catalog source tree.
- [ ] Tests validate every fixture/result against its advertised schema.
- [ ] Fuzz/property tests cover unknown fields, nulls, numeric boundaries, Unicode, and excessive nesting.

## UAT

1. Discover tools and compile every advertised schema with a JSON Schema 2020-12 validator.
2. Execute one success and one tool failure per operation family.
3. Validate `structuredContent` against `outputSchema`.
4. Send an out-of-range limit; expect a precise input validation error before execution.

## Non-goals

- Free-form prose as the canonical result.
- Independent hand-maintained duplicate schemas.
- Model-generated schema inference.
