// Package index provides deterministic, dependency-free SQL exports for indexing parsed workspace and semantic analysis results.
// It intentionally avoids using a SQLite driver; callers can pipe the emitted SQL directly into sqlite3.
package index

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/semantics"
	"github.com/ikemen-engine/ikemen-devtools/internal/workspace"
)

const (
	tableDocuments   = "ikm_documents"
	tableSymbols     = "ikm_symbols"
	tableReferences  = "ikm_references"
	tableDiagnostics = "ikm_diagnostics"

	ftsDocuments   = "ikm_documents_fts"
	ftsSymbols     = "ikm_symbols_fts"
	ftsReferences  = "ikm_references_fts"
	ftsDiagnostics = "ikm_diagnostics_fts"
)

// Export emits deterministic SQL statements for loading workspace and semantic results
// into a SQLite database. Output can be piped directly into sqlite3.
func Export(ws workspace.LoadResult, sr semantics.ResolveResult) string {
	documents := sortedDocuments(ws.Documents)
	symbols := sortedSymbols(documents)
	refs := sortedReferenceRows(resolveReferenceRows(documents, sr.References))
	diagnostics := sortedDiagnostics(append(append([]ir.Diagnostic{}, ws.Diagnostics...), sr.Diagnostics...))

	statements := []string{"BEGIN TRANSACTION;"}
	statements = append(statements, schemaSQL()...)
	statements = append(statements, refreshStatements()...)

	for _, doc := range documents {
		statements = append(statements, insertDocument(doc))
		statements = append(statements, insertDocumentFTS(doc))
	}

	for _, row := range symbols {
		statements = append(statements, insertSymbol(row.doc.Path, row.symbol))
		statements = append(statements, insertSymbolFTS(row.doc.Path, row.symbol))
	}

	for _, row := range refs {
		statements = append(statements, insertReference(row))
		statements = append(statements, insertReferenceFTS(row))
	}

	for _, diag := range diagnostics {
		statements = append(statements, insertDiagnostic(diag))
		statements = append(statements, insertDiagnosticFTS(diag))
	}

	statements = append(statements, "COMMIT;")
	return strings.Join(statements, "\n")
}

func schemaSQL() []string {
	return []string{
		"-- Stable schema for ikemen index output.",
		"CREATE TABLE IF NOT EXISTS " + tableDocuments + " (",
		"    path TEXT PRIMARY KEY NOT NULL,",
		"    file_type TEXT NOT NULL,",
		"    version TEXT NOT NULL",
		");",
		"CREATE TABLE IF NOT EXISTS " + tableSymbols + " (",
		"    id TEXT PRIMARY KEY NOT NULL,",
		"    document_path TEXT NOT NULL,",
		"    kind TEXT NOT NULL,",
		"    name TEXT NOT NULL,",
		"    section TEXT NOT NULL,",
		"    raw TEXT NOT NULL,",
		"    start_line INTEGER NOT NULL,",
		"    start_column INTEGER NOT NULL,",
		"    end_line INTEGER NOT NULL,",
		"    end_column INTEGER NOT NULL",
		");",
		"CREATE TABLE IF NOT EXISTS " + tableReferences + " (",
		"    id TEXT NOT NULL,",
		"    document_path TEXT NOT NULL,",
		"    kind TEXT NOT NULL,",
		"    name TEXT NOT NULL,",
		"    raw TEXT NOT NULL,",
		"    source_symbol TEXT NOT NULL,",
		"    target TEXT NOT NULL,",
		"    target_symbol_id TEXT,",
		"    target_path TEXT,",
		"    classification TEXT NOT NULL,",
		"    resolved INTEGER NOT NULL,",
		"    is_dynamic INTEGER NOT NULL,",
		"    start_line INTEGER NOT NULL,",
		"    start_column INTEGER NOT NULL,",
		"    end_line INTEGER NOT NULL,",
		"    end_column INTEGER NOT NULL,",
		"    PRIMARY KEY (",
		"        document_path,",
		"        source_symbol,",
		"        target,",
		"        kind,",
		"        start_line,",
		"        start_column,",
		"        end_line,",
		"        end_column",
		"    )",
		");",
		"CREATE TABLE IF NOT EXISTS " + tableDiagnostics + " (",
		"    path TEXT NOT NULL,",
		"    code TEXT NOT NULL,",
		"    severity TEXT NOT NULL,",
		"    message TEXT NOT NULL,",
		"    related_symbol TEXT,",
		"    start_line INTEGER NOT NULL,",
		"    start_column INTEGER NOT NULL,",
		"    end_line INTEGER NOT NULL,",
		"    end_column INTEGER NOT NULL",
		");",
		"-- FTS5: full-text index over documents",
		"CREATE VIRTUAL TABLE IF NOT EXISTS " + ftsDocuments + " USING fts5(path, file_type, version);",
		"-- FTS5: full-text index over symbols",
		"CREATE VIRTUAL TABLE IF NOT EXISTS " + ftsSymbols + " USING fts5(document_path, kind, name, section, raw);",
		"-- FTS5: full-text index over references",
		"CREATE VIRTUAL TABLE IF NOT EXISTS " + ftsReferences + " USING fts5(document_path, kind, name, raw, source_symbol, target, target_symbol_id, target_path, classification);",
		"-- FTS5: full-text index over diagnostics",
		"CREATE VIRTUAL TABLE IF NOT EXISTS " + ftsDiagnostics + " USING fts5(path, code, severity, message, related_symbol);",
	}
}

