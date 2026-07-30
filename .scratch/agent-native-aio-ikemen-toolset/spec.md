# Agent-Native AIO Ikemen Devtools Product Specification

**Product:** `ikm`
**Target:** stabilization alpha through v1.0
**Primary users:** AI coding agents, autonomous maintenance agents, mod authors using scripts
**Primary interfaces:** CLI and MCP stdio
**Secondary interface:** LSP stdio
**Explicitly excluded interface:** GUI

## Executive Summary

`ikm` will be one self-contained executable that understands an entire MUGEN/IKEMEN GO workspace. An agent must be able to install one binary, point it at a game root, inspect the workspace, query symbols and relationships, validate changes, preview safe edits, and optionally compare static conclusions with the real IKEMEN runtime.

The CLI is the universal contract. MCP is the first-class agent transport over the same operations. LSP remains a thin editor compatibility adapter. No interface may reimplement parsing, workspace loading, semantics, indexing, graph construction, patching, or runtime evidence.

The product remains local-first, offline by default, deterministic, bounded, and read-only by default. It will not ship a GUI, background cloud service, embedded model, arbitrary shell tool, or unrestricted filesystem access.

## Problem Statement

MUGEN and IKEMEN GO modding is a multi-format ecosystem rather than one language. A character or distribution spans DEF, CNS, ST, CMD, AIR, ZSS, Lua, JSON, binary assets, screenpack data, select rosters, stages, and runtime behavior. Ordinary text search cannot reliably distinguish declarations from references, exact targets from ambiguous candidates, authored quirks from errors, or static conclusions from runtime truth.

The current alpha proves the semantic seams, but agents still face product-level friction:

- They must preload individual MCP files instead of opening one bounded workspace.
- CLI commands, MCP tools, internal services, persistence, graphs, patching, runtime tracing, and exports are not yet one coherent capability surface.
- The persistent repository exists as an API but the executable does not bundle a database driver.
- Query pagination, output schemas, stable typed exit codes, root containment, budgets, and cache lifecycle are incomplete.
- MCP supports a useful subset but is not yet conformant with the complete 2026 stateless discovery and per-request metadata contract.
- JSON-RPC edge cases, including result/error exclusivity and batch handling, need a dedicated conformance gate.
- Existing patch logic is guarded but not exposed as a transactional, agent-safe workflow.
- Runtime-sensitive behavior such as `=/`, `stcommon`, duplicate states, and duplicate commands still lacks authoritative oracle fixtures.
- There is no release-grade cross-platform packaging, schema catalog, or agent recipe set.

From an agent's perspective, the missing product is: “Given this IKEMEN root and a bounded task, tell me exactly what exists, what it means, what depends on it, what is uncertain, what a proposed change would do, and whether the real runtime agrees.”

## Solution

Ship one executable named `ikm` with a shared application-operation layer:

```text
ikm
├── CLI operations
├── MCP stdio adapter
├── LSP stdio adapter
└── Shared application core
    ├── workspace discovery and snapshots
    ├── compatibility profiles
    ├── parsers and format adapters
    ├── semantic resolution
    ├── search and reference index
    ├── dependency/evidence graph
    ├── diagnostics and explanations
    ├── persistent rebuildable cache
    ├── guarded patch plans
    └── optional runtime oracle and trace bridge
```

An operation is defined once with typed input, typed output, authorization class, budget behavior, and deterministic ordering. CLI and MCP adapters invoke that same operation. LSP adapts editor events into the same workspace/session services.

The default workflow is:

```text
ikm workspace scan --root <game-root> --json
ikm query diagnostics --root <game-root> --json
ikm query symbols --root <game-root> --name <query> --json
ikm graph impact --root <game-root> --path <relative-path> --json
ikm patch preview --root <game-root> --plan <plan.json> --json
ikm mcp --root <game-root>
```

## Product Principles

1. **One binary, one semantic truth.** CLI, MCP, and LSP are adapters.
2. **Agent-readable first.** Stable schemas, deterministic ordering, bounded payloads, and typed failures are product requirements.
3. **CLI is universal.** Every stable capability is callable without MCP.
4. **MCP is first-class.** Read-only operations reach MCP parity in the same iteration as their CLI contract.
5. **No GUI core.** A future external UI may consume the contracts but cannot become a product dependency.
6. **Read-only by default.** Mutation and runtime execution require independent explicit enablement.
7. **Static claims are not runtime claims.** Runtime-sensitive facts carry evidence and confidence.
8. **Exact edges stay separate from search hits.** Fuzzy matches never become semantic references.
9. **No hidden authority.** Root, profile, budgets, cache, write access, and runtime access are explicit.
10. **Caches are disposable.** Source files and canonical manifests remain authoritative.
11. **Boring interfaces win.** JSON, JSONL, stdio, SQLite, stable URIs, and documented exit codes.
12. **Backward compatibility is intentional.** Pre-1.0 changes are versioned; existing commands receive migrations or aliases.

