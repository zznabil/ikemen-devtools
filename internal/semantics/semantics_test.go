package semantics

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/parser"
)

func TestResolveScopesSymbolsToCharacterPackage(t *testing.T) {
	root := t.TempDir()
	docs := []ir.Document{}
	for _, name := range []string{"A", "B"} {
		base := filepath.Join(root, "chars", name)
		docs = append(docs,
			*parser.Parse(filepath.Join(base, name+".def"), "[Info]\nname = "+name+"\n"),
			*parser.Parse(filepath.Join(base, "commands.cmd"), "[Command]\nname = \"x\"\n"),
			*parser.Parse(filepath.Join(base, "states.cns"), "[Statedef 100]\ntype = S\n[State 100, next]\ntype = ChangeState\nvalue = 100\ntrigger1 = command = \"x\"\n"),
		)
	}
	result := Resolve(NewMemoryWorkspace(docs...))
	for _, d := range result.Diagnostics {
		if d.Code == "ambiguous-state" || d.Code == "ambiguous-command" {
			t.Fatalf("cross-character symbol leaked into package resolution: %#v", d)
		}
	}
	for _, ref := range result.References {
		if !ref.IsDynamic && !ref.Resolved {
			t.Fatalf("package-local reference did not resolve: %#v", ref)
		}
	}
}

func TestResolveNumericStateAndCommandReferences(t *testing.T) {
	workspace := NewMemoryWorkspace(
		ir.Document{
			Path: "states.def",
			Symbols: []ir.Symbol{
				{
					ID:      symbolID("state:100", "states.def", 2),
					Kind:    ir.SymbolStateDef,
					Name:    "state:100",
					Section: "Statedef 100",
					Span:    span(2, 1, 2, 12),
				},
				{
					ID:   symbolID("state-controller:100", "states.def", 5),
					Kind: ir.SymbolStateController,
					Name: "state-controller:100",
					Span: span(5, 1, 5, 20),
				},
			},
			References: []ir.Reference{
				{
					ID:           referenceID("state", "states.def", 3),
					Kind:         ir.ReferenceState,
					Name:         "state:200",
					Target:       "state:200",
					SourceSymbol: symbolID("state-controller:100", "states.def", 5),
					Span:         span(3, 1, 3, 16),
					IsDynamic:    false,
				},
				{
					ID:           referenceID("command", "states.def", 4),
					Kind:         ir.ReferenceCommand,
					Name:         "command:jump",
					Target:       "command:jump",
					SourceSymbol: symbolID("state-controller:100", "states.def", 5),
					Span:         span(4, 1, 4, 24),
					IsDynamic:    false,
				},
			},
		},
		ir.Document{
			Path:     "commands.cmd",
			FileType: "cmd",
			Symbols: []ir.Symbol{
				{
					ID:      symbolID("command:jump", "commands.cmd", 2),
					Kind:    ir.SymbolCommand,
					Name:    "command:jump",
					Section: "Command",
					Span:    span(2, 1, 2, 17),
				},
			},
		},
		ir.Document{
			Path:     "next.def",
			FileType: "def",
			Symbols: []ir.Symbol{
				{
					ID:      symbolID("state:200", "next.def", 2),
					Kind:    ir.SymbolStateDef,
					Name:    "state:200",
					Section: "Statedef 200",
					Span:    span(2, 1, 2, 12),
				},
			},
		},
	)

	result := Resolve(workspace)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", result.Diagnostics)
	}

	resByID := make(map[string]ReferenceResolution)
	for _, ref := range result.References {
		resByID[ref.ReferenceID] = ref
	}

	stateRes, ok := resByID[referenceID("state", "states.def", 3)]
	if !ok {
		t.Fatalf("missing state reference resolution")
	}
	if !stateRes.Resolved {
		t.Fatalf("expected numeric state reference to resolve")
	}
	if stateRes.Classification != ExactResolution {
		t.Fatalf("expected exact state resolution, got %q", stateRes.Classification)
	}
	if stateRes.TargetSymbolID != symbolID("state:200", "next.def", 2) {
		t.Fatalf("unexpected state target: %q", stateRes.TargetSymbolID)
	}

	cmdRes := resByID[referenceID("command", "states.def", 4)]
	if !cmdRes.Resolved || cmdRes.Classification != ExactResolution || cmdRes.TargetSymbolID != symbolID("command:jump", "commands.cmd", 2) {
		t.Fatalf("expected command reference to resolve exactly, got %#v", cmdRes)
	}
}

