package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/semantics"
	"github.com/ikemen-engine/ikemen-devtools/internal/workspace"
)

func TestReportEmptyOutput(t *testing.T) {
	report := NewFromDiagnostics(nil)

	human := report.Human()
	if human != "No diagnostics." {
		t.Fatalf("expected canonical empty human output, got %q", human)
	}

	raw, err := report.JSON()
	if err != nil {
		t.Fatalf("expected JSON encoding to succeed: %v", err)
	}
	if strings.TrimSpace(string(raw)) != `{"diagnostics":[]}` {
		t.Fatalf("expected empty report JSON array output, got %q", string(raw))
	}
}

func TestReportRoundTripsFromEmptyAndPreservesShape(t *testing.T) {
	var report Report

	raw, err := report.JSON()
	if err != nil {
		t.Fatalf("expected JSON encoding of zero report to succeed: %v", err)
	}
	var decoded Report
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("expected JSON unmarshal to succeed: %v", err)
	}
	if len(decoded.Diagnostics) != 0 {
		t.Fatalf("expected zero diagnostics after unmarshal, got %d", len(decoded.Diagnostics))
	}

	reencoded, err := decoded.JSON()
	if err != nil {
		t.Fatalf("expected deterministic second encode: %v", err)
	}
	if string(raw) != string(reencoded) {
		t.Fatalf("expected deterministic empty output across repeated encodes: %q vs %q", raw, reencoded)
	}
}

func TestReportDeterministicOrderingAndJSONRoundTrip(t *testing.T) {
	a := []ir.Diagnostic{
		{
			Path:     "zeta.def",
			Code:     "warn-code",
			Severity: ir.SeverityWarning,
			Message:  "later in file",
			Start:    sourcePosition(6, 4),
			End:      sourcePosition(6, 9),
		},
		{
			Path:     "alpha.def",
			Code:     "error-code",
			Severity: ir.SeverityError,
			Message:  "first line",
			Start:    sourcePosition(2, 10),
			End:      sourcePosition(2, 12),
		},
		{
			Path:     "alpha.def",
			Code:     "error-code",
			Severity: ir.SeverityError,
			Message:  "earlier column",
			Start:    sourcePosition(2, 2),
			End:      sourcePosition(2, 5),
		},
		{
			Path:          "alpha.def",
			Code:          "info-code",
			Severity:      ir.SeverityInfo,
			Message:       "same span tie",
			Start:         sourcePosition(2, 2),
			End:           sourcePosition(2, 5),
			RelatedSymbol: "state:1",
		},
	}

	b := []ir.Diagnostic{
		a[2], a[0], a[1], a[3],
	}

	left := NewFromDiagnostics(a)
	right := NewFromDiagnostics(b)
	if !equalDiagnosticSets(left.Diagnostics, right.Diagnostics) {
		t.Fatalf("expected same deterministic order across input permutations:\nleft=%#v\nright=%#v", left.Diagnostics, right.Diagnostics)
	}
	if len(left.Diagnostics) != 4 {
		t.Fatalf("expected sorted diagnostics preserved, got %d", len(left.Diagnostics))
	}

	raw, err := left.JSON()
	if err != nil {
		t.Fatalf("expected JSON encoding to succeed: %v", err)
	}

	var decoded Report
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("expected JSON round-trip decode: %v", err)
	}
	if !equalDiagnosticSets(left.Diagnostics, decoded.Diagnostics) {
		t.Fatalf("expected decoded diagnostics to match sorted diagnostics: %#v vs %#v", decoded.Diagnostics, left.Diagnostics)
	}
}

func TestConvenienceAPIsForWorkspaceAndSemantics(t *testing.T) {
	ws := workspace.LoadResult{
		Diagnostics: []ir.Diagnostic{
			{
				Path:     "workspace.def",
				Code:     "missing-source",
				Severity: ir.SeverityError,
				Message:  "workspace",
				Start:    sourcePosition(3, 1),
				End:      sourcePosition(3, 2),
			},
		},
	}
	sem := semantics.ResolveResult{
		Diagnostics: []ir.Diagnostic{
			{
				Path:     "semantic.def",
				Code:     "undefined-state",
				Severity: ir.SeverityError,
				Message:  "semantic",
				Start:    sourcePosition(5, 1),
				End:      sourcePosition(5, 3),
			},
		},
	}
	diag := []ir.Diagnostic{
		{
			Path:     "direct.def",
			Code:     "x",
			Severity: ir.SeverityWarning,
			Message:  "direct",
			Start:    sourcePosition(1, 1),
			End:      sourcePosition(1, 2),
		},
	}

	wsJSON, err := JSONFromWorkspace(ws)
	if err != nil {
		t.Fatalf("expected JSONFromWorkspace: %v", err)
	}
	semJSON, err := JSONFromSemantics(sem)
	if err != nil {
		t.Fatalf("expected JSONFromSemantics: %v", err)
	}
	diagJSON, err := JSONFromDiagnostics(diag)
	if err != nil {
		t.Fatalf("expected JSONFromDiagnostics: %v", err)
	}

	var wsReport, semReport, diagReport Report
	if err := json.Unmarshal(wsJSON, &wsReport); err != nil {
		t.Fatalf("expected workspace JSON decode: %v", err)
	}
	if err := json.Unmarshal(semJSON, &semReport); err != nil {
		t.Fatalf("expected semantics JSON decode: %v", err)
	}
	if err := json.Unmarshal(diagJSON, &diagReport); err != nil {
		t.Fatalf("expected direct JSON decode: %v", err)
	}

	if len(wsReport.Diagnostics) != 1 || wsReport.Diagnostics[0].Code != "missing-source" {
		t.Fatalf("expected workspace decode to preserve diagnostics, got %#v", wsReport.Diagnostics)
	}
	if len(semReport.Diagnostics) != 1 || semReport.Diagnostics[0].Code != "undefined-state" {
		t.Fatalf("expected semantics decode to preserve diagnostics, got %#v", semReport.Diagnostics)
	}
	if len(diagReport.Diagnostics) != 1 || diagReport.Diagnostics[0].Path != "direct.def" {
		t.Fatalf("expected diagnostics decode to preserve diagnostics, got %#v", diagReport.Diagnostics)
	}

	compact := HumanFromDiagnostics(diag)
	if compact == "" || compact == "No diagnostics." {
		t.Fatalf("expected compact human output for direct diagnostics, got %q", compact)
	}
	if _, err := JSONFromWorkspaceResult(ws); err != nil {
		t.Fatalf("expected JSONFromWorkspaceResult alias: %v", err)
	}
	if _, err := JSONFromSemanticsResult(sem); err != nil {
		t.Fatalf("expected JSONFromSemanticsResult alias: %v", err)
	}
	if HumanFromWorkspaceResult(ws) == "" {
		t.Fatalf("expected HumanFromWorkspaceResult output")
	}
	if HumanFromSemanticsResult(sem) == "" {
		t.Fatalf("expected HumanFromSemanticsResult output")
	}
	if Compact(diag) != compact {
		t.Fatalf("expected Compact to match HumanFromDiagnostics")
	}
}