## Current Baseline

The alpha already contains:

- One Go executable with `check`, `index`, `corpus`, `metadata`, `lsp`, and `mcp`.
- Tolerant DEF/CNS/ST/CMD parsing and source spans.
- Compatibility profiles and corpus manifests.
- Semantic symbols, references, ambiguity classifications, and diagnostics.
- Expression, Lua, ZSS, AIR, select, character-manifest, and asset-reference adapters at varying depth.
- Deterministic SQL and SCIP-compatible JSONL exports.
- Document snapshots, incremental invalidation, cancellation, and a migration-aware repository API.
- Read-only LSP and MCP queries for diagnostics, symbols, hover, definition, and references.
- Guarded patch preview/apply primitives.
- Runtime oracle comparison and bounded trace bridge primitives.
- Release metadata and Ed25519 signing helpers.
- Unit, race, fuzz-smoke, benchmark, CLI, LSP, and MCP tests.

Measured reference corpus:

- 2,980 relevant authored and asset files.
- Approximately 23.97 GB including SFF and SND assets.
- Largest authored text file observed: approximately 1.2 MB.
- Current active-roster corpus command: approximately 1.6 seconds on the development machine.

## Goals

### v0.2 — Trustworthy Agent Kernel

- Conformant JSON-RPC and MCP transports.
- Versioned operation/output/error contracts.
- Explicit root/profile/config/budget behavior.
- Deterministic workspace discovery and snapshot manifests.
- Root-contained file access.

### v0.3 — Workspace Intelligence

- Rebuildable embedded SQLite cache.
- Incremental snapshot persistence.
- Fast diagnostics, symbol, reference, text, graph, inspect, and export operations.
- CLI coverage for all read-only capabilities.

### v0.4 — MCP-Native Workspace

- Stateless MCP 2026 discovery and request metadata.
- Deterministic MCP tools and resources with full JSON Schema.
- Pagination, cache hints, budgets, cancellation, clean stderr logging, and backward compatibility.

### v0.5 — Ecosystem Fidelity

- Stronger screenpack, select, stage, character, Lua, ZSS, expression, controller, and asset relationships.
- Provenance-rich ambiguity explanations.
- Runtime oracle fixtures and trace correlation for behavior static analysis cannot prove.

### v0.6 — Safe Agency

- Versioned patch-plan artifacts.
- Transactional multi-file apply with rollback.
- Exact semantic rename and bounded diagnostic fixes.
- MCP mutations disabled unless explicitly enabled.

### v1.0 — Product Release

- Cross-platform single binaries.
- Signed checksums, SBOM, dependency/license inventory, schema catalog, migration notes, and agent recipes.
- Real-distribution correctness, security, performance, and compiled-binary UAT gates.

## Non-Goals

- A GUI, custom IDE, web dashboard, or embedded editor.
- A hosted/cloud service.
- An embedded LLM or autonomous planning model.
- Arbitrary shell execution or a generic filesystem MCP server.
- Full binary SFF/SND editing in v1.
- Multiplayer/network control.
- Unprompted self-update or network access.
- Perfect static resolution of dynamic runtime expressions.
- Replacing the IKEMEN runtime as the final authority.

## Actors

- **Autonomous coding agent:** queries and changes a workspace through CLI or MCP.
- **Review agent:** evaluates diagnostics, graph impact, patch previews, and runtime evidence.
- **Mod author:** runs CLI commands directly or through scripts.
- **Editor client:** consumes the optional LSP adapter.
- **CI runner:** performs deterministic scans, policy gates, and artifact exports.
- **Runtime verifier:** explicitly launches a configured IKEMEN command through the bounded oracle/trace interface.

## User Stories

