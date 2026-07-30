// Package ecosystem provides deterministic, read-only analysis of select.def
// rosters and character DEF [Files] manifests.
package ecosystem

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/parser"
	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
)

const (
	StatusResolved  = "resolved"
	StatusMissing   = "missing"
	StatusDuplicate = "duplicate"
)

type Entry struct {
	Section      string        `json:"section"`
	Index        int           `json:"index"`
	DeclaredPath string        `json:"declaredPath"`
	ResolvedPath string        `json:"resolvedPath,omitempty"`
	Options      []string      `json:"options,omitempty"`
	Status       string        `json:"status"`
	Span         ir.SourceSpan `json:"span"`
}

type SelectReport struct {
	Path        string          `json:"path"`
	Characters  []Entry         `json:"characters"`
	Stages      []Entry         `json:"stages"`
	Diagnostics []ir.Diagnostic `json:"diagnostics"`
}

type ManifestReport struct {
	Path        string          `json:"path"`
	Files       []Entry         `json:"files"`
	Diagnostics []ir.Diagnostic `json:"diagnostics"`
}

func (r SelectReport) JSONString() string   { b, _ := json.Marshal(r); return string(b) }
func (r ManifestReport) JSONString() string { b, _ := json.Marshal(r); return string(b) }

func AnalyzeSelect(path string, p profile.CompatibilityProfile) SelectReport {
	if p.Name == "" {
		p = profile.NewStrictPortableProfile("")
	}
	r := SelectReport{Path: cleanAbs(path), Characters: []Entry{}, Stages: []Entry{}, Diagnostics: []ir.Diagnostic{}}
	if strings.TrimSpace(path) == "" {
		r.Path = ""
		r.Diagnostics = append(r.Diagnostics, diag("", "missing-select", ir.SeverityError, "select path is empty"))
		return r
	}
	data, err := os.ReadFile(r.Path)
	if err != nil {
		r.Diagnostics = append(r.Diagnostics, diag(r.Path, "select-read-error", ir.SeverityError, err.Error()))
		return r
	}
	seenChars, seenStages := map[string]bool{}, map[string]bool{}
	section := ""
	seenCharsSection := false
	for i, raw := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line := i + 1
		code := strings.TrimSpace(splitComment(raw))
		if code == "" {
			continue
		}
		if isHeader(code) {
			section = strings.TrimSpace(code[1 : len(code)-1])
			if strings.EqualFold(section, "characters") {
				seenCharsSection = true
			}
			continue
		}
		if !isRosterSection(section) || ignorable(code) {
			continue
		}
		fields := csv(code)
		if len(fields) == 0 {
			continue
		}
		selected := -1
		for j, f := range fields {
			if isDef(strings.TrimSpace(f)) {
				selected = j
				break
			}
		}
		if selected < 0 {
			r.Diagnostics = append(r.Diagnostics, lineDiag(r.Path, line, "invalid-roster-entry", ir.SeverityWarning, "roster line has no .def path"))
			continue
		}
		declared := strings.TrimSpace(fields[selected])
		declared = unquote(declared)
		resolved := p.ResolveSourcePath(filepath.Dir(r.Path), declared)
		entry := Entry{Section: section, Index: line, DeclaredPath: declared, ResolvedPath: cleanPath(resolved), Options: []string{}, Status: StatusResolved, Span: lineSpan(line)}
		for j, f := range fields {
			if j != selected && strings.TrimSpace(f) != "" {
				entry.Options = append(entry.Options, strings.TrimSpace(f))
			}
		}
		key := p.DedupKey(resolved)
		seen := seenChars
		target := &r.Characters
		if strings.EqualFold(section, "stages") || strings.EqualFold(section, "extrastages") {
			seen = seenStages
			target = &r.Stages
		}
		if seen[key] {
			entry.Status = StatusDuplicate
			r.Diagnostics = append(r.Diagnostics, lineDiag(r.Path, line, "duplicate-entry", ir.SeverityWarning, "entry resolves to a previously declared path"))
		} else if resolved == "" || !exists(resolved) {
			entry.Status = StatusMissing
			r.Diagnostics = append(r.Diagnostics, lineDiag(r.Path, line, "missing-entry", ir.SeverityError, "declared path does not resolve to a file"))
		} else {
			seen[key] = true
		}
		*target = append(*target, entry)
	}
	if !seenCharsSection {
		r.Diagnostics = append(r.Diagnostics, diag(r.Path, "missing-characters-section", ir.SeverityWarning, "select file has no [Characters] section"))
	}
	return r
}

