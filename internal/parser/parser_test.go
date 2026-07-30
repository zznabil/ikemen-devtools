package parser

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
)

func FuzzParseContract(f *testing.F) {
	f.Add(`[Command]
name = "jump"
`)
	f.Add(`[State 200]
type = ChangeState
value = 100
trigger1 = command = "jump"`)
	f.Add(`[State 100]
type = SelfState
value = 20
trigger1 = command = "foo"`)

	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 4096 {
			return
		}

		docA := Parse("fuzz.def", source)
		docB := Parse("fuzz.def", source)
		if docA == nil || docB == nil {
			t.Fatalf("expected parser to return a document for bounded fuzz input")
		}
		if docA.Version != ir.IdentityContractVersion || docB.Version != ir.IdentityContractVersion {
			t.Fatalf("unexpected document contract version: %q / %q", docA.Version, docB.Version)
		}
		if !reflect.DeepEqual(docA, docB) {
			t.Fatalf("fuzz parse output is not deterministic")
		}
	})
}

func TestParseSectionsAndKeyValues(t *testing.T) {
	src := `[Info]
name = Hero

[Statedef 100]
statetype = S
`
	doc := Parse("a.def", src)

	if doc.FileType != "def" {
		t.Fatalf("unexpected file type: %q", doc.FileType)
	}
	if len(doc.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(doc.Sections))
	}
	if doc.Sections[0].Header != "Info" {
		t.Fatalf("unexpected first section: %q", doc.Sections[0].Header)
	}
	if doc.Sections[1].Header != "Statedef 100" {
		t.Fatalf("unexpected second section: %q", doc.Sections[1].Header)
	}
	if len(doc.Sections[1].Lines) != 1 {
		t.Fatalf("expected one key-value line in statedef section, got %d", len(doc.Sections[1].Lines))
	}
	line := doc.Sections[1].Lines[0]
	if line.Kind != ir.SourceLineKeyValue || line.Key != "statetype" || line.Value != "S" {
		t.Fatalf("unexpected line: %#v", line)
	}

	if len(doc.Symbols) != 1 {
		t.Fatalf("expected one symbol, got %d", len(doc.Symbols))
	}
	if doc.Symbols[0].Kind != ir.SymbolStateDef || doc.Symbols[0].Name != "state:100" {
		t.Fatalf("unexpected state symbol: %#v", doc.Symbols[0])
	}
}

func TestParseCommentsAndKeyValues(t *testing.T) {
	src := `[Command]
;comment one
name = "jump"
trigger1 = command = "jump"
`
	doc := Parse("b.cmd", src)

	if len(doc.Sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(doc.Sections))
	}
	lines := doc.Sections[0].Lines
	if len(lines) != 3 {
		t.Fatalf("expected 3 stored lines, got %d", len(lines))
	}
	if lines[0].Kind != ir.SourceLineComment || lines[1].Kind != ir.SourceLineKeyValue || lines[2].Kind != ir.SourceLineKeyValue {
		t.Fatalf("unexpected line kinds: %#v", lines)
	}

	if len(doc.Symbols) != 1 {
		t.Fatalf("expected one symbol, got %d", len(doc.Symbols))
	}
	if doc.Symbols[0].Name != "command:jump" {
		t.Fatalf("unexpected command symbol: %s", doc.Symbols[0].Name)
	}
}

func TestParseSourceSpans(t *testing.T) {
	src := `[Command]
name = "jump"
`
	doc := Parse("c.cmd", src)
	line := doc.Sections[0].Lines[0]
	if line.Span.Start.Line != 2 {
		t.Fatalf("expected source line 2, got %#v", line.Span)
	}
	if line.Span.Start.Column != 1 {
		t.Fatalf("expected key start at column 1, got %d", line.Span.Start.Column)
	}
	if line.Span.End.Line != 2 || line.Span.End.Column <= line.Span.Start.Column {
		t.Fatalf("expected key span with non-empty end, got %#v", line.Span)
	}
}

func TestParseSourceSpansPreserveLeadingIndent(t *testing.T) {
	src := `   [Command]
   name = "jump"
`
	doc := Parse("indent.cmd", src)
	if len(doc.Sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(doc.Sections))
	}
	sectionLine := doc.Sections[0].Span
	if sectionLine.Start.Column != 4 {
		t.Fatalf("expected section line start column 4, got %d", sectionLine.Start.Column)
	}
	line := doc.Sections[0].Lines[0]
	if line.Span.Start.Column != 4 {
		t.Fatalf("expected indented key start column 4, got %d", line.Span.Start.Column)
	}
}