1. As an agent, I want one executable so that setup requires no language server, database, runtime, or package-manager installation.
2. As an agent, I want machine-readable help so that I can discover commands, schemas, exit codes, and capabilities without scraping prose.
3. As an agent, I want to pass an explicit game root so that the tool never guesses filesystem authority.
4. As an agent, I want a named compatibility profile so that path and precedence behavior is reproducible.
5. As an agent, I want a deterministic workspace snapshot so that every answer identifies the exact source state it analyzed.
6. As an agent, I want workspace-relative canonical paths so that results survive relocation and do not leak unrelated host paths.
7. As an agent, I want explicit unresolved, dynamic, and ambiguous results so that I never automate from invented certainty.
8. As an agent, I want diagnostics with codes, severity, spans, evidence, and suggested next checks so that failures are actionable.
9. As an agent, I want symbol search with stable semantic and occurrence identities so that results can be correlated across calls.
10. As an agent, I want definition and reference queries by position or identity so that I can navigate without text-search guesswork.
11. As an agent, I want text search clearly classified as lexical so that search hits are not confused with semantic edges.
12. As an agent, I want dependency, dependent, path, and impact graph queries so that I can bound a proposed change.
13. As an agent, I want character, stage, screenpack, file, and workspace inspection summaries so that I can understand an unfamiliar distribution quickly.
14. As an agent, I want pagination and truncation metadata so that large workspaces never silently overflow context.
15. As an agent, I want query budgets so that one malformed asset cannot exhaust time or memory.
16. As an agent, I want a persistent local cache so that repeated workspace queries are fast.
17. As an agent, I want the cache to be disposable and automatically rebuildable so that stale state never becomes authoritative.
18. As an agent, I want CLI and MCP operations to return the same data contract so that I can fall back between transports.
19. As an MCP client, I want modern stateless discovery and legacy compatibility so that current and older hosts can connect.
20. As an MCP client, I want tools with input and output schemas so that arguments and structured results can be validated.
21. As an MCP client, I want resources for manifests, files, diagnostics, symbols, and graphs so that context can be fetched without invoking actions.
22. As an MCP client, I want protocol stdout to contain only MCP messages so that logs never corrupt transport data.
23. As an editor, I want LSP to reuse the same snapshots and semantic service so that editor and agent answers agree.
24. As a CI runner, I want deterministic JSON/JSONL and typed exit codes so that policies are reliable.
25. As a CI runner, I want baseline comparison so that new diagnostic or compatibility regressions fail visibly.
26. As a mod author, I want binary asset metadata without reading entire multi-gigabyte assets so that workspace inventory remains fast.
27. As a mod author, I want authored slash, case, literal `=`, and fallback behavior preserved by the selected profile.
28. As a reviewer, I want every graph edge to include source, span, resolver, classification, and confidence so that impact claims are auditable.
29. As a reviewer, I want runtime-observed edges separated from static edges so that evidence sources remain clear.
30. As a runtime verifier, I want an explicit executable, working directory, timeout, and output budget so that runtime checks are controlled.
31. As a runtime verifier, I want differential fixtures for path and precedence behavior so that compatibility decisions are evidence-backed.
32. As an agent, I want patch preview to be the default so that no file changes occur during planning.
33. As an agent, I want patch plans bound to content hashes, identities, spans, and the workspace snapshot so that stale edits are rejected.
34. As an agent, I want multi-file apply to be atomic or rolled back so that partial edits cannot corrupt a package.
35. As an agent, I want rename and fixes restricted to exact semantic targets so that ambiguity cannot cause broad edits.
36. As an MCP host, I want write and runtime tools absent unless explicitly enabled so that capability discovery reflects real authority.
37. As a user, I want `version` and `doctor` commands so that environment and contract mismatches are diagnosable.
38. As a release consumer, I want signed checksums and an SBOM so that the single binary is verifiable.
39. As a maintainer, I want cross-platform conformance and corpus tests so that portability does not drift.
40. As a maintainer, I want every ticket to deliver an independently testable vertical slice so that agents can implement the roadmap safely.

## Public Command Contract

### Stable top-level commands

```text
ikm help
ikm version
ikm doctor
ikm workspace scan
ikm workspace status
ikm query diagnostics
ikm query symbols
ikm query definition
ikm query references
ikm query search
ikm graph dependencies
ikm graph dependents
ikm graph path
ikm graph impact
ikm inspect workspace
ikm inspect character
ikm inspect stage
ikm inspect file
ikm export jsonl
ikm export scip
ikm export sql
ikm patch preview
ikm patch apply
ikm patch rename
ikm patch fix
ikm runtime oracle
ikm runtime trace
ikm mcp
ikm lsp
```

Existing `check`, `index`, `corpus`, and `metadata` commands remain compatibility aliases until a documented v1 migration removes or permanently adopts them.

### Common arguments

