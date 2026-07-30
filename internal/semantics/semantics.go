package semantics

import (
	"fmt"
	"sort"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
)

// Workspace provides document access for the resolver.
type Workspace interface {
	Documents() []ir.Document
}

// MemoryWorkspace is an in-memory collection of documents.
type MemoryWorkspace struct {
	documents []ir.Document
}

// NewMemoryWorkspace builds a minimal workspace from the provided documents.
func NewMemoryWorkspace(docs ...ir.Document) *MemoryWorkspace {
	return &MemoryWorkspace{documents: append([]ir.Document(nil), docs...)}
}

// Documents returns the workspace documents.
func (w *MemoryWorkspace) Documents() []ir.Document {
	if w == nil {
		return nil
	}
	return append([]ir.Document(nil), w.documents...)
}

// SymbolIndexEntry stores an indexed symbol plus its originating path.
type SymbolIndexEntry struct {
	Name    string
	Symbols []IndexedSymbol
}

// IndexedSymbol augments an IR symbol with file path.
type IndexedSymbol struct {
	Path   string
	Symbol ir.Symbol
}

// ReferenceResolution stores deterministic semantic resolution outcomes for a single parsed reference.
// String fields are retained for legacy compatibility; identities are explicit for contract conformance.
type ReferenceResolution struct {
	ReferenceID       string
	ReferenceIdentity ir.Identity
	SourcePath        string
	SourceSymbol      string
	TargetSymbolID    string
	TargetIdentity    ir.Identity
	TargetPath        string
	Classification    string
	Resolved          bool
	IsDynamic         bool
}

const (
	ExactResolution     = "exact"
	AmbiguousResolution = "ambiguous"
	InvalidResolution   = "invalid"
	DynamicResolution   = "dynamic"
)

// ResolveResult contains deterministic semantic outcomes.
type ResolveResult struct {
	Index       []SymbolIndexEntry
	References  []ReferenceResolution
	Diagnostics []ir.Diagnostic
}

