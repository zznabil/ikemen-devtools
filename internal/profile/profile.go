package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type CasePolicy int

type SlashNormalization int

type ExternalPathPolicy int

type RuntimeToleranceFlag string

type RuntimeToleranceFlags map[RuntimeToleranceFlag]bool

const (
	CasePolicySensitive CasePolicy = iota
	CasePolicyInsensitive
)

const (
	SlashNormalizationPortable SlashNormalization = iota
	SlashNormalizationNativeOnly
)

const (
	ExternalPathPolicyAllowAll ExternalPathPolicy = iota
	ExternalPathPolicyWorkspaceOnly
)

const (
	ToleranceLeadingEqualsPath     RuntimeToleranceFlag = "leading-equals-path"
	ToleranceCaseInsensitiveLookup RuntimeToleranceFlag = "case-insensitive-lookup"
	ToleranceEmptyStateSections    RuntimeToleranceFlag = "empty-state-sections"
)

type CompatibilityProfile struct {
	Name string

	WorkspaceRoot string

	EqualsLiteralLeadingSlash bool
	SlashNormalization        SlashNormalization
	CasePolicy                CasePolicy
	FallbackRoots             []string
	ExternalPathPolicy        ExternalPathPolicy
	RuntimeTolerance          RuntimeToleranceFlags
}

func NewStrictPortableProfile(workspaceRoot string) CompatibilityProfile {
	return CompatibilityProfile{
		Name:                      "strict/portable",
		WorkspaceRoot:             workspaceRoot,
		EqualsLiteralLeadingSlash: false,
		SlashNormalization:        SlashNormalizationPortable,
		CasePolicy:                defaultCasePolicy(),
		FallbackRoots:             []string{".", "data"},
		ExternalPathPolicy:        ExternalPathPolicyAllowAll,
		RuntimeTolerance: RuntimeToleranceFlags{
			ToleranceCaseInsensitiveLookup: true,
		},
	}
}

func NewDistributionProfile(workspaceRoot string) CompatibilityProfile {
	return CompatibilityProfile{
		Name:                      "distribution",
		WorkspaceRoot:             workspaceRoot,
		EqualsLiteralLeadingSlash: true,
		SlashNormalization:        SlashNormalizationPortable,
		CasePolicy:                defaultCasePolicy(),
		FallbackRoots:             []string{".", "data"},
		ExternalPathPolicy:        ExternalPathPolicyAllowAll,
		RuntimeTolerance: RuntimeToleranceFlags{
			ToleranceLeadingEqualsPath:  true,
			ToleranceEmptyStateSections: true,
		},
	}
}

func defaultCasePolicy() CasePolicy {
	if runtime.GOOS == "windows" {
		return CasePolicyInsensitive
	}
	return CasePolicySensitive
}

func (p CompatibilityProfile) NormalizeCase(path string) string {
	path = strings.TrimSpace(path)
	if p.CasePolicy == CasePolicyInsensitive {
		return strings.ToLower(path)
	}
	return path
}

func (p CompatibilityProfile) DedupKey(path string) string {
	return p.NormalizeCase(filepath.Clean(strings.TrimSpace(path)))
}

func (p CompatibilityProfile) ResolveSourcePath(baseDir, raw string) string {
	value := p.NormalizeSourceValue(raw)
	if value == "" {
		return ""
	}

	if isAbsoluteValue(value) {
		if p.ExternalPathPolicy == ExternalPathPolicyWorkspaceOnly {
			if !p.isWithinWorkspaceRoot(value) {
				return ""
			}
		}
		return filepath.Clean(value)
	}

	for _, root := range p.resolveSourceRoots(baseDir) {
		candidate := filepath.Clean(filepath.Join(root, value))
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return filepath.Clean(filepath.Join(baseDir, value))
}

func (p CompatibilityProfile) normalizeSlashSeparators(value string) string {
	if p.SlashNormalization == SlashNormalizationNativeOnly {
		value = strings.ReplaceAll(value, "/", string(filepath.Separator))
		return filepath.Clean(value)
	}
	// Portable mode keeps both slash kinds while still producing a filesystem path candidate.
	value = strings.ReplaceAll(value, "\\", string(filepath.Separator))
	value = strings.ReplaceAll(value, "/", string(filepath.Separator))
	return filepath.Clean(value)
}

func (p CompatibilityProfile) NormalizeSourceValue(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	hadEquals := false
	if strings.HasPrefix(value, "=") {
		hadEquals = true
		value = strings.TrimSpace(strings.TrimPrefix(value, "="))
	}

	value = strings.TrimSpace(stripQuotes(value))
	if value == "" {
		return ""
	}

	if hadEquals {
		if p.EqualsLiteralLeadingSlash {
			value = "=" + value
		} else {
			value = strings.TrimLeft(value, `\\/`)
		}
	} else if strings.HasPrefix(value, `\`) && !isUNC(value) {
		value = strings.TrimLeft(value, `\/`)
	}

	value = p.normalizeSlashSeparators(value)
	value = strings.TrimSpace(value)
	if value == "" || value == "." {
		return ""
	}

	return value
}

func (p CompatibilityProfile) resolveSourceRoots(baseDir string) []string {
	roots := make([]string, 0, len(p.FallbackRoots)+1)
	seen := make(map[string]struct{})
	add := func(candidate string) {
		clean := filepath.Clean(strings.TrimSpace(candidate))
		if clean == "" {
			return
		}
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		roots = append(roots, clean)
	}

	add(baseDir)
	workspaceRoot := p.resolvedWorkspaceRoot()
	for _, root := range p.FallbackRoots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		if filepath.IsAbs(root) {
			add(root)
			continue
		}
		if workspaceRoot != "" {
			add(filepath.Join(workspaceRoot, root))
			continue
		}
		add(root)
	}
	return roots
}

func (p CompatibilityProfile) resolvedWorkspaceRoot() string {
	if strings.TrimSpace(p.WorkspaceRoot) != "" {
		return filepath.Clean(strings.TrimSpace(p.WorkspaceRoot))
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Clean(wd)
	}
	return ""
}

func (p CompatibilityProfile) isWithinWorkspaceRoot(path string) bool {
	root := p.resolvedWorkspaceRoot()
	if root == "" {
		return false
	}

	root = filepath.Clean(root)
	candidate := filepath.Clean(path)
	if p.CasePolicy == CasePolicyInsensitive {
		root = strings.ToLower(root)
		candidate = strings.ToLower(candidate)
	}

	if candidate == root {
		return true
	}

	sep := string(filepath.Separator)
	if strings.HasSuffix(root, sep) {
		root = strings.TrimRight(root, sep)
	}
	if root == "" {
		return false
	}
	return strings.HasPrefix(candidate, root+sep)
}

func (p CompatibilityProfile) String() string {
	return fmt.Sprintf(
		"profile(name=%q, workspaceRoot=%q, equalsLiteral=%t, casePolicy=%d, slash=%d, external=%d)",
		p.Name,
		p.WorkspaceRoot,
		p.EqualsLiteralLeadingSlash,
		p.CasePolicy,
		p.SlashNormalization,
		p.ExternalPathPolicy,
	)
}

func isAbsoluteValue(value string) bool {
	if isUNC(value) || hasDriveLetter(value) {
		return true
	}
	return filepath.IsAbs(value)
}

func isUNC(value string) bool {
	return strings.HasPrefix(value, `\\`) || strings.HasPrefix(value, "//")
}

func hasDriveLetter(value string) bool {
	return len(value) >= 2 && value[1] == ':'
}

func stripQuotes(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
