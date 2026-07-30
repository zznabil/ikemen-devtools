package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/parser"
	"github.com/ikemen-engine/ikemen-devtools/internal/semantics"
)

type Server struct {
	mu   sync.RWMutex
	docs map[string]ir.Document
}

func NewServer() *Server { return &Server{docs: make(map[string]ir.Document)} }

// SetDocument adds or replaces a parsed document. It never writes to disk.
func (s *Server) SetDocument(ctx context.Context, path string, source []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return errors.New("lsp: empty document path")
	}
	doc := parser.Parse(path, string(source))
	if doc == nil {
		return errors.New("lsp: parser returned nil document")
	}
	s.mu.Lock()
	s.docs[path] = *doc
	s.docs[filepath.Clean(path)] = *doc
	s.docs[filepath.ToSlash(path)] = *doc
	s.docs[uriForPath(path)] = *doc
	s.mu.Unlock()
	return nil
}

// RemoveDocument forgets an in-memory document and all of its lookup aliases.
func (s *Server) RemoveDocument(pathOrURI string) {
	if s == nil {
		return
	}
	path := pathForURI(pathOrURI)
	s.mu.Lock()
	delete(s.docs, pathOrURI)
	delete(s.docs, path)
	delete(s.docs, filepath.Clean(path))
	delete(s.docs, filepath.ToSlash(path))
	delete(s.docs, uriForPath(path))
	s.mu.Unlock()
}

// Serve processes framed JSON-RPC messages until EOF or cancellation.
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
		encoded, err := encodeFrame(resp)
		if err != nil {
			return err
		}
		if err := WriteFrame(w, encoded); err != nil {
			return err
		}
	}
}

// Handle decodes and dispatches one JSON-RPC request, returning a response.
// Malformed requests receive a parse error with a null id rather than panicking.
func (s *Server) Handle(ctx context.Context, body []byte) *Response {
	if err := ctx.Err(); err != nil {
		return &Response{JSONRPC: "2.0", ID: nil, Error: &Error{Code: -32800, Message: "request cancelled"}}
	}
	req, err := decodeRequest(body)
	if err != nil {
		return &Response{JSONRPC: "2.0", ID: nil, Error: &Error{Code: -32700, Message: "parse error"}}
	}
	id := rawID(req.ID)
	if isNotification(req.ID) {
		s.handleNotification(ctx, req.Method, req.Params)
		return nil
	}
	switch req.Method {
	case "initialize":
		return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{"capabilities": map[string]interface{}{"textDocumentSync": 1, "diagnosticProvider": true, "documentSymbolProvider": true, "hoverProvider": true, "definitionProvider": true, "referencesProvider": true}}}
	case "shutdown":
		return &Response{JSONRPC: "2.0", ID: id, Result: nil}
	case "textDocument/diagnostic":
		return s.diagnostics(ctx, id, req.Params)
	case "textDocument/documentSymbol":
		return s.symbols(ctx, id, req.Params)
	case "textDocument/hover":
		return s.hover(ctx, id, req.Params)
	case "textDocument/definition":
		return s.definition(ctx, id, req.Params)
	case "textDocument/references":
		return s.references(ctx, id, req.Params)
	default:
		return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: -32601, Message: "method not found"}}
	}
}

