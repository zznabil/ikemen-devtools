# 30 — MCP Server Workspace Configuration

**What to build:** Make MCP a workspace-scoped, stateless adapter over the shared service with explicit startup policy instead of ad hoc file preloads.

**Blocked by:** 13, 18, 06

**Status:** ready-for-agent

## User story

As an MCP client, I want to start `ikm mcp` against an explicit project root so that every tool operates on the same validated workspace and cache.

## Acceptance criteria

- [ ] `ikm mcp --root <path>` resolves the canonical root, effective config, profile, budgets, and persistent session before reading requests.
- [ ] The server never depends on deprecated client roots; a workspace can also be supplied by the documented server configuration mechanism.
- [ ] Startup failures are emitted on stderr and cause a nonzero process exit without corrupting JSON-RPC stdout.
- [ ] Active capabilities reflect policy flags for reads, writes, runtime execution, resources, and protocol versions.
- [ ] Read-only is the default; mutation and runtime execution require independent explicit flags.
- [ ] Requests cannot override the configured root or escape it through file parameters.
- [ ] Multiple server processes can read one workspace cache safely; lock conflicts return actionable typed errors.
- [ ] Tests cover relative/absolute roots, missing config, invalid profile, read-only root, and stdio cleanliness.

## UAT

1. Start MCP against a fixture root and complete discovery/initialization.
2. Query a known diagnostic and confirm it comes from that workspace snapshot.
3. Pass an outside-root file parameter; expect a typed invalid-params response.
4. Redirect stdout to a JSON-RPC parser and confirm startup logs appear only on stderr.

## Non-goals

- A resident daemon.
- Client-selected roots after startup.
- Network transport authentication.
