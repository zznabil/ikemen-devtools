package syntax

import (
	"bytes"
	"reflect"
	"testing"
)

func TestStandardProviderPreservesLFLineEndings(t *testing.T) {
	source := []byte("[Info]\nname = Hero\n[State 10]\n")
	provider := NewStandardProvider()
	doc := provider.Parse("lf.def", source)

	if !bytes.Equal(doc.RoundTrip(), source) {
		t.Fatalf("expected round trip output to match LF source")
	}
	if !reflect.DeepEqual(doc.Source.LineEndings(), []string{"\n", "\n", "\n", ""}) {
		t.Fatalf("unexpected line endings: %#v", doc.Source.LineEndings())
	}
}

func TestStandardProviderPreservesCRLFLineEndings(t *testing.T) {
	source := []byte("[Info]\r\nname = Hero\r\n[State 10]\r\n")
	provider := NewStandardProvider()
	doc := provider.Parse("crlf.def", source)

	if !bytes.Equal(doc.RoundTrip(), source) {
		t.Fatalf("expected round trip output to match CRLF source")
	}
	if !reflect.DeepEqual(doc.Source.LineEndings(), []string{"\r\n", "\r\n", "\r\n", ""}) {
		t.Fatalf("unexpected line endings: %#v", doc.Source.LineEndings())
	}
}

func TestStandardProviderCapturesCommentsBlankLines(t *testing.T) {
	source := []byte("; top-level comment\n\n[Command]\nname = \"jump\" ; inline comment\n")
	provider := NewStandardProvider()
	doc := provider.Parse("comments.def", source)

	got := make([]TokenKind, len(doc.Tokens))
	for i, token := range doc.Tokens {
		got[i] = token.Kind
	}
	want := []TokenKind{TokenComment, TokenBlank, TokenSection, TokenKeyValue, TokenComment, TokenBlank}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected token kinds\n got: %#v\nwant: %#v", got, want)
	}

	if len(doc.Sections) != 1 || doc.Sections[0].Span.Start.Line != 3 {
		t.Fatalf("expected one section at line 3, got %#v", doc.Sections)
	}
}

func TestStandardProviderCapturesMalformedLines(t *testing.T) {
	source := []byte("[State 1]\nnot a key value\nbadkey noeq\n[Command]\nname jump\n")
	provider := NewStandardProvider()
	doc := provider.Parse("malformed.def", source)

	gotMalformed := 0
	for _, token := range doc.Tokens {
		if token.Kind == TokenMalformed {
			gotMalformed++
		}
	}
	if gotMalformed != 3 {
		t.Fatalf("expected 3 malformed tokens, got %d", gotMalformed)
	}

	lines := []int{}
	for _, token := range doc.Tokens {
		if token.Kind == TokenMalformed {
			lines = append(lines, token.Span.Start.Line)
		}
	}
	if !reflect.DeepEqual(lines, []int{2, 3, 5}) {
		t.Fatalf("unexpected malformed lines %#v", lines)
	}
}

func TestStandardProviderRoundTripPreservesInput(t *testing.T) {
	source := []byte("[State 1]\ntrigger1 = command = \"jump\"\n\nbad line\n")
	provider := NewStandardProvider()
	doc := provider.Parse("roundtrip.def", source)
	if !bytes.Equal(doc.RoundTrip(), source) {
		t.Fatalf("expected round trip to preserve original bytes")
	}

	reparsed := provider.Parse("roundtrip.def", doc.RoundTrip())
	if !reflect.DeepEqual(doc.Tokens, reparsed.Tokens) {
		t.Fatalf("reparsed tokens changed\ngot:  %#v\nwant: %#v", reparsed.Tokens, doc.Tokens)
	}
	if !reflect.DeepEqual(doc.Sections, reparsed.Sections) {
		t.Fatalf("reparsed sections changed\ngot:  %#v\nwant: %#v", reparsed.Sections, doc.Sections)
	}
}

func TestStandardProviderIsLosslessAcrossSupportedExtensions(t *testing.T) {
	provider := NewStandardProvider()
	cases := []struct {
		name   string
		path   string
		source []byte
	}{
		{
			name:   "def",
			path:   "snippet.def",
			source: []byte("[Info]\nname = Hero\n[Statedef 100]\n"),
		},
		{
			name:   "cns",
			path:   "snippet.cns",
			source: []byte("[State 200]\ntype = ChangeState\nvalue = 100\ntrigger1 = command = \"jump\"\n"),
		},
		{
			name:   "cmd",
			path:   "snippet.cmd",
			source: []byte("[Command]\nname = \"jump\"\n"),
		},
		{
			name:   "st",
			path:   "snippet.st",
			source: []byte("[State 300]\ntype = ChangeState\nvalue = 200\ntrigger1 = command = \"foo\"\n"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := provider.Parse(tc.path, tc.source)
			if doc == nil {
				t.Fatalf("provider parse returned nil for %q", tc.path)
			}
			if doc.Document == nil {
				t.Fatalf("expected document semantic output for %q", tc.path)
			}
			if !bytes.Equal(doc.RoundTrip(), tc.source) {
				t.Fatalf("expected round trip output to preserve bytes for %q", tc.path)
			}
		})
	}
}
