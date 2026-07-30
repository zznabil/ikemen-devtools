package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
	"github.com/ikemen-engine/ikemen-devtools/internal/workspace"
)

const ManifestVersion = "0.1.0"

const (
	entryStatusResolved     = "resolved"
	entryStatusDeduplicated = "deduplicated"
	entryStatusMissing      = "missing"
)

// Manifest summarizes a corpus run.
type Manifest struct {
	Version             string            `json:"version"`
	Profile             string            `json:"profile"`
	SelectPath          string            `json:"selectPath"`
	Entries             []ManifestEntry   `json:"entries"`
	DeclaredSourceCount int               `json:"declaredSourceCount"`
	ResolvedSourceCount int               `json:"resolvedSourceCount"`
	ErrorCount          int               `json:"errorCount"`
	WarningCount        int               `json:"warningCount"`
	DiagnosticsByCode   []DiagnosticCount `json:"diagnosticsByCode"`
	Process             ProcessMetadata   `json:"process"`
}

// ManifestEntry records one parsed [Characters] line.
type ManifestEntry struct {
	Index             int               `json:"index"`
	Status            string            `json:"status"`
	DeclaredPath      string            `json:"declaredPath"`
	ResolvedPath      string            `json:"resolvedPath,omitempty"`
	Span              ir.SourceSpan     `json:"span"`
	Options           []string          `json:"options"`
	Diagnostics       []ir.Diagnostic   `json:"diagnostics"`
	DiagnosticsByCode []DiagnosticCount `json:"diagnosticsByCode"`
}

// DiagnosticCount tracks one diagnostic code bucket.
type DiagnosticCount struct {
	Code     string `json:"code"`
	Count    int    `json:"count"`
	Errors   int    `json:"errors"`
	Warnings int    `json:"warnings"`
	Infos    int    `json:"infos"`
}