func TestWorkspaceAndSemanticsMergeAndSortForOutput(t *testing.T) {
	ws := workspace.LoadResult{
		Diagnostics: []ir.Diagnostic{
			{
				Path:     "workspace.def",
				Code:     "missing-source",
				Severity: ir.SeverityError,
				Message:  "from workspace",
				Start:    sourcePosition(9, 2),
				End:      sourcePosition(9, 3),
			},
		},
	}
	sem := semantics.ResolveResult{
		Diagnostics: []ir.Diagnostic{
			{
				Path:          "semantic.def",
				Code:          "undefined-state",
				Severity:      ir.SeverityError,
				Message:       "from semantics",
				Start:         sourcePosition(1, 1),
				End:           sourcePosition(1, 4),
				RelatedSymbol: "state:1",
			},
			{
				Path:          "semantic.def",
				Code:          "duplicate-definition",
				Severity:      ir.SeverityWarning,
				Message:       "from semantics",
				Start:         sourcePosition(2, 1),
				End:           sourcePosition(2, 10),
				RelatedSymbol: "state:2",
			},
		},
	}

	report := FromWorkspaceAndSemantics(ws, sem)
	if len(report.Diagnostics) != 3 {
		t.Fatalf("expected 3 merged diagnostics, got %d", len(report.Diagnostics))
	}
	if report.Diagnostics[0].Path != "semantic.def" || report.Diagnostics[1].Path != "semantic.def" || report.Diagnostics[2].Path != "workspace.def" {
		t.Fatalf("expected stable path-based ordering, got %#v", report.Diagnostics)
	}

	human := report.Human()
	for _, expected := range []string{
		"semantic.def",
		"workspace.def",
		"from workspace",
		"from semantics",
		"undefined-state",
		"duplicate-definition",
		"state:1",
	} {
		if !strings.Contains(human, expected) {
			t.Fatalf("expected human output to include %q, got %q", expected, human)
		}
	}
}

func TestJSONFromWorkspaceAndSemanticsIsSortedAndStable(t *testing.T) {
	ws := workspace.LoadResult{Diagnostics: []ir.Diagnostic{
		{Path: "zeta.def", Code: "z", Severity: ir.SeverityError, Start: sourcePosition(3, 1), End: sourcePosition(3, 2)},
	}}
	sem := semantics.ResolveResult{Diagnostics: []ir.Diagnostic{
		{Path: "alpha.def", Code: "a", Severity: ir.SeverityWarning, Start: sourcePosition(1, 1), End: sourcePosition(1, 2)},
	}}

	a, err := JSONFromWorkspaceAndSemantics(ws, sem)
	if err != nil {
		t.Fatalf("expected workspace+semantics JSON helper to succeed: %v", err)
	}
	b, err := JSONFromWorkspaceAndSemantics(ws, sem)
	if err != nil {
		t.Fatalf("expected deterministic second helper call: %v", err)
	}
	if string(a) != string(b) {
		t.Fatalf("expected deterministic helper output: %q vs %q", a, b)
	}

	var decoded Report
	if err := json.Unmarshal(a, &decoded); err != nil {
		t.Fatalf("expected helper JSON to decode: %v", err)
	}
	if len(decoded.Diagnostics) != 2 {
		t.Fatalf("expected 2 decoded diagnostics, got %d", len(decoded.Diagnostics))
	}
	if decoded.Diagnostics[0].Path != "alpha.def" || decoded.Diagnostics[1].Path != "zeta.def" {
		t.Fatalf("expected helper output ordering by path, got %#v", decoded.Diagnostics)
	}
}

func equalDiagnosticSets(lhs, rhs []ir.Diagnostic) bool {
	if len(lhs) != len(rhs) {
		return false
	}
	for i := range lhs {
		if lhs[i] != rhs[i] {
			return false
		}
	}
	return true
}

func sourcePosition(line, column int) ir.SourcePosition {
	return ir.SourcePosition{Line: line, Column: column}
}
