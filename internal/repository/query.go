package repository

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
)

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
	Visited    int    `json:"visited"`
}
type QueryOptions struct {
	Limit      int
	Cursor     string
	MaxVisited int
}

func queryOffset(cursor string) int {
	if cursor == "" {
		return 0
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0
	}
	var n int
	_, _ = fmt.Sscan(string(b), &n)
	if n < 0 {
		return 0
	}
	return n
}

func (r *Repository) QueryDiagnostics(ctx context.Context, opts QueryOptions) (Page[DiagnosticSnapshot], error) {
	if r == nil || r.db == nil {
		return Page[DiagnosticSnapshot]{}, ErrNilDatabase
	}
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	if opts.MaxVisited <= 0 {
		opts.MaxVisited = opts.Limit * 10
	}
	if opts.Limit > opts.MaxVisited {
		opts.Limit = opts.MaxVisited
	}
	if err := ctxErr(ctx); err != nil {
		return Page[DiagnosticSnapshot]{}, err
	}
	offset := queryOffset(opts.Cursor)
	rows, err := r.db.QueryContext(ctx, "SELECT path,code,severity,message,COALESCE(related_symbol,''),start_line,start_column,end_line,end_column FROM "+diagnosticTable+" ORDER BY path,code,severity,message,start_line,start_column LIMIT ? OFFSET ?", opts.MaxVisited, offset)
	if err != nil {
		return Page[DiagnosticSnapshot]{}, err
	}
	defer rows.Close()
	p := Page[DiagnosticSnapshot]{Items: make([]DiagnosticSnapshot, 0, opts.Limit)}
	for rows.Next() {
		p.Visited++
		var d DiagnosticSnapshot
		if err := rows.Scan(&d.Path, &d.Code, &d.Severity, &d.Message, &d.RelatedSymbol, &d.StartLine, &d.StartColumn, &d.EndLine, &d.EndColumn); err != nil {
			return p, err
		}
		if len(p.Items) < opts.Limit {
			p.Items = append(p.Items, d)
		}
	}
	if err := rows.Err(); err != nil {
		return p, err
	}
	if p.Visited > len(p.Items) {
		p.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d", len(p.Items))))
	}
	return p, nil
}

// ChangedDocuments returns canonical paths whose hashes changed, including additions and deletions.
func ChangedDocuments(previous, current map[string]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for p, h := range current {
		seen[p] = true
		if previous[p] != h {
			out = append(out, p)
		}
	}
	for p := range previous {
		if !seen[p] {
			out = append(out, p)
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && strings.Compare(out[j], out[j-1]) < 0; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
func (r *Repository) QuerySymbols(ctx context.Context, opts QueryOptions) (Page[SymbolSnapshot], error) {
	if r == nil || r.db == nil {
		return Page[SymbolSnapshot]{}, ErrNilDatabase
	}
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	if opts.MaxVisited <= 0 {
		opts.MaxVisited = opts.Limit * 10
	}
	offset := queryOffset(opts.Cursor)
	rows, err := r.db.QueryContext(ctx, "SELECT id,document_path,kind,name,section,raw,start_line,start_column,end_line,end_column FROM "+symbolTable+" ORDER BY document_path,id,start_line,start_column LIMIT ? OFFSET ?", opts.MaxVisited, offset)
	if err != nil {
		return Page[SymbolSnapshot]{}, err
	}
	defer rows.Close()
	p := Page[SymbolSnapshot]{Items: []SymbolSnapshot{}}
	for rows.Next() {
		p.Visited++
		var s SymbolSnapshot
		if err := rows.Scan(&s.ID, &s.DocumentPath, &s.Kind, &s.Name, &s.Section, &s.Raw, &s.StartLine, &s.StartColumn, &s.EndLine, &s.EndColumn); err != nil {
			return p, err
		}
		if len(p.Items) < opts.Limit {
			p.Items = append(p.Items, s)
		}
	}
	if p.Visited == opts.MaxVisited && len(p.Items) == opts.Limit {
		p.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprint(offset + p.Visited)))
	}
	return p, rows.Err()
}

func (r *Repository) QueryReferences(ctx context.Context, opts QueryOptions) (Page[ReferenceSnapshot], error) {
	if r == nil || r.db == nil {
		return Page[ReferenceSnapshot]{}, ErrNilDatabase
	}
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	rows, err := r.db.QueryContext(ctx, "SELECT id,document_path,kind,name,raw,source_symbol,target,COALESCE(target_symbol_id,''),COALESCE(target_path,''),classification,resolved,is_dynamic,start_line,start_column,end_line,end_column FROM "+referenceTable+" ORDER BY document_path,id,start_line,start_column LIMIT ?", opts.Limit)
	if err != nil {
		return Page[ReferenceSnapshot]{}, err
	}
	defer rows.Close()
	p := Page[ReferenceSnapshot]{Items: []ReferenceSnapshot{}}
	for rows.Next() {
		var x ReferenceSnapshot
		var resolved, dynamic int
		if err := rows.Scan(&x.ID, &x.DocumentPath, &x.Kind, &x.Name, &x.Raw, &x.SourceSymbol, &x.Target, &x.TargetSymbolID, &x.TargetPath, &x.Classification, &resolved, &dynamic, &x.StartLine, &x.StartColumn, &x.EndLine, &x.EndColumn); err != nil {
			return p, err
		}
		x.Resolved = resolved != 0
		x.IsDynamic = dynamic != 0
		p.Items = append(p.Items, x)
	}
	return p, rows.Err()
}

type TextResult struct {
	Path  string `json:"path"`
	Match string `json:"match"`
}

func (r *Repository) SearchText(ctx context.Context, term string, opts QueryOptions) (Page[TextResult], error) {
	if r == nil || r.db == nil {
		return Page[TextResult]{}, ErrNilDatabase
	}
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	rows, err := r.db.QueryContext(ctx, "SELECT path,message FROM "+diagnosticTable+" WHERE message LIKE ? ORDER BY path,message LIMIT ?", "%"+term+"%", opts.Limit)
	if err != nil {
		return Page[TextResult]{}, err
	}
	defer rows.Close()
	p := Page[TextResult]{Items: []TextResult{}}
	for rows.Next() {
		var x TextResult
		if err := rows.Scan(&x.Path, &x.Match); err != nil {
			return p, err
		}
		p.Items = append(p.Items, x)
	}
	return p, rows.Err()
}
func (r *Repository) QueryEdges(ctx context.Context, opts QueryOptions) (Page[DependencyEdge], error) {
	if r == nil || r.db == nil {
		return Page[DependencyEdge]{}, ErrNilDatabase
	}
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	rows, err := r.db.QueryContext(ctx, "SELECT source_path,target_path,edge_type FROM "+dependencyEdgeTable+" ORDER BY source_path,target_path,edge_type LIMIT ?", opts.Limit)
	if err != nil {
		return Page[DependencyEdge]{}, err
	}
	defer rows.Close()
	p := Page[DependencyEdge]{Items: []DependencyEdge{}}
	for rows.Next() {
		var x DependencyEdge
		if err := rows.Scan(&x.SourcePath, &x.TargetPath, &x.Type); err != nil {
			return p, err
		}
		p.Items = append(p.Items, x)
	}
	return p, rows.Err()
}