// ProcessMetadata captures deterministic process metadata.
type ProcessMetadata struct {
	Command   string `json:"command"`
	GoVersion string `json:"goVersion"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// BuildManifest parses a select file and executes workspace loads for each entry.
func BuildManifest(selectPath string, p profile.CompatibilityProfile) Manifest {
	if p.Name == "" {
		p = profile.NewDistributionProfile("")
	}

	m := Manifest{
		Version: ManifestVersion,
		Profile: p.Name,
		Process: newProcessMetadata(),
		Entries: make([]ManifestEntry, 0),
	}

	cleanSelect := strings.TrimSpace(selectPath)
	if cleanSelect == "" {
		diag := makeDiagnostic(cleanSelect, ir.SourceSpan{Start: ir.SourcePosition{Line: 1, Column: 1}, End: ir.SourcePosition{Line: 1, Column: 1}}, "missing-select", ir.SeverityError, "select path is empty")
		return finalizeManifest(&m, []ir.Diagnostic{diag})
	}
	absSelect, err := filepath.Abs(cleanSelect)
	if err == nil {
		cleanSelect = absSelect
	}
	m.SelectPath = filepath.Clean(cleanSelect)

	selectData, err := os.ReadFile(m.SelectPath)
	if err != nil {
		diag := makeDiagnostic(m.SelectPath, ir.SourceSpan{Start: ir.SourcePosition{Line: 1, Column: 1}, End: ir.SourcePosition{Line: 1, Column: 1}}, "select-read-error", ir.SeverityError, err.Error())
		return finalizeManifest(&m, []ir.Diagnostic{diag})
	}

	entries, parseDiags := parseSelectEntries(string(selectData), m.SelectPath)
	m.DeclaredSourceCount = len(entries)

	allDiags := append([]ir.Diagnostic(nil), parseDiags...)
	seen := map[string]struct{}{}
	for i, entry := range entries {
		result := ManifestEntry{
			Index:        i,
			DeclaredPath: entry.declaredPath,
			Span:         entry.span,
			Options:      append([]string(nil), entry.options...),
		}
		if result.Options == nil {
			result.Options = []string{}
		}

		resolved := resolveCharacterPath(m.SelectPath, entry.declaredPath, p)
		result.ResolvedPath = resolved
		key := p.DedupKey(resolved)
		if _, ok := seen[key]; ok {
			result.Status = entryStatusDeduplicated
			d := makeDiagnostic(m.SelectPath, entry.span, "deduplicated-source", ir.SeverityInfo, "character source already processed")
			result.Diagnostics = []ir.Diagnostic{d}
			result.DiagnosticsByCode = countsFromDiagnostics(result.Diagnostics)
			allDiags = append(allDiags, d)
			m.Entries = append(m.Entries, result)
			continue
		}
		seen[key] = struct{}{}

		ws := workspace.LoadWorkspaceWithProfile(resolved, p)
		if hasErrors(ws.Diagnostics) {
			result.Status = entryStatusMissing
		} else {
			result.Status = entryStatusResolved
			m.ResolvedSourceCount++
		}
		result.Diagnostics = append([]ir.Diagnostic(nil), ws.Diagnostics...)
		if len(result.Diagnostics) == 0 {
			result.Diagnostics = []ir.Diagnostic{}
		}
		result.DiagnosticsByCode = countsFromDiagnostics(result.Diagnostics)
		allDiags = append(allDiags, ws.Diagnostics...)
		m.Entries = append(m.Entries, result)
	}

	return finalizeManifest(&m, allDiags)
}

func finalizeManifest(m *Manifest, diags []ir.Diagnostic) Manifest {
	m.DiagnosticsByCode = countsFromDiagnostics(diags)
	m.ErrorCount, m.WarningCount = countsTotalsFromSlice(diags)
	if m.DeclaredSourceCount < 0 {
		m.DeclaredSourceCount = 0
	}
	if m.Entries == nil {
		m.Entries = []ManifestEntry{}
	}
	return *m
}

// JSON renders the manifest with deterministic key and slice order.
func (m Manifest) JSON() ([]byte, error) {
	return json.Marshal(m)
}

// Human renders a compact deterministic textual summary.
func (m Manifest) Human() string {
	parts := make([]string, 0, 8+len(m.Entries))
	parts = append(parts, fmt.Sprintf("Profile: %s", m.Profile))
	parts = append(parts, fmt.Sprintf("Select: %s", m.SelectPath))
	parts = append(parts, fmt.Sprintf("Declared sources: %d", m.DeclaredSourceCount))
	parts = append(parts, fmt.Sprintf("Resolved sources: %d", m.ResolvedSourceCount))
	parts = append(parts, fmt.Sprintf("Errors: %d", m.ErrorCount))
	parts = append(parts, fmt.Sprintf("Warnings: %d", m.WarningCount))
	parts = append(parts, "Entries:")
	for _, e := range m.Entries {
		resolved := e.ResolvedPath
		if resolved == "" {
			resolved = "-"
		}
		parts = append(parts, fmt.Sprintf("- %s (%s): %s", e.DeclaredPath, e.Status, resolved))
	}
	return strings.Join(parts, "\n")
}

func newProcessMetadata() ProcessMetadata {
	return ProcessMetadata{
		Command:   "ikm corpus",
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

type selectEntry struct {
	declaredPath string
	span         ir.SourceSpan
	options      []string
}

func parseSelectEntries(source, selectPath string) ([]selectEntry, []ir.Diagnostic) {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	entries := make([]selectEntry, 0)
	diagnostics := make([]ir.Diagnostic, 0)
	inside := false
	seenCharacters := false

	for i, rawLine := range lines {
		lineNo := i + 1
		codeLine, _ := splitLine(rawLine)
		trimmed := strings.TrimSpace(codeLine)
		if trimmed == "" {
			continue
		}

		if isSectionHeader(trimmed) {
			inside = strings.EqualFold(strings.TrimSpace(trimmed[1:len(trimmed)-1]), "characters")
			if inside {
				seenCharacters = true
			}
			continue
		}
		if !inside {
			continue
		}

		if isIgnorableCharactersLine(trimmed) {
			continue
		}

		entry, ok := parseSelectEntryLine(trimmed, lineNo)
		if ok {
			entries = append(entries, entry)
			continue
		}
	}

	if !seenCharacters {
		diagnostics = append(diagnostics, makeDiagnostic(
			selectPath,
			ir.SourceSpan{Start: ir.SourcePosition{Line: 1, Column: 1}, End: ir.SourcePosition{Line: 1, Column: 1}},
			"missing-characters-section",
			ir.SeverityWarning,
			"select file has no [Characters] section",
		))
	}

	return entries, diagnostics
}

func isSectionHeader(line string) bool {
	if len(line) < 2 {
		return false
	}
	if line[0] != '[' || line[len(line)-1] != ']' {
		return false
	}
	return true
}

func isIgnorableCharactersLine(trimmed string) bool {
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, ";") || strings.HasPrefix(lower, "#") {
		return true
	}
	if strings.HasPrefix(lower, "randomselect") {
		return true
	}
	return isSeparatorLine(lower)
}

func isSeparatorLine(line string) bool {
	if len(strings.TrimSpace(line)) < 3 {
		return false
	}
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == ' ' || ch == '\t' {
			continue
		}
		if ch != '-' && ch != '=' && ch != '*' && ch != '_' && ch != '.' {
			return false
		}
	}
	return true
}

func parseSelectEntryLine(line string, lineNo int) (selectEntry, bool) {
	fields := splitCSVFields(line)
	if len(fields) == 0 {
		return selectEntry{}, false
	}

	selected := -1
	for i := range fields {
		value := unquote(strings.TrimSpace(fields[i].Raw))
		if isDefPath(value) {
			selected = i
			break
		}
	}
	if selected < 0 {
		return selectEntry{}, false
	}

	path := unquote(strings.TrimSpace(fields[selected].Raw))
	if path == "" || !isDefPath(path) {
		return selectEntry{}, false
	}

	opts := make([]string, 0, len(fields)-1)
	for i, f := range fields {
		if i == selected {
			continue
		}
		if value := strings.TrimSpace(unquote(f.Raw)); value != "" {
			opts = append(opts, value)
		}
	}

	start, end := fieldSpan(fields[selected])
	if start < 1 {
		start = 1
	}
	if end < start {
		end = start
	}

	return selectEntry{
		declaredPath: path,
		span:         ir.SourceSpan{Start: ir.SourcePosition{Line: lineNo, Column: start}, End: ir.SourcePosition{Line: lineNo, Column: end}},
		options:      opts,
	}, true
}

type csvField struct {
	Raw   string
	Start int
	End   int
}

func splitCSVFields(line string) []csvField {
	fields := make([]csvField, 0)
	inQuote := false
	escaped := false
	fieldStart := 0

	for i := 0; i <= len(line); i++ {
		if i < len(line) {
			ch := line[i]
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
			if ch != ',' || inQuote {
				continue
			}
		}

		field := line[fieldStart:i]
		fields = append(fields, csvField{Raw: field, Start: fieldStart, End: i})
		fieldStart = i + 1
	}

	return fields
}

func fieldSpan(field csvField) (int, int) {
	trimmed := strings.TrimSpace(field.Raw)
	if trimmed == "" {
		return 0, 0
	}
	value := unquote(trimmed)
	if value == "" {
		return 0, 0
	}
	startOffset := len(field.Raw) - len(strings.TrimLeft(field.Raw, " \t"))
	start := field.Start + startOffset + 1
	end := start + len(value)
	if start < 1 {
		start = 1
	}
	if end < start {
		end = start
	}
	return start, end
}

func isDefPath(value string) bool {
	ext := strings.ToLower(filepath.Ext(value))
	return ext == ".def"
}

func unquote(text string) string {
	if len(text) < 2 {
		return text
	}
	if (text[0] == '\'' && text[len(text)-1] == '\'') || (text[0] == '"' && text[len(text)-1] == '"') {
		return text[1 : len(text)-1]
	}
	return text
}

func splitLine(raw string) (code string, comment string) {
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
		if ch == ';' || ch == '#' {
			commentIdx = i
			break
		}
	}
	if commentIdx >= 0 {
		return strings.TrimRight(raw[:commentIdx], " \t"), strings.TrimSpace(raw[commentIdx+1:])
	}
	return strings.TrimRight(raw, " \t"), ""
}

func resolveCharacterPath(selectPath, declared string, p profile.CompatibilityProfile) string {
	normalized := p.NormalizeSourceValue(strings.TrimSpace(declared))
	if normalized == "" {
		return ""
	}
	selectDir := filepath.Dir(selectPath)
	roots := []string{
		selectDir,
		filepath.Dir(selectDir),
		filepath.Join(filepath.Dir(selectDir), "chars"),
	}
	var first string
	for _, root := range roots {
		candidate := p.ResolveSourcePath(root, normalized)
		if first == "" {
			first = candidate
		}
		if candidate == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return canonicalPath(candidate)
		}
	}
	return canonicalPath(first)
}

func canonicalPath(path string) string {
	if path == "" {
		return ""
	}
	clean := filepath.Clean(strings.TrimSpace(path))
	if abs, err := filepath.Abs(clean); err == nil {
		clean = abs
	}
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		return filepath.Clean(resolved)
	}
	return clean
}

func hasErrors(diags []ir.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == ir.SeverityError {
			return true
		}
	}
	return false
}

func countsTotalsFromSlice(diags []ir.Diagnostic) (errors int, warnings int) {
	for _, d := range diags {
		switch d.Severity {
		case ir.SeverityError:
			errors++
		case ir.SeverityWarning:
			warnings++
		}
	}
	return errors, warnings
}

func countsFromDiagnostics(diags []ir.Diagnostic) []DiagnosticCount {
	counts := map[string]DiagnosticCount{}
	for _, d := range diags {
		c := counts[d.Code]
		c.Code = d.Code
		c.Count++
		switch d.Severity {
		case ir.SeverityError:
			c.Errors++
		case ir.SeverityWarning:
			c.Warnings++
		case ir.SeverityInfo:
			c.Infos++
		}
		counts[d.Code] = c
	}

	codes := make([]string, 0, len(counts))
	for code := range counts {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	out := make([]DiagnosticCount, 0, len(codes))
	for _, code := range codes {
		out = append(out, counts[code])
	}
	return out
}

func makeDiagnostic(path string, span ir.SourceSpan, code string, severity ir.Severity, message string) ir.Diagnostic {
	start := span.Start
	end := span.End
	if start.Line < 1 {
		start.Line = 1
	}
	if start.Column < 1 {
		start.Column = 1
	}
	if end.Line < 1 {
		end.Line = start.Line
	}
	if end.Column < 1 {
		end.Column = start.Column
	}
	if strings.TrimSpace(message) == "" {
		message = code
	}
	return ir.Diagnostic{
		Path:          path,
		Code:          code,
		Severity:      severity,
		Message:       message,
		Start:         start,
		End:           end,
		RelatedSymbol: "",
	}
}
