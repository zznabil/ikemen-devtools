// Package capability defines the shared, typed public-operation registry.
package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Authority string

const (
	AuthorityRead    Authority = "read"
	AuthorityWrite   Authority = "write"
	AuthorityRuntime Authority = "runtime"
)

type Budget struct {
	MaxItems    int    `json:"maxItems,omitempty"`
	MaxBytes    int    `json:"maxBytes,omitempty"`
	MaxDuration string `json:"maxDuration,omitempty"`
}

type Pagination struct {
	Supported   bool `json:"supported"`
	DefaultSize int  `json:"defaultSize,omitempty"`
	MaximumSize int  `json:"maximumSize,omitempty"`
}

type Ordering struct {
	Keys       []string `json:"keys"`
	Stable     bool     `json:"stable"`
	Descending bool     `json:"descending,omitempty"`
}

type Authorization struct {
	Authority        Authority `json:"authority"`
	RequiresApproval bool      `json:"requiresApproval,omitempty"`
}
type Schema struct {
	Schema               string         `json:"$schema,omitempty"`
	ID                   string         `json:"$id,omitempty"`
	Type                 string         `json:"type"`
	Properties           map[string]any `json:"properties,omitempty"`
	Required             []string       `json:"required,omitempty"`
	AdditionalProperties *bool          `json:"additionalProperties,omitempty"`
}

// Descriptor is the transport-neutral declaration shared by every adapter.
type Descriptor struct {
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Authorization Authorization `json:"authorization"`
	Input         Schema        `json:"input"`
	Output        Schema        `json:"output"`
	Budget        Budget        `json:"budget"`
	Pagination    Pagination    `json:"pagination"`
	Ordering      Ordering      `json:"ordering"`
}

// Capability keeps the operation's input and output types attached to its
// declaration. Execution remains in ordinary semantic services; the registry
// does not use reflection or generic handlers.
type Capability[I any, O any] struct {
	Descriptor Descriptor
	Execute    func(context.Context, I) (O, error)
}

// Registration is an erased view used only for metadata and transport binding.
type Registration struct {
	Descriptor Descriptor
}

type Registry struct {
	entries map[string]Registration
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]Registration)}
}

// Register adds a transport-neutral declaration to the registry.
func (r *Registry) Register(d Descriptor) error {
	if r == nil {
		return fmt.Errorf("nil registry")
	}
	if d.Name == "" {
		return fmt.Errorf("capability name is required")
	}
	if d.Authorization.Authority != AuthorityRead && d.Authorization.Authority != AuthorityWrite && d.Authorization.Authority != AuthorityRuntime {
		return fmt.Errorf("capability %q has invalid authority %q", d.Name, d.Authorization.Authority)
	}
	if _, exists := r.entries[d.Name]; exists {
		return fmt.Errorf("capability %q already registered", d.Name)
	}
	if d.Ordering.Stable && len(d.Ordering.Keys) == 0 {
		return fmt.Errorf("capability %q has stable ordering without keys", d.Name)
	}
	r.entries[d.Name] = Registration{Descriptor: d}
	return nil
}

// RegisterTyped preserves compile-time input/output types at construction while
// storing only metadata in the erased registry.
func RegisterTyped[I any, O any](r *Registry, c Capability[I, O]) error {
	return r.Register(c.Descriptor)
}

