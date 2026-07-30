# Safety boundaries

The default product is static analysis, not game execution.

- **No runtime driver by default.** The CLI and protocol packages do not start Ikemen, attach to a process, or claim live state. A future runtime bridge must be an explicit separately deployed capability.
- **LSP/MCP are read-only.** `SetDocument` accepts caller-owned bytes in memory. The servers do not write files, expose patch/rename tools, or mutate a workspace. `ikm mcp` reads only paths explicitly supplied with repeatable `--file`; it does not discover or walk a workspace. MCP exposes only diagnostics, symbols, hover, definition, and references.
- **SQLite is host-supplied.** Repository APIs accept an injected `*sql.DB`; this module does not select or bundle a driver. Apply migrations only to a database you control and back it up before upgrades.
- **CLI filesystem behavior is explicit.** `check`, `index`, `corpus`, and `mcp --file` read paths supplied by the caller; the CLI currently uses the distribution profile and allows profile external paths. `index --output` and `corpus --output` reject outputs that alias loaded source files and write through a temporary file/rename sequence.
- **Protocol results are not authority.** Dynamic and ambiguous references are not exact targets. Clients must preserve profile, source span, and classification when presenting or automating a result.

If a host wraps these packages for agents, add an explicit workspace-root/capability policy, bounds on bytes/time/rows/graph depth, and a confirmation gate for any future mutation. Do not treat FTS matches as semantic references or infer permissions from document/protocol content.
