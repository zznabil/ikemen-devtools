package index

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/semantics"
	"github.com/ikemen-engine/ikemen-devtools/internal/workspace"
)

func TestExportEmitsStableSchemaAndFtsTables(t *testing.T) {
	t.Helper()

	ws := workspace.LoadResult{
		Documents: []ir.Document{
			{
				Version:  ir.Version,
				Path:     "hero.def",
				FileType: "def",
				Symbols: []ir.Symbol{
					{
						ID:      "command:jump:2:hero.def",
						Kind:    ir.SymbolCommand,
						Name:    "command:jump",
						Section: "Command",
						Span:    sourceSpan(2, 1, 2, 16),
						Raw:     `"jump"`,
					},
				},
				References: []ir.Reference{
					{
						ID:           "ref:command:3:hero.def",
						Kind:         ir.ReferenceCommand,
						Name:         "command:jump",
						Raw:          `"jump"`,
						SourceSymbol: "command:jump:2:hero.def",
						Target:       "command:jump",
						Span:         sourceSpan(3, 1, 3, 16),
						IsDynamic:    false,
					},
				},
			},
		},
		Diagnostics: []ir.Diagnostic{
			{
				Path:          "hero.def",
				Code:          "test-info",
				Severity:      ir.SeverityInfo,
				Message:       "fixture",
				Start:         sourcePosition(1, 1),
				End:           sourcePosition(1, 8),
				RelatedSymbol: "command:jump",
			},
		},
	}

	sem := semantics.ResolveResult{
		References: []semantics.ReferenceResolution{
			{
				ReferenceID:    "ref:command:3:hero.def",
				SourcePath:     "hero.def",
				SourceSymbol:   "command:jump:2:hero.def",
				Resolved:       true,
				TargetSymbolID: "command:jump:2:hero.def",
				TargetPath:     "hero.def",
			},
		},
		Diagnostics: []ir.Diagnostic{
			{
				Path:     "hero.def",
				Code:     "dup",
				Severity: ir.SeverityWarning,
				Message:  "secondary",
				Start:    sourcePosition(2, 2),
				End:      sourcePosition(2, 7),
			},
		},
	}

	sql := Export(ws, sem)

	mustContain(t, sql, "BEGIN TRANSACTION;")
	mustContain(t, sql, "COMMIT;")

	mustContain(t, sql, "CREATE TABLE IF NOT EXISTS ikm_documents")
	mustContain(t, sql, "CREATE TABLE IF NOT EXISTS ikm_symbols")
	mustContain(t, sql, "CREATE TABLE IF NOT EXISTS ikm_references")
	mustContain(t, sql, "CREATE TABLE IF NOT EXISTS ikm_diagnostics")

	mustContain(t, sql, "CREATE VIRTUAL TABLE IF NOT EXISTS ikm_documents_fts")
	mustContain(t, sql, "CREATE VIRTUAL TABLE IF NOT EXISTS ikm_symbols_fts")
	mustContain(t, sql, "CREATE VIRTUAL TABLE IF NOT EXISTS ikm_references_fts")
	mustContain(t, sql, "CREATE VIRTUAL TABLE IF NOT EXISTS ikm_diagnostics_fts")

	mustContain(t, sql, "USING fts5")

	mustContain(t, sql, "-- FTS5: full-text index over documents")
	mustContain(t, sql, "-- FTS5: full-text index over symbols")
	mustContain(t, sql, "-- FTS5: full-text index over references")
	mustContain(t, sql, "-- FTS5: full-text index over diagnostics")
}
func TestExportMatchesReferencesWithCanonicalPath(t *testing.T) {
	t.Helper()

	docPath := filepath.Join("chars", "hero.def")
	canonicalDocPath := filepath.ToSlash(docPath)

	ws := workspace.LoadResult{
		Documents: []ir.Document{
			{
				Version:  ir.Version,
				Path:     docPath,
				FileType: "def",
				Symbols: []ir.Symbol{
					{
						ID:      "command:jump:2:" + canonicalDocPath,
						Kind:    ir.SymbolCommand,
						Name:    "command:jump",
						Section: "Command",
						Span:    sourceSpan(2, 1, 2, 16),
						Raw:     `"jump"`,
					},
				},
				References: []ir.Reference{
					{
						ID:           "ref:command:3:" + canonicalDocPath,
						Kind:         ir.ReferenceCommand,
						Name:         "command:jump",
						Raw:          `command = "jump"`,
						SourceSymbol: "state:controller:3:" + canonicalDocPath,
						Target:       "command:jump",
						Span:         sourceSpan(3, 1, 3, 22),
						IsDynamic:    false,
					},
				},
			},
		},
	}

	sem := semantics.ResolveResult{
		References: []semantics.ReferenceResolution{
			{
				ReferenceID:    "ref:command:3:" + canonicalDocPath,
				SourcePath:     canonicalDocPath,
				SourceSymbol:   "state:controller:3:" + canonicalDocPath,
				Resolved:       true,
				TargetSymbolID: "command:jump:2:" + canonicalDocPath,
				TargetPath:     canonicalDocPath,
				IsDynamic:      false,
			},
		},
	}

	sql := Export(ws, sem)

	mustContain(t, sql, "'command = \"jump\"'")
	mustContain(t, sql, "INSERT INTO ikm_references")
}

