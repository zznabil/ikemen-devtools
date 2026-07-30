package mutation

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/ikemen-engine/ikemen-devtools/internal/patch"
	"sync"
)

type Authorizer struct {
	mu         sync.Mutex
	AllowWrite bool
	tokens     map[string]patch.PatchPlan
}

func NewAuthorizer(allow bool) *Authorizer {
	return &Authorizer{AllowWrite: allow, tokens: map[string]patch.PatchPlan{}}
}
func (a *Authorizer) Issue(plan patch.PatchPlan) (string, error) {
	if a == nil || !a.AllowWrite {
		return "", errors.New("mutation write capability disabled")
	}
	b, _ := plan.JSON()
	h := sha256.Sum256(b)
	token := hex.EncodeToString(h[:])
	a.mu.Lock()
	a.tokens[token] = plan
	a.mu.Unlock()
	return token, nil
}
func (a *Authorizer) Consume(token string, currentSnapshot string) (patch.PatchPlan, error) {
	if a == nil {
		return patch.PatchPlan{}, errors.New("mutation authorizer unavailable")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.tokens[token]
	if !ok {
		return patch.PatchPlan{}, errors.New("invalid or consumed mutation token")
	}
	if currentSnapshot != "" && p.InputSnapshot != currentSnapshot {
		return patch.PatchPlan{}, errors.New("stale mutation token")
	}
	delete(a.tokens, token)
	return p, nil
}
