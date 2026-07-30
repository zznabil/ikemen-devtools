package capability

import (
	"context"
	"testing"
)

func TestDefaultRegistryIsDeterministicAndTyped(t *testing.T) {
	r := DefaultRegistry()
	got := r.List()
	if len(got) != 5 {
		t.Fatalf("capability count = %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Name >= got[i].Name {
			t.Fatalf("registry order is not stable: %#v", got)
		}
	}
	var in DocumentInput
	c := Capability[DocumentInput, DocumentResult]{Descriptor: got[0], Execute: func(context.Context, DocumentInput) (DocumentResult, error) { return DocumentResult{}, nil }}
	if c.Execute == nil {
		t.Fatal("typed capability lost executor type")
	}
	_ = in
}

func TestAvailabilityFilteringAndDerivedBindings(t *testing.T) {
	r := NewRegistry()
	for _, authority := range []Authority{AuthorityRead, AuthorityWrite, AuthorityRuntime} {
		if err := r.Register(Descriptor{Name: string(authority), Authorization: Authorization{Authority: authority}, Input: Schema{Type: "object"}, Output: Schema{Type: "object"}}); err != nil {
			t.Fatal(err)
		}
	}
	read := r.Filter(Availability{Read: true})
	if len(read) != 1 || read[0].Name != "read" {
		t.Fatalf("read filter = %#v", read)
	}
	bindings := r.MCPDefinitions(Availability{Write: true})
	if len(bindings) != 1 || bindings[0].Name != "write" {
		t.Fatalf("MCP bindings = %#v", bindings)
	}
	if err := ValidateBinding(read[0], Binding{Name: "read", InputSchema: read[0].Input, OutputSchema: read[0].Output}); err != nil {
		t.Fatal(err)
	}
}

func TestBindingDriftFails(t *testing.T) {
	d := Descriptor{Name: "x", Authorization: Authorization{Authority: AuthorityRead}, Input: Schema{Type: "object"}, Output: Schema{Type: "object"}}
	if err := ValidateBinding(d, Binding{Name: d.Name, InputSchema: Schema{Type: "string"}, OutputSchema: d.Output}); err == nil {
		t.Fatal("schema drift was accepted")
	}
	if err := ValidateBinding(d, Binding{Name: "other", InputSchema: d.Input, OutputSchema: d.Output}); err == nil {
		t.Fatal("name drift was accepted")
	}
}

func TestRegistrationRejectsInvalidAndDuplicateDeclarations(t *testing.T) {
	r := NewRegistry()
	d := Descriptor{Name: "x", Authorization: Authorization{Authority: "unknown"}}
	if err := r.Register(d); err == nil {
		t.Fatal("invalid authority accepted")
	}
	d.Authorization.Authority = AuthorityRead
	if err := r.Register(d); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(d); err == nil {
		t.Fatal("duplicate accepted")
	}
}
