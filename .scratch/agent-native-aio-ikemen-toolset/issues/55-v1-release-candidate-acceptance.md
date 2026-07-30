# 55 — v1 Release Candidate Acceptance

**What to build:** Execute and record the final product acceptance gate for the agent-native, single-binary v1 release.

**Blocked by:** 53, 54

**Status:** ready-for-agent

## User story

As the product owner, I want one auditable release-candidate decision so that “v1” means the whole promised workflow works, not merely that individual packages compile.

## Acceptance criteria

- [ ] Build the release candidate exclusively through the signed release workflow.
- [ ] Run all unit, integration, golden, fuzz smoke, race, protocol conformance, security, mutation recovery, compatibility, and performance gates.
- [ ] Run the PRD’s end-to-end UAT scenarios on a disposable representative copy of the real IKEMEN distribution.
- [ ] Prove one binary supplies CLI and MCP as first-class agent interfaces plus the optional LSP adapter.
- [ ] Verify read-only operation by default; mutation and runtime execution remain independently gated and absent from MCP capability discovery unless enabled.
- [ ] Verify deterministic, versioned, bounded JSON outputs and parity across CLI/MCP for shared operations.
- [ ] Verify the SQLite index is disposable and reconstructible from source files.
- [ ] Record environment, binary hash, corpus identity, results, waivers, known limitations, and rollback instructions in a release evidence artifact.
- [ ] No critical/high safety defect, protocol conformance failure, data-loss failure, or unexplained performance threshold breach may be waived.
- [ ] Tagging v1 remains a separate explicit maintainer action after the evidence artifact is approved.

## UAT

1. On a clean Windows machine, verify the artifact and run `doctor`.
2. Scan and index the representative distribution; execute diagnostic, symbol, reference, graph, impact, inspect, and export workflows.
3. Connect an MCP client, discover capabilities/resources, run the same read operations, and compare canonical results.
4. Prepare, review, apply, and recover a guarded mutation in a disposable copy.
5. Run the loader oracle/trace on an allowed fixture and confirm source-to-runtime correlation.
6. Delete the cache, rebuild it, and confirm equivalent query results.

## Non-goals

- Publishing or tagging without maintainer approval.
- Adding new scope during release-candidate validation.
- Declaring every IKEMEN dialect ambiguity resolved.