func TestExportEscapesSingleQuotesProperly(t *testing.T) {
	t.Helper()

	ws := workspace.LoadResult{
		Documents: []ir.Document{
			{
				Version:  ir.Version,
				Path:     "o'connor.def",
				FileType: "def",
				Symbols: []ir.Symbol{
					{
						ID:      "command:it's:2:o'connor.def",
						Kind:    ir.SymbolCommand,
						Name:    "command:it's",
						Section: "Command",
						Span:    sourceSpan(2, 1, 2, 16),
						Raw:     `it's`,
					},
				},
				References: []ir.Reference{
					{
						ID:           "ref:command:4:o'connor.def",
						Kind:         ir.ReferenceCommand,
						Name:         "command:it's",
						Raw:          `command = "it's"`,
						SourceSymbol: "command:it's:2:o'connor.def",
						Target:       "command:it's",
						Span:         sourceSpan(4, 1, 4, 19),
					},
				},
			},
		},
		Diagnostics: []ir.Diagnostic{
			{
				Path:     "o'connor.def",
				Code:     "quoted-diagnostic",
				Severity: ir.SeverityError,
				Message:  "it's broken",
				Start:    sourcePosition(5, 3),
				End:      sourcePosition(5, 15),
			},
		},
	}

	sem := semantics.ResolveResult{
		References: []semantics.ReferenceResolution{
			{
				ReferenceID:    "ref:command:4:o'connor.def",
				SourcePath:     "o'connor.def",
				SourceSymbol:   "command:it's:2:o'connor.def",
				Resolved:       true,
				TargetSymbolID: "command:it's:2:o'connor.def",
				TargetPath:     "o'connor.def",
			},
		},
	}

	sql := Export(ws, sem)

	mustContain(t, sql, "('o''connor.def'")
	mustContain(t, sql, "'command:it''s'")
	mustContain(t, sql, "'it''s broken'")
}

