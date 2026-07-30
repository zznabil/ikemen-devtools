package mutation

import (
	"context"
	"errors"
	"github.com/ikemen-engine/ikemen-devtools/internal/patch"
)

var ErrAmbiguous = errors.New("mutation: ambiguous target")
var ErrUnsupported = errors.New("mutation: unsupported target")

type Candidate struct{ ID, Path, Reason string }
type Proposal struct {
	Plan       patch.PatchPlan
	Confidence string
	Candidates []Candidate
	Skipped    []Candidate
	Refusal    string
}
type Provider interface {
	Prepare(context.Context, Request) (Proposal, error)
	Plan(context.Context, Request) (Proposal, error)
	Validate(context.Context, Proposal) error
}
type Request struct {
	Root           string
	SymbolID       string
	Replacement    string
	Snapshot       string
	AllowAmbiguous bool
}
type Registry struct{ providers map[string]Provider }

func NewRegistry() *Registry { return &Registry{providers: map[string]Provider{}} }
func (r *Registry) Register(kind string, p Provider) error {
	if kind == "" || p == nil {
		return errors.New("mutation: invalid provider")
	}
	if _, ok := r.providers[kind]; ok {
		return errors.New("mutation: provider already registered")
	}
	r.providers[kind] = p
	return nil
}
func (r *Registry) Get(kind string) (Provider, bool) { p, ok := r.providers[kind]; return p, ok }
