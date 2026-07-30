package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
)

func TestLoadWorkspaceWithDistributionProfileResolvesLeadingEqualsAsLiteralPath(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "chars")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}

	eqCmdPath := filepath.Join(baseDir, "=", "cmd.cmd")
	if err := os.MkdirAll(filepath.Dir(eqCmdPath), 0o755); err != nil {
		t.Fatalf("mkdirAll equals path: %v", err)
	}
	if err := os.WriteFile(eqCmdPath, []byte("[Command]\nname = \"jump\"\n"), 0o644); err != nil {
		t.Fatalf("write equals command: %v", err)
	}

	defPath := filepath.Join(baseDir, "hero.def")
	if err := os.WriteFile(defPath, []byte(`[Files]
cmd = =/cmd.cmd
`), 0o644); err != nil {
	}

	result := LoadWorkspaceWithProfile(defPath, profile.NewDistributionProfile(""))
	if len(result.Documents) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(result.Documents))
	}
	if result.Documents[1].Path != filepath.Clean(eqCmdPath) {
		t.Fatalf("expected equals-literal path %q, got %q", filepath.Clean(eqCmdPath), result.Documents[1].Path)
	}
}

func TestLoadWorkspaceWithStrictPortableProfileStripsLeadingEquals(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "chars")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}

	cmdPath := filepath.Join(baseDir, "cmd.cmd")
	if err := os.WriteFile(cmdPath, []byte("[Command]\nname = \"run\"\n"), 0o644); err != nil {
		t.Fatalf("write command: %v", err)
	}

	defPath := filepath.Join(baseDir, "hero.def")
	if err := os.WriteFile(defPath, []byte(`[Files]
cmd = =/cmd.cmd
`), 0o644); err != nil {
	}

	result := LoadWorkspaceWithProfile(defPath, profile.NewStrictPortableProfile(""))
	if len(result.Documents) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(result.Documents))
	}
	if result.Documents[1].Path != filepath.Clean(cmdPath) {
		t.Fatalf("expected strict-portable path %q, got %q", filepath.Clean(cmdPath), result.Documents[1].Path)
	}
}

func TestLoadWorkspaceWithProfileCanResolveFallbackDataDirectory(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "chars")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}

	dataPath := filepath.Join(root, "data", "hero.st")
	if err := os.WriteFile(dataPath, []byte("[Statedef 100]\n"), 0o644); err != nil {
		t.Fatalf("write fallback source: %v", err)
	}

	if err := os.WriteFile(filepath.Join(baseDir, "hero.def"), []byte(`[Files]
st = hero.st
`), 0o644); err != nil {
		t.Fatalf("write def: %v", err)
	}

	result := LoadWorkspaceWithProfile(filepath.Join(baseDir, "hero.def"), profile.NewStrictPortableProfile(root))
	if len(result.Documents) != 2 {
		t.Fatalf("expected root and fallback source, got %d", len(result.Documents))
	}
	if result.Documents[1].Path != filepath.Clean(dataPath) {
		t.Fatalf("expected data fallback path %q, got %q", filepath.Clean(dataPath), result.Documents[1].Path)
	}
}