type DocumentInput struct {
	URI string `json:"uri"`
}
type PositionInput struct {
	URI       string `json:"uri"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
}
type DocumentResult struct {
	Items []any `json:"items"`
}

func documentSchema() Schema {
	return Schema{Type: "object", Properties: map[string]any{"uri": map[string]any{"type": "string", "minLength": 1}}, Required: []string{"uri"}}
}
func positionSchema() Schema {
	return Schema{Type: "object", Properties: map[string]any{"uri": map[string]any{"type": "string", "minLength": 1}, "line": map[string]any{"type": "integer", "minimum": 0}, "character": map[string]any{"type": "integer", "minimum": 0}}, Required: []string{"uri", "line", "character"}}
}

// DefaultRegistry declares every read-only operation shared by CLI and MCP.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	doc := documentSchema()
	pos := positionSchema()
	names := []string{"document_diagnostics", "document_symbols", "hover", "definition", "references", "workspace_status", "diagnostics", "symbols", "search", "graph_dependencies", "graph_dependents", "graph_path", "graph_impact", "edge_explain", "inspect_workspace", "inspect_character", "inspect_stage", "inspect_file", "export_jsonl", "export_scip", "export_sql"}
	for _, name := range names {
		input := doc
		if strings.Contains(name, "search") || strings.Contains(name, "graph") || strings.Contains(name, "inspect") || strings.Contains(name, "export") {
			input = Schema{Type: "object", Properties: map[string]any{"root": map[string]any{"type": "string", "minLength": 1}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000}, "cursor": map[string]any{"type": "string"}}, Required: []string{"root"}}
		}
		if name == "hover" || name == "definition" {
			input = pos
		}
		_ = r.Register(Descriptor{Name: name, Description: "Read-only IKEMEN workspace operation.", Authorization: Authorization{Authority: AuthorityRead}, Input: input, Output: Schema{Type: "object", Properties: map[string]any{"schemaVersion": map[string]any{"type": "string"}, "operation": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "result": map[string]any{"type": "object"}}}, Budget: Budget{MaxItems: 1000, MaxBytes: 1048576, MaxDuration: "30s"}, Pagination: Pagination{Supported: true, DefaultSize: 100, MaximumSize: 1000}, Ordering: Ordering{Keys: []string{"path", "name", "id"}, Stable: true}})
	}
	return r
}

func (r *Registry) Get(name string) (Descriptor, bool) {
	if r == nil {
		return Descriptor{}, false
	}
	e, ok := r.entries[name]
	return e.Descriptor, ok
}

func (r *Registry) List() []Descriptor {
	if r == nil {
		return nil
	}
	out := make([]Descriptor, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, e.Descriptor)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

type Availability struct {
	Read, Write, Runtime bool
}

func AllAvailability() Availability { return Availability{Read: true, Write: true, Runtime: true} }

func (r *Registry) Filter(a Availability) []Descriptor {
	out := make([]Descriptor, 0)
	for _, d := range r.List() {
		allowed := (d.Authorization.Authority == AuthorityRead && a.Read) || (d.Authorization.Authority == AuthorityWrite && a.Write) || (d.Authorization.Authority == AuthorityRuntime && a.Runtime)
		if allowed {
			out = append(out, d)
		}
	}
	return out
}

type Binding struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	InputSchema  Schema `json:"inputSchema"`
	OutputSchema Schema `json:"outputSchema"`
}

func (r *Registry) CLIBindings(a Availability) []Binding    { return bindings(r.Filter(a)) }
func (r *Registry) MCPDefinitions(a Availability) []Binding { return bindings(r.Filter(a)) }
func bindings(ds []Descriptor) []Binding {
	out := make([]Binding, len(ds))
	for i, d := range ds {
		out[i] = Binding{Name: d.Name, Description: d.Description, InputSchema: d.Input, OutputSchema: d.Output}
	}
	return out
}

func ValidateBinding(d Descriptor, b Binding) error {
	if d.Name != b.Name {
		return fmt.Errorf("binding name %q disagrees with capability %q", b.Name, d.Name)
	}
	if !equalJSON(d.Input, b.InputSchema) {
		return fmt.Errorf("%q input schema drift", d.Name)
	}
	if !equalJSON(d.Output, b.OutputSchema) {
		return fmt.Errorf("%q output schema drift", d.Name)
	}
	return nil
}

func equalJSON(a, b any) bool {
	x, err1 := json.Marshal(a)
	y, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(x) == string(y)
}
