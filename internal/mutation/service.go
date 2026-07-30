package mutation

import (
	"context"
	"errors"
	"github.com/ikemen-engine/ikemen-devtools/internal/patch"
)

type Service struct{ Authorizer *Authorizer }

func (s Service) Apply(ctx context.Context, root, token, snapshot string) (patch.ApplyResult, error) {
	if err := ctx.Err(); err != nil {
		return patch.ApplyResult{}, err
	}
	if s.Authorizer == nil {
		return patch.ApplyResult{}, errors.New("mutation authorization required")
	}
	plan, err := s.Authorizer.Consume(token, snapshot)
	if err != nil {
		return patch.ApplyResult{}, err
	}
	if err = plan.Validate(); err != nil {
		return patch.ApplyResult{}, err
	}
	edits := patch.Patch{Edits: plan.Edits}
	return patch.ApplyAtomic(root, edits)
}
