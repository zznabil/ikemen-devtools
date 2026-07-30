// Package mcp exposes read-only semantic queries over JSON-RPC.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/ikemen-engine/ikemen-devtools/internal/lsp"
)

const MaxFrameSize = lsp.MaxFrameSize

var ErrMalformedFrame = errors.New("mcp: malformed stdio message")

var supportedProtocolVersions = []string{"2026-07-28", "2025-11-25", "2025-06-18", "2024-11-05"}

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type Response = lsp.Response
type Error = lsp.Error

// Server is an in-process, read-only MCP facade over the semantic LSP server.
type Server struct {
	lsp     *lsp.Server
	version string
}

func NewServer() *Server { return NewServerWithVersion("0.0.0-dev") }

func NewServerWithVersion(version string) *Server {
	if strings.TrimSpace(version) == "" {
		version = "0.0.0-dev"
	}
	return &Server{lsp: lsp.NewServer(), version: version}
}

// SetDocument supplies an in-memory document; it never reads or writes disk.
func (s *Server) SetDocument(ctx context.Context, path string, source []byte) error {
	if s == nil || s.lsp == nil {
		return errors.New("mcp: nil server")
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
		return nil, ErrMalformedFrame
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
	message := make([]byte, 0, len(body)+1)
	message = append(message, body...)
	message = append(message, '\n')
	n, err := w.Write(message)
	if err != nil {
		return err
	}
	if n != len(message) {
		return io.ErrShortWrite
	}
	return nil
}

// Serve processes newline-delimited requests until EOF or cancellation.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		body, err := ReadFrame(br)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		resp := s.Handle(ctx, body)
		if resp == nil {
			continue
		}
		encoded, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		if err := WriteFrame(w, encoded); err != nil {
			return err
		}
	}
}

// Handle dispatches one request. Malformed input always returns a parse error.
func (s *Server) Handle(ctx context.Context, body []byte) *Response {
	if s == nil || s.lsp == nil {
		return errorResponse(nil, -32603, "server unavailable")
	}
	var req Request
	if err := json.Unmarshal(body, &req); err != nil || req.JSONRPC != "2.0" || req.Method == "" {
		return errorResponse(nil, -32700, "parse error")
	}
	id := rawID(req.ID)
	if len(bytes.TrimSpace(req.ID)) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return errorResponse(id, -32800, "request cancelled")
	}
	switch req.Method {
	case "initialize":
		protocolVersion := negotiateLegacyVersion(req.Params)
		return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      serverInfo(s.version),
		}}
	case "server/discover":
		return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{
			"resultType":        "complete",
			"supportedVersions": append([]string(nil), supportedProtocolVersions...),
			"capabilities":      map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":        serverInfo(s.version),
			"instructions":      "Use tools/list, then tools/call. Documents must be explicitly preloaded by the host.",
		}}
	case "ping":
		return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{}}
	case "tools/list":
		return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{"tools": toolDefinitions()}}
	case "tools/call":
		return s.toolCall(ctx, id, req.Params)
	case "shutdown":
		return &Response{JSONRPC: "2.0", ID: id, Result: nil}
	case "textDocument/diagnostic", "textDocument/documentSymbol", "textDocument/hover", "textDocument/definition", "textDocument/references",
		"document/diagnostics", "document/symbols", "document/hover", "document/definition", "document/references":
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
	if err := json.Unmarshal(raw, &p); err != nil || p.Name == "" {
		return errorResponse(id, -32602, "invalid params")
	}
	method, ok := map[string]string{
		"document_diagnostics": "textDocument/diagnostic",
		"document_symbols":     "textDocument/documentSymbol",
		"hover":                "textDocument/hover",
		"definition":           "textDocument/definition",
		"references":           "textDocument/references",
	}[p.Name]
	if !ok {
		return errorResponse(id, -32602, "unknown tool")
	}
	params, err := toolParams(method, p.Arguments)
	if err != nil {
		return errorResponse(id, -32602, err.Error())
	}
	innerBody := append([]byte(`{"jsonrpc":"2.0","id":0,"method":"`+method+`","params":`), params...)
	innerBody = append(innerBody, '}')
	inner := s.lsp.Handle(ctx, innerBody)
	if inner.Error != nil {
		return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": inner.Error.Message}},
			"isError": true,
		}}
	}
	encoded, _ := json.Marshal(inner.Result)
	return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{
		"content":           []map[string]string{{"type": "text", "text": string(encoded)}},
		"structuredContent": inner.Result,
	}}
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
		delete(args, "uri")
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

func toolDefinitions() []map[string]interface{} {
	documentSchema := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           map[string]interface{}{"uri": map[string]interface{}{"type": "string", "minLength": 1}},
		"required":             []string{"uri"},
	}
	positionSchema := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"uri":       map[string]interface{}{"type": "string", "minLength": 1},
			"line":      map[string]interface{}{"type": "integer", "minimum": 0},
			"character": map[string]interface{}{"type": "integer", "minimum": 0},
		},
		"required": []string{"uri", "line", "character"},
	}
	return []map[string]interface{}{
		{"name": "document_diagnostics", "description": "Return parser and semantic diagnostics for a preloaded IKEMEN document.", "inputSchema": documentSchema},
		{"name": "document_symbols", "description": "Return stable symbols for a preloaded IKEMEN document.", "inputSchema": documentSchema},
		{"name": "hover", "description": "Explain the symbol at an IKEMEN document position.", "inputSchema": positionSchema},
		{"name": "definition", "description": "Find the definition at an IKEMEN document position.", "inputSchema": positionSchema},
		{"name": "references", "description": "Find references at an IKEMEN document position.", "inputSchema": positionSchema},
	}
}

func nonNegativeInteger(value interface{}) (int, bool) {
	number, ok := value.(float64)
	if !ok || number < 0 || number != float64(int(number)) {
		return 0, false
	}
	return int(number), true
}

func negotiateLegacyVersion(raw json.RawMessage) string {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(raw, &params)
	for _, supported := range supportedProtocolVersions[1:] {
		if params.ProtocolVersion == supported {
			return supported
		}
	}
	return supportedProtocolVersions[1]
}

func serverInfo(version string) map[string]interface{} {
	return map[string]interface{}{"name": "ikemen-devtools", "version": version}
}

func errorResponse(id interface{}, code int, message string) *Response {
	return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: code, Message: message}}
}

func rawID(raw json.RawMessage) interface{} {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var value interface{}
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	return value
}
