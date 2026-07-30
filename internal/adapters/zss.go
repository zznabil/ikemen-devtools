package adapters

import (
	"regexp"
	"strings"
)

var zssSectionRE = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*(?:[;#].*)?$`)
var zssDependencyRE = regexp.MustCompile(`(?i)^\s*(include|require)\s*(?:=\s*)?(["']?[^"'\s;#]+["']?)`)

// ParseZSS discovers INI-like motif/state sections without interpreting values.
func ParseZSS(path string, source []byte) *ZSSDocument {
	s := string(source)
	doc := &ZSSDocument{Path: path, Source: s, Completeness: Completeness{Complete: true}}
	lines := sourceLines(s)
	for _, line := range lines {
		trim := strings.TrimSpace(line.text)
		if trim == "" {
			continue
		}
		if isZSSComment(trim) {
			doc.Comments = append(doc.Comments, Comment{Text: line.text[strings.Index(line.text, trim):], Span: spanAt(line, strings.Index(line.text, trim), len(trim))})
			continue
		}
		if m := zssSectionRE.FindStringSubmatch(line.text); m != nil {
			name := strings.TrimSpace(m[1])
			start := strings.Index(line.text, "[")
			close := strings.Index(line.text[start:], "]") + start + 1
			doc.Sections = append(doc.Sections, ZSSSection{Name: name, Header: line.text[start:close], Span: spanAt(line, start, close-start), BodySpan: Span{Start: offsetPosition(s, line.endOffset), End: offsetPosition(s, line.endOffset)}})
			continue
		}
		if strings.HasPrefix(trim, "[") {
			doc.Diagnostics = append(doc.Diagnostics, Diagnostic{Code: "zss.unterminated-section", Message: "section header is missing a closing ]", Span: spanAt(line, strings.Index(line.text, "["), len(line.text)-strings.Index(line.text, "["))})
		}
		if m := zssDependencyRE.FindStringSubmatch(trim); m != nil {
			value := strings.Trim(m[2], "\"'")
			start := strings.Index(line.text, strings.TrimSpace(trim))
			kind := DependencyInclude
			if strings.EqualFold(m[1], "require") {
				kind = DependencyRequire
			}
			doc.Dependencies = append(doc.Dependencies, Dependency{Kind: kind, Path: value, Span: spanAt(line, start, len(strings.TrimSpace(trim)))})
		}
	}
	for i := range doc.Sections {
		sec := &doc.Sections[i]
		end := len(s)
		if i+1 < len(doc.Sections) {
			end = doc.Sections[i+1].Span.Start.Offset
		}
		bodyStart := lineAfterOffset(s, sec.Span.End.Offset)
		if bodyStart > end {
			bodyStart = end
		}
		sec.BodySpan = Span{Start: offsetPosition(s, bodyStart), End: offsetPosition(s, end)}
		for _, line := range lines {
			if line.start >= bodyStart && line.start < end {
				sec.Lines = append(sec.Lines, ZSSLine{Text: line.text, Span: spanAt(line, 0, len(line.text))})
			}
		}
	}
	doc.Completeness.Complete = len(doc.Diagnostics) == 0
	return doc
}

func isZSSComment(s string) bool {
	return strings.HasPrefix(s, ";") || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//")
}
