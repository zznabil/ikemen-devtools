# 29 — Gate complete read-only CLI parity

**What to build:** A compiled-binary UAT proving every read-only operation is discoverable, deterministic, bounded, and consistent with the shared service contract.

**Blocked by:** 21 — Ship diagnostics query CLI; 22 — Ship symbol search CLI; 23 — Ship definition and reference CLI; 24 — Ship bounded lexical search CLI; 25 — Ship dependency graph CLI; 26 — Ship impact and edge-explanation CLI; 27 — Ship workspace, character, stage, and file inspection; 28 — Ship canonical export CLI

**Status:** ready-for-agent

- [ ] Machine-readable help enumerates operations, schemas, flags, authority, budgets, and exit codes.
- [ ] Compiled CLI results equal direct service results for representative fixtures.
- [ ] Deterministic, pagination, budget, cancellation, and typed-failure goldens pass.
- [ ] Existing commands have tested aliases/migration messages.
- [ ] Real-distribution read-only UAT passes without extra runtime dependencies.
