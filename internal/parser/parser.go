package parser

import (
	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/syntax"
)

// Parse converts an authored snippet of .def/.cns/.cmd/.st text into IR.
// It now routes through the syntax scanner adapter for deterministic token capture.
func Parse(path, source string) *ir.Document {
	parsed := syntax.NewStandardProvider().Parse(path, []byte(source))
	if parsed == nil {
		return nil
	}
	return parsed.Document
}
