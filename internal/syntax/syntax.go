package syntax

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/parser/adapter"
)

// Provider parses syntax documents and exposes a lossless source snapshot.
//
// It is intentionally minimal and deterministic to support incremental experiments.
type Provider interface {
	Parse(path string, source []byte) *ParsedDocument
}

// StandardLibraryProvider is the concrete in-tree syntax provider.
type StandardLibraryProvider struct{}

// StandardProvider is kept as a compatibility alias.
type StandardProvider = StandardLibraryProvider

// ParsedDocument is the unified output for syntax extraction and semantic parsing.
type ParsedDocument struct {
	Path     string
	Source   *SourceSnapshot
	Tokens   []Token
	Sections []Section
	Document *ir.Document
}

// SourceSnapshot is preserved for compatibility with existing callers.
type SourceSnapshot struct {
	raw              []byte
	lineEndings      []string
	normalizedPath   string
	fileType         string
	contentHash      string
	version          string
	identityContract string
	parsedDoc        *ir.Document
}

type TokenKind = adapter.TokenKind

type Token = adapter.Token
type Section = adapter.Section

const (
	TokenSection   = adapter.TokenSection
	TokenKeyValue  = adapter.TokenKeyValue
	TokenComment   = adapter.TokenComment
	TokenBlank     = adapter.TokenBlank
	TokenMalformed = adapter.TokenMalformed
)

func NewStandardLibraryProvider() Provider {
	return &StandardLibraryProvider{}
}

// NewStandardProvider returns the default concrete provider.
// It is retained for compatibility.
func NewStandardProvider() Provider {
	return NewStandardLibraryProvider()
}

// Parse returns a document with a deterministic source snapshot and parsed semantic IR.
func (StandardLibraryProvider) Parse(path string, source []byte) *ParsedDocument {
	snapshot := newSourceSnapshot(path, source)
	if snapshot == nil {
		return nil
	}
	tokens, sections := parseSyntaxLines(source)
	parsedDoc := adapter.FromSyntax(snapshot.NormalizedPath(), sections, tokens)
	snapshot.parsedDoc = parsedDoc
	return &ParsedDocument{
		Path:     snapshot.NormalizedPath(),
		Source:   snapshot,
		Tokens:   tokens,
		Sections: sections,
		Document: parsedDoc,
	}
}

// RoundTrip returns the original bytes used to create the snapshot.
func (doc *ParsedDocument) RoundTrip() []byte {
	if doc == nil || doc.Source == nil {
		return nil
	}
	return doc.Source.Bytes()
}

func (s *SourceSnapshot) NormalizedPath() string {
	if s == nil {
		return ""
	}
	return s.normalizedPath
}

func (s *SourceSnapshot) FileType() string {
	if s == nil {
		return ""
	}
	return s.fileType
}

func (s *SourceSnapshot) Bytes() []byte {
	if s == nil {
		return nil
	}
	return append([]byte(nil), s.raw...)
}

func (s *SourceSnapshot) LineEndings() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.lineEndings...)
}

func (s *SourceSnapshot) Hash() string {
	if s == nil {
		return ""
	}
	return s.contentHash
}

func (s *SourceSnapshot) Version() string {
	if s == nil {
		return ""
	}
	return s.version
}

func (s *SourceSnapshot) IdentityContract() string {
	if s == nil {
		return ""
	}
	return s.identityContract
}

func (s *SourceSnapshot) ParsedDocument() *ir.Document {
	if s == nil {
		return nil
	}
	return s.parsedDoc
}

func newSourceSnapshot(path string, source []byte) *SourceSnapshot {
	if source == nil {
		return nil
	}
	normalizedPath := normalizePath(path)

	h := sha256.Sum256(source)
	lines := splitLines(source)
	endings := make([]string, 0, len(lines))
	for _, line := range lines {
		endings = append(endings, line.ending)
	}
	fileType := detectFileType(normalizedPath)
	return &SourceSnapshot{
		raw:              append([]byte(nil), source...),
		lineEndings:      endings,
		normalizedPath:   normalizedPath,
		fileType:         fileType,
		contentHash:      hex.EncodeToString(h[:]),
		version:          ir.IdentityContractVersion,
		identityContract: ir.IdentityContractVersion,
	}
}

func parseSyntaxLines(source []byte) ([]Token, []Section) {
	lines := splitLines(source)
	tokens := make([]Token, 0, len(lines))
	sections := make([]Section, 0)

	for _, line := range lines {
		codeText, commentText, commentIdx := splitLine(line.text)
		trimmedCode := strings.TrimSpace(codeText)

		if trimmedCode == "" {
			if commentText != "" {
				tokens = append(tokens, makeCommentToken(line.no, line.text, commentText, commentIdx))
			} else {
				tokens = append(tokens, Token{Kind: TokenBlank, Span: tokenSpan(line.text, "", line.no)})
			}
			continue
		}

		if header, ok := parseSectionHeader(trimmedCode); ok {
			header = strings.TrimSpace(header)
			span := tokenSpan(line.text, strings.TrimSpace(trimmedCode), line.no)
			sections = append(sections, Section{Header: header, Kind: sectionKind(header), Span: span})
			tokens = append(tokens, Token{Kind: TokenSection, Span: span, Text: header})
			if commentText != "" {
				tokens = append(tokens, makeCommentToken(line.no, line.text, commentText, commentIdx))
			}
			continue
		}

		key, value, hasEq := splitKeyValue(trimmedCode)
		lineSpan := tokenSpan(line.text, strings.TrimSpace(trimmedCode), line.no)
		if !hasEq || key == "" {
			tokens = append(tokens, Token{Kind: TokenMalformed, Text: strings.TrimSpace(trimmedCode), Span: lineSpan})
			if commentText != "" {
				tokens = append(tokens, makeCommentToken(line.no, line.text, commentText, commentIdx))
			}
			continue
		}

		tokens = append(tokens, Token{Kind: TokenKeyValue, Key: key, Value: value, Text: strings.TrimSpace(trimmedCode), Span: lineSpan})
		if commentText != "" {
			tokens = append(tokens, makeCommentToken(line.no, line.text, commentText, commentIdx))
		}
	}

	return tokens, sections
}