func AnalyzeCharacterDEF(path string, p profile.CompatibilityProfile) ManifestReport {
	if p.Name == "" {
		p = profile.NewStrictPortableProfile("")
	}
	r := ManifestReport{Path: cleanAbs(path), Files: []Entry{}, Diagnostics: []ir.Diagnostic{}}
	if strings.TrimSpace(path) == "" {
		r.Path = ""
		r.Diagnostics = append(r.Diagnostics, diag("", "missing-def", ir.SeverityError, "DEF path is empty"))
		return r
	}
	data, err := os.ReadFile(r.Path)
	if err != nil {
		r.Diagnostics = append(r.Diagnostics, diag(r.Path, "def-read-error", ir.SeverityError, err.Error()))
		return r
	}
	doc := parser.Parse(r.Path, string(data))
	if doc == nil {
		r.Diagnostics = append(r.Diagnostics, diag(r.Path, "def-parse-error", ir.SeverityError, "unable to parse DEF"))
		return r
	}
	seen := map[string]bool{}
	index := 0
	for _, sec := range doc.Sections {
		if !strings.EqualFold(strings.TrimSpace(sec.Header), "files") {
			continue
		}
		for _, line := range sec.Lines {
			if line.Kind != ir.SourceLineKeyValue || !manifestKey(line.Key) {
				continue
			}
			index++
			declared := p.NormalizeSourceValue(line.Value)
			resolved := p.ResolveSourcePath(filepath.Dir(r.Path), declared)
			e := Entry{Section: "Files", Index: index, DeclaredPath: declared, ResolvedPath: cleanPath(resolved), Status: StatusResolved, Span: line.Span}
			key := p.DedupKey(resolved)
			if seen[key] {
				e.Status = StatusDuplicate
				d := diag(r.Path, "duplicate-manifest-entry", ir.SeverityWarning, "manifest path resolves to a previously declared file")
				d.Start, d.End = line.Span.Start, line.Span.End
				r.Diagnostics = append(r.Diagnostics, d)
			} else if resolved == "" || !exists(resolved) {
				e.Status = StatusMissing
				d := diag(r.Path, "missing-manifest-file", ir.SeverityError, "declared manifest path does not resolve to a file")
				d.Start, d.End = line.Span.Start, line.Span.End
				r.Diagnostics = append(r.Diagnostics, d)
			} else {
				seen[key] = true
			}
			r.Files = append(r.Files, e)
		}
	}
	return r
}

func manifestKey(k string) bool {
	k = strings.ToLower(strings.TrimSpace(k))
	if k == "cmd" || k == "cns" || k == "st" || k == "stcommon" {
		return true
	}
	if strings.HasPrefix(k, "st") {
		_, err := strconv.Atoi(k[2:])
		return k[2:] != "" && err == nil
	}
	return false
}

