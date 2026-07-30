# 54 — Docs, Schema Catalog, Agent Recipes, and Migrations

**What to build:** Ship task-oriented documentation and machine-readable contracts that let a new human or AI agent use and upgrade the product without source-code archaeology.

**Blocked by:** 36, 48, 52

**Status:** ready-for-agent

## User story

As a new agent integrator, I want authoritative commands, schemas, examples, and migration notes so that I can use `ikm` correctly on the first attempt.

## Acceptance criteria

- [ ] Publish a concise architecture overview explaining core operations and the CLI/MCP/LSP adapter boundary.
- [ ] Document installation, workspace config, profiles, cache lifecycle, budgets, security model, and compatibility guarantees.
- [ ] Generate a versioned schema catalog for configuration, JSON envelopes, entities, graph/query results, patch plans, MCP tools, and MCP resources.
- [ ] Provide copy-paste CLI recipes and equivalent MCP request/result examples for scan, diagnostics, symbols, references, graph, impact, inspect, export, and guarded mutation.
- [ ] Include agent recipes for capability discovery, bounded pagination, stale snapshot recovery, cancellation, plan-review-apply, and runtime oracle evidence.
- [ ] Publish upgrade/migration guidance for config, database/schema, output contract, tool names, and deprecations.
- [ ] Examples are tested as fixtures or executable documentation against the release binary.
- [ ] Troubleshooting maps stable error/finding codes to remedies and explicitly separates stderr logs from stdout protocol data.
- [ ] Documentation states that no GUI, daemon, cloud service, bundled model, or autonomous runtime execution is required.

## UAT

1. Give the packaged binary and documentation to a clean agent session with no repository source.
2. Have it discover capabilities, scan a fixture, query references, inspect an edge, and prepare a safe patch.
3. Validate every request/result against the published schemas.
4. Follow an old-schema migration fixture and confirm the documented result.

## Non-goals

- Marketing site.
- IDE-specific tutorials beyond minimal LSP client configuration.
- Unversioned examples that bypass canonical operations.
