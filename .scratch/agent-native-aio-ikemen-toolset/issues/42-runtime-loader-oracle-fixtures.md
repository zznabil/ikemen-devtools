# 42 — Runtime Loader Oracle Fixtures

**What to build:** Turn the bundled IKEMEN runtime into an opt-in compatibility oracle with disposable fixtures, bounded execution, log capture, and source correlation.

**Blocked by:** 41, 14

**Status:** ready-for-agent

## User story

As a maintainer, I want to compare static predictions with actual loader outcomes so that compatibility claims are based on engine evidence.

## Acceptance criteria

- [ ] Define a runtime-oracle contract with executable identity, workspace/fixture identity, arguments, timeout, output budgets, exit classification, log artifacts, and correlation results.
- [ ] Execution is disabled by default and requires an explicit allowed executable plus runtime capability.
- [ ] Commands run without a shell, from a disposable test root, with bounded stdout/stderr/log files and process-tree termination.
- [ ] Build representative valid/invalid fixtures for roster, character, stage, asset, CNS/CMD/AIR, Lua/ZSS, and profile cases.
- [ ] Parse `Ikemen.log`/exit evidence into typed loader outcomes without treating text matching as infallible.
- [ ] Compare static edges/diagnostics against outcomes and record confirmed, contradicted, inconclusive, and unsupported cases.
- [ ] Fixture results are reproducible and never modify the user’s persistent `save/` data.
- [ ] Tests use a fake executable for deterministic unit/integration coverage; real runtime UAT is separately opt-in.

## UAT

1. Run valid and broken disposable character fixtures through the authorized runtime.
2. Confirm timeout, exit, log, and correlated source evidence are captured within budgets.
3. Attempt execution without capability/allowlist; expect policy refusal and no process.
4. Confirm the original game workspace and persistent save files are unchanged.

## Non-goals

- Sandboxing an untrusted engine as a security boundary.
- Automated gameplay.
- Treating runtime behavior as a substitute for static provenance.