func isRosterSection(s string) bool {
	return strings.EqualFold(s, "characters") || strings.EqualFold(s, "stages") || strings.EqualFold(s, "extrastages")
}
func isDef(s string) bool {
	return strings.HasSuffix(strings.ToLower(unquote(strings.TrimSpace(s))), ".def")
}
func ignorable(s string) bool {
	l := strings.ToLower(strings.TrimSpace(s))
	return l == "randomselect" || strings.HasPrefix(l, ";") || strings.HasPrefix(l, "#") || separator(l)
}
func separator(s string) bool {
	if len(s) < 3 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune(" -*=_ .", c) {
			return false
		}
	}
	return true
}
func csv(s string) []string {
	var out []string
	start, quote := 0, byte(0)
	for i := 0; i < len(s); i++ {
		if quote != 0 {
			if s[i] == quote {
				quote = 0
			}
		} else if s[i] == '\'' || s[i] == '"' {
			quote = s[i]
		} else if s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
func unquote(s string) string {
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}
func splitComment(s string) string {
	for i, c := range s {
		if c == ';' {
			return s[:i]
		}
	}
	return s
}
func isHeader(s string) bool { return len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']' }
func cleanAbs(s string) string {
	s = filepath.Clean(strings.TrimSpace(s))
	if a, err := filepath.Abs(s); err == nil {
		return a
	}
	return s
}
func cleanPath(s string) string {
	if s == "" {
		return ""
	}
	return cleanAbs(s)
}
func exists(s string) bool { st, err := os.Stat(s); return err == nil && !st.IsDir() }
func lineSpan(line int) ir.SourceSpan {
	return ir.SourceSpan{Start: ir.SourcePosition{Line: line, Column: 1}, End: ir.SourcePosition{Line: line, Column: 1}}
}
func diag(path, code string, severity ir.Severity, msg string) ir.Diagnostic {
	return ir.Diagnostic{Path: path, Code: code, Severity: severity, Message: msg, Start: ir.SourcePosition{Line: 1, Column: 1}, End: ir.SourcePosition{Line: 1, Column: 1}}
}
func lineDiag(path string, line int, code string, severity ir.Severity, msg string) ir.Diagnostic {
	d := diag(path, code, severity, msg)
	d.Start.Line, d.End.Line = line, line
	return d
}

type AIRAction struct {
	Number    int           `json:"number"`
	Rows      int           `json:"rows"`
	LoopStart bool          `json:"loopStart"`
	Span      ir.SourceSpan `json:"span"`
}

type AIRAsset struct {
	Kind         string        `json:"kind"`
	DeclaredPath string        `json:"declaredPath"`
	ResolvedPath string        `json:"resolvedPath,omitempty"`
	Status       string        `json:"status"`
	Span         ir.SourceSpan `json:"span"`
}

type AIRReport struct {
	Path        string          `json:"path"`
	Actions     []AIRAction     `json:"actions"`
	Assets      []AIRAsset      `json:"assets"`
	Diagnostics []ir.Diagnostic `json:"diagnostics"`
}

func (r AIRReport) JSONString() string { b, _ := json.Marshal(r); return string(b) }

// AnalyzeAIR validates AIR action metadata and filename references without loading
// any binary asset. Filename references use the conventional sprite/animation/sound
// key-value forms and are resolved through the supplied compatibility profile.
func AnalyzeAIR(path string, p profile.CompatibilityProfile) AIRReport {
	if p.Name == "" {
		p = profile.NewStrictPortableProfile("")
	}
	r := AIRReport{Path: cleanAbs(path), Actions: []AIRAction{}, Assets: []AIRAsset{}, Diagnostics: []ir.Diagnostic{}}
	if strings.TrimSpace(path) == "" {
		r.Path = ""
		r.Diagnostics = append(r.Diagnostics, diag("", "missing-air", ir.SeverityError, "AIR path is empty"))
		return r
	}
	data, err := os.ReadFile(r.Path)
	if err != nil {
		r.Diagnostics = append(r.Diagnostics, diag(r.Path, "air-read-error", ir.SeverityError, err.Error()))
		return r
	}
	seenActions := map[int]bool{}
	seenAssets := map[string]bool{}
	currentIndex := -1
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for i, raw := range lines {
		lineNo := i + 1
		code := strings.TrimSpace(splitComment(raw))
		if code == "" {
			continue
		}
		lower := strings.ToLower(code)
		if strings.HasPrefix(lower, "[begin action") && strings.HasSuffix(code, "]") {
			rawNumber := strings.TrimSpace(code[len("[Begin Action") : len(code)-1])
			number, parseErr := strconv.Atoi(rawNumber)
			action := AIRAction{Number: number, Span: lineSpan(lineNo)}
			r.Actions = append(r.Actions, action)
			currentIndex = len(r.Actions) - 1
			if parseErr != nil || number < 0 {
				r.Diagnostics = append(r.Diagnostics, lineDiag(r.Path, lineNo, "invalid-action-number", ir.SeverityError, "action number must be a non-negative integer"))
			} else if seenActions[number] {
				r.Diagnostics = append(r.Diagnostics, lineDiag(r.Path, lineNo, "duplicate-action", ir.SeverityWarning, fmt.Sprintf("action %d is declared more than once", number)))
			} else {
				seenActions[number] = true
			}
			continue
		}
		if strings.EqualFold(code, "[end action]") {
			currentIndex = -1
			continue
		}
		if kind, value, ok := airAssetKeyValue(code); ok {
			r.addAIRAsset(kind, value, lineNo, p, seenAssets)
			continue
		}
		if currentIndex < 0 {
			continue
		}
		if strings.EqualFold(lower, "loopstart") {
			r.Actions[currentIndex].LoopStart = true
			continue
		}
		if strings.HasPrefix(lower, "clsn") {
			if invalidAIRClsnRange(code) {
				r.Diagnostics = append(r.Diagnostics, lineDiag(r.Path, lineNo, "invalid-clsn-range", ir.SeverityError, "collision box range must have minimum coordinates no greater than maximum coordinates"))
			}
			continue
		}
		fields := csv(code)
		if len(fields) < 5 {
			r.Diagnostics = append(r.Diagnostics, lineDiag(r.Path, lineNo, "malformed-action-row", ir.SeverityError, "action row requires at least five comma-separated fields"))
			continue
		}
		values := make([]int, 5)
		valid := true
		for j := range values {
			n, parseErr := strconv.Atoi(strings.TrimSpace(fields[j]))
			if parseErr != nil {
				valid = false
				break
			}
			values[j] = n
		}
		if !valid {
			r.Diagnostics = append(r.Diagnostics, lineDiag(r.Path, lineNo, "malformed-action-row", ir.SeverityError, "action row fields must be integers"))
			continue
		}
		r.Actions[currentIndex].Rows++
		if values[0] < 0 || values[1] < 0 {
			r.Diagnostics = append(r.Diagnostics, lineDiag(r.Path, lineNo, "invalid-sprite-range", ir.SeverityError, "sprite group and index must be non-negative"))
		}
		if values[4] < -1 {
			r.Diagnostics = append(r.Diagnostics, lineDiag(r.Path, lineNo, "invalid-duration", ir.SeverityError, "sprite duration must be -1 or non-negative"))
		}
	}
	if currentIndex >= 0 {
		r.Diagnostics = append(r.Diagnostics, lineDiag(r.Path, len(lines), "unterminated-action", ir.SeverityError, "action block is missing [End Action]"))
	}
	return r
}

func (r *AIRReport) addAIRAsset(kind, declared string, line int, p profile.CompatibilityProfile, seen map[string]bool) {
	declared = p.NormalizeSourceValue(declared)
	resolved := p.ResolveSourcePath(filepath.Dir(r.Path), declared)
	asset := AIRAsset{Kind: kind, DeclaredPath: declared, ResolvedPath: cleanPath(resolved), Status: StatusResolved, Span: lineSpan(line)}
	key := p.DedupKey(resolved)
	if seen[key] {
		asset.Status = StatusDuplicate
		r.Diagnostics = append(r.Diagnostics, lineDiag(r.Path, line, "duplicate-air-asset", ir.SeverityWarning, "asset path resolves to a previously declared file"))
	} else if declared == "" || resolved == "" || !exists(resolved) {
		asset.Status = StatusMissing
		r.Diagnostics = append(r.Diagnostics, lineDiag(r.Path, line, "missing-air-asset", ir.SeverityError, "declared asset path does not resolve to a file"))
	} else {
		seen[key] = true
	}
	r.Assets = append(r.Assets, asset)
}

func airAssetKeyValue(line string) (string, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.ToLower(strings.TrimSpace(parts[0]))
	kind := ""
	switch key {
	case "sprite", "spritefile", "sff":
		kind = "sprite"
	case "animation", "animationfile", "anim", "air":
		kind = "animation"
	case "sound", "soundfile", "snd":
		kind = "sound"
	default:
		return "", "", false
	}
	return kind, strings.TrimSpace(parts[1]), true
}

func invalidAIRClsnRange(line string) bool {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return false
	}
	fields := csv(parts[1])
	if len(fields) < 4 {
		return false
	}
	values := make([]int, 4)
	for i := range values {
		n, err := strconv.Atoi(strings.TrimSpace(fields[i]))
		if err != nil {
			return false
		}
		values[i] = n
	}
	return values[0] > values[2] || values[1] > values[3]
}
