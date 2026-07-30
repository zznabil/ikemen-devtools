package adapter

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
)

// TokenKind represents a normalized syntax token classification.
type TokenKind string

const (
	TokenSection   TokenKind = "section"
	TokenKeyValue  TokenKind = "key-value"
	TokenComment   TokenKind = "comment"
	TokenBlank     TokenKind = "blank"
	TokenMalformed TokenKind = "malformed"
)

// Token is a normalized syntax token used by parser adapter entrypoints.
type Token struct {
	Kind  TokenKind
	Span  ir.SourceSpan
	Text  string
	Key   string
	Value string
}

// Section is a normalized section token payload from syntax provider output.
type Section struct {
	Header string
	Kind   ir.SectionKind
	Span   ir.SourceSpan
}

// FromSyntax builds a parser document from syntax provider output.
func FromSyntax(path string, sections []Section, tokens []Token) *ir.Document {
	doc := ir.NewDocument(path, detectFileType(path))
	pathID := canonicalPathID(path)
	var currentSectionIdx = -1
	var controller *stateControllerContext

	for _, token := range tokens {
		lineNo := token.Span.Start.Line
		switch token.Kind {
		case TokenSection:
			header := strings.TrimSpace(token.Text)
			if header == "" {
				doc.Diagnostics = append(doc.Diagnostics, makeDiagnostic(path, ir.SeverityError, "empty section header", "malformed-section", lineNo, token.Span))
				currentSectionIdx = -1
				controller = nil
				continue
			}

			section := ir.Section{
				Header: header,
				Kind:   sectionKind(header),
				Span:   token.Span,
			}
			doc.Sections = append(doc.Sections, section)
			currentSectionIdx = len(doc.Sections) - 1
			controller = nil

			switch section.Kind {
			case ir.SectionStatedef:
				stateArg := sectionHeaderArgument(header)
				stateName := canonicalStateName(stateArg)
				doc.Symbols = append(doc.Symbols, ir.Symbol{
					ID:       stateSymbolID(stateName, lineNo, pathID),
					Identity: symbolIdentity(stateName, lineNo, pathID),
					Kind:     ir.SymbolStateDef,
					Name:     stateName,
					Span:     section.Span,
					Section:  section.Header,
					Raw:      strings.TrimSpace(stateArg),
				})
				if strings.TrimSpace(stateArg) == "" {
					doc.Diagnostics = append(doc.Diagnostics, makeDiagnostic(path, ir.SeverityError, "missing state id", "malformed-statedef", lineNo, section.Span))
				}
			case ir.SectionState:
				stateArg := sectionHeaderArgument(header)
				sectionName := canonicalStateControllerName(stateArg)
				symbol := ir.Symbol{
					ID:       stateControllerSymbolID(sectionName, lineNo, pathID),
					Identity: symbolIdentity(sectionName, lineNo, pathID),
					Kind:     ir.SymbolStateController,
					Name:     sectionName,
					Span:     section.Span,
					Section:  section.Header,
					Raw:      strings.TrimSpace(stateArg),
				}
				doc.Symbols = append(doc.Symbols, symbol)
				if strings.TrimSpace(stateArg) == "" {
					doc.Diagnostics = append(doc.Diagnostics, makeDiagnostic(path, ir.SeverityError, "missing state id", "malformed-state", lineNo, section.Span))
				}
				controller = &stateControllerContext{
					symbolID:      symbol.ID,
					stateName:     sectionName,
					sectionHeader: section.Header,
				}
			}
			continue

		case TokenComment:
			if currentSectionIdx >= 0 {
				s := &doc.Sections[currentSectionIdx]
				s.Lines = append(s.Lines, ir.SourceLine{
					Kind: ir.SourceLineComment,
					Text: token.Text,
					Span: token.Span,
				})
			}
			continue

		case TokenBlank:
			continue
		}

		if currentSectionIdx < 0 {
			doc.Diagnostics = append(doc.Diagnostics, makeDiagnostic(path, ir.SeverityError, "key outside section", "orphan-line", lineNo, token.Span))
			continue
		}

		section := &doc.Sections[currentSectionIdx]
		if token.Kind == TokenMalformed {
			lineSpan := token.Span
			doc.Diagnostics = append(doc.Diagnostics, makeDiagnostic(path, ir.SeverityError, "expected key=value", "malformed-line", lineNo, lineSpan))
			s := &doc.Sections[currentSectionIdx]
			s.Lines = append(s.Lines, ir.SourceLine{Kind: ir.SourceLineMalformed, Text: strings.TrimSpace(token.Text), Span: lineSpan})
			continue
		}

		if token.Kind != TokenKeyValue {
			continue
		}

		key, value := token.Key, token.Value
		lineSpan := token.Span
		if key == "" {
			doc.Diagnostics = append(doc.Diagnostics, makeDiagnostic(path, ir.SeverityError, "empty key", "malformed-line", lineNo, lineSpan))
			continue
		}

		section.Lines = append(section.Lines, ir.SourceLine{Kind: ir.SourceLineKeyValue, Key: key, Value: value, Span: lineSpan})

		lowerKey := strings.ToLower(key)
		if section.Kind == ir.SectionCommand && lowerKey == "name" {
			cmd := stripQuotes(strings.TrimSpace(value))
			rawName := strings.TrimSpace(value)
			if len(rawName) < 2 || rawName[0] != '"' || rawName[len(rawName)-1] != '"' {
				doc.Diagnostics = append(doc.Diagnostics, makeDiagnostic(path, ir.SeverityWarning, "command name should be quoted", "unquoted-command-name", lineNo, lineSpan))
			}
			if cmd == "" {
				doc.Diagnostics = append(doc.Diagnostics, makeDiagnostic(path, ir.SeverityWarning, "empty command name", "malformed-command", lineNo, lineSpan))
				continue
			}
			doc.Symbols = append(doc.Symbols, ir.Symbol{
				ID:       commandSymbolID(cmd, lineNo, pathID),
				Identity: symbolIdentity("command:"+cmd, lineNo, pathID),
				Kind:     ir.SymbolCommand,
				Name:     "command:" + cmd,
				Span:     lineSpan,
				Section:  section.Header,
				Raw:      cmd,
			})
		}

		if section.Kind == ir.SectionState && controller != nil {
			switch lowerKey {
			case "type":
				controller.controllerType = strings.ToLower(stripQuotes(value))
				if controller.controllerType != "changestate" && controller.controllerType != "selfstate" {
					controller.pendingValue = ""
					controller.pendingValuePresent = false
					continue
				}
				controller.emitStateReference(&doc, lineSpan, pathID)
			case "value":
				controller.pendingValue = strings.TrimSpace(value)
				controller.pendingValueSpan = lineSpan
				controller.pendingValuePresent = true
				if controller.controllerType == "changestate" || controller.controllerType == "selfstate" {
					controller.emitStateReference(&doc, lineSpan, pathID)
				}
			}
		}

		if section.Kind == ir.SectionState && isTriggerKey(lowerKey) {
			for occurrence, cmd := range extractCommandReferences(strings.TrimSpace(value)) {
				if cmd == "" {
					continue
				}
				id := referenceID("command", lineNo, pathID, strconv.Itoa(occurrence))
				identity := referenceIdentity("command:"+cmd, lineNo, pathID)
				identity.StoreID = id
				doc.References = append(doc.References, ir.Reference{
					ID:           id,
					Identity:     identity,
					Kind:         ir.ReferenceCommand,
					Name:         "command:" + cmd,
					Raw:          value,
					SourceSymbol: sectionRefSymbol(section, controller),
					Target:       "command:" + cmd,
					Span:         lineSpan,
					IsDynamic:    false,
				})
			}
		} else if section.Kind == ir.SectionState && strings.HasPrefix(lowerKey, "trigger") {
			doc.Diagnostics = append(doc.Diagnostics, makeDiagnostic(path, ir.SeverityWarning, "invalid trigger key", "malformed-trigger", lineNo, lineSpan))
		}
	}

	return &doc
}