func TestResolveUndefinedReferences(t *testing.T) {
	workspace := NewMemoryWorkspace(
		ir.Document{
			Path: "main.def",
			Symbols: []ir.Symbol{
				{
					ID:   symbolID("state-controller:100", "main.def", 2),
					Kind: ir.SymbolStateController,
					Name: "state-controller:100",
					Span: span(2, 1, 2, 20),
				},
			},
			References: []ir.Reference{
				{
					ID:           referenceID("state", "main.def", 10),
					Kind:         ir.ReferenceState,
					Name:         "state:999",
					Target:       "state:999",
					SourceSymbol: symbolID("state-controller:100", "main.def", 2),
					Span:         span(3, 1, 3, 16),
					IsDynamic:    false,
				},
				{
					ID:           referenceID("command", "main.def", 11),
					Kind:         ir.ReferenceCommand,
					Name:         "command:unknown",
					Target:       "command:unknown",
					SourceSymbol: symbolID("state-controller:100", "main.def", 2),
					Span:         span(4, 1, 4, 23),
					IsDynamic:    false,
				},
			},
		},
		ir.Document{
			Path:     "cmds.cmd",
			FileType: "cmd",
			Symbols: []ir.Symbol{
				{
					ID:   symbolID("command:jump", "cmds.cmd", 2),
					Kind: ir.SymbolCommand,
					Name: "command:jump",
					Span: span(2, 1, 2, 17),
				},
			},
		},
	)

	result := Resolve(workspace)
	if len(result.Diagnostics) != 2 {
		t.Fatalf("expected two diagnostics, got %d", len(result.Diagnostics))
	}

	refByID := make(map[string]ReferenceResolution)
	for _, ref := range result.References {
		refByID[ref.ReferenceID] = ref
	}
	stateRef := refByID[referenceID("state", "main.def", 10)]
	if stateRef.Classification != InvalidResolution {
		t.Fatalf("expected state reference classification %q, got %q", InvalidResolution, stateRef.Classification)
	}
	commandRef := refByID[referenceID("command", "main.def", 11)]
	if commandRef.Classification != InvalidResolution {
		t.Fatalf("expected command reference classification %q, got %q", InvalidResolution, commandRef.Classification)
	}

	codes := collectCodes(result.Diagnostics)
	sort.Strings(codes)
	expected := []string{"undefined-command", "undefined-state"}
	sort.Strings(expected)
	if !reflect.DeepEqual(codes, expected) {
		t.Fatalf("unexpected diagnostic codes: %#v", codes)
	}

	for _, d := range result.Diagnostics {
		switch d.Code {
		case "undefined-state":
			if d.Path != "main.def" {
				t.Fatalf("unexpected path for undefined-state: %q", d.Path)
			}
			if d.Start.Line != 3 {
				t.Fatalf("unexpected line for undefined-state: %#v", d.Start)
			}
		case "undefined-command":
			if d.Start.Line != 4 {
				t.Fatalf("unexpected line for undefined-command: %#v", d.Start)
			}
		default:
			t.Fatalf("unexpected code %q", d.Code)
		}
	}
}

func TestResolveStateAmbiguityWhileDuplicateCommandsResolve(t *testing.T) {
	workspace := NewMemoryWorkspace(
		ir.Document{
			Path: "main.def",
			Symbols: []ir.Symbol{
				{ID: symbolID("state:10", "main.def", 2), Kind: ir.SymbolStateDef, Name: "state:10", Span: span(2, 1, 2, 11)},
				{ID: symbolID("command:jump", "main.def", 3), Kind: ir.SymbolCommand, Name: "command:jump", Span: span(3, 1, 3, 20)},
			},
			References: []ir.Reference{
				{ID: referenceID("state", "main.def", 10), Kind: ir.ReferenceState, Target: "state:10", SourceSymbol: "state-controller:1", Span: span(10, 1, 10, 12)},
				{ID: referenceID("command", "main.def", 11), Kind: ir.ReferenceCommand, Target: "command:jump", SourceSymbol: "state-controller:1", Span: span(11, 1, 11, 16)},
			},
		},
		ir.Document{
			Path: "duplicate.def",
			Symbols: []ir.Symbol{
				{ID: symbolID("state:10", "duplicate.def", 2), Kind: ir.SymbolStateDef, Name: "state:10", Span: span(2, 1, 2, 11)},
				{ID: symbolID("command:jump", "duplicate.def", 3), Kind: ir.SymbolCommand, Name: "command:jump", Span: span(3, 1, 3, 20)},
			},
		},
	)

	result := Resolve(workspace)
	codes := collectCodes(result.Diagnostics)
	sort.Strings(codes)
	expected := []string{"ambiguous-state"}
	if !reflect.DeepEqual(codes, expected) {
		t.Fatalf("expected ambiguity diagnostics %#v, got %#v", expected, codes)
	}
	for _, ref := range result.References {
		if ref.TargetPath == "duplicate.def" {
			if !ref.Resolved || ref.Classification != ExactResolution {
				t.Fatalf("duplicate command names should resolve deterministically, got %#v", ref)
			}
		} else if ref.Classification != AmbiguousResolution || ref.Resolved {
			t.Fatalf("expected unresolved ambiguous state reference, got %#v", ref)
		}
	}
	for _, d := range result.Diagnostics {
		if d.Code == "ambiguous-state" {
			if d.Severity != ir.SeverityError {
				t.Fatalf("expected ambiguity diagnostic to be an error, got %#v", d)
			}
		}
	}
}

