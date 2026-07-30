package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestHandleMCPInitializeAndTools(t *testing.T) {
	s := NewServerWithVersion("1.2.3")
	resp := s.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`))
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected initialize result %#v", resp.Result)
	}
	if result["protocolVersion"] != "2025-11-25" {
		t.Fatalf("unexpected negotiated protocol version %#v", result)
	}
	raw, _ := json.Marshal(result)
	if !bytes.Contains(raw, []byte(`"version":"1.2.3"`)) {
		t.Fatalf("missing server version: %s", raw)
	}
	resp = s.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}
	raw, _ = json.Marshal(resp.Result)
	for _, name := range []string{"document_diagnostics", "document_symbols", "hover", "definition", "references"} {
		if !bytes.Contains(raw, []byte(name)) {
			t.Fatalf("tools/list missing %s: %s", name, raw)
		}
	}
	if !bytes.Contains(raw, []byte(`"required":["uri"]`)) || !bytes.Contains(raw, []byte(`"character"`)) {
		t.Fatalf("tools/list must publish useful input schemas: %s", raw)
	}
}

func TestToolCallUsesSemanticQueries(t *testing.T) {
	s := NewServer()
	ctx := context.Background()
	if err := s.SetDocument(ctx, "hero.cmd", []byte("[Command]\nname = \"jump\"\n")); err != nil {
		t.Fatal(err)
	}
	resp := s.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"document_symbols","arguments":{"uri":"file://hero.cmd"}}}`))
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	if !bytes.Contains(raw, []byte("command:jump")) {
		t.Fatalf("symbols result %s", raw)
	}
}

func TestMalformedAndCancelledRequestsAreSafe(t *testing.T) {
	s := NewServer()
	resp := s.Handle(context.Background(), []byte("{"))
	if resp.Error == nil || resp.Error.Code != -32700 {
		t.Fatalf("malformed response %#v", resp)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp = s.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if resp.Error == nil || resp.Error.Code != -32800 {
		t.Fatalf("cancelled response %#v", resp)
	}
}

func TestServeFramedJSONRPC(t *testing.T) {
	s := NewServer()
	var in, out bytes.Buffer
	if err := WriteFrame(&in, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Serve(context.Background(), &in, &out); err != nil {
		t.Fatal(err)
	}
	body, err := ReadFrame(&out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"jsonrpc":"2.0"`)) {
		t.Fatalf("response %s", body)
	}
	if bytes.Contains(out.Bytes(), []byte("Content-Length:")) {
		t.Fatalf("MCP stdio must be newline-delimited, got %q", out.String())
	}
}

func TestServerDiscoverAndNotificationBehavior(t *testing.T) {
	s := NewServerWithVersion("1.2.3")
	resp := s.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{}}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("discover response %#v", resp)
	}
	raw, _ := json.Marshal(resp.Result)
	for _, want := range []string{"2026-07-28", "2025-11-25", `"resultType":"complete"`, `"version":"1.2.3"`} {
		if !bytes.Contains(raw, []byte(want)) {
			t.Fatalf("discover missing %s: %s", want, raw)
		}
	}

	if got := s.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); got != nil {
		t.Fatalf("notification must not receive a response: %#v", got)
	}

	var in, out bytes.Buffer
	if err := WriteFrame(&in, []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Serve(context.Background(), &in, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("notification wrote a response: %q", out.String())
	}
}

func TestToolCallAcceptsFlatPositionArguments(t *testing.T) {
	s := NewServer()
	ctx := context.Background()
	if err := s.SetDocument(ctx, "hero.cmd", []byte("[Command]\nname = \"jump\"\n")); err != nil {
		t.Fatal(err)
	}
	resp := s.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"hover","arguments":{"uri":"file://hero.cmd","line":1,"character":8}}}`))
	if resp == nil || resp.Error != nil || resp.Result == nil {
		t.Fatalf("flat position tool call failed: %#v", resp)
	}
}

func TestMCPNewlineFramingRejectsEmbeddedNewlines(t *testing.T) {
	var out bytes.Buffer
	if err := WriteFrame(&out, []byte("{\n}")); err == nil {
		t.Fatal("expected embedded newline rejection")
	}
}

func TestNullIDIsStillARequest(t *testing.T) {
	response := NewServer().Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":null,"method":"ping"}`))
	if response == nil || response.Error != nil {
		t.Fatalf("null-id request was treated as a notification: %#v", response)
	}
}
