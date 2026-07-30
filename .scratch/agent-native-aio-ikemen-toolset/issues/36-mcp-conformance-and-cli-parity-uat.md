# 36 — MCP Conformance and CLI Parity UAT

**What to build:** Establish the complete MCP release gate using current-protocol, legacy-fallback, schema, resource, tool, concurrency, and CLI-parity fixtures.

**Blocked by:** 35

**Status:** ready-for-agent

## User story

As a maintainer, I want executable protocol evidence so that “MCP support” means interoperable behavior, not a happy-path demo.

## Acceptance criteria

- [ ] Test modern discovery metadata, selected protocol, initialization, capability listing, and shutdown.
- [ ] Test documented legacy initialize fallback without weakening modern responses.
- [ ] Validate JSON-RPC result/error exclusivity, IDs, notifications, errors, and batch behavior.
- [ ] Validate all resource and tool schemas plus representative successful, partial, stale, cancelled, and failed results.
- [ ] Compare every shared read operation with CLI JSON output after canonical normalization.
- [ ] Run tests through the compiled `ikm mcp` stdio process, not only in-process handlers.
- [ ] Include multiple client-behavior fixtures and a transcript artifact suitable for release evidence.
- [ ] The gate fails on stdout contamination, nondeterminism, unbounded result, root escape, schema mismatch, or parity drift.

## UAT

1. Run the conformance harness against a release-mode binary and representative fixture.
2. Complete both modern and legacy negotiation paths.
3. Exercise resources, tools, pagination, cancellation, batches, and errors.
4. Confirm all transcripts validate and normalized CLI/MCP results match.

## Non-goals

- Certification by an external organization.
- Supporting undeclared protocol drafts.
- Testing mutation or runtime capabilities.