func TestExportIsDeterministicAcrossInputOrdering(t *testing.T) {
	t.Helper()

	wsA := workspace.LoadResult{Documents: []ir.Document{
		{
			Version:  ir.Version,
			Path:     "zeta.def",
			FileType: "def",
			Symbols: []ir.Symbol{
				{ID: "state:10:3:zeta.def", Kind: ir.SymbolStateDef, Name: "state:10", Section: "Statedef 10", Span: sourceSpan(3, 1, 3, 12)},
				{ID: "command:beta:4:zeta.def", Kind: ir.SymbolCommand, Name: "command:beta", Section: "Command", Span: sourceSpan(4, 1, 4, 18)},
			},
			References: []ir.Reference{{
				ID:           "ref:state:12:zeta.def",
				Kind:         ir.ReferenceState,
				Name:         "state:9",
				Raw:          "9",
				SourceSymbol: "state:10:3:zeta.def",
				Target:       "state:9",
				Span:         sourceSpan(12, 1, 12, 8),
			}},
		},
		{
			Version:  ir.Version,
			Path:     "alpha.def",
			FileType: "def",
			Symbols: []ir.Symbol{
				{ID: "state:2:1:alpha.def", Kind: ir.SymbolStateDef, Name: "state:2", Section: "Statedef 2", Span: sourceSpan(1, 1, 1, 11)},
			},
			References: []ir.Reference{{
				ID:           "ref:state:1:alpha.def",
				Kind:         ir.ReferenceState,
				Name:         "state:1",
				Raw:          "1",
				SourceSymbol: "state:2:1:alpha.def",
				Target:       "state:1",
				Span:         sourceSpan(1, 1, 1, 10),
			}},
		},
	}}

	wsB := workspace.LoadResult{Documents: []ir.Document{wsA.Documents[1], wsA.Documents[0]}}

	sem := semantics.ResolveResult{References: []semantics.ReferenceResolution{
		{ReferenceID: "ref:state:1:alpha.def", SourcePath: "alpha.def", SourceSymbol: "state:2:1:alpha.def", Resolved: true, TargetSymbolID: "state:1:1:alpha.def", TargetPath: "alpha.def"},
		{ReferenceID: "ref:state:12:zeta.def", SourcePath: "zeta.def", SourceSymbol: "state:10:3:zeta.def", Resolved: false, IsDynamic: true},
	}}

	semReverse := semantics.ResolveResult{References: []semantics.ReferenceResolution{
		{ReferenceID: "ref:state:12:zeta.def", SourcePath: "zeta.def", SourceSymbol: "state:10:3:zeta.def", Resolved: false, IsDynamic: true},
		{ReferenceID: "ref:state:1:alpha.def", SourcePath: "alpha.def", SourceSymbol: "state:2:1:alpha.def", Resolved: true, TargetSymbolID: "state:1:1:alpha.def", TargetPath: "alpha.def"},
	}}

	left := Export(wsA, sem)
	middle := Export(wsA, semReverse)
	right := Export(wsB, sem)
	if left != middle {
		t.Fatalf("expected deterministic output independent of reference order")
	}
	if left != right {
		t.Fatalf("expected deterministic output independent of document order")
	}
}
func TestExportClearsExistingRowsBetweenRuns(t *testing.T) {
	t.Helper()

	ws := workspace.LoadResult{
		Documents: []ir.Document{
			{
				Version:  ir.Version,
				Path:     "hero.def",
				FileType: "def",
			},
		},
	}

	sql := Export(ws, semantics.ResolveResult{})

	deleteIdx := strings.Index(sql, "DELETE FROM ikm_documents;")
	if deleteIdx < 0 {
		t.Fatalf("expected document clear statement in SQL")
	}
	insertIdx := strings.Index(sql, "INSERT INTO ikm_documents (path, file_type, version) VALUES")
	if insertIdx < 0 {
		t.Fatalf("expected document insert statement in SQL")
	}
	if insertIdx < deleteIdx {
		t.Fatalf("expected deletes to happen before inserts")
	}
	for _, want := range []string{
		"DELETE FROM ikm_references;",
		"DELETE FROM ikm_diagnostics;",
		"DELETE FROM ikm_symbols;",
		"DELETE FROM ikm_documents;",
		"DELETE FROM ikm_references_fts;",
		"DELETE FROM ikm_diagnostics_fts;",
		"DELETE FROM ikm_symbols_fts;",
		"DELETE FROM ikm_documents_fts;",
	} {
		mustContain(t, sql, want)
	}
}

func TestExportPersistsReferenceClassification(t *testing.T) {
	t.Helper()

	ws := workspace.LoadResult{
		Documents: []ir.Document{
			{
				Version:  ir.Version,
				Path:     "hero.def",
				FileType: "def",
				References: []ir.Reference{
					{
						ID:           "ref:command:3:hero.def",
						Kind:         ir.ReferenceCommand,
						Name:         "command:jump",
						Raw:          `"jump"`,
						SourceSymbol: "state:100:4:hero.def",
						Target:       "command:jump",
						Span:         sourceSpan(3, 1, 3, 10),
					},
				},
			},
		},
	}
	sem := semantics.ResolveResult{
		References: []semantics.ReferenceResolution{
			{
				ReferenceID:    "ref:command:3:hero.def",
				SourcePath:     "hero.def",
				SourceSymbol:   "state:100:4:hero.def",
				Resolved:       true,
				TargetSymbolID: "command:jump:3:hero.def",
				TargetPath:     "hero.def",
				Classification: semantics.AmbiguousResolution,
			},
		},
	}

	sql := Export(ws, sem)

	mustContain(t, sql, "'ambiguous'")
	mustContain(t, sql, "INSERT INTO ikm_references_fts (document_path, kind, name, raw, source_symbol, target, target_symbol_id, target_path, classification)")
}

