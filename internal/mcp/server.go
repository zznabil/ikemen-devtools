// Package mcp exposes read-only semantic queries over JSON-RPC.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ikemen-engine/ikemen-devtools/internal/capability"
	"github.com/ikemen-engine/ikemen-devtools/internal/lsp"
	"github.com/ikemen-engine/ikemen-devtools/internal/mutation"
	"github.com/ikemen-engine/ikemen-devtools/internal/patch"
)

const MaxFrameSize = lsp.MaxFrameSize

var ErrMalformedFrame = errors.New("mcp: malformed stdio message")
var ErrFrameTooLarge = errors.New("mcp: frame too large")

var supportedProtocolVersions = []string{"2026-07-28", "2025-11-25", "2025-06-18", "2024-11-05"}

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

const (
	modernProtocolVersion  = "2026-07-28"
	protocolVersionMeta    = "io.modelcontextprotocol/protocolVersion"
	clientInfoMeta         = "io.modelcontextprotocol/clientInfo"
	clientCapabilitiesMeta = "io.modelcontextprotocol/clientCapabilities"
	serverInfoMeta         = "io.modelcontextprotocol/serverInfo"
)

var legacyProtocolVersions = []string{"2025-11-25", "2025-06-18", "2024-11-05"}

type Response = lsp.Response
type Error = lsp.Error

// Server is an in-process, read-only MCP facade over the semantic LSP server.
type Server struct {
	lsp        *lsp.Server
	version    string
	registry   *capability.Registry
	root       string
	allowWrite bool
	authorizer *mutation.Authorizer
}

func NewServer() *Server { return NewServerWithVersion("0.0.0-dev") }

func NewServerWithVersion(version string) *Server {
	if strings.TrimSpace(version) == "" {
		version = "0.0.0-dev"
	}
	return &Server{lsp: lsp.NewServer(), version: version, registry: capability.DefaultRegistry()}
}

// NewServerWithPolicy enables guarded mutation tools only with explicit write authority.
func NewServerWithPolicy(version string, allowWrite bool) *Server {
	s := NewServerWithVersion(version)
	s.allowWrite = allowWrite
	if allowWrite {
		s.authorizer = mutation.NewAuthorizer(true)
		for _, name := range []string{"patch_preview", "patch_apply", "rename_prepare", "fix_prepare"} {
			_ = s.registry.Register(capability.Descriptor{Name: name, Description: "Explicitly authorized workspace mutation operation.", Authorization: capability.Authorization{Authority: capability.AuthorityWrite, RequiresApproval: true}, Input: capability.Schema{Type: "object"}, Output: capability.Schema{Type: "object"}, Budget: capability.Budget{MaxItems: 1000, MaxBytes: 1048576, MaxDuration: "30s"}, Ordering: capability.Ordering{Keys: []string{"path"}, Stable: true}})
		}
	}
	return s
}

// NewServerWithRoot confines preloaded documents to root.
func NewServerWithRoot(root string) *Server {
	s := NewServer()
	_ = s.SetRoot(root)
	return s
}

func (s *Server) SetRoot(root string) error {
	if strings.TrimSpace(root) == "" {
		s.root = ""
		return nil
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	s.root = filepath.Clean(abs)
	return nil
}

// SetDocument supplies an in-memory document; it never reads or writes disk.
func (s *Server) SetDocument(ctx context.Context, path string, source []byte) error {
	if s == nil || s.lsp == nil {
		return errors.New("mcp: nil server")
	}
	if s.root != "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(s.root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("mcp: document path escapes workspace root")
		}
	}
	return s.lsp.SetDocument(ctx, path, source)
}