func TestParseDuplicateTriggerKeysPreserved(t *testing.T) {
	src := `[State 200]
type = ChangeState
value = 500
trigger1 = command = "a"
trigger1 = command = "b"
`
	doc := Parse("d.st", src)

	triggers := 0
	for _, line := range doc.Sections[0].Lines {
		if line.Kind == ir.SourceLineKeyValue && line.Key == "trigger1" {
			triggers++
		}
	}
	if triggers != 2 {
		t.Fatalf("expected duplicate trigger1 lines preserved, got %d", triggers)
	}

	if len(doc.References) != 3 {
		t.Fatalf("expected 3 references (1 state, 2 command), got %d", len(doc.References))
	}
	if doc.References[0].Kind != ir.ReferenceState {
		t.Fatalf("first reference expected as state transition, got %q", doc.References[0].Kind)
	}
}

func TestParseCompoundCommandReferences(t *testing.T) {
	src := `[State 300]
type = HitDef
trigger1 = command = "a" || command = "b"
`
	doc := Parse("compound.st", src)

	if len(doc.References) != 2 {
		t.Fatalf("expected 2 command references, got %d", len(doc.References))
	}
	for i, ref := range []struct {
		name string
	}{{
		name: "command:a",
	}, {
		name: "command:b",
	}} {
		if doc.References[i].Target != ref.name {
			t.Fatalf("unexpected target order %d: %q", i, doc.References[i].Target)
		}
	}
}

func TestParseParenthesizedCommandReferences(t *testing.T) {
	src := `[State 300]
type = HitDef
trigger1 = (command = "a" || command = "b")
`
	doc := Parse("compound2.st", src)
	if len(doc.References) != 2 {
		t.Fatalf("expected 2 command references, got %d", len(doc.References))
	}
	if doc.References[0].Target != "command:a" || doc.References[1].Target != "command:b" {
		t.Fatalf("unexpected compound reference targets: %#v", []string{doc.References[0].Target, doc.References[1].Target})
	}
}
func TestParseStateIDNormalization(t *testing.T) {
	src := `[State 1]
[Statedef +1]
[State -10]
type = SelfState
value = +1
`
	doc := Parse("norm.st", src)

	if len(doc.Symbols) != 3 {
		t.Fatalf("expected three symbols, got %d", len(doc.Symbols))
	}
	if doc.Symbols[1].Name != "state:-10" {
		t.Fatalf("expected statedef +1 normalized to state:-10, got %q", doc.Symbols[1].Name)
	}
	if doc.Symbols[1].Kind != ir.SymbolStateDef {
		t.Fatalf("expected statedef symbol kind, got %q", doc.Symbols[1].Kind)
	}
	if len(doc.References) != 1 {
		t.Fatalf("expected one reference, got %d", len(doc.References))
	}
	ref := doc.References[0]
	if ref.Name != "state:-10" || ref.Target != "state:-10" {
		t.Fatalf("expected +1 normalized state reference, got name=%q target=%q", ref.Name, ref.Target)
	}
	if doc.Symbols[2].Name != "state-controller:-10" {
		t.Fatalf("expected state controller name normalized as state-controller:-10, got %q", doc.Symbols[2].Name)
	}
}

func TestParseSelfStateReference(t *testing.T) {
	src := `[State 100]
type = SelfState
value = 500
`
	doc := Parse("selfstate.st", src)
	if len(doc.References) != 1 {
		t.Fatalf("expected one reference, got %d", len(doc.References))
	}
	ref := doc.References[0]
	if ref.Kind != ir.ReferenceState {
		t.Fatalf("expected state reference, got %q", ref.Kind)
	}
	if ref.Target != "state:500" {
		t.Fatalf("unexpected state target: %q", ref.Target)
	}
	if ref.IsDynamic {
		t.Fatalf("expected static state reference")
	}
}

