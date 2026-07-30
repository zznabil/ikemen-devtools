package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/parser"
	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
)

// LoadResult is the workspace output after resolving file references.
type LoadResult struct {
	Documents    []ir.Document
	Diagnostics  []ir.Diagnostic
	ConfigDigest string
}

// LoadWorkspaceConfigured resolves the root-scoped .ikm configuration before loading an entry point.
func LoadWorkspaceConfigured(defPath string, flags ConfigFlags) (LoadResult, error) {
	root, err := filepath.Abs(filepath.Dir(defPath))
	if err != nil {
		return LoadResult{}, err
	}
	cfg, err := ResolveConfig(root, flags)
	if err != nil {
		return LoadResult{}, err
	}
	result := LoadWorkspaceWithProfile(defPath, cfg.ProfileValue())
	result.ConfigDigest = cfg.Digest()
	return result, nil
}

// LoadWorkspace parses a character .def and resolves its [Files] section sources.
// Sources are loaded in [Files]-encounter order, resolved relative to the DEF directory,
// parsed through internal/parser, and de-duplicated by absolute path.
func LoadWorkspace(defPath string) LoadResult {
	return LoadWorkspaceWithProfile(defPath, profile.NewStrictPortableProfile(""))
}

// LoadWorkspaceWithProfile behaves like LoadWorkspace but uses an explicit compatibility
// profile for path normalization and resolution.
func LoadWorkspaceWithProfile(defPath string, p profile.CompatibilityProfile) LoadResult {
	if p.Name == "" {
		p = profile.NewStrictPortableProfile("")
	}

	result := LoadResult{}
	if strings.TrimSpace(defPath) == "" {
		result.Diagnostics = append(result.Diagnostics, makeWorkspaceDiagnostic(
			"",
			ir.SourceSpan{Start: ir.SourcePosition{Line: 1, Column: 1}, End: ir.SourcePosition{Line: 1, Column: 1}},
			"empty-workspace-path",
			ir.SeverityError,
			"workspace definition path is empty",
		))
		return result
	}

	cleanDefPath := filepath.Clean(strings.TrimSpace(defPath))
	absDefPath, err := filepath.Abs(cleanDefPath)
	if err == nil {
		cleanDefPath = absDefPath
	}

	root, parseErr := parseWorkspaceFile(cleanDefPath)
	if parseErr != nil {
		result.Diagnostics = append(result.Diagnostics,
			makeWorkspaceDiagnostic(
				cleanDefPath,
				ir.SourceSpan{Start: ir.SourcePosition{Line: 1, Column: 1}, End: ir.SourcePosition{Line: 1, Column: 1}},
				"missing-source",
				ir.SeverityError,
				parseErr.Error(),
			),
		)
		return result
	}
	result.Diagnostics = append(result.Diagnostics, root.Diagnostics...)

	result.Documents = append(result.Documents, *root)
	refs := sourceReferences(root, p)
	baseDir := filepath.Dir(cleanDefPath)
	seen := map[string]struct{}{p.DedupKey(canonicalWorkspacePath(cleanDefPath)): {}}

	for _, ref := range refs {
		resolved := p.ResolveSourcePath(baseDir, ref.path)
		if resolved == "" {
			continue
		}
		resolved = canonicalWorkspacePath(resolved)

		key := p.DedupKey(resolved)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		sourceDoc, parseErr := parseWorkspaceFile(resolved)
		if parseErr != nil {
			result.Diagnostics = append(result.Diagnostics,
				makeWorkspaceDiagnostic(
					cleanDefPath,
					ref.span,
					"missing-source",
					ir.SeverityError,
					fmt.Sprintf("unable to read source %q", ref.path),
				),
			)
			continue
		}

		result.Documents = append(result.Documents, *sourceDoc)
		result.Diagnostics = append(result.Diagnostics, sourceDoc.Diagnostics...)
	}

	return result
}

type sourceRef struct {
	path string
	span ir.SourceSpan
}

func sourceReferences(doc *ir.Document, p profile.CompatibilityProfile) []sourceRef {
	if doc == nil {
		return nil
	}

	refs := make([]sourceRef, 0)
	for _, section := range doc.Sections {
		if !strings.EqualFold(strings.TrimSpace(section.Header), "files") {
			continue
		}
		for _, line := range section.Lines {
			if line.Kind != ir.SourceLineKeyValue {
				continue
			}
			if !isWorkspaceFileKey(line.Key) {
				continue
			}
			value := p.NormalizeSourceValue(line.Value)
			if value == "" {
				continue
			}
			refs = append(refs, sourceRef{path: value, span: line.Span})
		}
	}
	return refs
}

func isWorkspaceFileKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "cmd", "cns", "st", "stcommon":
		return true
	}

	if strings.HasPrefix(key, "st") {
		n := key[2:]
		if n == "" {
			return false
		}
		if _, err := strconv.Atoi(n); err == nil {
			return true
		}
	}
	return false
}

func parseWorkspaceFile(path string) (*ir.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc := parser.Parse(path, string(data))
	return doc, nil
}

func canonicalWorkspacePath(path string) string {
	clean := filepath.Clean(strings.TrimSpace(path))
	if abs, err := filepath.Abs(clean); err == nil {
		clean = abs
	}
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		return filepath.Clean(resolved)
	}
	return clean
}

func makeWorkspaceDiagnostic(path string, span ir.SourceSpan, code string, severity ir.Severity, message string) ir.Diagnostic {
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
