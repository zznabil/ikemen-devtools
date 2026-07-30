package operations

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/contract"
	"github.com/ikemen-engine/ikemen-devtools/internal/graph"
	"github.com/ikemen-engine/ikemen-devtools/internal/semantics"
	"github.com/ikemen-engine/ikemen-devtools/internal/workspace"
)

var absoluteHostPath = regexp.MustCompile(`[A-Za-z]:[\\\\/]|(^|["\s])[/\\\\]{2}`)

func TestPaginationStableAndCursor(t *testing.T) {
	type row struct {
		Name string `json:"name"`
	}
	a := []row{{"z"}, {"a"}, {"m"}}
	got, p, tr := paginate(a, Options{Limit: 2})
	if len(got) != 2 || got[0].Name != "a" || p.NextCursor == "" || !tr.Truncated {
		t.Fatalf("unexpected first page: %#v %#v %#v", got, p, tr)
	}
	got2, _, _ := paginate(a, Options{Limit: 2, Cursor: p.NextCursor})
	if len(got2) != 1 || got2[0].Name != "z" {
		t.Fatalf("unexpected second page: %#v", got2)
	}
}
func TestDisplayPathNeverLeaksAbsoluteWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "chars", "hero.def")
	if got := displayPath(root, inside); got != "chars/hero.def" {
		t.Fatalf("inside path = %q", got)
	}
	if got := displayPath(root, filepath.Join(root, "..", "secret.def")); filepath.IsAbs(filepath.FromSlash(got)) || strings.Contains(got, filepath.Base(root)) {
		t.Fatalf("outside path leaked host information: %q", got)
	}
}
func TestEnvelopeCanonicalFields(t *testing.T) {
	e := envelope("query.symbols", workspace.LoadResult{}, semantics.ResolveResult{}, "snap", []any{}, contract.Page{}, contract.Truncation{})
	b, err := e.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		t.Fatal("invalid json")
	}
	for _, k := range []string{"schemaVersion", "operation", "tool", "status", "workspace", "snapshot", "result", "diagnostics", "page", "truncated"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("missing %s", k)
		}
	}
}
func TestExportFormatsDeterministic(t *testing.T) {
	root := t.TempDir()
	def := filepath.Join(root, "hero.def")
	os.WriteFile(filepath.Join(root, "hero.st"), []byte("[Statedef 100]\n"), 0600)
	os.WriteFile(def, []byte("[Files]\nst = hero.st\n"), 0600)
	for _, kind := range []string{"jsonl", "scip", "sql"} {
		a, e := Export(context.Background(), Options{Root: def}, kind)
		if e != nil {
			t.Fatal(kind, e)
		}
		b, e := Export(context.Background(), Options{Root: def}, kind)
		if e != nil {
			t.Fatal(kind, e)
		}
		x := a.Envelope.Result.(map[string]any)["content"].(string)
		y := b.Envelope.Result.(map[string]any)["content"].(string)
		if strings.TrimSpace(x) == "" {
			t.Fatalf("empty %s", kind)
		}
		if strings.TrimSpace(strings.ReplaceAll(x, `"snapshot":"`+a.Envelope.Snapshot.ID+`"`, `"snapshot":"<snapshot>"`)) != strings.TrimSpace(strings.ReplaceAll(y, `"snapshot":"`+b.Envelope.Snapshot.ID+`"`, `"snapshot":"<snapshot>"`)) {
			t.Fatalf("non-deterministic %s", kind)
		}
		raw, _ := json.Marshal(a.Envelope.Result)
		if absoluteHostPath.Match(raw) {
			t.Fatalf("absolute host path leaked in arbitrary %s result field: %s", kind, raw)
		}
		if strings.Contains(x, root) {
			t.Fatalf("absolute host path leaked in %s export: %q", kind, x)
		}
		if kind == "jsonl" && !strings.Contains(x, `"type":"file"`) {
			t.Fatalf("jsonl omitted analyzable file record: %q", x)
		}
	}
	if got := normalizeExportPath(root, filepath.Join(root, "chars", "hero.def")); got != "chars/hero.def" {
		t.Fatalf("workspace-relative Windows path = %q", got)
	}
}
func TestSanitizeExportContentWindowsBackslashRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "game")
	rootWin := strings.ReplaceAll(root, `/`, `\`)
	content := `path=` + rootWin + `\data\hero.def other=` + rootWin + `boy\keep`
	got := sanitizeExportContent(root, content)
	if strings.Contains(got, rootWin+`\data`) || strings.Contains(got, strings.ReplaceAll(rootWin, `\`, `/`)+`/data`) {
		t.Fatalf("windows absolute path leaked: %q", got)
	}
	if !strings.Contains(got, "boy") {
		t.Fatalf("boundary sanitizer removed non-root prefix: %q", got)
	}
}
func TestGraphDiagnosticsUseWorkspaceRelativePaths(t *testing.T) {
	root := t.TempDir()
	selectPath := filepath.Join(root, "select.def")
	if err := os.WriteFile(selectPath, []byte("x\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Graph(context.Background(), Options{Root: root, Kind: "dependencies", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	result := r.Envelope.Result.(map[string]any)
	for _, d := range result["diagnostics"].([]graph.Diagnostic) {
		if filepath.IsAbs(d.Span.Path) {
			t.Fatalf("absolute graph path leaked: %q", d.Span.Path)
		}
	}
}
func TestSearchMissingRootReturnsStructuredResult(t *testing.T) {
	r, err := Search(context.Background(), Options{Root: "missing-select.def", Query: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Envelope.Operation != "query.search" {
		t.Fatal(r.Envelope.Operation)
	}
}