func refreshStatements() []string {
	return []string{
		"DELETE FROM " + tableReferences + ";",
		"DELETE FROM " + tableDiagnostics + ";",
		"DELETE FROM " + tableSymbols + ";",
		"DELETE FROM " + tableDocuments + ";",
		"DELETE FROM " + ftsReferences + ";",
		"DELETE FROM " + ftsDiagnostics + ";",
		"DELETE FROM " + ftsSymbols + ";",
		"DELETE FROM " + ftsDocuments + ";",
	}
}

func insertDocument(doc ir.Document) string {
	return fmt.Sprintf(
		"INSERT INTO %s (path, file_type, version) VALUES (%s, %s, %s);",
		tableDocuments,
		quoteString(doc.Path),
		quoteString(doc.FileType),
		quoteString(doc.Version),
	)
}

func insertDocumentFTS(doc ir.Document) string {
	return fmt.Sprintf(
		"INSERT INTO %s (path, file_type, version) VALUES (%s, %s, %s);",
		ftsDocuments,
		quoteString(doc.Path),
		quoteString(doc.FileType),
		quoteString(doc.Version),
	)
}

func insertSymbol(docPath string, sym ir.Symbol) string {
	return fmt.Sprintf(
		"INSERT INTO %s (id, document_path, kind, name, section, raw, start_line, start_column, end_line, end_column) VALUES (%s, %s, %s, %s, %s, %s, %d, %d, %d, %d);",
		tableSymbols,
		quoteString(sym.ID),
		quoteString(docPath),
		quoteString(string(sym.Kind)),
		quoteString(sym.Name),
		quoteString(sym.Section),
		quoteString(sym.Raw),
		sym.Span.Start.Line,
		sym.Span.Start.Column,
		sym.Span.End.Line,
		sym.Span.End.Column,
	)
}

func insertSymbolFTS(docPath string, sym ir.Symbol) string {
	return fmt.Sprintf(
		"INSERT INTO %s (document_path, kind, name, section, raw) VALUES (%s, %s, %s, %s, %s);",
		ftsSymbols,
		quoteString(docPath),
		quoteString(string(sym.Kind)),
		quoteString(sym.Name),
		quoteString(sym.Section),
		quoteString(sym.Raw),
	)
}

func insertReference(row referenceExportRow) string {
	return fmt.Sprintf(
		"INSERT INTO %s (id, document_path, kind, name, raw, source_symbol, target, target_symbol_id, target_path, classification, resolved, is_dynamic, start_line, start_column, end_line, end_column) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %d, %d, %d, %d, %d, %d);",
		tableReferences,
		quoteString(row.id),
		quoteString(row.documentPath),
		quoteString(row.kind),
		quoteString(row.name),
		quoteString(row.raw),
		quoteString(row.sourceSymbol),
		quoteString(row.target),
		quoteNullableString(row.targetSymbolID),
		quoteNullableString(row.targetPath),
		quoteString(classificationValue(row.classification)),
		boolToInt(row.resolved),
		boolToInt(row.isDynamic),
		normalizeLine(row.startLine),
		normalizeColumn(row.startColumn),
		normalizeLine(row.endLine),
		normalizeColumn(row.endColumn),
	)
}

