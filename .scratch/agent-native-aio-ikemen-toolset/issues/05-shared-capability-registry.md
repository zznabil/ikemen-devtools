# 05 — Build the shared capability registry

**What to build:** A typed registry that defines each public operation once and binds it to CLI and MCP without reimplementing business behavior.

**Blocked by:** 04 — Define the versioned operation, output, and exit contract

**Status:** ready-for-agent

- [ ] Each capability declares name, authority class, typed input/output, budgets, pagination, and ordering.
- [ ] CLI bindings and MCP definitions are derived from or validated against the same capability declaration.
- [ ] Read, write, and runtime capability sets can be filtered before advertisement.
- [ ] Existing semantic services remain ordinary typed modules, not reflection-driven generic handlers.
- [ ] Drift tests fail when CLI/MCP schemas or availability disagree.