- `--root <path>`: explicit workspace authority.
- `--profile <name>`: `distribution` or `strict-portable`; default recorded in output.
- `--config <path>`: explicit config file.
- `--cache <path|off>`: persistent cache selection.
- `--snapshot <id>`: require a specific snapshot.
- `--json`: canonical JSON result envelope.
- `--jsonl`: streaming records where the operation supports it.
- `--limit <n>` and `--cursor <opaque>`: bounded pagination.
- `--timeout <duration>`: cannot exceed the configured maximum.
- `--max-bytes <n>`: cannot exceed the configured maximum.
- `--allow-external <path>`: explicit additional read root.
- `--log-level <level>` and `--log-format <text|json>`: stderr only.
- `--write`: explicit CLI mutation permission.
- `--allow-runtime`: explicit runtime execution permission.

Flags override workspace configuration. Workspace configuration overrides documented defaults. There is no user-global configuration in v1.

### CLI output envelope

All `--json` operations return:

- `schemaVersion`
- `operation`
- `toolVersion`
- `status`: `complete`, `partial`, `blocked`, or `failed`
- `workspace`: root URI, profile, and configuration digest
- `snapshot`: content-addressed snapshot identity
- `result`: operation-specific typed payload
- `diagnostics`: operation diagnostics
- `page`: limit, returned count, next cursor
- `truncated`: boolean plus reasons
- `evidence`: relevant provenance summary

Wall-clock timestamps, elapsed times, absolute host paths, and nondeterministic invocation IDs are omitted from canonical output unless explicitly requested.

### Typed exit codes

| Code | Meaning |
|---:|---|
| 0 | Operation completed without error-severity findings |
| 1 | Operation completed and reported error-severity findings |
| 2 | Invalid command, arguments, config, or schema |
| 3 | Input, root, path, or filesystem failure |
| 4 | Internal, transport, persistence, or protocol failure |
| 5 | Cancelled or budget exceeded |
| 6 | Stale snapshot, stale hash, conflict, or unsafe mutation |
| 7 | Runtime oracle or trace command failure |

Legacy exit behavior remains available during the pre-v1 migration window and is never silently mixed with typed mode.

## Workspace Contract

### Workspace authority

The canonical root is an absolute, symlink-resolved directory selected by `--root` or explicit server configuration. MCP roots are not used. A workspace operation may read only:

- Files contained by the canonical root.
- Additional canonical roots explicitly supplied with `--allow-external`.
- Files already represented by an authorized snapshot.

Traversal, symlink, junction, alternate-stream, case-folding, and separator checks occur before every read and write, not only during discovery.

### Configuration

The optional workspace configuration is `.ikm/config.json`. It is versioned and may define:

- profile
- entry points
- cache mode
- includes and excludes
- budgets
- allowed external roots
- enabled format adapters
- write and runtime policies

The config cannot grant write or runtime authority by itself. The current invocation must also opt in.

### Discovery

Discovery is entry-point driven:

1. Runtime config and selected motif.
2. Active system and select definitions.
3. Roster characters and stages.
4. Character/stage manifests and referenced sources/assets.
5. Lua, mods, shaders, fonts, sounds, and additional configured roots.

A bounded fallback inventory identifies orphaned packages without treating every file as active. `.ikmignore` applies deterministic gitignore-style excludes. Generated cache/output directories are excluded automatically.

### Snapshot and manifest

A snapshot contains:

- schema and identity-contract versions
- canonical root URI
- profile/config digest
- sorted entry points
- sorted file records with relative path, kind, size, content hash where required, and parse status
- sorted source, semantic, asset, roster, stage, configuration, and runtime-evidence edges
- diagnostics and ambiguity candidates
- aggregate counts and budget/truncation state

The snapshot ID is a hash of canonical semantic inputs, not timestamps or absolute root strings. Relocating an unchanged workspace preserves relative identities.

### Asset policy

Text sources are parsed within per-file and aggregate byte limits. SFF, SND, fonts, images, and audio are inventoried by metadata and bounded header reads. Full multi-gigabyte asset hashing is opt-in and streamed. Binary contents are not exposed as MCP text resources.

## Persistence Contract

- The executable bundles one CGO-free SQLite driver so the product remains one deployable binary.
- The baseline implementation candidate is the current stable `modernc.org/sqlite`, pinned with its matching `modernc.org/libc` version and audited licenses.
- The Go baseline moves from unsupported Go 1.20 to supported Go 1.26.
- Default cache path: `.ikm/index.sqlite`.
- The cache is rebuildable and never authoritative.
- Cache schema, tool version, identity contract, profile digest, and snapshot ID are stored.
- Incompatible cache schemas rebuild by default; explicit export databases use migrations.
- One writer lock is allowed. Concurrent readers use a committed snapshot.
- A cancelled scan leaves the last committed snapshot intact.
- FTS hits remain classified as lexical search results.

