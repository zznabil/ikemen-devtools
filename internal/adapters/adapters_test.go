package adapters

import "testing"

func TestParseZSSSectionsCommentsDependenciesAndLosslessSource(t *testing.T) {
	source := "; motif\r\n[Info]\r\nname = " + `"Arcade"` + " ; display\r\ninclude = " + `"common.zss"` + "\r\n\r\n[StateDef 0]\r\n; state comment\r\nvalue = 1\r\n"
	doc := ParseZSS("motif.zss", []byte(source))
	if doc == nil || doc.Source != source {
		t.Fatalf("source was not preserved: %#v", doc)
	}
	if len(doc.Sections) != 2 {
		t.Fatalf("sections = %d", len(doc.Sections))
	}
	if doc.Sections[0].Name != "Info" || doc.Sections[1].Name != "StateDef 0" {
		t.Fatalf("unexpected sections: %#v", doc.Sections)
	}
	if len(doc.Dependencies) != 1 || doc.Dependencies[0].Path != "common.zss" || doc.Dependencies[0].Kind != DependencyInclude {
		t.Fatalf("dependencies = %#v", doc.Dependencies)
	}
	if len(doc.Comments) != 2 {
		t.Fatalf("comments = %#v", doc.Comments)
	}
	if doc.Sections[0].Span.Start.Line != 2 || doc.Sections[0].Span.Start.Column != 1 {
		t.Fatalf("section span = %#v", doc.Sections[0].Span)
	}
	if got := source[doc.Dependencies[0].Span.Start.Offset:doc.Dependencies[0].Span.End.Offset]; got != `include = "common.zss"` {
		t.Fatalf("dependency span = %q", got)
	}
}

func TestParseZSSMalformedSectionsDiagnosticIsDeterministic(t *testing.T) {
	doc := ParseZSS("bad.zss", []byte("[ok]\nkey = value\n[unterminated\n"))
	if len(doc.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", doc.Diagnostics)
	}
	if doc.Diagnostics[0].Code != "zss.unterminated-section" || doc.Diagnostics[0].Span.Start.Line != 3 {
		t.Fatalf("diagnostic = %#v", doc.Diagnostics[0])
	}
}

func TestParseLuaFunctionsDependenciesCommentsAndSpans(t *testing.T) {
	source := "-- header\nlocal M = {}\n\nfunction M.start(arg)\n  local x = require(\"common\")\n  include('extra.lua')\nend\n\nlocal function helper()\nend\n"
	doc := ParseLua("main.lua", []byte(source))
	if doc == nil || doc.Source != source {
		t.Fatalf("source was not preserved")
	}
	if len(doc.Functions) != 2 || doc.Functions[0].Name != "M.start" || doc.Functions[1].Name != "helper" {
		t.Fatalf("functions = %#v", doc.Functions)
	}
	if len(doc.Dependencies) != 2 || doc.Dependencies[0].Path != "common" || doc.Dependencies[1].Path != "extra.lua" {
		t.Fatalf("dependencies = %#v", doc.Dependencies)
	}
	if len(doc.Comments) != 1 || doc.Comments[0].Text != "-- header" {
		t.Fatalf("comments = %#v", doc.Comments)
	}
	if doc.Functions[0].Span.Start.Line != 4 || doc.Functions[0].Span.End.Line != 7 {
		t.Fatalf("function span = %#v", doc.Functions[0].Span)
	}
}

func TestParseLuaMalformedSyntaxDiagnosticIsDeterministic(t *testing.T) {
	doc := ParseLua("bad.lua", []byte("function broken(\n  return 1\n"))
	if len(doc.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", doc.Diagnostics)
	}
	if doc.Diagnostics[0].Code != "lua.unterminated-function" || doc.Diagnostics[0].Span.Start.Line != 1 {
		t.Fatalf("diagnostic = %#v", doc.Diagnostics[0])
	}
}
