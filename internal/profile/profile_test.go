package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDistributionProfileKeepsLeadingEqualsComponent(t *testing.T) {
	type tCase struct {
		name string
		raw  string
	}

	for _, tc := range []tCase{
		{name: "slash", raw: "=/cmd.cmd"},
		{name: "backslash", raw: "=\\cmd.cmd"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			baseDir := filepath.Join(root, "chars")
			eqCmdPath := filepath.Join(baseDir, "=", "cmd.cmd")
			if err := os.MkdirAll(filepath.Dir(eqCmdPath), 0o755); err != nil {
				t.Fatalf("mkdirAll: %v", err)
			}
			if err := os.WriteFile(eqCmdPath, []byte("[Command]\n"), 0o644); err != nil {
				t.Fatalf("write equals command: %v", err)
			}

			p := NewDistributionProfile(root)
			got := p.ResolveSourcePath(baseDir, tc.raw)
			expected := filepath.Clean(eqCmdPath)
			if got != expected {
				t.Fatalf("expected %q, got %q", expected, got)
			}
		})
	}
}

func TestStrictPortableProfileStripsLeadingEqualsPrefix(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "chars")
	cmdPath := filepath.Join(baseDir, "cmd.cmd")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}
	if err := os.WriteFile(cmdPath, []byte("[Command]\n"), 0o644); err != nil {
		t.Fatalf("write command: %v", err)
	}

	p := NewStrictPortableProfile(root)
	got := p.ResolveSourcePath(baseDir, "=/cmd.cmd")
	expected := filepath.Clean(cmdPath)
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestStrictPortableProfileFallsBackToWorkspaceDataRoot(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "chars")
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}
	fallbackPath := filepath.Join(root, "data", "shared.st")
	if err := os.WriteFile(fallbackPath, []byte("[Statedef 100]\n"), 0o644); err != nil {
		t.Fatalf("write fallback: %v", err)
	}

	p := NewStrictPortableProfile(root)
	got := p.ResolveSourcePath(baseDir, "shared.st")
	expected := filepath.Clean(fallbackPath)
	if got != expected {
		t.Fatalf("expected fallback path %q, got %q", expected, got)
	}
}

func TestProfileCasePolicyControlsDedupKey(t *testing.T) {
	p := NewStrictPortableProfile("")
	p.CasePolicy = CasePolicyInsensitive

	if p.DedupKey("Path/CASE.TXT") != p.DedupKey("path/case.txt") {
		t.Fatal("case-insensitive dedupe policy should normalize path keys")
	}

	p.CasePolicy = CasePolicySensitive
	if p.DedupKey("Path/CASE.TXT") == p.DedupKey("path/case.txt") {
		t.Fatal("case-sensitive dedupe policy should preserve case for path keys")
	}
}
func TestExternalPathPolicyCanRestrictWorkspaceScope(t *testing.T) {
	root := t.TempDir()
	insideRoot := filepath.Join(root, "inner")
	inside := filepath.Join(insideRoot, "inner.txt")
	outsideRoot := filepath.Join(root, "other-space")
	outside := filepath.Join(outsideRoot, "outer.txt")

	if err := os.MkdirAll(insideRoot, 0o755); err != nil {
		t.Fatalf("mkdirAll inside: %v", err)
	}
	if err := os.WriteFile(inside, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write inside file: %v", err)
	}
	if err := os.MkdirAll(outsideRoot, 0o755); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}
	if err := os.WriteFile(outside, []byte("no"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	p := NewStrictPortableProfile(insideRoot)
	p.ExternalPathPolicy = ExternalPathPolicyWorkspaceOnly
	if got := p.ResolveSourcePath(insideRoot, inside); filepath.Clean(got) != filepath.Clean(inside) {
		t.Fatalf("expected in-scope absolute path %q, got %q", inside, got)
	}
	if got := p.ResolveSourcePath(insideRoot, outside); got != "" {
		t.Fatalf("expected rejected out-of-scope absolute path, got %q", got)
	}
}
