# Architecture

`internal/workspace` discovers and bounds source files; parser, semantics, graph, and repository services build disposable indexes. `internal/operations` owns canonical read operations. `cmd/ikm` is the human/agent CLI adapter; `internal/mcp` exposes the same registry through MCP; `internal/lsp` is an optional protocol adapter. Adapters do not own domain logic, so CLI and MCP results share the versioned envelope and capability registry.

The index is a cache, never source of truth. Roots are contained, symlinks are checked, inputs and outputs are budgeted, and read-only is the default authority.
