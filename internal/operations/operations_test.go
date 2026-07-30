package operations

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/contract"
	"github.com/ikemen-engine/ikemen-devtools/internal/semantics"
	"github.com/ikemen-engine/ikemen-devtools/internal/workspace"
)

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
		if x != y || strings.TrimSpace(x) == "" {
			t.Fatalf("non-deterministic empty %s", kind)
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