func TestCommandNamesMustBeQuoted(t *testing.T) {
	src := `[Command]
name = jump
[Command]
name = "slash"
`
	doc := Parse("command-quote.def", src)
	if len(doc.Symbols) != 2 {
		t.Fatalf("expected two command symbols, got %d", len(doc.Symbols))
	}
	if doc.Symbols[0].Name != "command:jump" {
		t.Fatalf("expected first command symbol, got %q", doc.Symbols[0].Name)
	}
	if len(doc.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic for unquoted command name, got %d", len(doc.Diagnostics))
	}
	if doc.Diagnostics[0].Code != "unquoted-command-name" {
		t.Fatalf("expected unquoted-command-name diagnostic, got %q", doc.Diagnostics[0].Code)
	}
	if doc.Diagnostics[0].Severity != ir.SeverityWarning {
		t.Fatalf("expected warning severity, got %q", doc.Diagnostics[0].Severity)
	}
}

func TestValidateTriggerKeyNames(t *testing.T) {
	src := `[State 200]
type = ChangeState
value = 500
trigger1 = command = "a"
trigger1 = command = "b"
triggerall = command = "c"
triggerfoo = command = "x"
`
	doc := Parse("triggers.st", src)

	if len(doc.References) != 4 {
		t.Fatalf("expected 4 references (state + 3 valid triggers), got %d", len(doc.References))
	}

	triggers := 0
	for _, line := range doc.Sections[0].Lines {
		if line.Kind == ir.SourceLineKeyValue && strings.HasPrefix(strings.ToLower(line.Key), "trigger") {
			triggers++
		}
	}
	if triggers != 4 {
		t.Fatalf("expected 4 trigger-like stored lines, got %d", triggers)
	}

	hasWarning := false
	for _, d := range doc.Diagnostics {
		if d.Code == "malformed-trigger" && d.Severity == ir.SeverityWarning {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Fatalf("expected malformed-trigger warning for triggerfoo")
	}
}

func TestHashNotASemicolonComment(t *testing.T) {
	src := `[State 300]
type = ChangeState
value = 1 # inline comment
`
	doc := Parse("hash-comment.st", src)
	if len(doc.References) != 1 {
		t.Fatalf("expected one reference, got %d", len(doc.References))
	}
	if !doc.References[0].IsDynamic {
		t.Fatalf("expected hash-prefixed inline text to produce dynamic reference")
	}
}

func hasLineKind(lines []ir.SourceLine, want ir.SourceLineKind) bool {
	for _, line := range lines {
		if line.Kind == want {
			return true
		}
	}
	return false
}

func TestParseMalformedLineRecovery(t *testing.T) {
	src := `[State 300]
type = ChangeState
not a key value
value = 700
trigger1 = command = "jump"
`
	doc := Parse("e.st", src)

	if len(doc.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(doc.Diagnostics))
	}
	if doc.Diagnostics[0].Severity != ir.SeverityError {
		t.Fatalf("unexpected diagnostic severity: %q", doc.Diagnostics[0].Severity)
	}

	if len(doc.References) != 2 {
		t.Fatalf("expected 2 references (state + command), got %d", len(doc.References))
	}
	if !hasLineKind(doc.Sections[0].Lines, ir.SourceLineMalformed) {
		t.Fatalf("expected malformed line preserved, got %#v", doc.Sections[0].Lines)
	}
	if !hasLineKind(doc.Sections[0].Lines, ir.SourceLineKeyValue) {
		t.Fatalf("expected key-value line recovery, got %#v", doc.Sections[0].Lines)
	}
}

func TestStateControllerTypeClearsPendingValue(t *testing.T) {
	src := `[State 500]
value = 700
type = VelSet
trigger1 = command = "jump"
type = ChangeState
value = 42
`
	doc := Parse("z.st", src)

	if got, want := len(doc.References), 2; got != want {
		t.Fatalf("expected %d references, got %d", want, got)
	}
	for _, ref := range doc.References {
		if ref.Kind != ir.ReferenceCommand {
			if ref.Target != "state:42" {
				t.Fatalf("unexpected state reference target: %q", ref.Target)
			}
		}
	}
}

func TestStateDefAndControllerExtraction(t *testing.T) {
	src := `[Statedef 400]
[State 400]
type = ChangeState
value = 100
`
	doc := Parse("f.def", src)
	if len(doc.Symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(doc.Symbols))
	}
	if doc.Symbols[0].Kind != ir.SymbolStateDef {
		t.Fatalf("first symbol should be statedef, got %#v", doc.Symbols[0])
	}
	if doc.Symbols[1].Kind != ir.SymbolStateController {
		t.Fatalf("second symbol should be state-controller, got %#v", doc.Symbols[1])
	}
	if doc.Symbols[1].Name == "" {
		t.Fatalf("expected controller symbol name")
	}
}

