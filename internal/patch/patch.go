package patch

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
)

var (
	ErrPathEscape       = errors.New("patch: path escapes workspace")
	ErrStaleHash        = errors.New("patch: stale content hash")
	ErrOverlap          = errors.New("patch: overlapping edits")
	ErrOutOfRange       = errors.New("patch: span out of range")
	ErrOldText          = errors.New("patch: expected old text mismatch")
	ErrIdentityContract = errors.New("patch: identity contract mismatch")
)

type Span struct {
	ByteStart int
	ByteEnd   int
	Start     ir.SourcePosition
	End       ir.SourcePosition
}

type Edit struct {
	Path             string
	ContentHash      string
	IdentityContract string
	Identity         ir.Identity
	Span             Span
	OldText          string
	NewText          string
}

type Patch struct{ Edits []Edit }

type FilePreview struct {
	Path    string
	OldHash string
	NewHash string
	Bytes   []byte
}

type PreviewResult struct{ Files []FilePreview }
type ApplyResult struct{ Files []FilePreview }

// PreviewPatch validates edits and deterministically returns resulting file bytes without writing.
func PreviewPatch(root string, p Patch) (PreviewResult, error) { return prepare(root, p) }

// Preview is an alias for PreviewPatch.
func Preview(root string, p Patch) (PreviewResult, error) { return PreviewPatch(root, p) }

// Preview computes a guarded patch preview without changing files.
func (p Patch) Preview(root string) (PreviewResult, error) { return PreviewPatch(root, p) }

// ApplyPatch validates and writes a patch, returning the resulting content hashes.
func ApplyPatch(root string, p Patch) (ApplyResult, error) {
	result, err := prepare(root, p)
	if err != nil {
		return ApplyResult{}, err
	}
	for _, file := range result.Files {
		path, err := resolvePath(root, file.Path)
		if err != nil {
			return ApplyResult{}, err
		}
		if err := os.WriteFile(path, file.Bytes, 0o644); err != nil {
			return ApplyResult{}, fmt.Errorf("patch: write %s: %w", file.Path, err)
		}
	}
	return ApplyResult{Files: result.Files}, nil
}

// Apply is an alias for ApplyPatch.
func Apply(root string, p Patch) (ApplyResult, error)  { return ApplyPatch(root, p) }
func (p Patch) Apply(root string) (ApplyResult, error) { return ApplyPatch(root, p) }