func TestResolveDynamicStateReferenceWarning(t *testing.T) {
	workspace := NewMemoryWorkspace(
		ir.Document{
			Path: "dynamic.def",
			References: []ir.Reference{
				{
					ID:           referenceID("state", "dynamic.def", 1),
					Kind:         ir.ReferenceState,
					Name:         "state:VarState",
					Target:       "state:VarState",
					SourceSymbol: symbolID("state-controller:1", "dynamic.def", 1),
					Span:         span(1, 1, 1, 20),
					IsDynamic:    true,
				},
			},
			Symbols: []ir.Symbol{
				{
					ID:   symbolID("state-controller:1", "dynamic.def", 1),
					Kind: ir.SymbolStateController,
					Name: "state-controller:1",
					Span: span(1, 1, 1, 20),
				},
			},
		},
	)

	result := Resolve(workspace)
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %d", len(result.Diagnostics))
	}
	d := result.Diagnostics[0]
	if d.Code != "dynamic-reference" {
		t.Fatalf("expected dynamic-reference diagnostic, got %q", d.Code)
	}
	if d.Severity != ir.SeverityWarning {
		t.Fatalf("expected warning for dynamic reference, got %q", d.Severity)
	}
	res := result.References[0]
	if res.Resolved {
		t.Fatalf("dynamic references should not resolve")
	}
	if res.Classification != DynamicResolution {
		t.Fatalf("expected dynamic resolution classification, got %q", res.Classification)
	}
	if !res.IsDynamic {
		t.Fatalf("expected dynamic flag on resolved reference")
	}
}

func TestResolveDuplicateDefinitions(t *testing.T) {
	workspace := NewMemoryWorkspace(
		ir.Document{
			Path: "first.def",
			Symbols: []ir.Symbol{
				{ID: symbolID("state:300", "first.def", 2), Kind: ir.SymbolStateDef, Name: "state:300", Span: span(2, 1, 2, 12)},
				{ID: symbolID("command:punch", "first.def", 3), Kind: ir.SymbolCommand, Name: "command:punch", Span: span(3, 1, 3, 24)},
			},
		},
		ir.Document{
			Path: "first.def",
			Symbols: []ir.Symbol{
				{ID: symbolID("state:300", "first.def", 8), Kind: ir.SymbolStateDef, Name: "state:300", Span: span(8, 1, 8, 12)},
				{ID: symbolID("command:punch", "first.def", 9), Kind: ir.SymbolCommand, Name: "command:punch", Span: span(9, 1, 9, 24)},
			},
		},
	)

	result := Resolve(workspace)
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected one duplicate-definition diagnostic for repeated states, got %d", len(result.Diagnostics))
	}
	d := result.Diagnostics[0]
	if d.Code != "duplicate-definition" {
		t.Fatalf("expected duplicate-definition code, got %q", d.Code)
	}
	if d.Path != "first.def" {
		t.Fatalf("expected duplicate diagnostic on first path, got %q", d.Path)
	}
}