// ReadFrame reads one newline-delimited MCP stdio JSON-RPC message.
func ReadFrame(r io.Reader) ([]byte, error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	body, err := br.ReadBytes('\n')
	if errors.Is(err, io.EOF) && len(body) == 0 {
		return nil, io.EOF
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(body) > MaxFrameSize+1 {
		return nil, ErrFrameTooLarge
	}
	body = bytes.TrimSuffix(body, []byte{'\n'})
	body = bytes.TrimSuffix(body, []byte{'\r'})
	if len(body) == 0 {
		return nil, ErrMalformedFrame
	}
	return body, nil
}

// WriteFrame writes one newline-delimited MCP stdio JSON-RPC message.
func WriteFrame(w io.Writer, body []byte) error {
	if len(body) == 0 || len(body) > MaxFrameSize || bytes.ContainsAny(body, "\r\n") {
		return ErrMalformedFrame
	}
	message := append(append([]byte(nil), body...), '\n')
	n, err := w.Write(message)
	if err != nil {
		return err
	}
	if n != len(message) {
		return io.ErrShortWrite
	}
	return nil
}

// Serve processes newline-delimited requests and JSON-RPC batches until EOF.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	var writeMu sync.Mutex
	write := func(body []byte) error { writeMu.Lock(); defer writeMu.Unlock(); return WriteFrame(w, body) }
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		body, err := ReadFrame(br)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if errors.Is(err, ErrFrameTooLarge) {
			return err
		}
		if err != nil {
			if errors.Is(err, ErrMalformedFrame) {
				encoded, _ := json.Marshal(errorResponse(nil, -32600, "invalid request"))
				if e := write(encoded); e != nil {
					return e
				}
				continue
			}
			return err
		}
		var value interface{}
		if err := json.Unmarshal(body, &value); err != nil {
			encoded, _ := json.Marshal(errorResponse(nil, -32700, "parse error"))
			if e := write(encoded); e != nil {
				return e
			}
			continue
		}
		if batch, ok := value.([]interface{}); ok {
			responses := s.handleBatch(ctx, body, len(batch))
			if len(responses) == 0 {
				continue
			}
			encoded, err := json.Marshal(responses)
			if err != nil {
				return err
			}
			if err := write(encoded); err != nil {
				return err
			}
			continue
		}
		resp := s.Handle(ctx, body)
		if resp == nil {
			continue
		}
		encoded, e := json.Marshal(resp)
		if e != nil {
			return e
		}
		if err := write(encoded); err != nil {
			return err
		}
	}
}

func (s *Server) handleBatch(ctx context.Context, body []byte, count int) []*Response {
	if count == 0 {
		return []*Response{errorResponse(nil, -32600, "invalid request")}
	}
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		return []*Response{errorResponse(nil, -32600, "invalid request")}
	}
	responses := make([]*Response, 0, len(items))
	for _, item := range items {
		if response := s.Handle(ctx, item); response != nil {
			responses = append(responses, response)
		}
	}
	return responses
}

