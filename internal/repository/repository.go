package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	// ErrNilDatabase means no *sql.DB was provided.
	ErrNilDatabase = errors.New("repository: nil database")
	// ErrEmptyPath means a required document path was blank.
	ErrEmptyPath = errors.New("repository: document path is required")
	// ErrContextCanceled indicates the operation observed a cancelled context.
	ErrContextCanceled = errors.New("repository: context canceled")
	// ErrSchemaUnsupported means the on-disk schema is too new for this binary.
	ErrSchemaUnsupported = errors.New("repository: unsupported schema version")
	// ErrSnapshotNotFound indicates the requested path is missing.
	ErrSnapshotNotFound = errors.New("repository: snapshot not found")
	// ErrNoRows means a requested row does not exist.
	ErrNotFound = errors.New("repository: no rows")
)

// Repository stores semantic snapshots in SQLite.
type Repository struct {
	db *sql.DB
}

// DocumentSnapshot describes a single canonical document and its stored semantic payload.
type DocumentSnapshot struct {
	Path          string
	FileType      string
	Version       string
	ContentHash   string
	Generation    int64
	UpdatedAtUnix int64

	Symbols      []SymbolSnapshot
	References   []ReferenceSnapshot
	Diagnostics  []DiagnosticSnapshot
	DependencyIn []DependencyEdge
}

// SymbolSnapshot captures a resolved symbol row.
type SymbolSnapshot struct {
	ID           string
	DocumentPath string
	Kind         string
	Name         string
	Section      string
	Raw          string
	StartLine    int
	StartColumn  int
	EndLine      int
	EndColumn    int
}

// ReferenceSnapshot captures a resolved symbol reference row.
type ReferenceSnapshot struct {
	ID             string
	DocumentPath   string
	Kind           string
	Name           string
	Raw            string
	SourceSymbol   string
	Target         string
	TargetSymbolID string
	TargetPath     string
	Classification string
	Resolved       bool
	IsDynamic      bool
	StartLine      int
	StartColumn    int
	EndLine        int
	EndColumn      int
}

// DiagnosticSnapshot captures one diagnostic row.
type DiagnosticSnapshot struct {
	Path          string
	Code          string
	Severity      string
	Message       string
	RelatedSymbol string
	StartLine     int
	StartColumn   int
	EndLine       int
	EndColumn     int
}

// DependencyEdge captures a dependency relation between documents.
type DependencyEdge struct {
	SourcePath string
	TargetPath string
	Type       string
}

