package parser

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/parser/adapter"
	"github.com/ikemen-engine/ikemen-devtools/internal/syntax"
)

func TestParseUsesSyntaxAdapterForSupportedExtensions(t *testing.T) {
	sourceByExt := []struct {
		name   string
		path   string
		source string
	}{
		{
			name:   "def",
			path:   "snippets/../snippet.def",
			source: "[Command]\nname = \"jump\"\n",
		},
		{
			name:   "cns",
			path:   "snippet.cns",
			source: "[State 10]\ntype = ChangeState\nvalue = 100\n",
		},
		{
			name:   "cmd",
			path:   "snippet.cmd",
			source: "[Command]\nname = \"slash\"\n",
		},
		{
			name:   "st",
			path:   "snippet.st",
			source: "[State 20]\ntype = SelfState\nvalue = 500\n",
		},
	}

	provider := syntax.NewStandardProvider()
	for _, tc := range sourceByExt {
		t.Run(tc.name, func(t *testing.T) {
			parsed := provider.Parse(tc.path, []byte(tc.source))
			if parsed == nil {
				t.Fatalf("provider parse returned nil for %q", tc.path)
			}
			if parsed.Document == nil {
				t.Fatalf("provider parse returned nil document for %q", tc.path)
			}
			if !bytes.Equal(parsed.RoundTrip(), []byte(tc.source)) {
				t.Fatalf("provider round trip changed bytes for %q", tc.path)
			}

			got := Parse(tc.path, tc.source)
			if got == nil {
				t.Fatalf("parser parse returned nil for %q", tc.path)
			}
			if got.Path != parsed.Path {
				t.Fatalf("parser path %q did not use syntax provider normalized path %q for %q", got.Path, parsed.Path, tc.path)
			}

			fromAdapter := adapter.FromSyntax(parsed.Path, parsed.Sections, parsed.Tokens)
			if !reflect.DeepEqual(got, fromAdapter) {
				t.Fatalf("parser parse output diverged from adapter output for %q", tc.path)
			}
			if !reflect.DeepEqual(got, parsed.Document) {
				t.Fatalf("parser parse output diverged from parser adapter document for %q", tc.path)
			}
		})
	}
}
