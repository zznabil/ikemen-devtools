package adapters

import "strings"

type sourceLine struct {
	text                           string
	start, length, endOffset, line int
}

func sourceLines(s string) []sourceLine {
	if s == "" {
		return nil
	}
	out := []sourceLine{}
	start, no := 0, 1
	for start < len(s) {
		end := strings.IndexByte(s[start:], '\n')
		stop := len(s)
		next := len(s)
		if end >= 0 {
			stop = start + end
			next = stop + 1
		}
		text := s[start:stop]
		if strings.HasSuffix(text, "\r") {
			text = text[:len(text)-1]
		}
		out = append(out, sourceLine{text: text, start: start, length: len(text), endOffset: stop, line: no})
		no++
		start = next
	}
	return out
}
func position(offset, column, _ int) Position {
	return Position{Offset: offset, Line: 1, Column: column}
}
func offsetPosition(s string, offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(s) {
		offset = len(s)
	}
	line, col := 1, 1
	for i := 0; i < offset; i++ {
		if s[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return Position{Line: line, Column: col, Offset: offset}
}
func spanAt(l sourceLine, start, n int) Span {
	if start < 0 {
		start = 0
	}
	if start > len(l.text) {
		start = len(l.text)
	}
	if n < 0 {
		n = 0
	}
	if start+n > len(l.text) {
		n = len(l.text) - start
	}
	return Span{Start: Position{Line: l.line, Column: start + 1, Offset: l.start + start}, End: Position{Line: l.line, Column: start + n + 1, Offset: l.start + start + n}}
}
func lineAfterOffset(s string, offset int) int {
	if offset < 0 {
		return 0
	}
	i := strings.IndexByte(s[offset:], '\n')
	if i < 0 {
		return len(s)
	}
	return offset + i + 1
}