// New returns a repository bound to db after ensuring current schema migrations.
func New(ctx context.Context, db *sql.DB) (*Repository, error) {
	repo, err := Open(ctx, db)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

func Open(ctx context.Context, db *sql.DB) (*Repository, error) {
	if db == nil {
		return nil, ErrNilDatabase
	}
	if err := ApplyMigrations(ctx, db, CurrentSchemaVersion); err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

func ApplyMigrations(ctx context.Context, db *sql.DB, targetVersion int) error {
	if db == nil {
		return ErrNilDatabase
	}
	if targetVersion <= 0 {
		return nil
	}
	if err := ctxErr(ctx); err != nil {
		return ErrContextCanceled
	}

	currentVersion, err := currentSchemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if currentVersion > targetVersion || currentVersion > CurrentSchemaVersion {
		return fmt.Errorf("%w: current=%d target=%d", ErrSchemaUnsupported, currentVersion, targetVersion)
	}
	if currentVersion == targetVersion {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	version := currentVersion
	for _, migration := range Migrations {
		if version >= migration.Version || migration.Version > targetVersion {
			continue
		}
		for _, statement := range migration.Statements {
			if err := ctxErr(ctx); err != nil {
				return ErrContextCanceled
			}
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migration %d (%s) failed: %w", migration.Version, migration.Name, err)
			}
		}
		version = migration.Version
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// UpsertDocumentSnapshot stores one document snapshot and all semantic rows for that document.
func (r *Repository) UpsertDocumentSnapshot(ctx context.Context, snapshot DocumentSnapshot) error {
	if r == nil || r.db == nil {
		return ErrNilDatabase
	}
	if err := ctxErr(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(snapshot.Path) == "" {
		return ErrEmptyPath
	}
	snapshot = normalizeSnapshot(snapshot)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := upsertDocument(ctx, tx, snapshot); err != nil {
		return err
	}
	if err := upsertSymbols(ctx, tx, snapshot.DocumentPath(), snapshot.Symbols); err != nil {
		return err
	}
	if err := upsertReferences(ctx, tx, snapshot.DocumentPath(), snapshot.References); err != nil {
		return err
	}
	if err := upsertDiagnostics(ctx, tx, snapshot.DocumentPath(), snapshot.Diagnostics); err != nil {
		return err
	}
	if err := upsertDependencyEdges(ctx, tx, snapshot.DocumentPath(), snapshot.DependencyIn); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// DeleteDocumentSnapshot removes a document and all dependent semantic rows atomically.
func (r *Repository) DeleteDocumentSnapshot(ctx context.Context, path string) error {
	if r == nil || r.db == nil {
		return ErrNilDatabase
	}
	if err := ctxErr(ctx); err != nil {
		return err
	}
	path = canonicalizePath(path)
	if path == "" {
		return ErrEmptyPath
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, table := range []string{referenceTable, symbolTable, diagnosticTable, dependencyEdgeTable} {
		column := "document_path"
		if table == diagnosticTable {
			column = "path"
		}
		if table == dependencyEdgeTable {
			column = "source_path"
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE "+column+" = ?", path); err != nil {
			return err
		}
		if table == dependencyEdgeTable {
			if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE target_path = ?", path); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM "+documentTable+" WHERE path = ?", path); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// ReadDocumentSnapshot returns one snapshot and all semantic rows for a document path.
func (r *Repository) ReadDocumentSnapshot(ctx context.Context, path string) (DocumentSnapshot, error) {
	if r == nil || r.db == nil {
		return DocumentSnapshot{}, ErrNilDatabase
	}
	if err := ctxErr(ctx); err != nil {
		return DocumentSnapshot{}, err
	}
	path = canonicalizePath(path)
	if path == "" {
		return DocumentSnapshot{}, ErrEmptyPath
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return DocumentSnapshot{}, err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		_ = tx.Rollback()
	}()

	doc, err := readDocumentRow(ctx, tx, path)
	if err != nil {
		return DocumentSnapshot{}, err
	}
	symbols, err := readSymbols(ctx, tx, path)
	if err != nil {
		return DocumentSnapshot{}, err
	}
	references, err := readReferences(ctx, tx, path)
	if err != nil {
		return DocumentSnapshot{}, err
	}
	diagnostics, err := readDiagnostics(ctx, tx, path)
	if err != nil {
		return DocumentSnapshot{}, err
	}
	edges, err := readDependencyEdges(ctx, tx, path)
	if err != nil {
		return DocumentSnapshot{}, err
	}

	if err := tx.Commit(); err != nil {
		return DocumentSnapshot{}, err
	}
	committed = true

	snapshot := DocumentSnapshot{
		Path:          doc.path,
		FileType:      doc.fileType,
		Version:       doc.version,
		ContentHash:   doc.contentHash,
		Generation:    doc.generation,
		UpdatedAtUnix: doc.updatedAt,
		Symbols:       symbols,
		References:    references,
		Diagnostics:   diagnostics,
		DependencyIn:  edges,
	}
	return snapshot, nil
}

// ListDocuments returns all stored document rows in deterministic path order.
func (r *Repository) ListDocuments(ctx context.Context) ([]DocumentSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, ErrNilDatabase
	}
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, selectAllDocumentsSQL)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	docs := make([]DocumentSnapshot, 0)
	for rows.Next() {
		var doc documentRow
		if err := rows.Scan(&doc.path, &doc.fileType, &doc.version, &doc.contentHash, &doc.updatedAt, &doc.generation); err != nil {
			return nil, err
		}
		docs = append(docs, doc.toSnapshot())
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return docs, nil
}

// ReadSymbols reads symbols for one document in deterministic order.
func (r *Repository) ReadSymbols(ctx context.Context, path string) ([]SymbolSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, ErrNilDatabase
	}
	path = canonicalizePath(path)
	if path == "" {
		return nil, ErrEmptyPath
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		_ = tx.Rollback()
	}()
	symbols, err := readSymbols(ctx, tx, path)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return symbols, nil
}

// ReadReferences reads references for one document in deterministic order.
func (r *Repository) ReadReferences(ctx context.Context, path string) ([]ReferenceSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, ErrNilDatabase
	}
	path = canonicalizePath(path)
	if path == "" {
		return nil, ErrEmptyPath
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		_ = tx.Rollback()
	}()
	references, err := readReferences(ctx, tx, path)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return references, nil
}

// ReadDiagnostics reads diagnostics for one document in deterministic order.
func (r *Repository) ReadDiagnostics(ctx context.Context, path string) ([]DiagnosticSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, ErrNilDatabase
	}
	path = canonicalizePath(path)
	if path == "" {
		return nil, ErrEmptyPath
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		_ = tx.Rollback()
	}()
	diagnostics, err := readDiagnostics(ctx, tx, path)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return diagnostics, nil
}

// ReadDependencyEdges reads dependency edges for one source document in deterministic order.
func (r *Repository) ReadDependencyEdges(ctx context.Context, sourcePath string) ([]DependencyEdge, error) {
	if r == nil || r.db == nil {
		return nil, ErrNilDatabase
	}
	sourcePath = canonicalizePath(sourcePath)
	if sourcePath == "" {
		return nil, ErrEmptyPath
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		_ = tx.Rollback()
	}()
	edges, err := readDependencyEdges(ctx, tx, sourcePath)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return edges, nil
}

func (r *Repository) documentPathExists(ctx context.Context, path string) (bool, error) {
	if r == nil || r.db == nil {
		return false, ErrNilDatabase
	}
	_, err := r.ReadDocumentSnapshot(ctx, path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrSnapshotNotFound) {
		return false, nil
	}
	return false, err
}

func (r *Repository) DocumentPathExists(ctx context.Context, path string) (bool, error) {
	return r.documentPathExists(ctx, path)
}

func currentSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	if err := ctxErr(ctx); err != nil {
		return 0, err
	}
	row := db.QueryRowContext(ctx, currentSchemaVersionSQL)
	var version int
	err := row.Scan(&version)
	if err == nil {
		return version, nil
	}
	if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "no such table") {
		return 0, nil
	}
	return 0, fmt.Errorf("read schema version: %w", err)
}

func (r *DocumentSnapshot) DocumentPath() string {
	return canonicalizePath(r.Path)
}

func normalizeSnapshot(in DocumentSnapshot) DocumentSnapshot {
	out := in
	out.Path = canonicalizePath(in.Path)
	out.FileType = strings.TrimSpace(out.FileType)
	out.Version = strings.TrimSpace(out.Version)
	out.ContentHash = strings.TrimSpace(out.ContentHash)
	out.UpdatedAtUnix = currentUnix()
	out.Symbols = append([]SymbolSnapshot(nil), out.Symbols...)
	out.References = append([]ReferenceSnapshot(nil), out.References...)
	out.Diagnostics = append([]DiagnosticSnapshot(nil), out.Diagnostics...)
	out.DependencyIn = append([]DependencyEdge(nil), out.DependencyIn...)

	sort.Slice(out.Symbols, func(i, j int) bool {
		left := out.Symbols[i]
		right := out.Symbols[j]
		if left.DocumentPath != right.DocumentPath {
			return left.DocumentPath < right.DocumentPath
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.StartLine != right.StartLine {
			return left.StartLine < right.StartLine
		}
		if left.StartColumn != right.StartColumn {
			return left.StartColumn < right.StartColumn
		}
		return left.ID < right.ID
	})

	sort.Slice(out.References, func(i, j int) bool {
		left := out.References[i]
		right := out.References[j]
		if left.DocumentPath != right.DocumentPath {
			return left.DocumentPath < right.DocumentPath
		}
		if left.ID != right.ID {
			return left.ID < right.ID
		}
		if left.SourceSymbol != right.SourceSymbol {
			return left.SourceSymbol < right.SourceSymbol
		}
		if left.StartLine != right.StartLine {
			return left.StartLine < right.StartLine
		}
		if left.StartColumn != right.StartColumn {
			return left.StartColumn < right.StartColumn
		}
		return left.Name < right.Name
	})

	sort.Slice(out.Diagnostics, func(i, j int) bool {
		left := out.Diagnostics[i]
		right := out.Diagnostics[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.StartLine != right.StartLine {
			return left.StartLine < right.StartLine
		}
		if left.StartColumn != right.StartColumn {
			return left.StartColumn < right.StartColumn
		}
		if left.EndLine != right.EndLine {
			return left.EndLine < right.EndLine
		}
		if left.EndColumn != right.EndColumn {
			return left.EndColumn < right.EndColumn
		}
		if left.Severity != right.Severity {
			return left.Severity < right.Severity
		}
		return left.Message < right.Message
	})

	sort.Slice(out.DependencyIn, func(i, j int) bool {
		left := out.DependencyIn[i]
		right := out.DependencyIn[j]
		if left.SourcePath != right.SourcePath {
			return left.SourcePath < right.SourcePath
		}
		if left.TargetPath != right.TargetPath {
			return left.TargetPath < right.TargetPath
		}
		return left.Type < right.Type
	})

	for i := range out.Symbols {
		out.Symbols[i].DocumentPath = canonicalizePath(out.Symbols[i].DocumentPath)
		out.Symbols[i].StartLine = normalizeLine(out.Symbols[i].StartLine)
		out.Symbols[i].StartColumn = normalizeColumn(out.Symbols[i].StartColumn)
		out.Symbols[i].EndLine = normalizeLine(out.Symbols[i].EndLine)
		out.Symbols[i].EndColumn = normalizeColumn(out.Symbols[i].EndColumn)
	}
	for i := range out.References {
		out.References[i].DocumentPath = canonicalizePath(out.References[i].DocumentPath)
		out.References[i].StartLine = normalizeLine(out.References[i].StartLine)
		out.References[i].StartColumn = normalizeColumn(out.References[i].StartColumn)
		out.References[i].EndLine = normalizeLine(out.References[i].EndLine)
		out.References[i].EndColumn = normalizeColumn(out.References[i].EndColumn)
	}
	for i := range out.Diagnostics {
		out.Diagnostics[i].Path = canonicalizePath(out.Diagnostics[i].Path)
		out.Diagnostics[i].StartLine = normalizeLine(out.Diagnostics[i].StartLine)
		out.Diagnostics[i].StartColumn = normalizeColumn(out.Diagnostics[i].StartColumn)
		out.Diagnostics[i].EndLine = normalizeLine(out.Diagnostics[i].EndLine)
		out.Diagnostics[i].EndColumn = normalizeColumn(out.Diagnostics[i].EndColumn)
	}
	for i := range out.DependencyIn {
		out.DependencyIn[i].SourcePath = canonicalizePath(out.DependencyIn[i].SourcePath)
		out.DependencyIn[i].TargetPath = canonicalizePath(out.DependencyIn[i].TargetPath)
	}
	return out
}

func currentUnix() int64 {
	return time.Now().Unix()
}

func upsertDocument(ctx context.Context, tx *sql.Tx, snapshot DocumentSnapshot) error {
	doc := normalizeSnapshot(snapshot)
	if ctxErr(ctx) != nil {
		return ctxErr(ctx)
	}
	if _, err := tx.ExecContext(ctx, deleteDocumentSQL, doc.Path); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, upsertDocumentSQL, doc.Path, doc.FileType, doc.Version, doc.ContentHash, doc.UpdatedAtUnix, doc.Generation); err != nil {
		return fmt.Errorf("upsert document %q: %w", doc.Path, err)
	}
	return nil
}

func upsertSymbols(ctx context.Context, tx *sql.Tx, documentPath string, symbols []SymbolSnapshot) error {
	if _, err := tx.ExecContext(ctx, deleteSymbolsByDocumentSQL, documentPath); err != nil {
		return err
	}
	for _, symbol := range symbols {
		if ctxErr(ctx) != nil {
			return ErrContextCanceled
		}
		s := symbol
		s.DocumentPath = canonicalizePath(s.DocumentPath)
		s.StartLine = normalizeLine(s.StartLine)
		s.StartColumn = normalizeColumn(s.StartColumn)
		s.EndLine = normalizeLine(s.EndLine)
		s.EndColumn = normalizeColumn(s.EndColumn)
		if s.DocumentPath == "" {
			s.DocumentPath = canonicalizePath(documentPath)
		}
		if _, err := tx.ExecContext(ctx, upsertSymbolSQL,
			s.ID,
			s.DocumentPath,
			s.Kind,
			s.Name,
			s.Section,
			s.Raw,
			s.StartLine,
			s.StartColumn,
			s.EndLine,
			s.EndColumn,
		); err != nil {
			return err
		}
	}
	return nil
}

func upsertReferences(ctx context.Context, tx *sql.Tx, documentPath string, references []ReferenceSnapshot) error {
	if _, err := tx.ExecContext(ctx, deleteReferencesByDocumentSQL, documentPath); err != nil {
		return err
	}
	for _, reference := range references {
		if ctxErr(ctx) != nil {
			return ErrContextCanceled
		}
		r := reference
		r.DocumentPath = canonicalizePath(r.DocumentPath)
		if r.DocumentPath == "" {
			r.DocumentPath = canonicalizePath(documentPath)
		}
		r.StartLine = normalizeLine(r.StartLine)
		r.StartColumn = normalizeColumn(r.StartColumn)
		r.EndLine = normalizeLine(r.EndLine)
		r.EndColumn = normalizeColumn(r.EndColumn)
		if _, err := tx.ExecContext(ctx, upsertReferenceSQL,
			r.ID,
			r.DocumentPath,
			r.Kind,
			r.Name,
			r.Raw,
			r.SourceSymbol,
			r.Target,
			nilToEmpty(r.TargetSymbolID),
			nilToEmpty(r.TargetPath),
			r.Classification,
			boolToInt(r.Resolved),
			boolToInt(r.IsDynamic),
			r.StartLine,
			r.StartColumn,
			r.EndLine,
			r.EndColumn,
		); err != nil {
			return err
		}
	}
	return nil
}

func upsertDiagnostics(ctx context.Context, tx *sql.Tx, documentPath string, diagnostics []DiagnosticSnapshot) error {
	if _, err := tx.ExecContext(ctx, deleteDiagnosticsByDocumentSQL, documentPath); err != nil {
		return err
	}
	for _, diagnostic := range diagnostics {
		if ctxErr(ctx) != nil {
			return ErrContextCanceled
		}
		d := diagnostic
		d.Path = canonicalizePath(d.Path)
		if d.Path == "" {
			d.Path = canonicalizePath(documentPath)
		}
		d.StartLine = normalizeLine(d.StartLine)
		d.StartColumn = normalizeColumn(d.StartColumn)
		d.EndLine = normalizeLine(d.EndLine)
		d.EndColumn = normalizeColumn(d.EndColumn)
		if _, err := tx.ExecContext(ctx, upsertDiagnosticSQL,
			d.Path,
			d.Code,
			d.Severity,
			d.Message,
			nilToEmpty(d.RelatedSymbol),
			d.StartLine,
			d.StartColumn,
			d.EndLine,
			d.EndColumn,
		); err != nil {
			return err
		}
	}
	return nil
}

func upsertDependencyEdges(ctx context.Context, tx *sql.Tx, documentPath string, edges []DependencyEdge) error {
	sourcePath := canonicalizePath(documentPath)
	if _, err := tx.ExecContext(ctx, deleteDependencyEdgesBySourceSQL, sourcePath); err != nil {
		return err
	}

	seen := map[string]struct{}{}
	for _, edge := range edges {
		if ctxErr(ctx) != nil {
			return ErrContextCanceled
		}
		e := edge
		e.SourcePath = canonicalizePath(e.SourcePath)
		e.TargetPath = canonicalizePath(e.TargetPath)
		if e.SourcePath == "" {
			e.SourcePath = sourcePath
		}
		key := e.SourcePath + "\x00" + e.TargetPath + "\x00" + e.Type
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if _, err := tx.ExecContext(ctx, upsertDependencySQL, e.SourcePath, e.TargetPath, e.Type); err != nil {
			return err
		}
	}
	return nil
}

type documentRow struct {
	path        string
	fileType    string
	version     string
	contentHash string
	updatedAt   int64
	generation  int64
}

func (r documentRow) toSnapshot() DocumentSnapshot {
	return DocumentSnapshot{
		Path:          r.path,
		FileType:      r.fileType,
		Version:       r.version,
		ContentHash:   r.contentHash,
		UpdatedAtUnix: r.updatedAt,
		Generation:    r.generation,
	}
}

func readDocumentRow(ctx context.Context, tx *sql.Tx, path string) (documentRow, error) {
	if ctxErr(ctx) != nil {
		return documentRow{}, ErrContextCanceled
	}
	row := tx.QueryRowContext(ctx, selectDocumentSQL, path)
	var out documentRow
	err := row.Scan(&out.path, &out.fileType, &out.version, &out.contentHash, &out.updatedAt, &out.generation)
	if errors.Is(err, sql.ErrNoRows) {
		return documentRow{}, ErrSnapshotNotFound
	}
	if err != nil {
		return documentRow{}, err
	}
	return out, nil
}

func readSymbols(ctx context.Context, tx *sql.Tx, path string) ([]SymbolSnapshot, error) {
	if ctxErr(ctx) != nil {
		return nil, ErrContextCanceled
	}
	rows, err := tx.QueryContext(ctx, selectSymbolsByDocumentSQL, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	symbols := make([]SymbolSnapshot, 0)
	for rows.Next() {
		var symbol SymbolSnapshot
		if err := rows.Scan(
			&symbol.ID,
			&symbol.DocumentPath,
			&symbol.Kind,
			&symbol.Name,
			&symbol.Section,
			&symbol.Raw,
			&symbol.StartLine,
			&symbol.StartColumn,
			&symbol.EndLine,
			&symbol.EndColumn,
		); err != nil {
			return nil, err
		}
		symbols = append(symbols, symbol)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return symbols, nil
}

func readReferences(ctx context.Context, tx *sql.Tx, path string) ([]ReferenceSnapshot, error) {
	if ctxErr(ctx) != nil {
		return nil, ErrContextCanceled
	}
	rows, err := tx.QueryContext(ctx, selectReferencesByDocumentSQL, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	references := make([]ReferenceSnapshot, 0)
	for rows.Next() {
		var reference ReferenceSnapshot
		var resolved, dynamic int
		if err := rows.Scan(
			&reference.ID,
			&reference.DocumentPath,
			&reference.Kind,
			&reference.Name,
			&reference.Raw,
			&reference.SourceSymbol,
			&reference.Target,
			&reference.TargetSymbolID,
			&reference.TargetPath,
			&reference.Classification,
			&resolved,
			&dynamic,
			&reference.StartLine,
			&reference.StartColumn,
			&reference.EndLine,
			&reference.EndColumn,
		); err != nil {
			return nil, err
		}
		reference.Resolved = resolved == 1
		reference.IsDynamic = dynamic == 1
		references = append(references, reference)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return references, nil
}

func readDiagnostics(ctx context.Context, tx *sql.Tx, path string) ([]DiagnosticSnapshot, error) {
	if ctxErr(ctx) != nil {
		return nil, ErrContextCanceled
	}
	rows, err := tx.QueryContext(ctx, selectDiagnosticsByDocumentSQL, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	diagnostics := make([]DiagnosticSnapshot, 0)
	for rows.Next() {
		var diag DiagnosticSnapshot
		if err := rows.Scan(
			&diag.Path,
			&diag.Code,
			&diag.Severity,
			&diag.Message,
			&diag.RelatedSymbol,
			&diag.StartLine,
			&diag.StartColumn,
			&diag.EndLine,
			&diag.EndColumn,
		); err != nil {
			return nil, err
		}
		diagnostics = append(diagnostics, diag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return diagnostics, nil
}

func readDependencyEdges(ctx context.Context, tx *sql.Tx, sourcePath string) ([]DependencyEdge, error) {
	if ctxErr(ctx) != nil {
		return nil, ErrContextCanceled
	}
	rows, err := tx.QueryContext(ctx, selectDependencyEdgesBySourceSQL, sourcePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	edges := make([]DependencyEdge, 0)
	for rows.Next() {
		var edge DependencyEdge
		if err := rows.Scan(&edge.SourcePath, &edge.TargetPath, &edge.Type); err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return edges, nil
}

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	err := ctx.Err()
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return ErrContextCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrContextCanceled
	}
	return err
}

func canonicalizePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || trimmed == "." {
		return ""
	}
	clean := filepath.Clean(trimmed)
	return filepath.ToSlash(clean)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nilToEmpty(in string) interface{} {
	if in == "" {
		return nil
	}
	return in
}

func normalizeLine(v int) int {
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

const (
	currentSchemaVersionSQL          = "SELECT version FROM " + schemaVersionTable + " ORDER BY version DESC LIMIT 1;"
	upsertDocumentSQL                = "INSERT INTO " + documentTable + " (path, file_type, version, content_hash, updated_at, generation) VALUES (?, ?, ?, ?, ?, ?);"
	deleteDocumentSQL                = "DELETE FROM " + documentTable + " WHERE path = ?;"
	deleteSymbolsByDocumentSQL       = "DELETE FROM " + symbolTable + " WHERE document_path = ?;"
	deleteReferencesByDocumentSQL    = "DELETE FROM " + referenceTable + " WHERE document_path = ?;"
	deleteDiagnosticsByDocumentSQL   = "DELETE FROM " + diagnosticTable + " WHERE path = ?;"
	deleteDependencyEdgesBySourceSQL = "DELETE FROM " + dependencyEdgeTable + " WHERE source_path = ?;"
	upsertSymbolSQL                  = "INSERT INTO " + symbolTable + " (id, document_path, kind, name, section, raw, start_line, start_column, end_line, end_column) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);"
	upsertReferenceSQL               = "INSERT INTO " + referenceTable + " (id, document_path, kind, name, raw, source_symbol, target, target_symbol_id, target_path, classification, resolved, is_dynamic, start_line, start_column, end_line, end_column) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);"
	upsertDiagnosticSQL              = "INSERT INTO " + diagnosticTable + " (path, code, severity, message, related_symbol, start_line, start_column, end_line, end_column) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);"
	upsertDependencySQL              = "INSERT INTO " + dependencyEdgeTable + " (source_path, target_path, edge_type) VALUES (?, ?, ?);"

	selectAllDocumentsSQL            = "SELECT path, file_type, version, content_hash, updated_at, generation FROM " + documentTable + " ORDER BY path;"
	selectDocumentSQL                = "SELECT path, file_type, version, content_hash, updated_at, generation FROM " + documentTable + " WHERE path = ?;"
	selectSymbolsByDocumentSQL       = "SELECT id, document_path, kind, name, section, raw, start_line, start_column, end_line, end_column FROM " + symbolTable + " WHERE document_path = ? ORDER BY document_path, kind, name, start_line, start_column, end_line, end_column, id;"
	selectReferencesByDocumentSQL    = "SELECT id, document_path, kind, name, raw, source_symbol, target, target_symbol_id, target_path, classification, resolved, is_dynamic, start_line, start_column, end_line, end_column FROM " + referenceTable + " WHERE document_path = ? ORDER BY document_path, id, source_symbol, start_line, start_column, end_line, end_column, name;"
	selectDiagnosticsByDocumentSQL   = "SELECT path, code, severity, message, related_symbol, start_line, start_column, end_line, end_column FROM " + diagnosticTable + " WHERE path = ? ORDER BY path, code, severity, message, start_line, start_column, end_line, end_column;"
	selectDependencyEdgesBySourceSQL = "SELECT source_path, target_path, edge_type FROM " + dependencyEdgeTable + " WHERE source_path = ? ORDER BY source_path, target_path, edge_type;"
)
