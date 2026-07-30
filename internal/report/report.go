package report

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/semantics"
	"github.com/ikemen-engine/ikemen-devtools/internal/workspace"
)

type Report struct {
	Diagnostics []ir.Diagnostic `json:"diagnostics"`
}

func NewFromDiagnostics(diags []ir.Diagnostic) Report {
	out := append([]ir.Diagnostic(nil), diags...)
	if out == nil {
		out = []ir.Diagnostic{}
	}
	normalizeDiagnostics(out)
	return Report{Diagnostics: out}
}

func NewFromWorkspace(result workspace.LoadResult) Report {
	return NewFromDiagnostics(result.Diagnostics)
}

func NewFromSemantics(result semantics.ResolveResult) Report {
	return NewFromDiagnostics(result.Diagnostics)
}

func NewFromWorkspaceAndSemantics(workspaceResult workspace.LoadResult, semanticResult semantics.ResolveResult) Report {
	merged := make([]ir.Diagnostic, 0, len(workspaceResult.Diagnostics)+len(semanticResult.Diagnostics))
	merged = append(merged, workspaceResult.Diagnostics...)
	merged = append(merged, semanticResult.Diagnostics...)
	return NewFromDiagnostics(merged)
}

func FromDiagnostics(diags []ir.Diagnostic) Report {
	return NewFromDiagnostics(diags)
}

func FromWorkspace(result workspace.LoadResult) Report {
	return NewFromWorkspace(result)
}

func FromWorkspaceResult(result workspace.LoadResult) Report {
	return NewFromWorkspace(result)
}

func FromSemantics(result semantics.ResolveResult) Report {
	return NewFromSemantics(result)
}

func FromSemanticsResult(result semantics.ResolveResult) Report {
	return NewFromSemantics(result)
}

func FromWorkspaceAndSemantics(workspaceResult workspace.LoadResult, semanticResult semantics.ResolveResult) Report {
	return NewFromWorkspaceAndSemantics(workspaceResult, semanticResult)
}

func JSONFromDiagnostics(diags []ir.Diagnostic) ([]byte, error) {
	return NewFromDiagnostics(diags).JSON()
}

func HumanFromDiagnostics(diags []ir.Diagnostic) string {
	return NewFromDiagnostics(diags).Human()
}

func Compact(diags []ir.Diagnostic) string {
	return HumanFromDiagnostics(diags)
}

func Marshal(diags []ir.Diagnostic) ([]byte, error) {
	return JSONFromDiagnostics(diags)
}

func (r Report) JSON() ([]byte, error) {
	copy := Report{Diagnostics: cloneAndSort(r.Diagnostics)}
	return json.Marshal(copy)
}

func (r Report) Human() string {
	diags := cloneAndSort(r.Diagnostics)
	if len(diags) == 0 {
		return "No diagnostics."
	}

	var lines strings.Builder
	for i := range diags {
		if i > 0 {
			lines.WriteByte('\n')
		}
		lines.WriteString(formatDiagnosticLine(diags[i]))
	}
	return lines.String()
}

func JSONFromWorkspace(result workspace.LoadResult) ([]byte, error) {
	return NewFromWorkspace(result).JSON()
}

func JSONFromWorkspaceResult(result workspace.LoadResult) ([]byte, error) {
	return JSONFromWorkspace(result)
}

func HumanFromWorkspace(result workspace.LoadResult) string {
	return NewFromWorkspace(result).Human()
}

func HumanFromWorkspaceResult(result workspace.LoadResult) string {
	return HumanFromWorkspace(result)
}

func JSONFromSemantics(result semantics.ResolveResult) ([]byte, error) {
	return NewFromSemantics(result).JSON()
}

func JSONFromSemanticsResult(result semantics.ResolveResult) ([]byte, error) {
	return JSONFromSemantics(result)
}

func HumanFromSemantics(result semantics.ResolveResult) string {
	return NewFromSemantics(result).Human()
}

func HumanFromSemanticsResult(result semantics.ResolveResult) string {
	return HumanFromSemantics(result)
}

func HumanFromWorkspaceAndSemantics(workspaceResult workspace.LoadResult, semanticResult semantics.ResolveResult) string {
	return FromWorkspaceAndSemantics(workspaceResult, semanticResult).Human()
}

func JSONFromWorkspaceAndSemantics(workspaceResult workspace.LoadResult, semanticResult semantics.ResolveResult) ([]byte, error) {
	return FromWorkspaceAndSemantics(workspaceResult, semanticResult).JSON()
}

func normalizeDiagnostics(diags []ir.Diagnostic) {
	sort.Slice(diags, func(i, j int) bool {
		lhs, rhs := diags[i], diags[j]
		if lhs.Path != rhs.Path {
			return lhs.Path < rhs.Path
		}
		if lhs.Start.Line != rhs.Start.Line {
			return lhs.Start.Line < rhs.Start.Line
		}
		if lhs.Start.Column != rhs.Start.Column {
			return lhs.Start.Column < rhs.Start.Column
		}
		if lhs.End.Line != rhs.End.Line {
			return lhs.End.Line < rhs.End.Line
		}
		if lhs.End.Column != rhs.End.Column {
			return lhs.End.Column < rhs.End.Column
		}
		if lhs.Code != rhs.Code {
			return lhs.Code < rhs.Code
		}
		if lhs.Severity != rhs.Severity {
			return lhs.Severity < rhs.Severity
		}
		if lhs.RelatedSymbol != rhs.RelatedSymbol {
			return lhs.RelatedSymbol < rhs.RelatedSymbol
		}
		if lhs.Message != rhs.Message {
			return lhs.Message < rhs.Message
		}
		return false
	})
}

func cloneAndSort(diags []ir.Diagnostic) []ir.Diagnostic {
	copyDiags := append([]ir.Diagnostic(nil), diags...)
	if copyDiags == nil {
		copyDiags = []ir.Diagnostic{}
	}
	normalizeDiagnostics(copyDiags)
	return copyDiags
}

func formatDiagnosticLine(d ir.Diagnostic) string {
	related := ""
	if d.RelatedSymbol != "" {
		related = " [" + d.RelatedSymbol + "]"
	}
	return fmt.Sprintf(
		"%s:%d:%d-%d:%d [%s] %s: %s%s",
		d.Path,
		d.Start.Line,
		d.Start.Column,
		d.End.Line,
		d.End.Column,
		d.Severity,
		d.Code,
		d.Message,
		related,
	)
}