func TestExportDisambiguatesReferenceIDsOnCollision(t *testing.T) {
	t.Helper()

	ws := workspace.LoadResult{
		Documents: []ir.Document{
			{
				Version:  ir.Version,
				Path:     "hero.def",
				FileType: "def",
				References: []ir.Reference{
					{
						ID:           "ref:command:3:hero.def",
						Kind:         ir.ReferenceCommand,
						Name:         "command:jump",
						Raw:          `command = "jump"`,
						SourceSymbol: "state:100:4:hero.def",
						Target:       "command:jump",
						Span:         sourceSpan(3, 1, 3, 20),
					},
					{
						ID:           "ref:command:3:hero.def",
						Kind:         ir.ReferenceCommand,
						Name:         "command:run",
						Raw:          `command = "run"`,
						SourceSymbol: "state:100:4:hero.def",
						Target:       "command:run",
						Span:         sourceSpan(3, 1, 3, 20),
					},
				},
			},
		},
	}
	sem := semantics.ResolveResult{
		References: []semantics.ReferenceResolution{
			{
				ReferenceID:    "ref:command:3:hero.def",
				SourcePath:     "hero.def",
				SourceSymbol:   "state:100:4:hero.def",
				Resolved:       true,
				TargetSymbolID: "command:jump:3:hero.def",
				TargetPath:     "hero.def",
				Classification: semantics.ExactResolution,
			},
			{
				ReferenceID:    "ref:command:3:hero.def",
				SourcePath:     "hero.def",
				SourceSymbol:   "state:100:4:hero.def",
				Resolved:       true,
				TargetSymbolID: "command:run:3:hero.def",
				TargetPath:     "hero.def",
				Classification: semantics.ExactResolution,
			},
		},
	}

	sql := Export(ws, sem)
	needs := strings.Count(sql, "INSERT INTO ikm_references (")
	if needs != 2 {
		t.Fatalf("expected two reference inserts, got %d", needs)
	}
	mustContain(t, sql, "'ref:command:3:hero.def'")
	mustContain(t, sql, "'ref:command:3:hero.def:1'")
}

func TestExportHandlesEmptyWorkspaceWithoutDataRows(t *testing.T) {
	t.Helper()

	sql := Export(workspace.LoadResult{}, semantics.ResolveResult{})

	mustContain(t, sql, "BEGIN TRANSACTION;")
	mustContain(t, sql, "COMMIT;")
	mustContain(t, sql, "CREATE TABLE IF NOT EXISTS ikm_documents")
	mustContain(t, sql, "CREATE TABLE IF NOT EXISTS ikm_symbols")
	mustContain(t, sql, "CREATE TABLE IF NOT EXISTS ikm_references")
	mustContain(t, sql, "CREATE TABLE IF NOT EXISTS ikm_diagnostics")

	if strings.Contains(sql, "INSERT INTO ikm_documents (") {
		t.Fatalf("expected no document inserts for empty workspace")
	}
	if strings.Contains(sql, "INSERT INTO ikm_symbols (") {
		t.Fatalf("expected no symbol inserts for empty workspace")
	}
	if strings.Contains(sql, "INSERT INTO ikm_references (") {
		t.Fatalf("expected no reference inserts for empty workspace")
	}
	if strings.Contains(sql, "INSERT INTO ikm_diagnostics (") {
		t.Fatalf("expected no diagnostic inserts for empty workspace")
	}
}

func mustContain(t *testing.T, got, needle string) {
	t.Helper()
	if !strings.Contains(got, needle) {
		t.Fatalf("expected SQL output to contain %q, got:\n%s", needle, got)
	}
}

func sourcePosition(line, column int) ir.SourcePosition {
	return ir.SourcePosition{Line: line, Column: column}
}

func sourceSpan(startLine, startColumn, endLine, endColumn int) ir.SourceSpan {
	return ir.SourceSpan{Start: sourcePosition(startLine, startColumn), End: sourcePosition(endLine, endColumn)}
}