func TestIdentityFieldsAreDeterministic(t *testing.T) {
	src := `[Statedef 500]
[State 500]
type = ChangeState
value = 600
trigger1 = command = "foo"
[Command]
name = "foo"
`
	docA := Parse("g.def", src)
	docB := Parse("g.def", src)

	a, errA := json.Marshal(docA)
	b, errB := json.Marshal(docB)
	if errA != nil || errB != nil {
		t.Fatalf("marshal failed: %v %v", errA, errB)
	}
	if string(a) != string(b) {
		t.Fatalf("document JSON is not deterministic")
	}

	if len(docA.Symbols) != 3 || len(docB.Symbols) != 3 {
		t.Fatalf("unexpected symbol count: got %d/%d", len(docA.Symbols), len(docB.Symbols))
	}
	if docA.Symbols[0].Identity != docB.Symbols[0].Identity ||
		docA.Symbols[1].Identity != docB.Symbols[1].Identity ||
		docA.Symbols[2].Identity != docB.Symbols[2].Identity {
		t.Fatalf("symbol identity is not deterministic")
	}
}

func TestParseIDsIncludeCanonicalPathComponent(t *testing.T) {
	path := filepath.Join("chars", "..", "chars", "hero.def")
	src := `[Command]
name = "jump"
[State 20]
type = ChangeState
value = 200
`
	doc := Parse(path, src)
	if len(doc.Symbols) != 2 {
		t.Fatalf("expected two symbols, got %d", len(doc.Symbols))
	}
	if len(doc.References) != 1 {
		t.Fatalf("expected one reference, got %d", len(doc.References))
	}
	if doc.Version != ir.IdentityContractVersion {
		t.Fatalf("expected document contract %q, got %q", ir.IdentityContractVersion, doc.Version)
	}
	canonicalPath := canonicalPathID(path)
	if !strings.Contains(doc.Symbols[0].Identity.OccurrenceID, canonicalPath) {
		t.Fatalf("expected command symbol occurrence to include canonical path %q, got %q", canonicalPath, doc.Symbols[0].Identity.OccurrenceID)
	}
	if !strings.Contains(doc.References[0].Identity.OccurrenceID, canonicalPath) {
		t.Fatalf("expected state reference occurrence to include canonical path %q, got %q", canonicalPath, doc.References[0].Identity.OccurrenceID)
	}
	if doc.Symbols[0].Identity.StoreID != doc.Symbols[0].ID {
		t.Fatalf("expected symbol store id %q to match legacy id %q", doc.Symbols[0].Identity.StoreID, doc.Symbols[0].ID)
	}
	if doc.References[0].Identity.StoreID != doc.References[0].ID {
		t.Fatalf("expected reference store id %q to match legacy id %q", doc.References[0].Identity.StoreID, doc.References[0].ID)
	}
}

func TestParseDeterministicOrderingStableJSON(t *testing.T) {
	src := `[Statedef 500]
[State 500]
type = ChangeState
value = 600
trigger1 = command = "foo"
[Command]
name = "foo"
`
	docA := Parse("g.def", src)
	docB := Parse("g.def", src)

	a, errA := json.Marshal(docA)
	b, errB := json.Marshal(docB)
	if errA != nil || errB != nil {
		t.Fatalf("marshal failed: %v %v", errA, errB)
	}
	if string(a) != string(b) {
		t.Fatalf("document JSON is not deterministic")
	}

	if len(docA.Symbols) != 3 {
		t.Fatalf("expected deterministic symbol count 3, got %d", len(docA.Symbols))
	}
	if docA.Symbols[0].Kind != ir.SymbolStateDef || docA.Symbols[1].Kind != ir.SymbolStateController || docA.Symbols[2].Kind != ir.SymbolCommand {
		t.Fatalf("unexpected symbol ordering: %#v", docA.Symbols)
	}
}

