package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/patch"
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
	resp := s.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"test","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}}}`))
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

func TestBatchRequestsAndResponseExclusivity(t *testing.T) {
	s := NewServer()
	var in, out bytes.Buffer
	body := `[{"jsonrpc":"2.0","id":"a","method":"ping"},{"jsonrpc":"2.0","id":null,"method":"ping"},{"jsonrpc":"2.0","method":"initialized"},{"jsonrpc":"2.0","id":9007199254740993,"method":"missing"}]`
	if err := WriteFrame(&in, []byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := s.Serve(context.Background(), &in, &out); err != nil {
		t.Fatal(err)
	}
	frame, err := ReadFrame(&out)
	if err != nil {
		t.Fatal(err)
	}
	var responses []json.RawMessage
	if err := json.Unmarshal(frame, &responses); err != nil || len(responses) != 3 {
		t.Fatalf("unexpected batch responses: %s", frame)
	}
	for _, raw := range responses {
		var shape map[string]json.RawMessage
		if err := json.Unmarshal(raw, &shape); err != nil {
			t.Fatal(err)
		}
		_, hasResult := shape["result"]
		_, hasError := shape["error"]
		if hasResult == hasError {
			t.Fatalf("response must contain exactly one of result/error: %s", raw)
		}
	}
	if !bytes.Contains(responses[2], []byte(`"id":9007199254740993`)) {
		t.Fatalf("numeric request id was not preserved: %s", responses[2])
	}
}

func TestBatchAndRequestErrorClasses(t *testing.T) {
	s := NewServer()
	tests := []struct {
		name string
		body string
		code int
	}{
		{"parse", `{`, -32700},
		{"invalid request", `{}`, -32600},
		{"method not found", `{"jsonrpc":"2.0","id":1,"method":"missing"}`, -32601},
		{"invalid params", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":[]}`, -32602},
	}
	for _, test := range tests {
		response := s.Handle(context.Background(), []byte(test.body))
		if response == nil || response.Error == nil || response.Error.Code != test.code || response.Result != nil {
			t.Errorf("%s response = %#v", test.name, response)
		}
	}
	response := s.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":null,"method":"ping"}`))
	if response == nil || response.Error != nil {
		t.Fatalf("explicit null id must remain a request: %#v", response)
	}
}
func TestResourcesListReadAndTemplates(t *testing.T) {
	s := NewServer()
	for _, method := range []string{"resources/list", "resources/templates/list"} {
		r := s.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":{}}`))
		if r.Error != nil {
			t.Fatal(method, r.Error)
		}
	}
	r := s.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"ikm://workspace"}}`))
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	bad := s.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"ikm://file/../../outside"}}`))
	if bad.Error == nil || bad.Error.Code != -32602 {
		t.Fatalf("expected invalid resource URI: %#v", bad)
	}
}

func TestReadOnlyRegistryParityAndSchemas(t *testing.T) {
	s := NewServer()
	r := s.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	raw, _ := json.Marshal(r.Result)
	for _, name := range []string{"diagnostics", "symbols", "search", "graph_impact", "inspect_workspace", "export_jsonl"} {
		if !bytes.Contains(raw, []byte(name)) {
			t.Fatalf("missing registry operation %s", name)
		}
	}
}
func TestCompiledParityTranscriptShape(t *testing.T) {
	s := NewServer()
	var in, out bytes.Buffer
	in.WriteString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}` + "\n")
	in.WriteString(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n")
	if err := s.Serve(context.Background(), &in, &out); err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("transcript lines=%d", len(lines))
	}
	for _, line := range lines {
		var v map[string]any
		if json.Unmarshal(line, &v) != nil || v["jsonrpc"] != "2.0" {
			t.Fatalf("invalid transcript %s", line)
		}
	}
}
func TestServerRootContainmentAndWritePolicy(t *testing.T) {
	s := NewServerWithRoot(t.TempDir())
	if err := s.SetDocument(context.Background(), "../outside.cmd", nil); err == nil {
		t.Fatal("expected root escape refusal")
	}
	if raw, _ := json.Marshal(NewServer().toolDefinitions()); bytes.Contains(raw, []byte("patch_apply")) {
		t.Fatal("mutation tools must be absent by default")
	}
	if raw, _ := json.Marshal(NewServerWithPolicy("1", true).toolDefinitions()); !bytes.Contains(raw, []byte("patch_apply")) {
		t.Fatal("write-enabled server must advertise patch_apply")
	}
}

func TestMCPMutationLifecycle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "hero.cmd")
	source := []byte("hello\n")
	if err := os.WriteFile(path, source, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(source)
	plan := patch.PatchPlan{
		Version: patch.PlanVersion, WorkspaceRoot: root, InputSnapshot: "snap-1",
		Edits: []patch.Edit{{Path: "hero.cmd", ContentHash: hex.EncodeToString(sum[:]), IdentityContract: ir.IdentityContractVersion, Span: patch.Span{ByteStart: 0, ByteEnd: 5}, OldText: "hello", NewText: "world"}},
	}
	s := NewServerWithPolicy("1", true)
	if err := s.SetRoot(root); err != nil {
		t.Fatal(err)
	}
	call := func(id int, name string, args map[string]interface{}) *Response {
		body, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": map[string]interface{}{"name": name, "arguments": args}})
		return s.Handle(ctx, body)
	}
	preview := call(1, "patch_preview", map[string]interface{}{"plan": plan})
	if preview.Error != nil {
		t.Fatal(preview.Error)
	}
	if got, _ := os.ReadFile(path); string(got) != string(source) {
		t.Fatalf("preview mutated file: %q", got)
	}
	result := preview.Result.(map[string]interface{})
	token, ok := result["token"].(string)
	if !ok || token == "" {
		t.Fatalf("preview token missing: %#v", result)
	}
	stale := plan
	stale.InputSnapshot = "snap-old"
	if resp := call(2, "patch_apply", map[string]interface{}{"plan": stale, "token": token}); resp.Error == nil {
		t.Fatal("stale apply must be refused")
	}
	applied := call(3, "patch_apply", map[string]interface{}{"plan": plan, "token": token})
	if applied.Error != nil {
		t.Fatal(applied.Error)
	}
	if got, _ := os.ReadFile(path); string(got) != "world\n" {
		t.Fatalf("atomic apply result %q", got)
	}
	if resp := call(4, "patch_apply", map[string]interface{}{"plan": plan, "token": token}); resp.Error == nil {
		t.Fatal("mutation token must be single-use")
	}
	if resp := call(5, "rename_prepare", map[string]interface{}{}); resp.Error == nil {
		t.Fatal("ambiguous/unavailable mutation provider must refuse")
	}
}

func TestConcurrentServeFramesRemainParseable(t *testing.T) {
	s := NewServer()
	var in, out bytes.Buffer
	for range 8 {
		in.WriteString(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	}
	if err := s.Serve(context.Background(), &in, &out); err != nil {
		t.Fatal(err)
	}
	for out.Len() > 0 {
		frame, err := ReadFrame(&out)
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if json.Unmarshal(frame, &v) != nil || v["jsonrpc"] != "2.0" {
			t.Fatalf("invalid response %s", frame)
		}
	}
}