func insertReferenceFTS(row referenceExportRow) string {
	return fmt.Sprintf(
		"INSERT INTO %s (document_path, kind, name, raw, source_symbol, target, target_symbol_id, target_path, classification) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s);",
		ftsReferences,
		quoteString(row.documentPath),
		quoteString(row.kind),
		quoteString(row.name),
		quoteString(row.raw),
		quoteString(row.sourceSymbol),
		quoteString(row.target),
		quoteNullableString(row.targetSymbolID),
		quoteNullableString(row.targetPath),
		quoteString(classificationValue(row.classification)),
	)
}

func insertDiagnostic(d ir.Diagnostic) string {
	return fmt.Sprintf(
		"INSERT INTO %s (path, code, severity, message, related_symbol, start_line, start_column, end_line, end_column) VALUES (%s, %s, %s, %s, %s, %d, %d, %d, %d);",
		tableDiagnostics,
		quoteString(d.Path),
		quoteString(d.Code),
		quoteString(string(d.Severity)),
		quoteString(d.Message),
		quoteNullableString(d.RelatedSymbol),
		normalizeStart(d.Start.Line),
		normalizeColumn(d.Start.Column),
		normalizeLine(d.End.Line),
		normalizeColumn(d.End.Column),
	)
}

func insertDiagnosticFTS(d ir.Diagnostic) string {
	return fmt.Sprintf(
		"INSERT INTO %s (path, code, severity, message, related_symbol) VALUES (%s, %s, %s, %s, %s);",
		ftsDiagnostics,
		quoteString(d.Path),
		quoteString(d.Code),
		quoteString(string(d.Severity)),
		quoteString(d.Message),
		quoteNullableString(d.RelatedSymbol),
	)
}

func quoteString(value string) string {
	escaped := strings.ReplaceAll(value, "\x00", "\\x00")
	return "'" + strings.ReplaceAll(escaped, "'", "''") + "'"
}