func TestResolveDuplicateCommandDefinitionsDeterministically(t *testing.T) {
	workspace := NewMemoryWorkspace(
		ir.Document{
			Path: "cmds/main.cmd",
			Symbols: []ir.Symbol{
				{ID: symbolID("command:jump", "cmds/main.cmd", 2), Kind: ir.SymbolCommand, Name: "command:jump", Span: span(2, 1, 2, 16)},
			},
		},
		ir.Document{
			Path: "cmds/alt.cmd",
			Symbols: []ir.Symbol{
				{ID: symbolID("command:jump", "cmds/alt.cmd", 3), Kind: ir.SymbolCommand, Name: "command:jump", Span: span(3, 1, 3, 17)},
			},
		},
		ir.Document{
			Path: "combat.def",
			Symbols: []ir.Symbol{
				{ID: symbolID("state-controller:0", "combat.def", 10), Kind: ir.SymbolStateController, Name: "state-controller:0", Span: span(10, 1, 10, 24)},
			},
			References: []ir.Reference{
				{
					ID:           referenceID("command", "combat.def", 11),
					Kind:         ir.ReferenceCommand,
					Name:         "command:jump",
					Target:       "command:jump",
					SourceSymbol: symbolID("state-controller:0", "combat.def", 10),
					Span:         span(11, 1, 11, 24),
					IsDynamic:    false,
				},
			},
		},
	)

	result := Resolve(workspace)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("duplicate command names are valid MUGEN input aliases: %#v", result.Diagnostics)
	}
	if len(result.References) != 1 {
		t.Fatalf("expected one reference resolution, got %d", len(result.References))
	}

	res := result.References[0]
	if res.Classification != ExactResolution || !res.Resolved {
		t.Fatalf("expected deterministic command resolution, got %#v", res)
	}
	if filepath.ToSlash(res.TargetPath) != "cmds/alt.cmd" {
		t.Fatalf("expected first sorted definition, got %q", res.TargetPath)
	}
}

