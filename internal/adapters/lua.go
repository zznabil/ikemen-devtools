package adapters

import (
	"regexp"
	"strings"
)

var luaFunctionRE = regexp.MustCompile(`^\s*(?:local\s+)?function\s+([A-Za-z_][A-Za-z0-9_.:]*)\s*\(`)
var luaAssignedFunctionRE = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_.:]*)\s*=\s*function\s*\(`)
var luaDependencyRE = regexp.MustCompile(`(?i)\b(require|include)\s*\(\s*["']([^"']+)["']\s*\)`)

// ParseLua discovers functions and static require/include calls without execution.
func ParseLua(path string, source []byte) *LuaDocument {
	s := string(source)
	doc := &LuaDocument{Path: path, Source: s}
	lines := sourceLines(s)
	type openFn struct {
		name        string
		line        sourceLine
		headerStart int
		depth       int
	}
	stack := []openFn{}
	for _, line := range lines {
		trim := strings.TrimSpace(line.text)
		if strings.HasPrefix(trim, "--") {
			idx := strings.Index(line.text, trim)
			doc.Comments = append(doc.Comments, Comment{Text: trim, Span: spanAt(line, idx, len(trim))})
			continue
		}
		for _, m := range luaDependencyRE.FindAllStringSubmatchIndex(line.text, -1) {
			kind := DependencyRequire
			if strings.EqualFold(line.text[m[2]:m[3]], "include") {
				kind = DependencyInclude
			}
			pathText := line.text[m[4]:m[5]]
			doc.Dependencies = append(doc.Dependencies, Dependency{Kind: kind, Path: pathText, Span: spanAt(line, m[0], m[1]-m[0])})
		}
		name, start, ok := "", 0, false
		if m := luaFunctionRE.FindStringSubmatch(line.text); m != nil {
			start = strings.Index(line.text, "function")
			ok = true
			name = m[1]
		} else if m := luaAssignedFunctionRE.FindStringSubmatch(line.text); m != nil {
			name = m[1]
			start = strings.Index(line.text, "function")
			ok = true
		}
		if ok {
			stack = append(stack, openFn{name: name, line: line, headerStart: start, depth: 1})
			continue
		}
		if len(stack) > 0 && luaBlockEnd(line.text) {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			endLine := line
			doc.Functions = append(doc.Functions, LuaFunction{Name: top.name, HeaderSpan: spanAt(top.line, top.headerStart, len(top.line.text)-top.headerStart), Span: Span{Start: Position{Line: top.line.line, Column: top.headerStart + 1, Offset: top.line.start + top.headerStart}, End: Position{Line: endLine.line, Column: len(endLine.text) + 1, Offset: endLine.start + len(endLine.text)}}})
		}
	}
	for _, top := range stack {
		doc.Diagnostics = append(doc.Diagnostics, Diagnostic{Code: "lua.unterminated-function", Message: "function is missing a matching end", Span: spanAt(top.line, top.headerStart, len(top.line.text)-top.headerStart)})
	}
	return doc
}
func luaBlockEnd(text string) bool {
	t := strings.TrimSpace(text)
	return t == "end" || strings.HasPrefix(t, "end ") || strings.HasPrefix(t, "end;")
}