## Query and Graph Contract

### Query families

- Diagnostics by scope, severity, code, path, and snapshot.
- Symbols by name, kind, semantic key, occurrence ID, file, and package.
- Definition and references by source position or stable identity.
- Lexical/FTS search by text and file kind.
- Inspection summaries for workspace, character, stage, and file.

### Graph model

Each edge includes:

- stable edge ID
- source and target identities
- edge kind
- origin path and span
- resolver/adapter name
- classification: exact, ambiguous, unresolved, dynamic, lexical, or runtime-observed
- candidate list when ambiguous
- confidence/evidence level
- snapshot ID

Graph operations:

- dependencies
- dependents
- shortest bounded paths
- impact radius
- explanation of why an edge exists

Impact defaults to exact and runtime-observed edges. Ambiguous, dynamic, and lexical edges appear in separate risk sections unless explicitly included.

## MCP Contract

### Transport and lifecycle

- Primary transport: newline-delimited JSON-RPC over stdio.
- Stdout contains protocol messages only.
- Logs use stderr.
- Protocol target: MCP `2026-07-28`.
- Modern requests carry required per-request `_meta`.
- `server/discover` reports versions, capabilities, server identity metadata, instructions, and cache fields in the current specification shape.
- Legacy initialization remains for `2025-11-25`, `2025-06-18`, and `2024-11-05` during the compatibility window.
- The server is stateless at the protocol layer. Workspace/cache state is keyed by explicit server configuration and snapshot IDs.
- Deprecated MCP roots, sampling, and protocol logging are not foundational dependencies.

### Resources

The MCP server exposes deterministic, paginated resources:

- `ikm://workspace/{snapshot}/manifest`
- `ikm://workspace/{snapshot}/diagnostics`
- `ikm://workspace/{snapshot}/graph`
- `ikm://workspace/{snapshot}/file/{relative-path}`
- `ikm://workspace/{snapshot}/symbol/{identity}`
- `ikm://workspace/{snapshot}/character/{identity}`
- `ikm://workspace/{snapshot}/stage/{identity}`

`resources/list`, `resources/templates/list`, and `resources/read` validate URI authority and root containment. Missing resources return `-32602`.

### Read-only tools

- `ikm.workspace.scan`
- `ikm.workspace.status`
- `ikm.diagnostics.list`
- `ikm.symbols.search`
- `ikm.definition.get`
- `ikm.references.list`
- `ikm.search.text`
- `ikm.graph.query`
- `ikm.inspect`
- `ikm.export`
- `ikm.runtime.compare` only when runtime authority is enabled

### Mutation tools

- `ikm.patch.preview`
- `ikm.patch.apply`
- `ikm.rename`
- `ikm.fix`

Only preview is advertised by default. Apply, rename, fix, and runtime tools are absent from `tools/list` unless the server starts with corresponding explicit authority. Tool annotations are hints, never the security boundary.

### Tool schemas and results

- JSON Schema 2020-12.
- Object-rooted input schemas.
- Output schemas for every structured result.
- Deterministic tool ordering.
- `resultType`, `content`, `structuredContent`, `isError`, pagination, cache, and truncation fields follow the negotiated protocol.
- Structured output also includes serialized text for compatibility where required.
- Schema depth and validation time are bounded.

### JSON-RPC conformance

- Request IDs preserve string, integer, and explicit `null` values.
- A notification is identified only by the absence of `id`.
- Notifications never receive responses.
- Success contains `result` and no `error`.
- Failure contains `error` and no `result`.
- Invalid request and parse errors are distinct.
- Batch requests preserve correlation and omit notification responses.
- Cancellation and budget failures return stable typed data without panics.

## LSP Contract

LSP remains a thin compatibility adapter targeting LSP 3.18:

- Standard `Content-Length` framing.
- Initialize/shutdown/exit lifecycle.
- Workspace folders and explicit root configuration mapped to workspace sessions.
- Full-document sync initially; incremental sync only after benchmarks justify it.
- Diagnostics, document/workspace symbols, hover, definition, references, and document links.
- Cancellation and deterministic results.
- No LSP-only semantic implementation.

LSP completeness is secondary to CLI/MCP completeness and cannot block an agent-native release unless the shared service regresses.

## Mutation Contract

### Patch plan

A patch plan is a versioned JSON artifact containing:

- plan schema version
- tool and identity-contract versions
- root/config/profile/snapshot identities
- operation intent and evidence
- ordered file edits
- content hashes
- semantic identities
- byte and line/column spans
- exact old and new text
- expected diagnostics before and after
- preview diff and per-file new hash

