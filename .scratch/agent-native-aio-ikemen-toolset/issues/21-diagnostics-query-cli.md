# 21 — Ship diagnostics query CLI

**What to build:** A complete diagnostics query by workspace, package, path, severity, code, and snapshot with explanations and pagination.

**Blocked by:** 19 — Build bounded query-store APIs

**Status:** ready-for-agent

- [ ] Results include code, severity, span, message, source adapter, evidence, candidates, and next checks.
- [ ] Scope/filter combinations are schema-validated and deterministic.
- [ ] Large results paginate without omissions or duplicates.
- [ ] Partial/budget/cancel states use the shared output and exit contract.
- [ ] Compiled CLI fixtures cover clean, error, ambiguous, missing, and malformed workspaces.
