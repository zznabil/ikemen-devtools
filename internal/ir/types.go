package ir

const (
	IdentityContractVersion = "0.2.0"
	Version                 = IdentityContractVersion
)

type Identity struct {
	ContractVersion string `json:"contractVersion"`
	SemanticKey     string `json:"semanticKey"`
	OccurrenceID    string `json:"occurrence"`
	StoreID         string `json:"store"`
}

// Severity is the diagnostic severity used by parser and later semantic passes.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// SourcePosition tracks a one-based line/column position in the original text.
type SourcePosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// SourceSpan tracks a half-open source range.
type SourceSpan struct {
	Start SourcePosition `json:"start"`
	End   SourcePosition `json:"end"`
}

type SourceLineKind string

const (
	SourceLineComment   SourceLineKind = "comment"
	SourceLineKeyValue  SourceLineKind = "key-value"
	SourceLineBlank     SourceLineKind = "blank"
	SourceLineMalformed SourceLineKind = "malformed"
)

// SourceLine stores source-aware line fragments in declaration order.
type SourceLine struct {
	Kind  SourceLineKind `json:"kind"`
	Key   string         `json:"key,omitempty"`
	Value string         `json:"value,omitempty"`
	Text  string         `json:"text,omitempty"`
	Span  SourceSpan     `json:"span"`
}

// SectionKind is the logical section bucket for parser extraction.
type SectionKind string

const (
	SectionOther    SectionKind = "other"
	SectionStatedef SectionKind = "statedef"
	SectionState    SectionKind = "state"
	SectionCommand  SectionKind = "command"
)

// Section contains parsed declaration lines for a single INI section.
type Section struct {
	Header string       `json:"header"`
	Kind   SectionKind  `json:"kind"`
	Span   SourceSpan   `json:"span"`
	Lines  []SourceLine `json:"lines"`
}

// SymbolKind identifies parser-extracted graph nodes.
type SymbolKind string

const (
	SymbolSection         SymbolKind = "section"
	SymbolStateDef        SymbolKind = "state"
	SymbolStateController SymbolKind = "state-controller"
	SymbolCommand         SymbolKind = "command"
)

// Symbol is a versioned, serializable unit of identity used by later stages.
type Symbol struct {
	ID       string     `json:"id"`
	Identity Identity   `json:"identity"`
	Kind     SymbolKind `json:"kind"`
	Name     string     `json:"name"`
	Span     SourceSpan `json:"span"`
	Section  string     `json:"section"`
	Raw      string     `json:"raw,omitempty"`
}

// ReferenceKind is the reference target category.
type ReferenceKind string

const (
	ReferenceState   ReferenceKind = "state"
	ReferenceCommand ReferenceKind = "command"
)

// Reference is an unresolved but deterministic reference from one symbol to another.
type Reference struct {
	ID           string        `json:"id"`
	Identity     Identity      `json:"identity"`
	Kind         ReferenceKind `json:"kind"`
	Name         string        `json:"name"`
	Raw          string        `json:"raw"`
	SourceSymbol string        `json:"sourceSymbol"`
	Target       string        `json:"target"`
	Span         SourceSpan    `json:"span"`
	IsDynamic    bool          `json:"isDynamic"`
}

// Diagnostic is stable, recoverable parser output.
type Diagnostic struct {
	Code          string         `json:"code"`
	Severity      Severity       `json:"severity"`
	Message       string         `json:"message"`
	Path          string         `json:"path"`
	Start         SourcePosition `json:"start"`
	End           SourcePosition `json:"end"`
	RelatedSymbol string         `json:"relatedSymbol,omitempty"`
}

// Document is the full parse result for one source file.
type Document struct {
	Version     string       `json:"version"`
	Path        string       `json:"path"`
	FileType    string       `json:"fileType"`
	Sections    []Section    `json:"sections"`
	Symbols     []Symbol     `json:"symbols"`
	References  []Reference  `json:"references"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func NewDocument(path, fileType string) Document {
	return Document{
		Version:  Version,
		Path:     path,
		FileType: fileType,
	}
}
