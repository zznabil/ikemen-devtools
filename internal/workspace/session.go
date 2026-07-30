package workspace

import (
	"context"
	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
	"sync"
)

type Session struct {
	mu         sync.RWMutex
	root       string
	cfg        WorkspaceConfig
	current    Snapshot
	generation uint64
	running    context.CancelFunc
}

func NewSession(root string, cfg WorkspaceConfig) *Session { return &Session{root: root, cfg: cfg} }
func (s *Session) Scan(ctx context.Context) (Snapshot, error) {
	s.mu.Lock()
	if s.running != nil {
		s.running()
	}
	work, cancel := context.WithCancel(ctx)
	s.running = cancel
	s.mu.Unlock()
	d, e := Discover(s.root, s.cfg)
	if e != nil {
		return Snapshot{}, e
	}
	select {
	case <-work.Done():
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.current, work.Err()
	default:
	}
	snap := d.Snapshot()
	s.mu.Lock()
	s.current = snap
	s.generation++
	s.running = nil
	s.mu.Unlock()
	return snap, nil
}
func (s *Session) Snapshot() Snapshot                    { s.mu.RLock(); defer s.mu.RUnlock(); return s.current }
func (s *Session) Generation() uint64                    { s.mu.RLock(); defer s.mu.RUnlock(); return s.generation }
func (s *Session) Profile() profile.CompatibilityProfile { return s.cfg.ProfileValue() }
