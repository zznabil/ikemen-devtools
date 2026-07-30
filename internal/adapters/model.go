package adapters

// Position is a one-based line/column and zero-based byte offset.
type Position struct{ Line, Column, Offset int }

// Span is a half-open range in the original source.
type Span struct{ Start, End Position }

// Diagnostic is deterministic and recoverable; parsing never executes source.
type Diagnostic struct {
	Code, Message string
	Span          Span
}

// Comment preserves the exact comment text and location.
type Comment struct {
	Text string
	Span Span
}

// DependencyKind identifies an include-like source dependency.
type DependencyKind string

const (
	DependencyInclude DependencyKind = "include"
	DependencyRequire DependencyKind = "require"
)

type Dependency struct {
	Kind DependencyKind
	Path string
	Span Span
}

type ZSSLine struct {
	Text string
	Span Span
}
type ZSSSection struct {
	Name, Header   string
	Span, BodySpan Span
	Lines          []ZSSLine
}
type ZSSDocument struct {
	Path, Source string
	Sections     []ZSSSection
	Comments     []Comment
	Dependencies []Dependency
	Diagnostics  []Diagnostic
	Completeness Completeness
}

type LuaFunction struct {
	Name             string
	Span, HeaderSpan Span
}
type LuaDocument struct {
	Path, Source string
	Functions    []LuaFunction
	Comments     []Comment
	Dependencies []Dependency
	Diagnostics  []Diagnostic
	Completeness Completeness
}

// Completeness records the bounded subset understood by an adapter.
type Completeness struct {
	Complete    bool
	Unsupported []string
}
