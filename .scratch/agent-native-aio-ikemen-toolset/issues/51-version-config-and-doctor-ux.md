# 51 — Version, Config, and Doctor UX

**What to build:** Complete the operational UX that lets humans and agents identify the exact binary, effective configuration, capabilities, cache state, and actionable setup problems.

**Blocked by:** 20, 30

**Status:** ready-for-agent

## User story

As an agent, I want self-describing version, configuration, and health commands so that I can adapt to an installation without guessing or scraping prose.

## Acceptance criteria

- [ ] `ikm version --json` reports binary version, commit, build time, Go version, target OS/architecture, schema versions, protocol versions, and feature gates.
- [ ] `ikm config show --effective` reports resolved values and provenance without exposing secrets or unstable absolute paths by default.
- [ ] `ikm capabilities` returns the canonical registry filtered by active policy and transport.
- [ ] `ikm doctor` checks workspace discovery, configuration, permissions, cache/schema health, supported file access, MCP stdio safety, runtime availability, and version compatibility.
- [ ] Every doctor finding has a stable code, severity, evidence, and actionable remediation.
- [ ] Human output is concise; JSON uses the standard envelope and typed exit status.
- [ ] Read-only doctor checks never repair or delete automatically.
- [ ] `doctor --fix` supports only separately documented, reversible maintenance actions and requires explicit selection.
- [ ] Golden tests cover healthy, first-run, corrupt-cache, read-only-root, missing-runtime, and incompatible-schema states.

## UAT

1. Run version, effective config, capabilities, and doctor from a valid workspace.
2. Confirm an agent can select supported operations using JSON fields only.
3. Corrupt a disposable cache and rerun doctor; expect a stable finding and rebuild guidance.
4. Confirm stdout remains valid JSON while progress and logs go to stderr.

## Non-goals

- Telemetry.
- Online update checks.
- A GUI settings editor.
