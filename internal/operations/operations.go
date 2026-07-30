package operations

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ikemen-engine/ikemen-devtools/internal/contract"
	"github.com/ikemen-engine/ikemen-devtools/internal/graph"
	"github.com/ikemen-engine/ikemen-devtools/internal/index"
	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
	"github.com/ikemen-engine/ikemen-devtools/internal/scip"
	"github.com/ikemen-engine/ikemen-devtools/internal/semantics"
	"github.com/ikemen-engine/ikemen-devtools/internal/workspace"
)

type Options struct {
	Root, Profile, Path, Name, Kind, Code, Severity, Query, Identity, Snapshot, Cursor string
	Limit                                                                              int
	IncludeDeclarations                                                                bool
}
type Result struct {
	Envelope contract.Envelope
	Export   []string
}

func entry(root string) string {
	root = strings.TrimSpace(root)
	if st, e := os.Stat(root); e == nil && st.IsDir() {
		for _, p := range []string{"select.def", "data/select.def", "system.def"} {
			q := filepath.Join(root, p)
			if _, e := os.Stat(q); e == nil {
				return q
			}
		}
	}
	return root
}
func Analyze(ctx context.Context, o Options) (workspace.LoadResult, semantics.ResolveResult, string, error) {
	if err := ctx.Err(); err != nil {
		return workspace.LoadResult{}, semantics.ResolveResult{}, "", err
	}
	p := profile.NewStrictPortableProfile("")
	if strings.EqualFold(o.Profile, "distribution") {
		p = profile.NewDistributionProfile("")
	}
	ws := workspace.LoadWorkspaceWithProfile(entry(o.Root), p)
	sem := semantics.Resolve(semantics.NewMemoryWorkspace(ws.Documents...))
	h := sha256.New()
	for _, d := range ws.Documents {
		io.WriteString(h, d.Path)
		io.WriteString(h, d.Version)
		for _, s := range d.Symbols {
			io.WriteString(h, s.ID)
		}
	}
	return ws, sem, hex.EncodeToString(h.Sum(nil))[:16], nil
}
func paginate[T any](v []T, o Options) ([]T, contract.Page, contract.Truncation) {
	sort.SliceStable(v, func(i, j int) bool {
		a, _ := json.Marshal(v[i])
		b, _ := json.Marshal(v[j])
		return string(a) < string(b)
	})
	lim := o.Limit
	if lim <= 0 {
		lim = 100
	}
	start := 0
	if o.Cursor != "" {
		fmt.Sscanf(o.Cursor, "%d", &start)
		if start < 0 {
			start = 0
		}
	}
	if start > len(v) {
		start = len(v)
	}
	end := start + lim
	if end > len(v) {
		end = len(v)
	}
	p := contract.Page{Limit: lim, Returned: end - start}
	if end < len(v) {
		p.NextCursor = fmt.Sprintf("%d", end)
	}
	return v[start:end], p, contract.Truncation{Truncated: end < len(v), Reasons: func() []string {
		if end < len(v) {
			return []string{"page-limit"}
		}
		return []string{}
	}()}
}
func envelope(op string, ws workspace.LoadResult, sem semantics.ResolveResult, snap string, result any, page contract.Page, trunc contract.Truncation) contract.Envelope {
	ds := append([]ir.Diagnostic{}, ws.Diagnostics...)
	ds = append(ds, sem.Diagnostics...)
	out := make([]contract.Diagnostic, 0, len(ds))
	for _, d := range ds {
		out = append(out, contract.Diagnostic{Code: d.Code, Severity: string(d.Severity), Message: d.Message, Path: d.Path, Evidence: map[string]any{"nextChecks": []string{}}})
	}
	return contract.Envelope{SchemaVersion: contract.SchemaVersion, Operation: op, Tool: "ikm", Status: contract.StatusComplete, Workspace: contract.Workspace{Profile: "strict-portable", Configuration: ws.ConfigDigest}, Snapshot: contract.Snapshot{ID: snap}, Result: result, Diagnostics: out, Page: page, Truncation: trunc}
}
func Diagnostics(ctx context.Context, o Options) (Result, error) {
	ws, sem, s, e := Analyze(ctx, o)
	if e != nil {
		return Result{}, e
	}
	type row struct {
		Code, Severity, Message, Path string
		Span                          ir.SourceSpan `json:"span"`
		Evidence                      any           `json:"evidence"`
		Candidates                    []string      `json:"candidates"`
		NextChecks                    []string      `json:"nextChecks"`
	}
	var v []row
	for _, d := range append(ws.Diagnostics, sem.Diagnostics...) {
		if o.Path != "" && !strings.Contains(filepath.ToSlash(d.Path), filepath.ToSlash(o.Path)) {
			continue
		}
		if o.Code != "" && d.Code != o.Code {
			continue
		}
		if o.Severity != "" && !strings.EqualFold(string(d.Severity), o.Severity) {
			continue
		}
		v = append(v, row{d.Code, string(d.Severity), d.Message, d.Path, ir.SourceSpan{Start: d.Start, End: d.End}, map[string]any{"source": "parser"}, []string{}, []string{"inspect source span"}})
	}
	items, p, t := paginate(v, o)
	return Result{Envelope: envelope("query.diagnostics", ws, sem, s, items, p, t)}, nil
}
func Symbols(ctx context.Context, o Options) (Result, error) {
	ws, sem, s, e := Analyze(ctx, o)
	if e != nil {
		return Result{}, e
	}
	type row struct {
		SemanticKey string        `json:"semanticKey"`
		Occurrence  string        `json:"occurrence"`
		Kind        ir.SymbolKind `json:"kind"`
		Path        string        `json:"path"`
		Span        ir.SourceSpan `json:"span"`
		Owner       string        `json:"owner,omitempty"`
		Snapshot    string        `json:"snapshot"`
		Name        string        `json:"name"`
	}
	var v []row
	for _, x := range sem.Index {
		for _, z := range x.Symbols {
			if o.Name != "" && !strings.Contains(strings.ToLower(z.Symbol.Name), strings.ToLower(o.Name)) {
				continue
			}
			if o.Kind != "" && !strings.EqualFold(string(z.Symbol.Kind), o.Kind) {
				continue
			}
			v = append(v, row{z.Symbol.Identity.SemanticKey, z.Symbol.ID, z.Symbol.Kind, z.Path, z.Symbol.Span, z.Symbol.Section, s, z.Symbol.Name})
		}
	}
	items, p, t := paginate(v, o)
	return Result{Envelope: envelope("query.symbols", ws, sem, s, items, p, t)}, nil
}
func References(ctx context.Context, o Options) (Result, error) {
	ws, sem, s, e := Analyze(ctx, o)
	if e != nil {
		return Result{}, e
	}
	v := sem.References
	if !o.IncludeDeclarations {
	}
	items, p, t := paginate(v, o)
	return Result{Envelope: envelope("query.references", ws, sem, s, items, p, t)}, nil
}
func Search(ctx context.Context, o Options) (Result, error) {
	ws, sem, s, e := Analyze(ctx, o)
	if e != nil {
		return Result{}, e
	}
	type hit struct {
		Path, Snippet, Classification, Snapshot string
		Score                                   int `json:"score"`
	}
	var v []hit
	for _, d := range ws.Documents {
		if o.Path != "" && !strings.Contains(filepath.ToSlash(d.Path), filepath.ToSlash(o.Path)) {
			continue
		}
		b, _ := os.ReadFile(d.Path)
		for n, l := range strings.Split(string(b), "\n") {
			if strings.Contains(strings.ToLower(l), strings.ToLower(o.Query)) {
				v = append(v, hit{d.Path, strings.TrimSpace(l), "lexical", s, 1})
				_ = n
			}
		}
	}
	items, p, t := paginate(v, o)
	return Result{Envelope: envelope("query.search", ws, sem, s, items, p, t)}, nil
}
func Graph(ctx context.Context, o Options) (Result, error) {
	ws, sem, s, e := Analyze(ctx, o)
	if e != nil {
		return Result{}, e
	}
	root := filepath.Dir(entry(o.Root))
	g, e := graph.Build(root, entry(o.Root))
	if e != nil {
		return Result{}, e
	}
	edges := append([]graph.Edge(nil), g.Edges...)
	if o.Kind == "dependencies" && o.Path != "" {
		edges = filterEdges(edges, o.Path, false)
	} else if o.Kind == "dependents" && o.Path != "" {
		edges = filterEdges(edges, o.Path, true)
	} else if o.Kind == "path" && o.Path != "" {
		edges = pathEdges(edges, o.Path)
	}
	items, p, t := paginate(edges, o)
	return Result{Envelope: envelope("graph."+o.Kind, ws, sem, s, map[string]any{"edges": items, "nodes": g.Nodes, "diagnostics": g.Diagnostics, "classification": "static"}, p, t)}, nil
}
func filterEdges(es []graph.Edge, p string, reverse bool) []graph.Edge {
	out := []graph.Edge{}
	for _, e := range es {
		if (!reverse && strings.Contains(e.From, p)) || (reverse && strings.Contains(e.To, p)) {
			out = append(out, e)
		}
	}
	return out
}
func pathEdges(es []graph.Edge, p string) []graph.Edge {
	seen := map[string]bool{p: true}
	for changed := true; changed; {
		changed = false
		for _, e := range es {
			if seen[e.From] && !seen[e.To] {
				seen[e.To] = true
				changed = true
			}
		}
	}
	out := []graph.Edge{}
	for _, e := range es {
		if seen[e.From] && seen[e.To] {
			out = append(out, e)
		}
	}
	return out
}
func Inspect(ctx context.Context, o Options) (Result, error) {
	ws, sem, s, e := Analyze(ctx, o)
	if e != nil {
		return Result{}, e
	}
	type summary struct {
		Path, Kind                       string
		Symbols, References, Diagnostics int
	}
	var v []summary
	for _, d := range ws.Documents {
		if o.Path != "" && filepath.Clean(d.Path) != filepath.Clean(o.Path) && !strings.HasSuffix(filepath.ToSlash(d.Path), filepath.ToSlash(o.Path)) {
			continue
		}
		v = append(v, summary{d.Path, d.FileType, len(d.Symbols), len(d.References), len(d.Diagnostics)})
	}
	items, p, t := paginate(v, o)
	return Result{Envelope: envelope("inspect."+o.Kind, ws, sem, s, map[string]any{"documents": items, "entryPoints": []string{entry(o.Root)}, "health": "ok", "budgets": map[string]any{"limit": o.Limit}}, p, t)}, nil
}
func Export(ctx context.Context, o Options, kind string) (Result, error) {
	ws, sem, s, e := Analyze(ctx, o)
	if e != nil {
		return Result{}, e
	}
	var b strings.Builder
	switch kind {
	case "sql":
		b.WriteString(index.Export(ws, sem))
	case "scip":
		e = scip.Export(&b, ws.Documents)
	default:
		for _, d := range ws.Documents {
			for _, x := range d.Symbols {
				r := map[string]any{"type": "symbol", "snapshot": s, "path": d.Path, "symbol": x}
				z, _ := json.Marshal(r)
				b.Write(z)
				b.WriteByte('\n')
			}
		}
	}
	if e != nil {
		return Result{}, e
	}
	return Result{Envelope: envelope("export."+kind, ws, sem, s, map[string]any{"format": kind, "records": strings.Count(b.String(), "\n"), "content": b.String()}, contract.Page{Returned: strings.Count(b.String(), "\n")}, contract.Truncation{})}, nil
}
func WriteJSON(w io.Writer, r Result) error {
	b, e := r.Envelope.CanonicalJSON()
	if e != nil {
		return e
	}
	_, e = fmt.Fprintln(w, string(b))
	return e
}

var _ = bufio.NewScanner
var _ = time.Second