// Handle dispatches one request. Parse and request-shape failures are distinct.
func (s *Server) Handle(ctx context.Context, body []byte) (response *Response) {
	defer func() {
		if recover() != nil {
			response = errorResponse(nil, -32603, "internal error")
		}
	}()
	if s == nil || s.lsp == nil {
		return errorResponse(nil, -32603, "internal error")
	}
	var value interface{}
	if err := json.Unmarshal(body, &value); err != nil {
		return errorResponse(nil, -32700, "parse error")
	}
	if _, ok := value.(map[string]interface{}); !ok {
		return errorResponse(nil, -32600, "invalid request")
	}
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		return errorResponse(nil, -32600, "invalid request")
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		return errorResponse(nil, -32600, "invalid request")
	}
	notification := len(bytes.TrimSpace(req.ID)) == 0
	id := rawID(req.ID)
	if err := ctx.Err(); err != nil {
		if notification {
			return nil
		}
		return errorResponse(id, -32800, "request cancelled")
	}
	if req.Method != "initialize" {
		if err := validateModernRequest(req); err != nil {
			if notification {
				return nil
			}
			return errorResponse(id, err.Code, err.Message, err.Data)
		}
	}
	if notification {
		return nil
	}
	switch req.Method {
	case "initialize":
		result, initErr := legacyInitializeResult(req.Params, s.version)
		if initErr != nil {
			return errorResponse(id, initErr.Code, initErr.Message, initErr.Data)
		}
		return &Response{JSONRPC: "2.0", ID: id, Result: result}
	case "server/discover":
		return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{
			"resultType":        "complete",
			"supportedVersions": append([]string(nil), supportedProtocolVersions...),
			"capabilities":      s.toolCapabilities(),
			"_meta":             map[string]interface{}{serverInfoMeta: serverInfo(s.version)},
			"instructions":      "Use tools/list, then tools/call. Documents must be explicitly preloaded by the host.",
			"ttlMs":             0,
			"cacheScope":        "public",
		}}
	case "ping":
		return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{}}
	case "tools/list":
		return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{"tools": s.toolDefinitions()}}
	case "resources/list":
		return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{"resources": s.resourcesList(req.Params)}}
	case "resources/read":
		return s.resourceRead(ctx, id, req.Params)
	case "resources/templates/list":
		return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{"resourceTemplates": []map[string]interface{}{{"uriTemplate": "ikm://file/{path}", "name": "Source file", "mimeType": "text/plain"}, {"uriTemplate": "ikm://snapshot/{id}", "name": "Workspace snapshot", "mimeType": "application/json"}}}}
	case "tools/call":
		return s.toolCall(ctx, id, req.Params)
	case "shutdown":
		return &Response{JSONRPC: "2.0", ID: id, Result: nil}
	case "textDocument/diagnostic", "textDocument/documentSymbol", "textDocument/hover", "textDocument/definition", "textDocument/references", "document/diagnostics", "document/symbols", "document/hover", "document/definition", "document/references":
		if strings.HasPrefix(req.Method, "document/") {
			req.Method = map[string]string{"document/diagnostics": "textDocument/diagnostic", "document/symbols": "textDocument/documentSymbol", "document/hover": "textDocument/hover", "document/definition": "textDocument/definition", "document/references": "textDocument/references"}[req.Method]
			body, _ = json.Marshal(Request{JSONRPC: req.JSONRPC, ID: req.ID, Method: req.Method, Params: req.Params})
		}
		return s.lsp.Handle(ctx, body)
	default:
		return errorResponse(id, -32601, "method not found")
	}
}
func (s *Server) toolCall(ctx context.Context, id interface{}, raw json.RawMessage) *Response {
	var p struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if json.Unmarshal(raw, &p) != nil || p.Name == "" {
		return errorResponse(id, -32602, "invalid params")
	}
	if p.Arguments != nil {
		if v, ok := p.Arguments["limit"].(float64); ok && (v < 1 || v > 1000 || v != float64(int(v))) {
			return errorResponse(id, -32602, "limit must be between 1 and 1000")
		}
	}
	if s.allowWrite {
		switch p.Name {
		case "patch_preview":
			var plan patch.PatchPlan
			if json.Unmarshal(mustArguments(p.Arguments, "plan"), &plan) != nil {
				return errorResponse(id, -32602, "invalid patch plan")
			}
			if err := plan.Validate(); err != nil {
				return errorResponse(id, -32602, err.Error())
			}
			if s.root != "" && !sameRoot(s.root, plan.WorkspaceRoot) {
				return errorResponse(id, -32006, "patch root differs from authorized root")
			}
			result, err := patch.PreviewPatch(plan.WorkspaceRoot, patch.Patch{Edits: plan.Edits})
			if err != nil {
				return errorResponse(id, -32006, err.Error())
			}
			token, err := s.authorizer.Issue(plan)
			if err != nil {
				return errorResponse(id, -32006, err.Error())
			}
			return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{"preview": result, "token": token}}
		case "patch_apply":
			var plan patch.PatchPlan
			if json.Unmarshal(mustArguments(p.Arguments, "plan"), &plan) != nil {
				return errorResponse(id, -32602, "invalid patch plan")
			}
			token, _ := p.Arguments["token"].(string)
			if s.root != "" && !sameRoot(s.root, plan.WorkspaceRoot) {
				return errorResponse(id, -32006, "patch root differs from authorized root")
			}
			if token == "" || s.authorizer == nil {
				return errorResponse(id, -32006, "authorization required")
			}
			result, err := (&mutation.Service{Authorizer: s.authorizer}).Apply(ctx, plan.WorkspaceRoot, token, plan.InputSnapshot)
			if err != nil {
				return errorResponse(id, -32006, err.Error())
			}
			return &Response{JSONRPC: "2.0", ID: id, Result: result}
		case "rename_prepare", "fix_prepare":
			return errorResponse(id, -32006, "mutation provider refused: semantic provider unavailable")
		}
	}
	method := map[string]string{"document_diagnostics": "textDocument/diagnostic", "document_symbols": "textDocument/documentSymbol", "hover": "textDocument/hover", "definition": "textDocument/definition", "references": "textDocument/references"}[p.Name]
	if method == "" {
		method = p.Name
	}
	params, err := toolParams(method, p.Arguments)
	if err != nil {
		return errorResponse(id, -32602, err.Error())
	}
	innerBody, _ := json.Marshal(Request{JSONRPC: "2.0", Method: method, Params: params})
	inner := s.lsp.Handle(ctx, innerBody)
	if inner.Error != nil {
		return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{"content": []map[string]string{{"type": "text", "text": inner.Error.Message}}, "isError": true}}
	}
	encoded, _ := json.Marshal(inner.Result)
	return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{"content": []map[string]string{{"type": "text", "text": string(encoded)}}, "structuredContent": inner.Result, "_meta": map[string]interface{}{"cache": "snapshot", "etag": "current"}}}
}

