# Corpus and compatibility profiles

The supported text slice is tolerant, INI-like parsing for `.def`, `.cns`, `.st`, and `.cmd`. Workspace loading follows `[Files]` keys `cmd`, `cns`, `st`, `stN`, and `stcommon`, in source order, and de-duplicates canonical paths. It does not yet parse expressions, ZSS, AIR, SFF, SND, motif/screenpack relationships, or Lua semantics.

Profiles are Go values, not CLI flags:

```go
strict := profile.NewStrictPortableProfile(workspaceRoot)
distribution := profile.NewDistributionProfile(workspaceRoot)
result := workspace.LoadWorkspaceWithProfile(defPath, distribution)
```

`strict/portable` removes a leading `=` marker and leading separators from source values. `distribution` preserves `=` as a literal path component, uses portable slash normalization, and enables the distribution's leading-equals and empty-state-section tolerances. Both profiles use `.` and `data` fallback roots and platform-aware case policy. `LoadWorkspace` defaults to strict/portable; the `ikm` CLI deliberately selects distribution.

To audit a roster, point `corpus` at the actual `select.def`:

```text
go run ./cmd/ikm corpus --json --output roster.json data/select.def
```

The manifest records selected entries, resolved paths, diagnostics, profile name, and counts. A nonzero exit indicates error diagnostics. A corpus result is evidence for the selected profile only; it is not a claim that every runtime convention is implemented.

Known limitations include runtime precedence not being a complete oracle, dynamic targets remaining non-exact, and profile behavior being explicit but not yet configurable from the CLI or protocol servers. Add a named profile and fixture before encoding a new compatibility exception.