### Preview

Preview is read-only and default. It reparses changed bytes in memory, re-runs affected semantics, computes graph impact, and reports introduced/resolved diagnostics.

### Apply

Apply requires:

- explicit CLI `--write` or MCP server write authority
- an unchanged root and snapshot
- matching per-file hashes and old text
- no ambiguous target
- a previously validated plan

Multi-file apply writes temporary files, flushes them, retains rollback bytes, commits deterministic renames, and restores all touched files if any commit step fails. Results contain final hashes and a rollback audit. Apply never changes files outside authorized roots.

## Runtime Oracle and Trace Contract

- Disabled by default.
- No shell interpretation.
- Explicit executable, arguments, working directory, timeout, stdout/stderr budgets, and environment allow-list.
- Runtime events use bounded JSONL.
- Runtime-observed evidence is stored separately from static evidence.
- Differential fixtures cover slash normalization, literal `=`, `cns`, `st`, numbered `st`, `stcommon`, common fallbacks, duplicate states, duplicate commands, and missing assets.
- A compatibility rule cannot become “exact” without a fixture or upstream source proving it.
- Runtime tools are not advertised by MCP unless explicitly enabled.

## Security and Safety

### Threat model

The workspace and its content are untrusted. DEF/CNS/CMD/Lua text, MCP requests, resource URIs, cache databases, patch plans, runtime output, and symlinks may be malicious or malformed.

### Required controls

- Root containment after canonicalization and link resolution.
- Junction/symlink escape tests on Windows and Unix.
- No shell execution.
- No network access in normal operations.
- Bounded reads, parse depth, schema depth, graph depth, result rows, output bytes, runtime output, and duration.
- Panic recovery at protocol boundaries with internal-error responses and stderr diagnostics.
- SQLite opened with controlled pragmas and no extension loading.
- Cache corruption triggers quarantine/rebuild, not trusted reads.
- Secrets and files outside indexed/authorized roots are never resources.
- Mutation and runtime capabilities are absent unless enabled.
- Patch apply is stale-safe, atomic, and auditable.
- Binary assets are not loaded wholly into memory by default.

## Default Budgets

Defaults are conservative and configurable downward. Raising them requires explicit invocation/config:

| Budget | Default |
|---|---:|
| Discovered files | 20,000 |
| Parsed text bytes per file | 8 MiB |
| Parsed text bytes per scan | 512 MiB |
| Binary header read per file | 1 MiB |
| MCP response bytes | 2 MiB |
| CLI response bytes | 16 MiB |
| Query rows per page | 100 |
| Maximum query rows | 10,000 |
| Graph depth | 8 |
| Graph nodes visited | 25,000 |
| Read query timeout | 5 seconds |
| MCP scan timeout | 60 seconds |
| CLI scan timeout | 120 seconds |
| Runtime timeout | 60 seconds |
| Runtime stdout/stderr each | 8 MiB |

Budget exhaustion returns `partial` plus an explicit reason and cursor when safe. It never silently drops results.

## Performance Objectives

On the reference Windows distribution:

- Active-roster corpus remains at or below 2.5 seconds after warm filesystem cache.
- Cold active-workspace semantic scan completes within 15 seconds.
- Full metadata inventory, without full binary hashing, completes within 30 seconds.
- Warm no-change rescan completes within 2 seconds.
- Warm indexed read queries are below 250 ms at p95.
- MCP adapter overhead is below 25 ms at p95 beyond the underlying operation.
- Peak resident memory stays below 750 MiB for the reference workspace.
- Cache size stays below twice the total parsed text size unless measured evidence justifies an exception.

Performance gates record hardware, OS, corpus snapshot, and tool version. CI uses relative regression thresholds where fixed timing would be unreliable.

## Success Metrics

- One downloaded binary completes scan, query, graph, export, and MCP workflows.
- 100% of stable read-only operations have CLI and MCP parity.
- 100% of MCP tools have valid input/output schemas and conformance fixtures.
- 100% of graph edges carry provenance and classification.
- Zero silent truncation.
- Zero unauthorized root escapes in adversarial tests.
- Zero file writes without explicit current-invocation authority.
- Zero partial multi-file mutations after injected failure tests.
- Real-distribution scan/query UAT passes on every release.
- Runtime-sensitive compatibility rules are fixture-backed or labeled unverified.
- Deterministic outputs are byte-identical for identical snapshots.

## Implementation Decisions

### Shared operation registry

A central operation registry defines:

- public operation name
- authorization class: read, write, or runtime
- typed input and output
- JSON schemas
- CLI binding
- MCP binding
- pagination support
- default and maximum budgets
- deterministic ordering rules