func toolParams(method string, args map[string]interface{}) ([]byte, error) {
	if args == nil {
		args = map[string]interface{}{}
	}
	if _, ok := args["textDocument"]; !ok {
		uri, ok := args["uri"].(string)
		if !ok || uri == "" {
			return nil, errors.New("missing uri")
		}
		args["textDocument"] = map[string]string{"uri": uri}
	}
	if method != "textDocument/diagnostic" && method != "textDocument/documentSymbol" {
		if _, ok := args["position"]; !ok {
			line, lineOK := nonNegativeInteger(args["line"])
			character, characterOK := nonNegativeInteger(args["character"])
			if !lineOK || !characterOK {
				return nil, errors.New("missing position")
			}
			args["position"] = map[string]int{"line": line, "character": character}
			delete(args, "line")

			delete(args, "character")
		}
	}
	return json.Marshal(args)
}
func (s *Server) resourcesList(raw json.RawMessage) []map[string]interface{} {
	out := []map[string]interface{}{{"uri": "ikm://workspace", "name": "Workspace summary", "description": "Active workspace metadata", "mimeType": "application/json"}, {"uri": "ikm://snapshot/current", "name": "Current snapshot", "description": "Active snapshot identity", "mimeType": "application/json"}}
	return out
}
func (s *Server) resourceRead(ctx context.Context, id interface{}, raw json.RawMessage) *Response {
	var p struct {
		URI string `json:"uri"`
	}
	if json.Unmarshal(raw, &p) != nil || p.URI == "" || !strings.HasPrefix(p.URI, "ikm://") {
		return errorResponse(id, -32602, "invalid resource URI")
	}
	if err := ctx.Err(); err != nil {
		return errorResponse(id, -32800, "request cancelled")
	}
	var body string
	switch p.URI {
	case "ikm://workspace":
		body = `{"schemaVersion":"0.1.0","status":"complete","authority":"read-only"}`
	case "ikm://snapshot/current":
		body = `{"id":"current"}`
	default:
		return errorResponse(id, -32602, "resource not found")
	}
	return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{"contents": []map[string]interface{}{{"uri": p.URI, "mimeType": "application/json", "text": body}}}}
}

func (s *Server) toolDefinitions() []map[string]interface{} {
	if s == nil || s.registry == nil {
		return nil
	}
	bindings := s.registry.MCPDefinitions(capability.Availability{Read: true, Write: s.allowWrite})
	out := make([]map[string]interface{}, 0, len(bindings))
	for _, binding := range bindings {
		in := binding.InputSchema
		in.Schema = "https://json-schema.org/draft/2020-12/schema"
		in.ID = "ikm://schema/" + binding.Name + "/input"
		outSchema := binding.OutputSchema
		outSchema.Schema = "https://json-schema.org/draft/2020-12/schema"
		outSchema.ID = "ikm://schema/" + binding.Name + "/output"
		out = append(out, map[string]interface{}{"name": binding.Name, "description": binding.Description, "inputSchema": in, "outputSchema": outSchema})
	}
	return out
}

