package mutation

import (
	"context"
	"errors"
	"github.com/ikemen-engine/ikemen-devtools/internal/patch"
	"sort"
)

type Reference struct {
	Path             string
	Start, End       int
	Old, New         string
	Hash             string
	IdentityContract string
	Classification   string
	Reason           string
}
type Resolver interface {
	Resolve(context.Context, Request) (Reference, []Reference, error)
}
type RenameProvider struct{ Resolver Resolver }

func (p RenameProvider) Prepare(ctx context.Context, r Request) (Proposal, error) {
	if p.Resolver == nil {
		return Proposal{Refusal: "resolver unavailable"}, errors.New("mutation: resolver unavailable")
	}
	def, refs, err := p.Resolver.Resolve(ctx, r)
	if err != nil {
		return Proposal{}, err
	}
	all := append([]Reference{def}, refs...)
	edits := make([]patch.Edit, 0, len(all))
	skipped := []Candidate{}
	for _, ref := range all {
		if ref.Classification != "exact" {
			skipped = append(skipped, Candidate{Path: ref.Path, Reason: "reference is " + ref.Classification})
			continue
		}
		edits = append(edits, patch.Edit{Path: ref.Path, ContentHash: ref.Hash, IdentityContract: ref.IdentityContract, Span: patch.Span{ByteStart: ref.Start, ByteEnd: ref.End}, OldText: ref.Old, NewText: ref.New})
	}
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].Path != edits[j].Path {
			return edits[i].Path < edits[j].Path
		}
		return edits[i].Span.ByteStart < edits[j].Span.ByteStart
	})
	if len(edits) == 0 {
		return Proposal{Skipped: skipped, Refusal: "no exact references"}, ErrAmbiguous
	}
	return Proposal{Confidence: "exact", Skipped: skipped, Plan: patch.PatchPlan{Version: patch.PlanVersion, WorkspaceRoot: r.Root, InputSnapshot: r.Snapshot, Edits: edits}}, nil
}
func (p RenameProvider) Plan(ctx context.Context, r Request) (Proposal, error) {
	return p.Prepare(ctx, r)
}
func (p RenameProvider) Validate(_ context.Context, proposal Proposal) error {
	if proposal.Refusal != "" {
		return errors.New(proposal.Refusal)
	}
	return nil
}