The registry prevents CLI/MCP drift without forcing every command into reflection-heavy generic code. Core services remain normal typed Go modules.

### Workspace service

The existing coordinator becomes the seed for a workspace session service. It owns discovery, snapshots, dependency invalidation, cancellation, and current committed results. File watchers are optional acceleration; correctness always falls back to content/hash validation.

### Persistence

The existing repository and migration concepts are retained. The product bundles a CGO-free SQLite driver for cross-compilation. Workspace caches rebuild on incompatible schema; user-requested exports use explicit migration/version rules.

### Semantic graph

The IR identity contract remains the source for symbol/reference identity. New ecosystem and runtime edges use the same snapshot and provenance conventions. Lexical search is stored and queried separately.

### Adapters

DEF/CNS/ST/CMD remain the deepest parser path. AIR, select/system/motif, Lua, ZSS, shaders, fonts, and binary assets first provide inventory and reference edges; deeper semantic parsing is added only where it improves agent decisions.

### Compatibility

The distribution profile preserves observed authored behavior. Strict-portable surfaces nonportable constructs. Promotion of an assumption into a compatibility rule requires an oracle fixture, upstream source, or explicit documented user decision.

### Release architecture

Release artifacts are one binary per supported OS/architecture, plus detached checksums, signature, SBOM, licenses, and schemas. “Single binary” means no runtime dependencies; rebuildable cache/config files are normal workspace state.

## Testing Decisions

### Externally observable seams

- Compiled CLI invocation with exact stdin/stdout/stderr/exit-code assertions.
- MCP wire messages through real stdio framing.
- LSP wire messages through `Content-Length` framing.
- Workspace scans against synthetic fixtures and the real distribution.
- SQLite cache open/rebuild/concurrent-read/cancel behavior.
- Patch preview/apply with injected write and rollback failures.
- Runtime bridge through injected fake runners and opt-in real IKEMEN fixtures.

### Required test layers

1. Unit tests for typed services and path/budget helpers.
2. Golden tests for canonical JSON, JSONL, schemas, manifests, and graph output.
3. Property tests for deterministic ordering and identity stability.
4. Fuzz tests for parsers, JSON-RPC frames, URIs, config, patch plans, trace JSONL, and cache metadata.
5. Protocol conformance scenarios for JSON-RPC, MCP modern/legacy, and LSP lifecycle.
6. Security tests for traversal, symlink/junction escape, oversized inputs, malformed databases, and permission failures.
7. Integration tests through the compiled binary.
8. Real-corpus correctness and performance gates.
9. Cross-platform CI for Windows, Linux, and macOS.
10. Release UAT from clean machines without Go installed.

### Existing seams to preserve

- Parser and identity determinism tests.
- Workspace/profile path-resolution tests.
- Corpus and ecosystem fixtures.
- Document snapshot and coordinator invalidation tests.
- Repository migration/query tests.
- Patch stale-hash, identity, span, overlap, and escape tests.
- Oracle/trace bounded execution tests.
- CLI, LSP, MCP, race, fuzz-smoke, and benchmark gates.

## Phases and Iterations

| Iteration | Outcome | Tickets |
|---|---|---|
| 0 — Trust the wire | Versioned operation contracts and conformant JSON-RPC/MCP | 01–06 |
| 1 — Understand one root | Safe deterministic workspace snapshots | 07–14 |
| 2 — Remember safely | Embedded rebuildable persistence and query store | 15–20 |
| 3 — Complete CLI intelligence | Agent-usable read-only CLI surface | 21–29 |
| 4 — Become MCP-native | Stateless resources/tools with CLI parity | 30–36 |
| 5 — Prove ecosystem behavior | Deeper relationships and runtime evidence | 37–44 |
| 6 — Enable safe agency | Transactional plans, rename, fixes, opt-in MCP writes | 45–50 |
| 7 — Ship v1 | Product UX, supply chain, performance, docs, release UAT | 51–55 |

### Iteration exit rule

An iteration exits only when:

- every ticket acceptance criterion passes;
- compiled-binary UAT for the iteration passes;
- canonical contracts and migration notes are updated;
- real-corpus correctness does not regress;
- new capabilities are bounded and documented;
- no task-owned race, vet, fuzz-smoke, formatting, or security test fails.

Work proceeds blockers-first. A later iteration may begin only on tickets whose explicit blockers are complete.

## UAT Scenarios

### Read-only happy path