func (s *Server) toolCapabilities() map[string]interface{} {
	if len(s.toolDefinitions()) == 0 {
		return map[string]interface{}{}
	}
	return map[string]interface{}{"tools": map[string]interface{}{}}
}
func nonNegativeInteger(value interface{}) (int, bool) {
	number, ok := value.(float64)
	if !ok || number < 0 || number != float64(int(number)) {
		return 0, false
	}
	return int(number), true
}
func validateModernRequest(req Request) *Error {
	var params map[string]json.RawMessage
	if len(bytes.TrimSpace(req.Params)) == 0 || json.Unmarshal(req.Params, &params) != nil || params == nil {
		if req.Method == "server/discover" {
			return &Error{Code: -32602, Message: "missing request metadata"}
		}
		return nil
	}
	var meta map[string]json.RawMessage
	rawMeta, ok := params["_meta"]
	if !ok {
		if req.Method == "server/discover" {
			return &Error{Code: -32602, Message: "missing request metadata"}
		}
		return nil
	}
	if json.Unmarshal(rawMeta, &meta) != nil || meta == nil {
		return &Error{Code: -32602, Message: "missing request metadata"}
	}
	var protocolVersion string
	if json.Unmarshal(meta[protocolVersionMeta], &protocolVersion) != nil || protocolVersion == "" {
		return &Error{Code: -32602, Message: "missing protocol version metadata"}
	}
	if protocolVersion != modernProtocolVersion {
		return &Error{
			Code:    -32022,
			Message: "Unsupported protocol version",
			Data: map[string]interface{}{
				"supported": append([]string(nil), supportedProtocolVersions...),
				"requested": protocolVersion,
			},
		}
	}
	var clientInfo map[string]interface{}
	if json.Unmarshal(meta[clientInfoMeta], &clientInfo) != nil || clientInfo == nil || nonEmptyString(clientInfo["name"]) == "" || nonEmptyString(clientInfo["version"]) == "" {
		return &Error{Code: -32602, Message: "missing client info metadata"}
	}
	var clientCapabilities map[string]interface{}
	if json.Unmarshal(meta[clientCapabilitiesMeta], &clientCapabilities) != nil || clientCapabilities == nil {
		return &Error{Code: -32602, Message: "missing client capabilities metadata"}
	}
	if req.Method == "server/discover" {
		for key := range params {
			if key != "_meta" {
				return &Error{Code: -32602, Message: "server/discover accepts only _meta"}
			}
		}
	}
	return nil
}

func nonEmptyString(value interface{}) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func legacyInitializeResult(raw json.RawMessage, version string) (map[string]interface{}, *Error) {
	var params map[string]json.RawMessage
	if len(bytes.TrimSpace(raw)) == 0 || json.Unmarshal(raw, &params) != nil || params == nil {
		return nil, &Error{Code: -32602, Message: "invalid initialize params"}
	}
	var requested string
	if json.Unmarshal(params["protocolVersion"], &requested) != nil || requested == "" {
		return nil, &Error{Code: -32602, Message: "missing protocol version"}
	}
	for _, supported := range legacyProtocolVersions {
		if requested == supported {
			return map[string]interface{}{
				"protocolVersion": requested,
				"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
				"serverInfo":      serverInfo(version),
			}, nil
		}
	}
	return nil, &Error{
		Code:    -32022,
		Message: "Unsupported protocol version",
		Data: map[string]interface{}{
			"supported": append([]string(nil), legacyProtocolVersions...),
			"requested": requested,
		},
	}
}

func serverInfo(version string) map[string]interface{} {
	return map[string]interface{}{"name": "ikemen-devtools", "version": version}
}

func errorResponse(id interface{}, code int, message string, data ...interface{}) *Response {
	var detail interface{}
	if len(data) > 0 {
		detail = data[0]
	}
	return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: code, Message: message, Data: detail}}
}

func rawID(raw json.RawMessage) interface{} {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var value interface{}
	if decoder.Decode(&value) != nil {
		return nil
	}
	return value
}
func mustArguments(args map[string]interface{}, key string) []byte {
	value, ok := args[key]
	if !ok {
		return nil
	}
	b, _ := json.Marshal(value)
	return b
}
func sameRoot(a, b string) bool {
	aa, ea := filepath.Abs(a)
	bb, eb := filepath.Abs(b)
	return ea == nil && eb == nil && filepath.Clean(aa) == filepath.Clean(bb)
}