func detectFileType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".def":
		return "def"
	case ".cns":
		return "cns"
	case ".cmd":
		return "cmd"
	case ".st":
		return "st"
	default:
		return "text"
	}
}

func sectionKind(header string) ir.SectionKind {
	parts := strings.Fields(strings.ToLower(header))
	if len(parts) == 0 {
		return ir.SectionOther
	}
	switch parts[0] {
	case "statedef":
		return ir.SectionStatedef
	case "state":
		return ir.SectionState
	case "command":
		return ir.SectionCommand
	default:
		return ir.SectionOther
	}
}

func sectionHeaderArgument(header string) string {
	parts := strings.Fields(header)
	if len(parts) <= 1 {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, parts[0]))
}

func canonicalStateName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "state:unknown"
	}
	if name == "+1" {
		name = "-10"
	}
	return "state:" + name
}

func canonicalStateControllerName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "state-controller:unknown"
	}
	if name == "+1" {
		name = "-10"
	}
	return "state-controller:" + name
}

func normalizeStateReferenceValue(value string) string {
	if strings.TrimSpace(value) == "+1" {
		return "-10"
	}
	return value
}

func makeDiagnostic(path string, severity ir.Severity, message, code string, line int, span ir.SourceSpan) ir.Diagnostic {
	colStart := span.Start.Column
	colEnd := span.End.Column
	if colEnd < colStart {
		colEnd = colStart
	}
	if colStart < 1 {
		colStart = 1
	}
	if colEnd < 1 {
		colEnd = 1
	}
	return ir.Diagnostic{
		Code:          code,
		Severity:      severity,
		Message:       message,
		Path:          path,
		Start:         ir.SourcePosition{Line: line, Column: colStart},
		End:           ir.SourcePosition{Line: line, Column: colEnd},
		RelatedSymbol: "",
	}
}

