# Migration and versioning

There are two versioned contracts:

- **IR identity contract:** `0.2.0` (`internal/ir.IdentityContractVersion`). Documents carry this value in `Document.Version`; semantic, occurrence, and store identities are separate fields.
- **Repository schema:** `2` (`internal/repository.CurrentSchemaVersion`). Migrations are named and ordered: schema 1 (`bootstrap`) and schema 2 (`dependency-edges`).

Opening a repository applies pending migrations in a transaction:

```go
repo, err := repository.Open(ctx, db) // db is *sql.DB
```

`repository.New` is an equivalent constructor. `ApplyMigrations(ctx, db, targetVersion)` can apply a specific supported target. A nil database is rejected, and a database newer than this binary returns `ErrSchemaUnsupported`; the code never silently downgrades or rewrites a newer schema.

The repository package accepts an injected `*sql.DB`; this module intentionally does not ship a SQLite driver. The host application must import/register a compatible `database/sql` SQLite driver and call `sql.Open` before `repository.Open`. The standalone `ikm index` command is different: it exports SQL text and does not open SQLite itself.

For upgrades, preserve the database until the new binary opens it successfully, take a backup, let the binary run its migrations, then verify a read/query. Do not copy a newer database into an older binary. Treat changes to the IR contract or identity fields as protocol-impacting even when the repository schema is unchanged.

## MVP to current slices

1. **Original MVP:** `check`/`index`, tolerant DEF/CNS/CMD/ST parsing, deterministic diagnostics and SQL export.
2. **Current slices:** distribution profile and corpus manifest (`corpus`), canonical release metadata (`metadata`), versioned identity contract, migration-aware repository API, and read-only LSP/MCP facades.
3. **Still not shipped:** a live runtime driver by default, CLI profile selection, write-capable MCP/LSP operations, and a bundled SQLite driver.
