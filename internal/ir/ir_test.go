package ir

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDocumentRoundTripsAsJSON(t *testing.T) {
	doc := Document{
		Version:  Version,
		Path:     "sample.def",
		FileType: "def",
		Sections: []Section{
			{
				Header: "Info",
				Kind:   SectionOther,
				Span: SourceSpan{
					Start: SourcePosition{Line: 1, Column: 1},
					End:   SourcePosition{Line: 1, Column: 7},
				},
				Lines: []SourceLine{
					{
						Kind: SourceLineComment,
						Text: "; this is a comment",
						Span: SourceSpan{
							Start: SourcePosition{Line: 2, Column: 1},
							End:   SourcePosition{Line: 2, Column: 20},
						},
					},
				},
			},
		},
		Symbols: []Symbol{
			{
				ID:      "state:100",
				Kind:    SymbolStateDef,
				Name:    "state:100",
				Section: "Statedef 100",
				Span: SourceSpan{
					Start: SourcePosition{Line: 3, Column: 1},
					End:   SourcePosition{Line: 3, Column: 12},
				},
			},
		},
		References: []Reference{
			{
				ID:           "ref:1",
				Kind:         ReferenceCommand,
				Name:         "command:jump",
				Raw:          `command = "jump"`,
				SourceSymbol: "command:jump",
				Target:       "command:jump",
				IsDynamic:    false,
				Span: SourceSpan{
					Start: SourcePosition{Line: 6, Column: 1},
					End:   SourcePosition{Line: 6, Column: 18},
				},
			},
		},
		Diagnostics: []Diagnostic{
			{
				Code:          "malformed-line",
				Severity:      SeverityError,
				Message:       "expected key=value",
				Path:          "sample.def",
				Start:         SourcePosition{Line: 5, Column: 1},
				End:           SourcePosition{Line: 5, Column: 10},
				RelatedSymbol: "state:100",
			},
		},
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal document: %v", err)
	}

	var got Document
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}

	if got.Version != doc.Version || got.Path != doc.Path || got.FileType != doc.FileType {
		t.Fatalf("document metadata mismatch: %#v != %#v", got, doc)
	}
	if len(got.Sections) != 1 || len(got.Symbols) != 1 || len(got.References) != 1 || len(got.Diagnostics) != 1 {
		t.Fatalf("unexpected JSON round-trip lengths: %#v", got)
	}
	if got.Symbols[0].Kind != doc.Symbols[0].Kind {
		t.Fatalf("symbol kind changed: %q != %q", got.Symbols[0].Kind, doc.Symbols[0].Kind)
	}
	if got.References[0].Kind != doc.References[0].Kind {
		t.Fatalf("reference kind changed: %q != %q", got.References[0].Kind, doc.References[0].Kind)
	}
}

func TestDocumentVersionIsStable(t *testing.T) {
	if got, want := Version, IdentityContractVersion; got != want {
		t.Fatalf("document version changed: got %q want %q", got, want)
	}
}

func TestDocumentUsesExternalIdentityContract(t *testing.T) {
	doc := Document{
		Version:  Version,
		Path:     "sample.def",
		FileType: "def",
	}
	if doc.Version != IdentityContractVersion {
		t.Fatalf("document contract version mismatch: got %q want %q", doc.Version, IdentityContractVersion)
	}
}

func TestSourceSpanCanBeCompared(t *testing.T) {
	left := SourceSpan{Start: SourcePosition{Line: 2, Column: 3}, End: SourcePosition{Line: 2, Column: 9}}
	right := SourceSpan{Start: SourcePosition{Line: 2, Column: 3}, End: SourcePosition{Line: 2, Column: 9}}
	if !reflect.DeepEqual(left, right) {
		t.Fatalf("source span equality failed")
	}
}