func makeCommentToken(lineNo int, raw, comment string, commentIdx int) Token {
	text := strings.TrimSpace(comment)
	if text == "" {
		text = ";"
	}
	start := commentIdx + 1
	if text != ";" && commentIdx >= 0 {
		offset := strings.Index(raw[commentIdx+1:], text)
		if offset >= 0 {
			start = commentIdx + 1 + offset + 1
		}
	}
	return Token{
		Kind: TokenComment,
		Span: tokenSpanAt(raw, text, lineNo, start),
		Text: text,
	}
}

func tokenSpan(raw, token string, lineNo int) ir.SourceSpan {
	return tokenSpanAt(raw, token, lineNo, -1)
}

func tokenSpanAt(raw, token string, lineNo, startColumn int) ir.SourceSpan {
	start := startColumn
	if start <= 0 {
		start = 1
		if token != "" {
			if idx := strings.Index(raw, token); idx >= 0 {
				start = idx + 1
			}
		} else {
			trimmed := strings.TrimSpace(raw)
			if trimmed == "" {
				start = 1
			} else if idx := strings.Index(raw, trimmed); idx >= 0 {
				start = idx + 1
			}
		}
	}
	end := start + len(token)
	if token == "" {
		t := strings.TrimRight(raw, "\r")
		if t == "" {
			end = start
		} else {
			end = start + len(t)
		}
	}
	if end < start {
		end = start
	}
	return ir.SourceSpan{
		Start: ir.SourcePosition{Line: lineNo, Column: start},
		End:   ir.SourcePosition{Line: lineNo, Column: end + 1},
	}
}

type lineInfo struct {
	no     int
	text   string
	ending string
}

func splitLines(source []byte) []lineInfo {
	if len(source) == 0 {
		return nil
	}

	lines := make([]lineInfo, 0)
	start := 0
	lineNo := 1

	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\r':
			if i+1 < len(source) && source[i+1] == '\n' {
				lines = append(lines, lineInfo{no: lineNo, text: string(source[start:i]), ending: "\r\n"})
				i++
				lineNo++
				start = i + 1
				continue
			}
			lines = append(lines, lineInfo{no: lineNo, text: string(source[start:i]), ending: "\r"})
			lineNo++
			start = i + 1
		case '\n':
			lines = append(lines, lineInfo{no: lineNo, text: string(source[start:i]), ending: "\n"})
			lineNo++
			start = i + 1
		}
	}

	if start <= len(source)-1 {
		lines = append(lines, lineInfo{no: lineNo, text: string(source[start:]), ending: ""})
	} else if start == len(source) {
		lines = append(lines, lineInfo{no: lineNo, text: "", ending: ""})
	}

	return lines
}

func splitLine(raw string) (string, string, int) {
	inQuote := false
	escaped := false
	commentIdx := -1
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		if ch == ';' {
			commentIdx = i
			break
		}
		if ch == '/' && i+1 < len(raw) && raw[i+1] == '/' && strings.TrimSpace(raw[:i]) == "" {
			commentIdx = i
			break
		}
	}

	if commentIdx >= 0 {
		delimiterLen := 1
		if strings.HasPrefix(raw[commentIdx:], "//") {
			delimiterLen = 2
		}
		return strings.TrimRight(raw[:commentIdx], " 	"), strings.TrimSpace(raw[commentIdx+delimiterLen:]), commentIdx
	}

	return strings.TrimRight(raw, " 	"), "", -1
}

func splitKeyValue(line string) (string, string, bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	if key == "" {
		return "", value, true
	}
	return key, value, true
}

func parseSectionHeader(line string) (string, bool) {
	trim := strings.TrimSpace(line)
	if len(trim) < 2 || trim == "[" {
		return "", false
	}
	if !strings.HasPrefix(trim, "[") || !strings.HasSuffix(trim, "]") {
		return "", false
	}
	return strings.TrimSpace(trim[1 : len(trim)-1]), true
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func detectFileType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".def":
		return "def"
	case ".cns":
		return "cns"
	case ".cmd":
		return "cmd"
	case ".st":
		return "st"
	default:
		return "text"
	}
}

func sectionKind(header string) ir.SectionKind {
	parts := strings.Fields(strings.ToLower(header))
	if len(parts) == 0 {
		return ir.SectionOther
	}
	switch parts[0] {
	case "statedef":
		return ir.SectionStatedef
	case "state":
		return ir.SectionState
	case "command":
		return ir.SectionCommand
	default:
		return ir.SectionOther
	}
}
