package mutation

import (
	"context"
	"errors"
	"github.com/ikemen-engine/ikemen-devtools/internal/patch"
)

type Fix struct {
	DiagnosticID  string
	Edit          patch.Edit
	Deterministic bool
	Reason        string
}
type FixProvider struct {
	Resolve func(context.Context, Request) ([]Fix, error)
}

func (p FixProvider) Prepare(ctx context.Context, r Request) (Proposal, error) {
	if p.Resolve == nil {
		return Proposal{Refusal: "fix resolver unavailable"}, errors.New("mutation: fix resolver unavailable")
	}
	fixes, err := p.Resolve(ctx, r)
	if err != nil {
		return Proposal{}, err
	}
	out := Proposal{Confidence: "exact", Plan: patch.PatchPlan{Version: patch.PlanVersion, WorkspaceRoot: r.Root, InputSnapshot: r.Snapshot}}
	for _, f := range fixes {
		if !f.Deterministic {
			out.Skipped = append(out.Skipped, Candidate{ID: f.DiagnosticID, Reason: "fix is not deterministic"})
			continue
		}
		out.Plan.Edits = append(out.Plan.Edits, f.Edit)
	}
	if len(out.Plan.Edits) == 0 {
		return out, errors.New("mutation: no deterministic fix")
	}
	return out, nil
}
func (p FixProvider) Plan(ctx context.Context, r Request) (Proposal, error) { return p.Prepare(ctx, r) }
func (provider FixProvider) Validate(_ context.Context, proposal Proposal) error {
	if proposal.Refusal != "" {
		return errors.New(proposal.Refusal)
	}
	return nil
}