func canonicalPathID(path string) string {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	return strings.TrimPrefix(clean, "./")
}

func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			s = s[1 : len(s)-1]
		}
	}
	return s
}

func sectionRefSymbol(section *ir.Section, ctx *stateControllerContext) string {
	if ctx != nil && ctx.symbolID != "" {
		return ctx.symbolID
	}
	if section == nil {
		return ""
	}
	return section.Header
}

func symbolIdentity(semanticKey string, lineNo int, pathID string) ir.Identity {
	occurrence := fmt.Sprintf("%s@%s:%d", semanticKey, pathID, lineNo)
	id := fmt.Sprintf("%s:%d:%s", semanticKey, lineNo, pathID)
	return ir.Identity{
		ContractVersion: ir.IdentityContractVersion,
		SemanticKey:     semanticKey,
		OccurrenceID:    occurrence,
		StoreID:         id,
	}
}

func referenceIdentity(semanticKey string, lineNo int, pathID string) ir.Identity {
	return ir.Identity{
		ContractVersion: ir.IdentityContractVersion,
		SemanticKey:     semanticKey,
		OccurrenceID:    fmt.Sprintf("%s:%d", pathID, lineNo),
	}
}

func stateSymbolID(name string, lineNo int, pathID string) string {
	return fmt.Sprintf("%s:%d:%s", name, lineNo, pathID)
}

func stateControllerSymbolID(name string, lineNo int, pathID string) string {
	return fmt.Sprintf("%s:%d:%s", name, lineNo, pathID)
}

func commandSymbolID(name string, lineNo int, pathID string) string {
	return fmt.Sprintf("command:%s:%d:%s", name, lineNo, pathID)
}

func referenceID(kind string, lineNo int, pathID string, occurrence ...string) string {
	suffix := ""
	if len(occurrence) > 0 && occurrence[0] != "" {
		suffix = ":" + occurrence[0]
	}
	return fmt.Sprintf("ref:%s:%d:%s%s", kind, lineNo, pathID, suffix)
}

func isTriggerKey(key string) bool {
	if key == "triggerall" {
		return true
	}
	if !strings.HasPrefix(key, "trigger") {
		return false
	}
	suffix := strings.TrimPrefix(key, "trigger")
	if suffix == "" {
		return false
	}
	_, err := strconv.Atoi(suffix)
	return err == nil
}

func extractCommandReferences(value string) []string {
	var out []string
	for offset := 0; offset < len(value); {
		segment := value[offset:]
		idx := strings.Index(strings.ToLower(segment), "command")
		if idx < 0 || idx > len(segment) {
			break
		}
		idx += offset
		end := idx + len("command")
		if end > len(value) {
			break
		}
		if end < len(value) && isIdentifierChar(value[end]) {
			offset = end
			continue
		}
		equals := strings.IndexByte(value[end:], '=')
		if equals < 0 {
			break
		}
		equals += end
		raw := strings.TrimSpace(value[equals+1:])
		cmd := parseReferenceValue(raw)
		if cmd != "" {
			out = append(out, cmd)
		}
		offset = equals + 1
	}
	return out
}

func isIdentifierChar(ch byte) bool {
	return ch == '_' || ch == '-' || ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z'
}

func parseReferenceValue(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "\"") {
		if end := strings.IndexByte(v[1:], '"'); end >= 0 {
			return v[1 : end+1]
		}
		return v[1:]
	}
	fields := strings.Fields(v)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// stateControllerContext tracks pending state transition context while scanning section.
type stateControllerContext struct {
	symbolID            string
	sectionHeader       string
	stateName           string
	controllerType      string
	pendingValue        string
	pendingValueSpan    ir.SourceSpan
	pendingValuePresent bool
}

func (ctx *stateControllerContext) emitStateReference(doc *ir.Document, fallbackSpan ir.SourceSpan, pathID string) {
	if ctx == nil || !ctx.pendingValuePresent {
		return
	}
	value := strings.TrimSpace(ctx.pendingValue)
	ctx.pendingValuePresent = false
	if ctx.controllerType != "changestate" && ctx.controllerType != "selfstate" {
		return
	}
	value = normalizeStateReferenceValue(stripQuotes(value))
	if value == "" {
		return
	}
	_, err := strconv.Atoi(value)
	isDynamic := err != nil
	id := referenceID("state", fallbackSpan.Start.Line, pathID)
	identity := referenceIdentity("state:"+value, fallbackSpan.Start.Line, pathID)
	identity.StoreID = id
	doc.References = append(doc.References, ir.Reference{
		ID:           id,
		Identity:     identity,
		Kind:         ir.ReferenceState,
		Name:         "state:" + value,
		Raw:          value,
		SourceSymbol: ctx.symbolID,
		Target:       "state:" + value,
		Span:         ctx.pendingValueSpan,
		IsDynamic:    isDynamic,
	})
}
