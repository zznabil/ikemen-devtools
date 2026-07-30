package scip

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
)

func testDocuments() []ir.Document {
	return []ir.Document{
		{
			Version: ir.Version, Path: `./z\unicode.zss`, FileType: "zss",
			Symbols:     []ir.Symbol{{ID: "state:2", Identity: ir.Identity{ContractVersion: ir.IdentityContractVersion, SemanticKey: "state:2", StoreID: "state:2"}, Kind: ir.SymbolStateDef, Name: "éclair", Span: ir.SourceSpan{Start: ir.SourcePosition{Line: 2, Column: 3}, End: ir.SourcePosition{Line: 2, Column: 9}}}},
			References:  []ir.Reference{{ID: "ref:1", Identity: ir.Identity{ContractVersion: ir.IdentityContractVersion, SemanticKey: "state:2", OccurrenceID: "ref:1"}, Kind: ir.ReferenceState, Name: "state:2", SourceSymbol: "state:1", Target: "state:2", Span: ir.SourceSpan{Start: ir.SourcePosition{Line: 1, Column: 5}, End: ir.SourcePosition{Line: 1, Column: 12}}}},
			Diagnostics: []ir.Diagnostic{{Path: `./z\unicode.zss`, Code: "bad-value", Severity: ir.SeverityWarning, Message: "bad é", Start: ir.SourcePosition{Line: 1, Column: 1}, End: ir.SourcePosition{Line: 1, Column: 3}, RelatedSymbol: "state:2"}},
		},
		{Version: ir.Version, Path: "a.zss", FileType: "zss"},
	}
}

func TestExportDeterministicAndCanonical(t *testing.T) {
	first, err := Marshal(testDocuments())
	if err != nil {
		t.Fatal(err)
	}
	second, err := Marshal(testDocuments())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("repeated exports differ")
	}
	text := string(first)
	if !strings.Contains(text, `"path":"a.zss"`) || !strings.Contains(text, `"path":"z/unicode.zss"`) {
		t.Fatalf("canonical paths missing: %s", text)
	}
	if strings.Index(text, `"path":"a.zss"`) > strings.Index(text, `"path":"z/unicode.zss"`) {
		t.Fatal("documents are not path ordered")
	}
}

func TestExportOccurrenceRolesAndUTF8Positions(t *testing.T) {
	data, err := Marshal(testDocuments())
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `"roles":["definition"]`) || !strings.Contains(text, `"roles":["reference"]`) {
		t.Fatalf("roles missing: %s", text)
	}
	if !strings.Contains(text, `"start":{"line":1,"character":2}`) {
		t.Fatalf("definition position missing: %s", text)
	}
}

func TestExportDiagnostics(t *testing.T) {
	data, err := Marshal(testDocuments())
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `"code":"bad-value"`) || !strings.Contains(text, `"severity":"warning"`) || !strings.Contains(text, `"message":"bad é"`) {
		t.Fatalf("diagnostic missing: %s", text)
	}
	if !strings.Contains(text, `"identityContractVersion":"0.2.0"`) {
		t.Fatalf("identity contract metadata missing: %s", text)
	}
}