func TestResolveClassifiesDataCommonReferencesAsRuntimeDynamic(t *testing.T) {
	doc := ir.Document{
		Path: filepath.Join("game", "data", "common.cns"),
		References: []ir.Reference{{
			ID:     "ref:command",
			Kind:   ir.ReferenceCommand,
			Target: "command:charge",
			Span:   span(3, 1, 3, 20),
		}},
	}
	result := Resolve(NewMemoryWorkspace(doc))
	if len(result.References) != 1 || result.References[0].Classification != DynamicResolution {
		t.Fatalf("expected runtime-dynamic common reference, got %#v", result.References)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "dynamic-reference" {
		t.Fatalf("expected one dynamic warning, got %#v", result.Diagnostics)
	}
}

func TestStableDiagnosticOrder(t *testing.T) {
	workspace := NewMemoryWorkspace(
		ir.Document{
			Path: "zeta.def",
			References: []ir.Reference{
				{
					ID:        referenceID("state", "zeta.def", 5),
					Kind:      ir.ReferenceState,
					Name:      "state:1",
					Target:    "state:1",
					Span:      span(5, 3, 5, 14),
					IsDynamic: true,
				},
			},
			Symbols: []ir.Symbol{
				{ID: symbolID("state-controller:z", "zeta.def", 1), Kind: ir.SymbolStateController, Name: "state-controller:z", Span: span(1, 1, 1, 12)},
			},
		},
		ir.Document{
			Path: "alpha.def",
			Symbols: []ir.Symbol{
				{ID: symbolID("state:1", "alpha.def", 2), Kind: ir.SymbolStateDef, Name: "state:1", Span: span(2, 1, 2, 12)},
				{ID: symbolID("command:jump", "alpha.def", 4), Kind: ir.SymbolCommand, Name: "command:jump", Span: span(4, 1, 4, 17)},
			},
			References: []ir.Reference{
				{
					ID:        referenceID("command", "alpha.def", 9),
					Kind:      ir.ReferenceCommand,
					Name:      "command:missing",
					Target:    "command:missing",
					Span:      span(9, 1, 9, 24),
					IsDynamic: false,
				},
			},
		},
		ir.Document{
			Path: "beta.def",
			Symbols: []ir.Symbol{
				{ID: symbolID("state:2", "beta.def", 2), Kind: ir.SymbolStateDef, Name: "state:2", Span: span(2, 1, 2, 12)},
			},
			References: []ir.Reference{
				{
					ID:        referenceID("state", "beta.def", 7),
					Kind:      ir.ReferenceState,
					Name:      "state:999",
					Target:    "state:999",
					Span:      span(7, 2, 7, 13),
					IsDynamic: false,
				},
			},
		},
	)

	result := Resolve(workspace)
	if len(result.Diagnostics) != 3 {
		t.Fatalf("expected 3 diagnostics, got %d", len(result.Diagnostics))
	}

	gotOrder := []string{}
	for _, d := range result.Diagnostics {
		gotOrder = append(gotOrder, fmt.Sprintf("%s:%d:%s", d.Path, d.Start.Line, d.Code))
	}
	expectedOrder := []string{
		"alpha.def:9:undefined-command",
		"beta.def:7:undefined-state",
		"zeta.def:5:dynamic-reference",
	}
	if !reflect.DeepEqual(gotOrder, expectedOrder) {
		t.Fatalf("expected diagnostics ordered as %#v, got %#v", expectedOrder, gotOrder)
	}
}

func TestDeterministicIndexingAcrossDocuments(t *testing.T) {
	workspace := NewMemoryWorkspace(
		ir.Document{
			Path: "zeta.def",
			Symbols: []ir.Symbol{
				{ID: symbolID("state:11", "zeta.def", 3), Kind: ir.SymbolStateDef, Name: "state:11", Span: span(3, 1, 3, 11)},
				{ID: symbolID("command:jump", "zeta.def", 8), Kind: ir.SymbolCommand, Name: "command:jump", Span: span(8, 1, 8, 16)},
			},
		},
		ir.Document{
			Path: "alpha.def",
			Symbols: []ir.Symbol{
				{ID: symbolID("state:1", "alpha.def", 2), Kind: ir.SymbolStateDef, Name: "state:1", Span: span(2, 1, 2, 11)},
				{ID: symbolID("command:fire", "alpha.def", 4), Kind: ir.SymbolCommand, Name: "command:fire", Span: span(4, 1, 4, 15)},
			},
		},
		ir.Document{
			Path: "alpha.def",
			Symbols: []ir.Symbol{
				{ID: symbolID("state:1", "alpha.def", 20), Kind: ir.SymbolStateDef, Name: "state:1", Span: span(20, 1, 20, 11)},
			},
		},
	)

	result := Resolve(workspace)
	names := make([]string, 0, len(result.Index))
	for _, entry := range result.Index {
		names = append(names, entry.Name)
	}
	expectedNames := []string{"command:fire", "command:jump", "state:1", "state:11"}
	if !reflect.DeepEqual(names, expectedNames) {
		t.Fatalf("unexpected index symbol order: %#v", names)
	}

	if len(result.Index[2].Symbols) != 2 {
		t.Fatalf("expected duplicate entries for state:1")
	}
	if result.Index[2].Symbols[0].Path != "alpha.def" || result.Index[2].Symbols[1].Path != "alpha.def" {
		t.Fatalf("unexpected state:1 symbol ordering: %#v", result.Index[2].Symbols)
	}
	if result.Index[2].Symbols[0].Symbol.Span.Start.Line > result.Index[2].Symbols[1].Symbol.Span.Start.Line {
		t.Fatalf("expected duplicate symbols sorted by source order, got %#v", result.Index[2].Symbols)
	}

	if result.Index[2].Symbols[0].Symbol.ID != symbolID("state:1", "alpha.def", 2) {
		t.Fatalf("expected earliest state:1 definition to be %q, got %q", symbolID("state:1", "alpha.def", 2), result.Index[2].Symbols[0].Symbol.ID)
	}
}
func TestResolveRepeatedReferencesPreserveIdentityDistinctly(t *testing.T) {
	controllerDoc := parser.Parse("combat.def", `[State 100]
type = ChangeState
value = 100
trigger1 = command = "jump" || command = "jump"
`)
	commandDoc := parser.Parse("commands.cmd", `[Command]
name = "jump"
`)

	result := Resolve(NewMemoryWorkspace(*controllerDoc, *commandDoc))
	commandRefs := make([]ReferenceResolution, 0, 2)
	for _, ref := range result.References {
		if ref.ReferenceIdentity.SemanticKey == "command:jump" {
			commandRefs = append(commandRefs, ref)
		}
	}
	if len(commandRefs) != 2 {
		t.Fatalf("expected 2 command references, got %d", len(commandRefs))
	}

	for i, ref := range commandRefs {
		if !ref.Resolved || ref.Classification != ExactResolution {
			t.Fatalf("reference %d expected exact resolution, got resolved=%v classification=%q", i, ref.Resolved, ref.Classification)
		}
		if ref.ReferenceIdentity.SemanticKey != "command:jump" {
			t.Fatalf("reference %d expected semantic key %q, got %q", i, "command:jump", ref.ReferenceIdentity.SemanticKey)
		}
		if ref.TargetIdentity.SemanticKey != "command:jump" {
			t.Fatalf("reference %d expected target semantic key %q, got %q", i, "command:jump", ref.TargetIdentity.SemanticKey)
		}
	}

	if commandRefs[0].ReferenceIdentity.OccurrenceID == "" {
		t.Fatalf("expected first reference occurrence id to be present")
	}
	if commandRefs[0].ReferenceIdentity.OccurrenceID != commandRefs[1].ReferenceIdentity.OccurrenceID {
		t.Fatalf("expected repeated reference occurrence IDs to match: %q, %q", commandRefs[0].ReferenceIdentity.OccurrenceID, commandRefs[1].ReferenceIdentity.OccurrenceID)
	}
	if commandRefs[0].ReferenceIdentity.StoreID == commandRefs[1].ReferenceIdentity.StoreID {
		t.Fatalf("expected repeated references to have distinct store IDs, got %q", commandRefs[0].ReferenceIdentity.StoreID)
	}
}

func TestResolveRepeatedReferencesAcrossDuplicateCommands(t *testing.T) {
	controllerDoc := parser.Parse("combat.def", `[State 100]
type = ChangeState
value = 100
trigger1 = command = "jump" || command = "jump"
`)
	commandDocA := parser.Parse("commands/a.cmd", `[Command]
name = "jump"
`)
	commandDocB := parser.Parse("commands/b.cmd", `[Command]
name = "jump"
`)

	result := Resolve(NewMemoryWorkspace(*controllerDoc, *commandDocA, *commandDocB))
	commandRefs := make([]ReferenceResolution, 0, 2)
	for _, ref := range result.References {
		if ref.ReferenceIdentity.SemanticKey == "command:jump" {
			commandRefs = append(commandRefs, ref)
		}
	}
	if len(commandRefs) != 2 {
		t.Fatalf("expected 2 command references, got %d", len(commandRefs))
	}

	for _, diag := range result.Diagnostics {
		if diag.Code == "ambiguous-command" {
			t.Fatalf("duplicate command names should not be ambiguous: %#v", result.Diagnostics)
		}
	}

	for i, ref := range commandRefs {
		if !ref.Resolved || ref.Classification != ExactResolution {
			t.Fatalf("reference %d expected exact resolution, got resolved=%v classification=%q", i, ref.Resolved, ref.Classification)
		}
		if ref.ReferenceIdentity.SemanticKey != "command:jump" {
			t.Fatalf("reference %d expected semantic key %q, got %q", i, "command:jump", ref.ReferenceIdentity.SemanticKey)
		}
	}

	if commandRefs[0].ReferenceIdentity.OccurrenceID == "" || commandRefs[1].ReferenceIdentity.OccurrenceID == "" {
		t.Fatalf("expected occurrence IDs to be present for repeated references")
	}
	if commandRefs[0].ReferenceIdentity.OccurrenceID != commandRefs[1].ReferenceIdentity.OccurrenceID {
		t.Fatalf("expected repeated references to share occurrence ID: %q, %q", commandRefs[0].ReferenceIdentity.OccurrenceID, commandRefs[1].ReferenceIdentity.OccurrenceID)
	}
	if commandRefs[0].ReferenceIdentity.StoreID == commandRefs[1].ReferenceIdentity.StoreID {
		t.Fatalf("expected repeated references to have distinct store IDs, got %q", commandRefs[0].ReferenceIdentity.StoreID)
	}
}

func symbolID(fullName string, path string, line int) string {
	return fmt.Sprintf("%s:%d:%s", fullName, line, filepath.ToSlash(filepath.Clean(path)))
}

func referenceID(kind string, path string, line int) string {
	return fmt.Sprintf("ref:%s:%d:%s", kind, line, filepath.ToSlash(filepath.Clean(path)))
}

func span(startLine, startColumn, endLine, endColumn int) ir.SourceSpan {
	return ir.SourceSpan{Start: ir.SourcePosition{Line: startLine, Column: startColumn}, End: ir.SourcePosition{Line: endLine, Column: endColumn}}
}

func collectCodes(diags []ir.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Code)
	}
	return out
}