func quoteNullableString(value string) string {
	if value == "" {
		return "NULL"
	}
	return quoteString(value)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func normalizeLine(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

func normalizeStart(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

func normalizeColumn(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

func classificationValue(v string) string {
	if v == "" {
		return "unresolved"
	}
	return v
}

type symbolExportRow struct {
	doc    ir.Document
	symbol ir.Symbol
}

type referenceExportRow struct {
	id             string
	documentPath   string
	kind           string
	name           string
	raw            string
	sourceSymbol   string
	target         string
	targetSymbolID string
	targetPath     string
	classification string
	resolved       bool
	isDynamic      bool
	startLine      int
	startColumn    int
	endLine        int
	endColumn      int
}

func sortedDocuments(docs []ir.Document) []ir.Document {
	clone := append([]ir.Document(nil), docs...)
	sort.Slice(clone, func(i, j int) bool {
		left := clone[i]
		right := clone[j]
		leftPath := canonicalPath(left.Path)
		rightPath := canonicalPath(right.Path)
		if leftPath != rightPath {
			return leftPath < rightPath
		}
		if left.FileType != right.FileType {
			return left.FileType < right.FileType
		}
		return left.Version < right.Version
	})
	return clone
}

func sortedSymbols(docs []ir.Document) []symbolExportRow {
	rows := make([]symbolExportRow, 0)
	for _, doc := range docs {
		for _, symbol := range doc.Symbols {
			rows = append(rows, symbolExportRow{doc: doc, symbol: symbol})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		a := rows[i]
		b := rows[j]
		aPath := canonicalPath(a.doc.Path)
		bPath := canonicalPath(b.doc.Path)
		if aPath != bPath {
			return aPath < bPath
		}
		if a.symbol.Kind != b.symbol.Kind {
			return string(a.symbol.Kind) < string(b.symbol.Kind)
		}
		if a.symbol.Name != b.symbol.Name {
			return a.symbol.Name < b.symbol.Name
		}
		if a.symbol.Span.Start.Line != b.symbol.Span.Start.Line {
			return a.symbol.Span.Start.Line < b.symbol.Span.Start.Line
		}
		if a.symbol.Span.Start.Column != b.symbol.Span.Start.Column {
			return a.symbol.Span.Start.Column < b.symbol.Span.Start.Column
		}
		return a.symbol.ID < b.symbol.ID
	})

	return rows
}

func resolveReferenceRows(docs []ir.Document, refs []semantics.ReferenceResolution) []referenceExportRow {
	if len(docs) == 0 && len(refs) == 0 {
		return nil
	}

	base := buildReferenceLookup(docs)
	rows := make([]referenceExportRow, 0)
	seenByKey := make(map[string]int)

	if len(refs) > 0 {
		for _, ref := range refs {
			sourcePath := canonicalPath(ref.SourcePath)
			keys := referenceResolutionLookupKeys(sourcePath, ref.ReferenceIdentity, ref.ReferenceID)
			parsed, ok := referenceCandidates(base, seenByKey, keys)

			entry := referenceExportRow{
				resolved:       ref.Resolved,
				isDynamic:      ref.IsDynamic,
				targetSymbolID: ref.TargetSymbolID,
				targetPath:     ref.TargetPath,
				classification: ref.Classification,
			}

			if ok {
				entry.id = parsed.ID
				entry.documentPath = sourcePath
				entry.kind = string(parsed.Kind)
				entry.name = parsed.Name
				entry.raw = parsed.Raw
				entry.sourceSymbol = parsed.SourceSymbol
				entry.target = parsed.Target
				entry.startLine = parsed.Span.Start.Line
				entry.startColumn = normalizeColumn(parsed.Span.Start.Column)
				entry.endLine = normalizeLine(parsed.Span.End.Line)
				entry.endColumn = normalizeColumn(parsed.Span.End.Column)
			} else {
				entry.id = referenceResultLookupID(ref.ReferenceIdentity, ref.ReferenceID)
				entry.documentPath = sourcePath
				entry.kind = inferredReferenceKind(ref)
				if ref.ReferenceIdentity.SemanticKey != "" {
					entry.name = ref.ReferenceIdentity.SemanticKey
				} else {
					entry.name = ref.SourceSymbol
				}
				entry.raw = ""
				entry.sourceSymbol = ref.SourceSymbol
				if ref.TargetIdentity.SemanticKey != "" {
					entry.target = ref.TargetIdentity.SemanticKey
				} else {
					entry.target = ref.TargetSymbolID
				}
				entry.startLine = 1
				entry.startColumn = 1
				entry.endLine = 1
				entry.endColumn = 2
			}

			if entry.id == "" {
				entry.id = referenceExportID(entry.kind, entry.startLine, entry.documentPath)
			}
			rows = append(rows, entry)
		}
		disambiguateReferenceIDs(rows)
		return rows
	}

	for _, doc := range docs {
		for _, ref := range doc.References {
			rows = append(rows, referenceExportRow{
				id:             referenceExportID(string(ref.Kind), ref.Span.Start.Line, doc.Path),
				documentPath:   canonicalPath(doc.Path),
				kind:           string(ref.Kind),
				name:           ref.Name,
				raw:            ref.Raw,
				sourceSymbol:   ref.SourceSymbol,
				target:         ref.Target,
				isDynamic:      ref.IsDynamic,
				classification: "unresolved",
				startLine:      normalizeLine(ref.Span.Start.Line),
				startColumn:    normalizeColumn(ref.Span.Start.Column),
				endLine:        normalizeLine(ref.Span.End.Line),
				endColumn:      normalizeColumn(ref.Span.End.Column),
			})
		}
	}
	disambiguateReferenceIDs(rows)
	return rows
}

func referenceCandidates(base referenceByKey, seen map[string]int, keys []string) (ir.Reference, bool) {
	for _, key := range keys {
		candidates := base[key]
		next := seen[key]
		if next >= len(candidates) {
			continue
		}
		seen[key] = next + 1
		return candidates[next], true
	}
	return ir.Reference{}, false
}

func referenceResultLookupID(identity ir.Identity, legacyID string) string {
	if identity.StoreID != "" {
		return identity.StoreID
	}
	if identity.OccurrenceID != "" {
		return identity.OccurrenceID
	}
	return legacyID
}

func disambiguateReferenceIDs(rows []referenceExportRow) {
	seen := make(map[string]int, len(rows))
	for i := range rows {
		base := rows[i].id
		if base == "" {
			base = referenceExportID(rows[i].kind, rows[i].startLine, rows[i].documentPath)
			rows[i].id = base
		}
		seen[base]++
		if seen[base] == 1 {
			continue
		}
		rows[i].id = base + ":" + fmt.Sprintf("%d", seen[base]-1)
	}
}

func referenceExportID(kind string, lineNo int, path string) string {
	return fmt.Sprintf("ref:%s:%d:%s", kind, normalizeLine(lineNo), canonicalPath(path))
}

type referenceByKey map[string][]ir.Reference

func buildReferenceLookup(docs []ir.Document) referenceByKey {
	out := make(referenceByKey)
	for _, doc := range docs {
		docPath := canonicalPath(doc.Path)
		for _, ref := range doc.References {
			for _, key := range referenceResolutionLookupKeys(docPath, ref.Identity, ref.ID) {
				out[key] = append(out[key], ref)
			}
		}
	}
	return out
}

func referenceResolutionLookupKeys(path string, identity ir.Identity, legacyID string) []string {
	keys := make([]string, 0, 3)
	seen := map[string]struct{}{}
	for _, key := range []string{identity.StoreID, identity.OccurrenceID, legacyID} {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, refLookupKey(path, key))
	}
	return keys
}

func refLookupKey(path, id string) string {
	return canonicalPath(path) + "\x00" + id
}

func sortedReferenceRows(rows []referenceExportRow) []referenceExportRow {
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]
		leftPath := canonicalPath(left.documentPath)
		rightPath := canonicalPath(right.documentPath)
		if leftPath != rightPath {
			return leftPath < rightPath
		}
		if left.id != right.id {
			return left.id < right.id
		}
		if left.sourceSymbol != right.sourceSymbol {
			return left.sourceSymbol < right.sourceSymbol
		}
		if left.startLine != right.startLine {
			return left.startLine < right.startLine
		}
		if left.startColumn != right.startColumn {
			return left.startColumn < right.startColumn
		}
		return left.name < right.name
	})
	return rows
}

