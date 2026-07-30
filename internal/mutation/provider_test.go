package mutation

import (
	"context"
	"github.com/ikemen-engine/ikemen-devtools/internal/patch"
	"testing"
)

type resolverStub struct {
	def  Reference
	refs []Reference
}

func (s resolverStub) Resolve(context.Context, Request) (Reference, []Reference, error) {
	return s.def, s.refs, nil
}
func TestRenameSkipsAmbiguous(t *testing.T) {
	p := RenameProvider{Resolver: resolverStub{def: Reference{Path: "a", Start: 0, End: 1, Old: "x", New: "y", Hash: "h", IdentityContract: "0.2.0", Classification: "exact"}, refs: []Reference{{Path: "b", Classification: "ambiguous"}}}}
	out, err := p.Prepare(context.Background(), Request{Root: ".", Snapshot: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Plan.Edits) != 1 || len(out.Skipped) != 1 {
		t.Fatalf("unexpected proposal: %#v", out)
	}
}
func TestAuthorizerConsumesOnceAndBindsSnapshot(t *testing.T) {
	a := NewAuthorizer(true)
	p := patch.PatchPlan{Version: patch.PlanVersion, InputSnapshot: "s", Edits: []patch.Edit{{Path: "a", ContentHash: "h", IdentityContract: "0.2.0"}}}
	tok, err := a.Issue(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Consume(tok, "wrong"); err == nil {
		t.Fatal("expected stale token")
	}
	if _, err = a.Consume(tok, "s"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Consume(tok, "s"); err == nil {
		t.Fatal("expected consumed token")
	}
}