func prepare(root string, p Patch) (PreviewResult, error) {
	if strings.TrimSpace(root) == "" {
		return PreviewResult{}, ErrPathEscape
	}
	groups := make(map[string][]Edit)
	for _, edit := range p.Edits {
		path, err := canonicalRelative(edit.Path)
		if err != nil {
			return PreviewResult{}, err
		}
		groups[path] = append(groups[path], edit)
	}
	paths := make([]string, 0, len(groups))
	for path := range groups {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out := PreviewResult{Files: make([]FilePreview, 0, len(paths))}
	for _, path := range paths {
		abs, err := resolvePath(root, path)
		if err != nil {
			return PreviewResult{}, err
		}
		original, err := os.ReadFile(abs)
		if err != nil {
			return PreviewResult{}, fmt.Errorf("patch: read %s: %w", path, err)
		}
		hash := digest(original)
		edits := groups[path]
		for i := range edits {
			canonical, err := canonicalRelative(edits[i].Path)
			if err != nil || canonical != path {
				return PreviewResult{}, ErrPathEscape
			}
			if edits[i].ContentHash == "" || edits[i].ContentHash != hash {
				return PreviewResult{}, fmt.Errorf("%w for %s", ErrStaleHash, path)
			}
			if edits[i].IdentityContract != ir.IdentityContractVersion || edits[i].Identity.ContractVersion != "" && edits[i].Identity.ContractVersion != ir.IdentityContractVersion {
				return PreviewResult{}, fmt.Errorf("%w for %s", ErrIdentityContract, path)
			}
			if err := validateSpan(edits[i].Span, original); err != nil {
				return PreviewResult{}, fmt.Errorf("%s: %w", path, err)
			}
			start, end := edits[i].Span.ByteStart, edits[i].Span.ByteEnd
			if string(original[start:end]) != edits[i].OldText {
				return PreviewResult{}, fmt.Errorf("%w for %s", ErrOldText, path)
			}
		}
		sort.SliceStable(edits, func(i, j int) bool {
			a, b := edits[i], edits[j]
			if a.Span.ByteStart != b.Span.ByteStart {
				return a.Span.ByteStart < b.Span.ByteStart
			}
			if a.Span.ByteEnd != b.Span.ByteEnd {
				return a.Span.ByteEnd < b.Span.ByteEnd
			}
			if a.NewText != b.NewText {
				return a.NewText < b.NewText
			}
			return a.OldText < b.OldText
		})
		for i := 1; i < len(edits); i++ {
			if edits[i-1].Span.ByteEnd > edits[i].Span.ByteStart {
				return PreviewResult{}, fmt.Errorf("%w for %s", ErrOverlap, path)
			}
		}
		result := append([]byte(nil), original...)
		for i := len(edits) - 1; i >= 0; i-- {
			s, e := edits[i].Span.ByteStart, edits[i].Span.ByteEnd
			result = append(append(append([]byte{}, result[:s]...), []byte(edits[i].NewText)...), result[e:]...)
		}
		out.Files = append(out.Files, FilePreview{Path: path, OldHash: hash, NewHash: digest(result), Bytes: result})
	}
	return out, nil
}

func validateSpan(s Span, data []byte) error {
	if s.ByteStart < 0 || s.ByteEnd < s.ByteStart || s.ByteEnd > len(data) {
		return ErrOutOfRange
	}
	if (s.Start != ir.SourcePosition{} || s.End != ir.SourcePosition{}) {
		if s.Start.Line < 1 || s.Start.Column < 1 || s.End.Line < s.Start.Line || s.End.Column < 1 {
			return ErrOutOfRange
		}
		start, end, ok := lineByteOffsets(data, s.Start, s.End)
		if !ok || start != s.ByteStart || end != s.ByteEnd {
			return ErrOutOfRange
		}
	}
	return nil
}

func lineByteOffsets(data []byte, start, end ir.SourcePosition) (int, int, bool) {
	starts := []int{0}
	for i, b := range data {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	if start.Line > len(starts) || end.Line > len(starts) {
		return 0, 0, false
	}
	toByte := func(pos ir.SourcePosition) (int, bool) {
		base := starts[pos.Line-1]
		lineEnd := len(data)
		for i := base; i < len(data); i++ {
			if data[i] == '\r' || data[i] == '\n' {
				lineEnd = i
				break
			}
		}
		col := pos.Column - 1
		if col < 0 || base+col > lineEnd {
			return 0, false
		}
		if base+col < lineEnd && !utf8.RuneStart(data[base+col]) {
			return 0, false
		}
		return base + col, true
	}
	a, ok := toByte(start)
	if !ok {
		return 0, 0, false
	}
	b, ok := toByte(end)
	return a, b, ok
}

func digest(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }

func canonicalRelative(path string) (string, error) {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" || filepath.IsAbs(filepath.FromSlash(path)) {
		return "", ErrPathEscape
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", ErrPathEscape
	}
	return clean, nil
}

func resolvePath(root, rel string) (string, error) {
	rel, err := canonicalRelative(rel)
	if err != nil {
		return "", err
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(base, filepath.FromSlash(rel))
	resolved := candidate
	if r, e := filepath.EvalSymlinks(candidate); e == nil {
		resolved = r
	}
	inside, err := filepath.Rel(base, resolved)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) || filepath.IsAbs(inside) {
		return "", ErrPathEscape
	}
	return candidate, nil
}