func canonicalPath(path string) string {
	clean := strings.TrimSpace(path)
	if clean == "" || clean == "." {
		return "memory"
	}
	clean = filepath.Clean(clean)
	return filepath.ToSlash(clean)
}

func inferredReferenceKind(ref semantics.ReferenceResolution) string {
	if strings.HasPrefix(ref.TargetIdentity.SemanticKey, "command:") {
		return string(ir.ReferenceCommand)
	}
	if strings.HasPrefix(ref.TargetIdentity.SemanticKey, "state:") {
		return string(ir.ReferenceState)
	}
	if strings.HasPrefix(ref.TargetSymbolID, "command:") {
		return string(ir.ReferenceCommand)
	}
	if strings.HasPrefix(ref.TargetSymbolID, "state:") {
		return string(ir.ReferenceState)
	}
	return "unknown"
}

func sortedDiagnostics(diags []ir.Diagnostic) []ir.Diagnostic {
	sort.Slice(diags, func(i, j int) bool {
		left := diags[i]
		right := diags[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Start.Line != right.Start.Line {
			return left.Start.Line < right.Start.Line
		}
		if left.Start.Column != right.Start.Column {
			return left.Start.Column < right.Start.Column
		}
		if left.End.Line != right.End.Line {
			return left.End.Line < right.End.Line
		}
		if left.End.Column != right.End.Column {
			return left.End.Column < right.End.Column
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.Severity != right.Severity {
			return left.Severity < right.Severity
		}
		if left.Message != right.Message {
			return left.Message < right.Message
		}
		return left.RelatedSymbol < right.RelatedSymbol
	})
	return diags
}
