package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var b bytes.Buffer
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"shutdown"}`)
	if err := WriteFrame(&b, body); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFrame(&b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("got %s", got)
	}
}
func TestInitializeCapabilities(t *testing.T) {
	r := NewServer().Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	v, _ := json.Marshal(r.Result)
	s := string(v)
	if !bytes.Contains([]byte(s), []byte(`"diagnosticProvider":true`)) || !bytes.Contains([]byte(s), []byte(`"documentSymbolProvider":true`)) || !bytes.Contains([]byte(s), []byte(`"textDocumentSync":1`)) {
		t.Fatalf("capabilities %s", s)
	}
}

func TestDocumentLifecycleNotifications(t *testing.T) {
	s := NewServer()
	ctx := context.Background()
	open := []byte(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file://hero.cmd","text":"[Command]\nname = \"jump\"\n"}}}`)
	if got := s.Handle(ctx, open); got != nil {
		t.Fatalf("didOpen notification returned %#v", got)
	}
	query := []byte(`{"jsonrpc":"2.0","id":1,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":"file://hero.cmd"}}}`)
	raw, _ := json.Marshal(s.Handle(ctx, query).Result)
	if !bytes.Contains(raw, []byte("command:jump")) {
		t.Fatalf("didOpen did not populate document: %s", raw)
	}

	change := []byte(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":"file://hero.cmd"},"contentChanges":[{"text":"[Command]\nname = \"dash\"\n"}]}}`)
	if got := s.Handle(ctx, change); got != nil {
		t.Fatalf("didChange notification returned %#v", got)
	}
	raw, _ = json.Marshal(s.Handle(ctx, query).Result)
	if !bytes.Contains(raw, []byte("command:dash")) || bytes.Contains(raw, []byte("command:jump")) {
		t.Fatalf("didChange did not replace document: %s", raw)
	}

	closeMessage := []byte(`{"jsonrpc":"2.0","method":"textDocument/didClose","params":{"textDocument":{"uri":"file://hero.cmd"}}}`)
	if got := s.Handle(ctx, closeMessage); got != nil {
		t.Fatalf("didClose notification returned %#v", got)
	}
	raw, _ = json.Marshal(s.Handle(ctx, query).Result)
	if bytes.Contains(raw, []byte("command:dash")) {
		t.Fatalf("didClose retained document: %s", raw)
	}
}

func TestServeDoesNotRespondToNotifications(t *testing.T) {
	var in, out bytes.Buffer
	if err := WriteFrame(&in, []byte(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)); err != nil {
		t.Fatal(err)
	}
	if err := NewServer().Serve(context.Background(), &in, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("notification wrote a response: %q", out.String())
	}
}

func TestNullIDIsStillARequest(t *testing.T) {
	response := NewServer().Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":null,"method":"initialize","params":{}}`))
	if response == nil || response.Error != nil {
		t.Fatalf("null-id request was treated as a notification: %#v", response)
	}
}
func TestDiagnosticsAndSymbols(t *testing.T) {
	s := NewServer()
	ctx := context.Background()
	if err := s.SetDocument(ctx, "hero.def", []byte("[Statedef 0]\nname = idle\nmalformed\n")); err != nil {
		t.Fatal(err)
	}
	r := s.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"textDocument/diagnostic","params":{"textDocument":{"uri":"file://hero.def"}}}`))
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	raw, _ := json.Marshal(r.Result)
	if !bytes.Contains(raw, []byte("malformed-line")) {
		t.Fatalf("diagnostics %s", raw)
	}
	r = s.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":2,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":"file://hero.def"}}}`))
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	raw, _ = json.Marshal(r.Result)
	if !bytes.Contains(raw, []byte("state:0")) {
		t.Fatalf("symbols %s", raw)
	}
}
func TestUnknownMethodErrorAndMalformedSafety(t *testing.T) {
	s := NewServer()
	r := s.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"wat"}`))
	if r.Error == nil || r.Error.Code != -32601 {
		t.Fatalf("unexpected %#v", r)
	}
	r = s.Handle(context.Background(), []byte("{"))
	if r.Error == nil || r.Error.Code != -32700 {
		t.Fatalf("unexpected malformed %#v", r)
	}
}
func TestHoverDefinitionReferences(t *testing.T) {
	s := NewServer()
	ctx := context.Background()
	if err := s.SetDocument(ctx, "hero.cmd", []byte("[Command]\nname = \"jump\"\n")); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDocument(ctx, "hero.st", []byte("[State 0]\ntype = Null\ntrigger1 = command = \"jump\"\n")); err != nil {
		t.Fatal(err)
	}
	hover := s.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"textDocument/hover","params":{"textDocument":{"uri":"file://hero.cmd"},"position":{"line":1,"character":8}}}`))
	if hover.Error != nil || hover.Result == nil {
		t.Fatalf("hover: %#v", hover)
	}
	hoverRaw, _ := json.Marshal(hover.Result)
	if !bytes.Contains(hoverRaw, []byte("command:jump")) {
		t.Fatalf("hover %s", hoverRaw)
	}
	def := s.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":2,"method":"textDocument/definition","params":{"textDocument":{"uri":"file://hero.st"},"position":{"line":2,"character":22}}}`))
	if def.Error != nil {
		t.Fatalf("definition: %#v", def)
	}
	var locations []struct {
		URI string `json:"uri"`
	}
	raw, _ := json.Marshal(def.Result)
	if err := json.Unmarshal(raw, &locations); err != nil || len(locations) != 1 || locations[0].URI != "file://hero.cmd" {
		t.Fatalf("definition %s", raw)
	}
	refs := s.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":3,"method":"textDocument/references","params":{"textDocument":{"uri":"file://hero.cmd"},"position":{"line":1,"character":8},"context":{"includeDeclaration":false}}}`))
	if refs.Error != nil {
		t.Fatalf("references: %#v", refs)
	}
	raw, _ = json.Marshal(refs.Result)
	if !bytes.Contains(raw, []byte("hero.st")) {
		t.Fatalf("references %s", raw)
	}
}

func TestReadOnlyLSPNoMatch(t *testing.T) {
	s := NewServer()
	ctx := context.Background()
	if err := s.SetDocument(ctx, "hero.cmd", []byte("[Command]\nname = \"jump\"\n")); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"textDocument/hover", "textDocument/definition", "textDocument/references"} {
		raw := []byte(`{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":{"textDocument":{"uri":"file://hero.cmd"},"position":{"line":0,"character":0}}}`)
		resp := s.Handle(ctx, raw)
		if resp.Error != nil {
			t.Fatalf("%s: %#v", method, resp)
		}
		if method == "textDocument/hover" {
			if resp.Result != nil {
				t.Fatalf("%s result %#v", method, resp.Result)
			}
		} else {
			result, ok := resp.Result.([]Location)
			if !ok || len(result) != 0 {
				t.Fatalf("%s result %#v", method, resp.Result)
			}
		}
	}
}
