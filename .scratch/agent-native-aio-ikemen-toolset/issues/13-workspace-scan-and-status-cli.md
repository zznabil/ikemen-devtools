# 13 — Ship workspace scan and status CLI

**What to build:** End-to-end `workspace scan` and `workspace status` commands over the shared session with canonical JSON, human summaries, budgets, and typed exits.

**Blocked by:** 12 — Build the incremental workspace session; 04 — Define the versioned operation, output, and exit contract

**Status:** ready-for-agent

- [ ] A clean binary can scan an explicit distribution root without other software.
- [ ] Status reports root/profile/config/snapshot, counts, cache mode, diagnostics, and truncation.
- [ ] JSON and human output agree while stdout/stderr remain separated.
- [ ] Budget, invalid-root, cancellation, and partial-result paths have compiled-binary tests.
- [ ] Legacy corpus/check behavior remains documented and regression-tested.
