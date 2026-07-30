package patch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// PlanVersion identifies the serialized mutation contract.
const PlanVersion = "1"

type PatchPlan struct {
	Version           string   `json:"version"`
	OperationID       string   `json:"operationId"`
	WorkspaceRoot     string   `json:"workspaceRoot"`
	WorkspaceIdentity string   `json:"workspaceIdentity"`
	Profile           string   `json:"profile,omitempty"`
	InputSnapshot     string   `json:"inputSnapshot"`
	Edits             []Edit   `json:"edits"`
	Postconditions    []string `json:"postconditions,omitempty"`
}
type TransactionError struct {
	Phase string
	Err   error
}

func (e *TransactionError) Error() string {
	return "patch transaction " + e.Phase + ": " + e.Err.Error()
}
func (e *TransactionError) Unwrap() error { return e.Err }

// ApplyAtomic stages all files and retains backups until every replacement succeeds.
func ApplyAtomic(root string, p Patch) (ApplyResult, error) {
	preview, err := prepare(root, p)
	if err != nil {
		return ApplyResult{}, err
	}
	type backup struct {
		path, temp string
		mode       os.FileMode
	}
	backups := make([]backup, 0, len(preview.Files))
	journal := Journal{Phase: "staging", Root: root}
	writeJournal := func() error { b, _ := json.Marshal(journal); return os.WriteFile(journalPath(root), b, 0600) }
	if err := writeJournal(); err != nil {
		return ApplyResult{}, &TransactionError{"journal", err}
	}
	rollback := func() {
		for _, b := range backups {
			_ = os.Remove(b.path)
			_ = os.Rename(b.temp, b.path)
		}
	}
	for _, f := range preview.Files {
		path, e := resolvePath(root, f.Path)
		if e != nil {
			rollback()
			return ApplyResult{}, &TransactionError{"validate", e}
		}
		info, e := os.Stat(path)
		if e != nil {
			rollback()
			return ApplyResult{}, &TransactionError{"validate", e}
		}
		tmp, e := os.CreateTemp(filepath.Dir(path), ".ikm-patch-*")
		if e != nil {
			rollback()
			return ApplyResult{}, &TransactionError{"stage", e}
		}
		name := tmp.Name()
		if _, e = tmp.Write(f.Bytes); e == nil {
			e = tmp.Sync()
		}
		if ce := tmp.Close(); e == nil {
			e = ce
		}
		if e != nil {
			_ = os.Remove(name)
			rollback()
			return ApplyResult{}, &TransactionError{"stage", e}
		}
		if e = os.Chmod(name, info.Mode()); e != nil {
			_ = os.Remove(name)
			rollback()
			return ApplyResult{}, &TransactionError{"stage", e}
		}
		backupPath := path + ".ikm-backup"
		_ = os.Remove(backupPath)
		if e = os.Rename(path, backupPath); e != nil {
			_ = os.Remove(name)
			rollback()
			return ApplyResult{}, &TransactionError{"backup", e}
		}
		backups = append(backups, backup{path: path, temp: backupPath, mode: info.Mode()})
		journal.Phase = "replacing"
		journal.Backups = append(journal.Backups, backupPath)
		if e = writeJournal(); e != nil {
			rollback()
			return ApplyResult{}, &TransactionError{"journal", e}
		}
		if e = os.Rename(name, path); e != nil {
			rollback()
			return ApplyResult{}, &TransactionError{"replace", e}
		}
	}
	for _, b := range backups {
		_ = os.Remove(b.temp)
	}
	_ = os.Remove(journalPath(root))
	return ApplyResult{Files: preview.Files}, nil
}

func (p PatchPlan) JSON() ([]byte, error) {
	if p.Version == "" {
		p.Version = PlanVersion
	}
	sort.SliceStable(p.Edits, func(i, j int) bool {
		if p.Edits[i].Path != p.Edits[j].Path {
			return p.Edits[i].Path < p.Edits[j].Path
		}
		return p.Edits[i].Span.ByteStart < p.Edits[j].Span.ByteStart
	})
	return json.Marshal(p)
}
func UnifiedDiff(root string, preview PreviewResult) string {
	var b strings.Builder
	for _, f := range preview.Files {
		fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n@@\n", f.Path, f.Path)
		old, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "-%s\n+%s\n", strings.TrimSuffix(string(old), "\n"), strings.TrimSuffix(string(f.Bytes), "\n"))
	}
	return b.String()
}

func (p PatchPlan) Validate() error {
	if p.Version == "" || p.Version != PlanVersion {
		return errors.New("patch: unsupported plan version")
	}
	if strings.TrimSpace(p.InputSnapshot) == "" {
		return errors.New("patch: missing input snapshot")
	}
	if len(p.Edits) == 0 {
		return errors.New("patch: plan has no edits")
	}
	for _, e := range p.Edits {
		if e.ContentHash == "" || e.IdentityContract == "" || e.Span.ByteStart < 0 || e.Span.ByteEnd < e.Span.ByteStart {
			return errors.New("patch: edit lacks hash, identity, or valid span")
		}
	}
	return nil
}

type Journal struct {
	Phase   string   `json:"phase"`
	Root    string   `json:"root"`
	Backups []string `json:"backups"`
}

func journalPath(root string) string { return filepath.Join(root, ".ikm-patch-journal.json") }
func Recover(root string) error {
	b, e := os.ReadFile(journalPath(root))
	if e != nil {
		return e
	}
	var j Journal
	if e = json.Unmarshal(b, &j); e != nil {
		return e
	}
	for _, backup := range j.Backups {
		target := strings.TrimSuffix(backup, ".ikm-backup")
		if _, e = os.Stat(backup); e == nil {
			_ = os.Remove(target)
			if e = os.Rename(backup, target); e != nil {
				return e
			}
		}
	}
	return os.Remove(journalPath(root))
}
