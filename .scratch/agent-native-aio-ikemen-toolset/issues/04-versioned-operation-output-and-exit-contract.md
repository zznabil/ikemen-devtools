# 04 — Define the versioned operation, output, and exit contract

**What to build:** A shared typed operation result envelope and typed CLI exit-code policy that can be consumed identically by CLI and MCP adapters.

**Blocked by:** None — can start immediately

**Status:** ready-for-agent

- [ ] Canonical output includes schema, operation, tool, workspace, snapshot, result, diagnostics, page, and truncation fields.
- [ ] Canonical output excludes timestamps, durations, absolute host paths, and random IDs by default.
- [ ] Exit codes distinguish findings, usage, input, internal, budget, conflict, and runtime failures.
- [ ] Existing commands retain a documented migration/legacy mode.
- [ ] Golden tests prove byte-identical output for identical inputs.