func TestParseDoesNotPanicForLoneSectionHeaders(t *testing.T) {
	for _, line := range []string{"[", "]", "[]"} {
		t.Run("case="+line, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Parse panicked for %q: %v", line, r)
				}
			}()

			doc := Parse("section-brace.def", line)
			if doc == nil {
				t.Fatalf("expected parser document result")
			}
			if len(doc.Sections) != 0 {
				t.Fatalf("expected no section to be parsed for %q, got %#v", line, doc.Sections)
			}
			if len(doc.Diagnostics) != 1 {
				t.Fatalf("expected one malformed/orphan diagnostic for %q, got %d", line, len(doc.Diagnostics))
			}
		})
	}
}

func TestParseIDsIncludeCanonicalPathComponentSimple(t *testing.T) {
	src := `[Command]
name = "jump"
[State 20]
type = ChangeState
value = 200
`
	path := filepath.Join("chars", "hero.def")
	doc := Parse(path, src)
	if len(doc.Symbols) != 2 {
		t.Fatalf("expected two symbols, got %d", len(doc.Symbols))
	}
	if len(doc.References) != 1 {
		t.Fatalf("expected one reference, got %d", len(doc.References))
	}

	canonicalPath := canonicalPathID(path)
	if !strings.Contains(doc.Symbols[0].ID, canonicalPath) {
		t.Fatalf("expected command symbol id to include canonical path %q, got %q", canonicalPath, doc.Symbols[0].ID)
	}
	if !strings.Contains(doc.References[0].ID, canonicalPath) {
		t.Fatalf("expected state reference id to include canonical path %q, got %q", canonicalPath, doc.References[0].ID)
	}
	if !strings.Contains(doc.References[0].SourceSymbol, canonicalPath) {
		t.Fatalf("expected state reference source symbol to include canonical path %q, got %q", canonicalPath, doc.References[0].SourceSymbol)
	}
}
func TestParseIDsAreWorkspaceUniqueAcrossPaths(t *testing.T) {
	src := `[Command]
name = "jump"
`
	a := Parse(filepath.Join("first", "hero.def"), src)
	b := Parse(filepath.Join("second", "hero.def"), src)

	if len(a.Symbols) != 1 || len(b.Symbols) != 1 {
		t.Fatalf("expected one command symbol in each document")
	}
	if a.Symbols[0].ID == b.Symbols[0].ID {
		t.Fatalf("expected path to make workspace symbol ids unique, got %q", a.Symbols[0].ID)
	}
}

func TestCompoundCommandReferencesHaveUniqueIDs(t *testing.T) {
	doc := Parse("compound.st", `[State 100]
type = ChangeState
value = 100
trigger1 = command = "a" || command = "b"
`)
	commandRefs := make([]ir.Reference, 0, 2)
	for _, ref := range doc.References {
		if ref.Kind == ir.ReferenceCommand {
			commandRefs = append(commandRefs, ref)
		}
	}
	if len(commandRefs) != 2 {
		t.Fatalf("expected two command references, got %d", len(commandRefs))
	}
	if commandRefs[0].Identity.OccurrenceID != commandRefs[1].Identity.OccurrenceID {
		t.Fatalf("expected compound command refs to share occurrence identity, got %q and %q", commandRefs[0].Identity.OccurrenceID, commandRefs[1].Identity.OccurrenceID)
	}
	if commandRefs[0].Identity.StoreID != commandRefs[0].ID || commandRefs[1].Identity.StoreID != commandRefs[1].ID {
		t.Fatalf("expected identity store ids to match legacy ids")
	}
	if commandRefs[0].Identity.StoreID == commandRefs[1].Identity.StoreID {
		t.Fatalf("expected compound command references to have unique store ids, got %q", commandRefs[0].Identity.StoreID)
	}
	if commandRefs[0].ID == commandRefs[1].ID {
		t.Fatalf("expected compound command references to have unique IDs, got %q", commandRefs[0].ID)
	}
	if commandRefs[0].Identity.SemanticKey != "command:a" || commandRefs[1].Identity.SemanticKey != "command:b" {
		t.Fatalf("unexpected repeated reference semantic keys: %#v", []string{commandRefs[0].Identity.SemanticKey, commandRefs[1].Identity.SemanticKey})
	}
}

// canonicalPathID normalizes paths in the same way the parser does for test assertions.
func canonicalPathID(path string) string {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	return strings.TrimPrefix(clean, "./")
}