// Resolve analyzes references across documents.
func Resolve(workspace Workspace) ResolveResult {
	if workspace == nil {
		return ResolveResult{}
	}
	documents := append([]ir.Document(nil), workspace.Documents()...)
	sort.Slice(documents, func(i, j int) bool {
		if documents[i].Path == documents[j].Path {
			return documents[i].FileType < documents[j].FileType
		}
		return documents[i].Path < documents[j].Path
	})

	type indexedSymbol struct {
		path   string
		symbol ir.Symbol
	}

	indexByName := map[string][]indexedSymbol{}

	for _, doc := range documents {
		for _, symbol := range doc.Symbols {
			switch symbol.Kind {
			case ir.SymbolStateDef, ir.SymbolCommand:
				indexByName[symbol.Name] = append(indexByName[symbol.Name], indexedSymbol{path: doc.Path, symbol: symbol})
			}
		}
	}
	result := ResolveResult{}

	names := make([]string, 0, len(indexByName))
	for name := range indexByName {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		symbols := indexByName[name]

		sort.Slice(symbols, func(i, j int) bool {
			if symbols[i].path != symbols[j].path {
				return symbols[i].path < symbols[j].path
			}
			if symbols[i].symbol.Span.Start.Line != symbols[j].symbol.Span.Start.Line {
				return symbols[i].symbol.Span.Start.Line < symbols[j].symbol.Span.Start.Line
			}
			if symbols[i].symbol.Span.Start.Column != symbols[j].symbol.Span.Start.Column {
				return symbols[i].symbol.Span.Start.Column < symbols[j].symbol.Span.Start.Column
			}
			return symbols[i].symbol.ID < symbols[j].symbol.ID
		})

		result.Index = append(result.Index, SymbolIndexEntry{
			Name:    name,
			Symbols: make([]IndexedSymbol, len(symbols)),
		})
		for i := range symbols {
			result.Index[len(result.Index)-1].Symbols[i] = IndexedSymbol{
				Path:   symbols[i].path,
				Symbol: symbols[i].symbol,
			}
		}

		if len(symbols) > 1 && symbols[0].symbol.Kind != ir.SymbolCommand {
			for i := 1; i < len(symbols); i++ {
				sym := symbols[i]
				if sym.path != symbols[0].path {
					continue
				}
				result.Diagnostics = append(result.Diagnostics, makeSemanticDiagnostic(
					sym.path,
					sym.symbol.Span,
					symbols[0].symbol.ID,
					"duplicate-definition",
					ir.SeverityError,
					fmt.Sprintf("duplicate definition for %s", name),
				))
			}
		}
	}

	for _, doc := range documents {
		for _, ref := range doc.References {
			res := ReferenceResolution{
				ReferenceID:       ref.ID,
				ReferenceIdentity: ref.Identity,
				SourcePath:        doc.Path,
				SourceSymbol:      ref.SourceSymbol,
				Classification:    InvalidResolution,
				IsDynamic:         ref.IsDynamic,
			}

			if ref.Kind == ir.ReferenceState {
				if ref.IsDynamic {
					res.Classification = DynamicResolution
					result.Diagnostics = append(result.Diagnostics, makeSemanticDiagnostic(
						doc.Path,
						ref.Span,
						ref.SourceSymbol,
						"dynamic-reference",
						ir.SeverityWarning,
						fmt.Sprintf("dynamic state reference %q cannot be resolved", ref.Target),
					))
					result.References = append(result.References, res)
					continue
				}

				targets := indexByName[ref.Target]
				if len(targets) == 0 {
					res.Classification = InvalidResolution
					result.Diagnostics = append(result.Diagnostics, makeSemanticDiagnostic(
						doc.Path,
						ref.Span,
						ref.SourceSymbol,
						"undefined-state",
						ir.SeverityError,
						fmt.Sprintf("undefined state %q", ref.Target),
					))
					result.References = append(result.References, res)
					continue
				}
				if len(targets) > 1 {
					res.Classification = AmbiguousResolution
					result.Diagnostics = append(result.Diagnostics, makeSemanticDiagnostic(
						doc.Path,
						ref.Span,
						ref.SourceSymbol,
						"ambiguous-state",
						ir.SeverityError,
						fmt.Sprintf("state reference %q matches %d definitions", ref.Target, len(targets)),
					))
					result.References = append(result.References, res)
					continue
				}
				res.Resolved = true
				res.Classification = ExactResolution
				res.TargetSymbolID = targets[0].symbol.ID
				res.TargetIdentity = targets[0].symbol.Identity
				res.TargetPath = targets[0].path
				result.References = append(result.References, res)
				continue
			}

			if ref.Kind == ir.ReferenceCommand {
				targets := indexByName[ref.Target]
				if len(targets) == 0 {
					result.Diagnostics = append(result.Diagnostics, makeSemanticDiagnostic(
						doc.Path,
						ref.Span,
						ref.SourceSymbol,
						"undefined-command",
						ir.SeverityError,
						fmt.Sprintf("undefined command %q", ref.Target),
					))
					result.References = append(result.References, res)
					continue
				}
				if len(targets) > 1 {
					res.Classification = AmbiguousResolution
					result.Diagnostics = append(result.Diagnostics, makeSemanticDiagnostic(
						doc.Path,
						ref.Span,
						ref.SourceSymbol,
						"ambiguous-command",
						ir.SeverityError,
						fmt.Sprintf("command reference %q matches %d definitions", ref.Target, len(targets)),
					))
					result.References = append(result.References, res)
					continue
				}
				res.Resolved = true
				res.Classification = ExactResolution
				res.TargetSymbolID = targets[0].symbol.ID
				res.TargetIdentity = targets[0].symbol.Identity
				res.TargetPath = targets[0].path
				result.References = append(result.References, res)
				continue
			}

			result.References = append(result.References, res)
		}
	}
	sort.Slice(result.References, func(i, j int) bool {
		if result.References[i].SourcePath != result.References[j].SourcePath {
			return result.References[i].SourcePath < result.References[j].SourcePath
		}
		leftIdentity := result.References[i].ReferenceIdentity
		rightIdentity := result.References[j].ReferenceIdentity
		leftIdentityKey := leftIdentity.OccurrenceID
		rightIdentityKey := rightIdentity.OccurrenceID
		if leftIdentityKey == "" {
			leftIdentityKey = result.References[i].ReferenceID
		}
		if rightIdentityKey == "" {
			rightIdentityKey = result.References[j].ReferenceID
		}
		if leftIdentityKey != rightIdentityKey {
			return leftIdentityKey < rightIdentityKey
		}
		leftStoreID := leftIdentity.StoreID
		rightStoreID := rightIdentity.StoreID
		if leftStoreID != rightStoreID {
			if leftStoreID == "" && rightStoreID != "" {
				return false
			}
			if leftStoreID != "" && rightStoreID == "" {
				return true
			}
			return leftStoreID < rightStoreID
		}
		if leftIdentity.SemanticKey != rightIdentity.SemanticKey {
			return leftIdentity.SemanticKey < rightIdentity.SemanticKey
		}
		if result.References[i].ReferenceID != result.References[j].ReferenceID {
			return result.References[i].ReferenceID < result.References[j].ReferenceID
		}
		return result.References[i].SourceSymbol < result.References[j].SourceSymbol
	})

	sort.Slice(result.Diagnostics, func(i, j int) bool {
		lhs, rhs := result.Diagnostics[i], result.Diagnostics[j]
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
		return lhs.RelatedSymbol < rhs.RelatedSymbol
	})

	return result
}

func makeSemanticDiagnostic(path string, span ir.SourceSpan, relatedSymbol, code string, severity ir.Severity, message string) ir.Diagnostic {
	start := span.Start
	end := span.End
	if end.Line == 0 {
		end.Line = start.Line
	}
	if end.Column == 0 {
		end.Column = start.Column + 1
	}
	return ir.Diagnostic{
		Path:          path,
		Code:          code,
		Severity:      severity,
		Message:       message,
		Start:         normalizeStart(start),
		End:           normalizeEnd(start, end),
		RelatedSymbol: relatedSymbol,
	}
}

func normalizeStart(pos ir.SourcePosition) ir.SourcePosition {
	if pos.Line < 1 {
		return ir.SourcePosition{Line: 1, Column: 1}
	}
	if pos.Column < 1 {
		return ir.SourcePosition{Line: pos.Line, Column: 1}
	}
	return pos
}

func normalizeEnd(start, end ir.SourcePosition) ir.SourcePosition {
	if end.Line < 1 {
		end.Line = start.Line
	}
	if end.Column < 1 {
		end.Column = 1
	}
	if end.Line < start.Line || (end.Line == start.Line && end.Column <= start.Column) {
		end.Line = start.Line
		end.Column = start.Column + 1
	}
	return end
}
