package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDeterministicDiscoveryAndSnapshot(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".ikm"), 0o755)
	os.WriteFile(filepath.Join(root, "a.def"), []byte("[Files]\n"), 0o644)
	cfg := DefaultWorkspaceConfig()
	cfg.Root = root
	cfg.EntryPoints = []string{"a.def"}
	a, e := Discover(root, cfg)
	if e != nil {
		t.Fatal(e)
	}
	b, e := Discover(root, cfg)
	if e != nil || a.Snapshot().ID != b.Snapshot().ID {
		t.Fatalf("nondeterministic snapshots")
	}
	if len(a.Files) != 1 || !a.Files[0].Active {
		t.Fatalf("active classification=%#v", a.Files)
	}
}
func TestSessionCancellationKeepsCommittedSnapshot(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultWorkspaceConfig()
	cfg.Root = root
	s := NewSession(root, cfg)
	if _, e := s.Scan(context.Background()); e != nil {
		t.Fatal(e)
	}
	before := s.Snapshot()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := s.Scan(ctx)
	if e == nil || s.Snapshot().ID != before.ID {
		t.Fatal("cancel replaced committed snapshot")
	}
}
