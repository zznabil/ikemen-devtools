package provenance

import (
	"sort"
	"strings"
)

type Class string

const (
	Exact       Class = "exact"
	Inferred    Class = "inferred"
	Lexical     Class = "lexical"
	Ambiguous   Class = "ambiguous"
	Unresolved  Class = "unresolved"
	Unsupported Class = "unsupported"
)

type Completeness string

const (
	Complete    Completeness = "complete"
	Partial     Completeness = "partial"
	Unavailable Completeness = "unavailable"
)

type Evidence struct {
	ID     string `json:"id"`
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	Syntax string `json:"syntax,omitempty"`
	Rule   string `json:"rule,omitempty"`
}
type Result struct {
	Class          Class        `json:"class"`
	Confidence     string       `json:"confidence"`
	Completeness   Completeness `json:"completeness"`
	Evidence       []Evidence   `json:"evidence"`
	Explanation    []string     `json:"explanation"`
	AmbiguityGroup string       `json:"ambiguityGroup,omitempty"`
	Refusal        string       `json:"refusal,omitempty"`
}
type Policy struct {
	Include map[Class]bool
	Minimum Class
}

func (r *Result) Normalize() {
	if r == nil {
		return
	}
	sort.Slice(r.Evidence, func(i, j int) bool {
		if r.Evidence[i].ID != r.Evidence[j].ID {
			return r.Evidence[i].ID < r.Evidence[j].ID
		}
		return r.Evidence[i].Path < r.Evidence[j].Path
	})
	r.Explanation = uniqueSorted(r.Explanation)
}
func uniqueSorted(in []string) []string {
	m := map[string]bool{}
	for _, v := range in {
		if strings.TrimSpace(v) != "" {
			m[v] = true
		}
	}
	o := make([]string, 0, len(m))
	for v := range m {
		o = append(o, v)
	}
	sort.Strings(o)
	return o
}
func ActionSafe(r Result) bool { return r.Class == Exact && r.Completeness == Complete }
func (p Policy) Allows(r Result) bool {
	if p.Include != nil && !p.Include[r.Class] {
		return false
	}
	if p.Minimum == "" {
		return true
	}
	rank := map[Class]int{Exact: 5, Inferred: 4, Lexical: 3, Ambiguous: 2, Unresolved: 1, Unsupported: 0}
	return rank[r.Class] >= rank[p.Minimum]
}
func Chain(steps []string, limit int) []string {
	if limit <= 0 {
		limit = 32
	}
	seen := map[string]bool{}
	out := make([]string, 0, min(limit, len(steps)))
	for _, s := range steps {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
		if len(out) >= limit {
			break
		}
	}
	return out
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