func (s *Server) handleNotification(ctx context.Context, method string, raw json.RawMessage) {
	switch method {
	case "initialized", "exit", "$/cancelRequest":
		return
	case "textDocument/didOpen":
		var p struct {
			TextDocument struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"textDocument"`
		}
		if json.Unmarshal(raw, &p) == nil && p.TextDocument.URI != "" {
			_ = s.SetDocument(ctx, pathForURI(p.TextDocument.URI), []byte(p.TextDocument.Text))
		}
	case "textDocument/didChange":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		if json.Unmarshal(raw, &p) == nil && p.TextDocument.URI != "" && len(p.ContentChanges) != 0 {
			_ = s.SetDocument(ctx, pathForURI(p.TextDocument.URI), []byte(p.ContentChanges[len(p.ContentChanges)-1].Text))
		}
	case "textDocument/didClose":
		var p textDocumentParams
		if json.Unmarshal(raw, &p) == nil && p.TextDocument.URI != "" {
			s.RemoveDocument(p.TextDocument.URI)
		}
	}
}

type textDocumentPositionParams struct {
	TextDocument struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"textDocument"`
	Position Position `json:"position"`
	Context  struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context"`
}

type textDocumentParams struct {
	TextDocument struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"textDocument"`
}

func (s *Server) document(ctx context.Context, p json.RawMessage) (ir.Document, error) {
	var params textDocumentParams
	if err := json.Unmarshal(p, &params); err != nil || params.TextDocument.URI == "" {
		return ir.Document{}, errors.New("invalid params")
	}
	if params.TextDocument.Text != "" {
		if err := s.SetDocument(ctx, pathForURI(params.TextDocument.URI), []byte(params.TextDocument.Text)); err != nil {
			return ir.Document{}, err
		}
	}
	key := params.TextDocument.URI
	s.mu.RLock()
	doc, ok := s.docs[key]
	if !ok {
		doc, ok = s.docs[pathForURI(key)]
	}
	s.mu.RUnlock()
	if !ok {
		return ir.Document{Path: pathForURI(key), Version: ir.Version}, nil
	}
	return doc, nil
}
func (s *Server) diagnostics(ctx context.Context, id interface{}, params json.RawMessage) *Response {
	doc, err := s.document(ctx, params)
	if err != nil {
		return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: -32602, Message: err.Error()}}
	}
	all := []ir.Document{doc}
	s.mu.RLock()
	for _, d := range s.docs {
		if d.Path != doc.Path {
			all = append(all, d)
		}
	}
	s.mu.RUnlock()
	sem := semantics.Resolve(semantics.NewMemoryWorkspace(all...))
	out := make([]Diagnostic, 0)
	for _, d := range append(append([]ir.Diagnostic{}, doc.Diagnostics...), sem.Diagnostics...) {
		if err := ctx.Err(); err != nil {
			return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: -32800, Message: "request cancelled"}}
		}
		if d.Path == doc.Path {
			out = append(out, convertDiagnostic(d))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Range.Start.Line < out[j].Range.Start.Line || (out[i].Range.Start.Line == out[j].Range.Start.Line && out[i].Range.Start.Character < out[j].Range.Start.Character)
	})
	return &Response{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{"kind": "full", "items": out}}
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
}
type DocumentSymbol struct {
	Name           string `json:"name"`
	Kind           int    `json:"kind"`
	Range          Range  `json:"range"`
	SelectionRange Range  `json:"selectionRange"`
}

func convertDiagnostic(d ir.Diagnostic) Diagnostic {
	return Diagnostic{Range: Range{Start: Position{d.Start.Line - 1, d.Start.Column - 1}, End: Position{d.End.Line - 1, d.End.Column - 1}}, Severity: map[ir.Severity]int{ir.SeverityError: 1, ir.SeverityWarning: 2, ir.SeverityInfo: 3}[d.Severity], Code: d.Code, Message: d.Message}
}
func convertRange(s ir.SourceSpan) Range {
	return Range{Start: Position{s.Start.Line - 1, s.Start.Column - 1}, End: Position{s.End.Line - 1, s.End.Column - 1}}
}
func (s *Server) symbols(ctx context.Context, id interface{}, params json.RawMessage) *Response {
	doc, err := s.document(ctx, params)
	if err != nil {
		return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: -32602, Message: err.Error()}}
	}
	out := make([]DocumentSymbol, 0, len(doc.Symbols))
	for _, sym := range doc.Symbols {
		if err := ctx.Err(); err != nil {
			return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: -32800, Message: "request cancelled"}}
		}
		r := convertRange(sym.Span)
		kind := 13
		if sym.Kind == ir.SymbolStateDef {
			kind = 5
		}
		if sym.Kind == ir.SymbolCommand {
			kind = 6
		}
		out = append(out, DocumentSymbol{Name: sym.Name, Kind: kind, Range: r, SelectionRange: r})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return &Response{JSONRPC: "2.0", ID: id, Result: out}
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type Hover struct {
	Contents HoverContents `json:"contents"`
	Range    Range         `json:"range"`
}

type HoverContents struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

func (s *Server) positionParams(ctx context.Context, p json.RawMessage) (ir.Document, textDocumentPositionParams, error) {
	var params textDocumentPositionParams
	if err := json.Unmarshal(p, &params); err != nil || params.TextDocument.URI == "" {
		return ir.Document{}, params, errors.New("invalid params")
	}
	doc, err := s.document(ctx, p)
	return doc, params, err
}

func (s *Server) workspaceDocuments(doc ir.Document) []ir.Document {
	out := []ir.Document{doc}
	seen := map[string]bool{doc.Path: true}
	s.mu.RLock()
	for _, d := range s.docs {
		if !seen[d.Path] {
			seen[d.Path] = true
			out = append(out, d)
		}
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func containsPosition(span ir.SourceSpan, pos Position) bool {
	line, col := pos.Line+1, pos.Character+1
	if line < span.Start.Line || line > span.End.Line {
		return false
	}
	if line == span.Start.Line && col < span.Start.Column {
		return false
	}
	if line == span.End.Line && span.End.Column > 0 && col > span.End.Column {
		return false
	}
	return true
}

func symbolAt(doc ir.Document, pos Position) *ir.Symbol {
	for i := range doc.Symbols {
		if containsPosition(doc.Symbols[i].Span, pos) {
			return &doc.Symbols[i]
		}
	}
	return nil
}

func referenceAt(doc ir.Document, pos Position) *ir.Reference {
	for i := range doc.References {
		if containsPosition(doc.References[i].Span, pos) {
			return &doc.References[i]
		}
	}
	return nil
}

func (s *Server) hover(ctx context.Context, id interface{}, params json.RawMessage) *Response {
	doc, p, err := s.positionParams(ctx, params)
	if err != nil {
		return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: -32602, Message: err.Error()}}
	}
	if err := ctx.Err(); err != nil {
		return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: -32800, Message: "request cancelled"}}
	}
	sym := symbolAt(doc, p.Position)
	if sym == nil {
		if ref := referenceAt(doc, p.Position); ref != nil {
			sem := semantics.Resolve(semantics.NewMemoryWorkspace(s.workspaceDocuments(doc)...))
			for _, resolved := range sem.References {
				if resolved.ReferenceID != ref.ID || !resolved.Resolved {
					continue
				}
				for _, d := range s.workspaceDocuments(doc) {
					for i := range d.Symbols {
						if d.Symbols[i].ID == resolved.TargetSymbolID {
							sym = &d.Symbols[i]
							break
						}
					}
				}
			}
		}
	}
	if sym == nil {
		return &Response{JSONRPC: "2.0", ID: id, Result: nil}
	}
	return &Response{JSONRPC: "2.0", ID: id, Result: Hover{
		Contents: HoverContents{Kind: "markdown", Value: "**" + sym.Name + "** (" + string(sym.Kind) + ")"},
		Range:    convertRange(sym.Span),
	}}
}

func (s *Server) definition(ctx context.Context, id interface{}, params json.RawMessage) *Response {
	doc, p, err := s.positionParams(ctx, params)
	if err != nil {
		return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: -32602, Message: err.Error()}}
	}
	if err := ctx.Err(); err != nil {
		return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: -32800, Message: "request cancelled"}}
	}
	target := ""
	if sym := symbolAt(doc, p.Position); sym != nil {
		target = sym.ID
	} else if ref := referenceAt(doc, p.Position); ref != nil {
		for _, resolved := range semantics.Resolve(semantics.NewMemoryWorkspace(s.workspaceDocuments(doc)...)).References {
			if resolved.ReferenceID == ref.ID && resolved.Resolved {
				target = resolved.TargetSymbolID
				break
			}
		}
	}
	out := make([]Location, 0)
	for _, d := range s.workspaceDocuments(doc) {
		for _, sym := range d.Symbols {
			if sym.ID == target {
				out = append(out, Location{URI: uriForPath(d.Path), Range: convertRange(sym.Span)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].URI < out[j].URI || (out[i].URI == out[j].URI && out[i].Range.Start.Line < out[j].Range.Start.Line)
	})
	return &Response{JSONRPC: "2.0", ID: id, Result: out}
}

func (s *Server) references(ctx context.Context, id interface{}, params json.RawMessage) *Response {
	doc, p, err := s.positionParams(ctx, params)
	if err != nil {
		return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: -32602, Message: err.Error()}}
	}
	if err := ctx.Err(); err != nil {
		return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: -32800, Message: "request cancelled"}}
	}
	sem := semantics.Resolve(semantics.NewMemoryWorkspace(s.workspaceDocuments(doc)...))
	target := ""
	targetIdentity := ir.Identity{}
	if sym := symbolAt(doc, p.Position); sym != nil {
		target, targetIdentity = sym.ID, sym.Identity
	} else if ref := referenceAt(doc, p.Position); ref != nil {
		for _, resolved := range sem.References {
			if resolved.ReferenceID == ref.ID && resolved.Resolved {
				target, targetIdentity = resolved.TargetSymbolID, resolved.TargetIdentity
				break
			}
		}
	}
	out := make([]Location, 0)
	for _, resolved := range sem.References {
		if !resolved.Resolved || (target != "" && resolved.TargetSymbolID != target && resolved.TargetIdentity != targetIdentity) {
			continue
		}
		for _, d := range s.workspaceDocuments(doc) {
			if d.Path != resolved.SourcePath {
				continue
			}
			for _, ref := range d.References {
				if ref.ID == resolved.ReferenceID {
					out = append(out, Location{URI: uriForPath(d.Path), Range: convertRange(ref.Span)})
				}
			}
		}
	}
	if p.Context.IncludeDeclaration && target != "" {
		for _, d := range s.workspaceDocuments(doc) {
			for _, sym := range d.Symbols {
				if sym.ID == target {
					out = append(out, Location{URI: uriForPath(d.Path), Range: convertRange(sym.Span)})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].URI < out[j].URI || (out[i].URI == out[j].URI && out[i].Range.Start.Line < out[j].Range.Start.Line)
	})
	return &Response{JSONRPC: "2.0", ID: id, Result: out}
}

func uriForPath(path string) string { return "file://" + filepath.ToSlash(path) }
func pathForURI(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return raw
	}
	p := u.Path
	if p == "" {
		p = u.Opaque
	}
	if p == "" {
		p = u.Host
	}
	if len(p) > 2 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}