1. Download `ikm` on a clean Windows machine.
2. Run `ikm version --json` and `ikm doctor --root <distribution> --json`.
3. Run a cold workspace scan.
4. Query diagnostics, symbols, references, a character inspection, and dependency impact.
5. Start MCP and repeat equivalent queries.
6. Verify snapshot IDs and result payloads agree.

### Ambiguity path

1. Load a fixture with duplicate candidates.
2. Query definition and impact.
3. Confirm no exact target is invented.
4. Confirm candidates and evidence appear deterministically.
5. Confirm rename/apply is refused.

### Budget path

1. Set a low file, byte, row, graph, or time budget.
2. Run scan/query through CLI and MCP.
3. Confirm partial status, explicit reason, cursor where applicable, stable typed exit/error, and no panic.

### Root-escape path

1. Create traversal, symlink, junction, mixed-separator, case-folding, and external absolute-path fixtures.
2. Run resources, query, export, and patch operations.
3. Confirm access is refused unless the exact external root was explicitly authorized.

### Cache path

1. Scan and query.
2. Modify one dependency.
3. Confirm only affected snapshots invalidate.
4. Corrupt or version-skew the cache.
5. Confirm quarantine/rebuild and preservation of source files.

### Mutation path

1. Generate an exact rename/fix plan.
2. Preview without write authority.
3. Change one source byte and confirm stale rejection.
4. Restore, apply with write authority, inject a mid-commit failure, and confirm rollback.
5. Apply successfully and confirm diagnostics/graph/snapshot update.

### Runtime path

1. Run without runtime authority and confirm the capability is absent/refused.
2. Enable a fake bounded runner and confirm events.
3. Run curated IKEMEN loader fixtures.
4. Confirm static and runtime evidence remain separately labeled.

### Release path

1. Start from a clean machine with no Go installation.
2. Verify checksum/signature.
3. Run help, version, doctor, scan, queries, MCP, LSP smoke, and read-only UAT.
4. Confirm no network access or extra runtime is required.

## Risks and Mitigations

1. **Scope explosion across formats.** Prioritize relationships and metadata before deep semantics.
2. **MCP specification churn.** Isolate transport/version adapters behind typed operations and conformance fixtures.
3. **SQLite dependency weight.** Benchmark binary size, memory, build time, supported targets, and licenses before adoption.
4. **Windows path complexity.** Make path authority a dedicated module with adversarial fixtures.
5. **Huge binary assets.** Use metadata/header reads; never default to full content.
6. **False semantic confidence.** Preserve classifications, candidates, evidence, and runtime separation.
7. **Partial mutations.** Require transactional staging and rollback before exposing apply.
8. **CLI/MCP drift.** Generate bindings and schemas from one operation registry.
9. **Cache corruption/staleness.** Treat caches as disposable and snapshot-bound.
10. **Backward-compatibility drag.** Version contracts and publish explicit pre-v1 migrations.
11. **Agents overusing expensive operations.** Publish costs, budgets, pagination, and narrow query alternatives.
12. **Runtime execution risk.** Disable by default, avoid shell interpretation, and bound everything.

## Out of Scope

- GUI implementation or design.
- Hosted MCP over public HTTP in v1.
- Authentication for remote multi-user servers.
- An MCP App or embedded web view.
- Cloud synchronization.
- Model inference, embeddings, or vector databases.
- Generic source control operations.
- Arbitrary process or shell tools.
- Automatic merge, release publication, or deployment.
- Full sprite/sound authoring.
- Live game memory modification.

## Further Notes

### Official contracts consulted

- MCP `2026-07-28` discovery and stateless protocol direction.
- MCP resources and tools, including pagination, cache fields, resource URI security, JSON Schema 2020-12, and structured tool results.
- MCP deprecation of roots, sampling, and protocol logging in favor of explicit configuration/resource URIs and stderr.
- JSON-RPC 2.0 request, notification, response exclusivity, error, and batch rules.
- LSP 3.18 as the current editor protocol target.
- Go 1.26 as the current supported Go baseline in July 2026.
- Current CGO-free `modernc.org/sqlite` package documentation as the embedded-driver candidate.

### Decision log

- No GUI.
- No daemon required for v1.
- CLI and MCP are peers over one operation core.
- LSP is optional and subordinate.
- MCP workspace authority comes from explicit server configuration, not deprecated roots.
- SQLite is a rebuildable cache, not source of truth.
- Writes and runtime execution are separately gated.
- Runtime evidence is required before hard-coding uncertain IKEMEN precedence behavior.

### Local planning artifacts

This specification is paired with one atomic ticket per file under the sibling `issues` directory. Tickets declare explicit blocking edges and are ordered by the iteration table above.
