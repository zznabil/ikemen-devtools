package document

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/parser"
)

var (
	ErrNilSource      = errors.New("document: source is nil")
	ErrBlankPath      = errors.New("document: path is blank")
	ErrInvalidVersion = errors.New("document: invalid parser document version")
	ErrNilParsedDoc   = errors.New("document: parser returned nil document")
)

// Snapshot stores an immutable view of a parsed source file.
type Snapshot struct {
	raw              []byte
	lineEndings      []string
	normalizedPath   string
	fileType         string
	contentHash      string
	version          string
	identityContract string
	parsedDoc        *ir.Document
}

// NewSnapshot constructs a deterministic snapshot from path and raw bytes.
func NewSnapshot(path string, source []byte) (*Snapshot, error) {
	if source == nil {
		return nil, ErrNilSource
	}

	normalizedPath := normalizePath(path)
	if strings.TrimSpace(path) == "" {
		return nil, ErrBlankPath
	}
	if normalizedPath == "" {
		return nil, ErrBlankPath
	}

	doc := parser.Parse(normalizedPath, string(source))
	if doc == nil {
		return nil, ErrNilParsedDoc
	}
	if doc.Version != ir.IdentityContractVersion {
		return nil, fmt.Errorf("%w: expected %q got %q", ErrInvalidVersion, ir.IdentityContractVersion, doc.Version)
	}

	h := sha256.Sum256(source)
	s := &Snapshot{
		raw:              append([]byte(nil), source...),
		lineEndings:      lineEndings(source),
		normalizedPath:   normalizedPath,
		fileType:         doc.FileType,
		contentHash:      hex.EncodeToString(h[:]),
		version:          doc.Version,
		identityContract: ir.IdentityContractVersion,
		parsedDoc:        doc,
	}

	return s, nil
}

// MustSnapshot creates a snapshot and panics on invalid input.
func MustSnapshot(path string, source []byte) *Snapshot {
	snapshot, err := NewSnapshot(path, source)
	if err != nil {
		panic(err)
	}
	return snapshot
}

// NormalizedPath returns the deterministic normalized document path.
func (s *Snapshot) NormalizedPath() string {
	if s == nil {
		return ""
	}
	return s.normalizedPath
}

// FileType returns the parser-derived document kind.
func (s *Snapshot) FileType() string {
	if s == nil {
		return ""
	}
	return s.fileType
}

// Hash returns the SHA-256 digest for the source bytes in hex.
func (s *Snapshot) Hash() string {
	if s == nil {
		return ""
	}
	return s.contentHash
}

// Version returns the parser document version stored in the snapshot.
func (s *Snapshot) Version() string {
	if s == nil {
		return ""
	}
	return s.version
}

// IdentityContract returns the identity contract version stored in the snapshot.
func (s *Snapshot) IdentityContract() string {
	if s == nil {
		return ""
	}
	return s.identityContract
}

// ParsedDocument returns the semantic document produced during snapshot creation.
func (s *Snapshot) ParsedDocument() *ir.Document {
	if s == nil {
		return nil
	}
	return s.parsedDoc
}

// Bytes returns a copy of the original bytes.
func (s *Snapshot) Bytes() []byte {
	if s == nil {
		return nil
	}
	return append([]byte(nil), s.raw...)
}

// LineEndings returns preserved per-line endings in declaration order.
func (s *Snapshot) LineEndings() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.lineEndings...)
}

// Equal compares two snapshots for deterministic equivalence.
func (s *Snapshot) Equal(other *Snapshot) bool {
	if s == nil || other == nil {
		return s == other
	}
	if s.normalizedPath != other.normalizedPath {
		return false
	}
	if s.fileType != other.fileType {
		return false
	}
	if s.contentHash != other.contentHash {
		return false
	}
	if s.version != other.version {
		return false
	}
	if s.identityContract != other.identityContract {
		return false
	}
	if s.parsedDoc == nil || other.parsedDoc == nil {
		if s.parsedDoc != other.parsedDoc {
			return false
		}
		return true
	}
	if len(s.raw) != len(other.raw) {
		return false
	}
	for i := range s.raw {
		if s.raw[i] != other.raw[i] {
			return false
		}
	}
	if len(s.lineEndings) != len(other.lineEndings) {
		return false
	}
	for i := range s.lineEndings {
		if s.lineEndings[i] != other.lineEndings[i] {
			return false
		}
	}
	return s.parsedDoc.Version == other.parsedDoc.Version &&
		s.parsedDoc.Path == other.parsedDoc.Path &&
		s.parsedDoc.FileType == other.parsedDoc.FileType
}

func lineEndings(source []byte) []string {
	lines := splitLines(source)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, line.ending)
	}
	return out
}

type lineInfo struct {
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
	_ = lineNo

	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\r':
			if i+1 < len(source) && source[i+1] == '\n' {
				lines = append(lines, lineInfo{text: string(source[start:i]), ending: "\r\n"})
				i++
				lineNo++
				start = i + 1
				continue
			}
			lines = append(lines, lineInfo{text: string(source[start:i]), ending: "\r"})
			lineNo++
			start = i + 1
		case '\n':
			lines = append(lines, lineInfo{text: string(source[start:i]), ending: "\n"})
			lineNo++
			start = i + 1
		}
	}

	if start <= len(source)-1 {
		lines = append(lines, lineInfo{text: string(source[start:]), ending: ""})
	} else if start == len(source) {
		lines = append(lines, lineInfo{text: "", ending: ""})
	}

	return lines
}

func normalizePath(path string) string {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	return strings.TrimPrefix(clean, "./")
}
