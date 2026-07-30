package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func TestMigrationMetadata(t *testing.T) {
	if CurrentSchemaVersion != 2 {
		t.Fatalf("expected schema version 2, got %d", CurrentSchemaVersion)
	}
	migrations := MigrationSQL()
	if len(migrations) != CurrentSchemaVersion {
		t.Fatalf("expected %d migrations, got %d", CurrentSchemaVersion, len(migrations))
	}
	for i, migration := range migrations {
		if migration.Version != i+1 || len(migration.Statements) == 0 {
			t.Fatalf("invalid migration metadata at %d: %+v", i, migration)
		}
	}
}

func TestMigrationSQLContainsRequiredTables(t *testing.T) {
	var all strings.Builder
	for _, migration := range MigrationSQL() {
		for _, statement := range migration.Statements {
			all.WriteString(strings.ToLower(statement))
		}
	}
	for _, table := range []string{"ikm_schema_version", "ikm_documents", "ikm_symbols", "ikm_references", "ikm_diagnostics", "ikm_dependency_edges"} {
		if !strings.Contains(all.String(), table) {
			t.Fatalf("migration SQL missing %s", table)
		}
	}
}

func TestNilDatabaseReturnsError(t *testing.T) {
	if _, err := New(context.Background(), nil); !errors.Is(err, ErrNilDatabase) {
		t.Fatalf("expected ErrNilDatabase from New, got %v", err)
	}
	if _, err := Open(context.Background(), nil); !errors.Is(err, ErrNilDatabase) {
		t.Fatalf("expected ErrNilDatabase from Open, got %v", err)
	}
	if err := ApplyMigrations(context.Background(), nil, CurrentSchemaVersion); !errors.Is(err, ErrNilDatabase) {
		t.Fatalf("expected ErrNilDatabase from ApplyMigrations, got %v", err)
	}
}

func TestCanceledContextFailsBeforeDatabaseUse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ApplyMigrations(ctx, &sql.DB{}, CurrentSchemaVersion); !errors.Is(err, ErrContextCanceled) {
		t.Fatalf("expected ErrContextCanceled, got %v", err)
	}
}

func TestSnapshotNormalizationIsDeterministic(t *testing.T) {
	input := DocumentSnapshot{Path: `src\\..\\src\\hero.def`, Symbols: []SymbolSnapshot{
		{ID: "b", Name: "z"},
		{ID: "a", Name: "a"},
	}}
	first := normalizeSnapshot(input)
	second := normalizeSnapshot(input)
	if first.Path != "src/hero.def" {
		t.Fatalf("expected canonical path, got %q", first.Path)
	}
	if first.Symbols[0].Name != "a" || first.Symbols[1].Name != "z" {
		t.Fatalf("expected deterministic symbol ordering, got %+v", first.Symbols)
	}
	first.UpdatedAtUnix = second.UpdatedAtUnix
	if first.Path != second.Path || first.Symbols[0].ID != second.Symbols[0].ID {
		t.Fatal("normalization was not deterministic")
	}
}
