package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
)

func TestLoadWorkspaceParsesSourceOrderAndResolverPaths(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "chars", "hero.def")
	cmdPath := filepath.Join(root, "chars", "hero.cmd")
	stPath := filepath.Join(root, "chars", "state.st")
	st2Path := filepath.Join(root, "chars", "stage", "extra.st")
	stCommonPath := filepath.Join(root, "chars", "common", "common.st")

	writeTextFile(t, cmdPath, `[Command]
name = "jump"
`)
	writeTextFile(t, stPath, `[Statedef 100]
`)
	writeTextFile(t, st2Path, `[State 100]
type = ChangeState
value = 100
`)
	writeTextFile(t, stCommonPath, `[State 200]
type = ChangeState
value = 200
`)
	writeTextFile(
		t,
		defPath,
		`[Info]
name = "Hero"
[Files]
cmd = =/hero.cmd
st = =/state.st
st2 = =/stage/extra.st
stcommon = =/common/common.st
`,
	)

	result := LoadWorkspace(defPath)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected zero diagnostics, got %d", len(result.Diagnostics))
	}

	expected := []string{
		filepath.Clean(defPath),
		filepath.Clean(cmdPath),
		filepath.Clean(stPath),
		filepath.Clean(st2Path),
		filepath.Clean(stCommonPath),
	}
	if len(result.Documents) != len(expected) {
		t.Fatalf("expected %d documents, got %d", len(expected), len(result.Documents))
	}
	for i, path := range expected {
		if result.Documents[i].Path != path {
			t.Fatalf("document %d path mismatch: got %q want %q", i, result.Documents[i].Path, path)
		}
	}
}

func TestLoadWorkspaceReportsMissingSourcesAsDiagnostics(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "fighter.def")
	cmdPath := filepath.Join(root, "fighter.cmd")

	writeTextFile(t, cmdPath, `[Command]
name = "run"
`)
	writeTextFile(t, defPath, `[files]
cmd = fighter.cmd
st = missing.st
`)

	result := LoadWorkspace(defPath)
	if len(result.Documents) != 2 {
		t.Fatalf("expected root and command documents, got %d", len(result.Documents))
	}
	if result.Documents[1].Path != filepath.Clean(cmdPath) {
		t.Fatalf("expected command source to load before missing one, got %q", result.Documents[1].Path)
	}

	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %d", len(result.Diagnostics))
	}
	d := result.Diagnostics[0]
	if d.Code != "missing-source" {
		t.Fatalf("expected missing-source diagnostic, got %q", d.Code)
	}
	if d.Path != filepath.Clean(defPath) {
		t.Fatalf("expected diagnostic path %q, got %q", filepath.Clean(defPath), d.Path)
	}
	if d.Start.Line != 3 {
		t.Fatalf("expected missing-file diagnostic on line 3, got line %d", d.Start.Line)
	}
	if d.Severity != ir.SeverityError {
		t.Fatalf("expected error severity, got %q", d.Severity)
	}
}

func TestLoadWorkspaceDeduplicatesSourcePaths(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "unit.def")
	sharedPath := filepath.Join(root, "shared.st")

	writeTextFile(t, sharedPath, `[State 100]
`)
	writeTextFile(t, defPath, `[Files]
st = shared.st
stcommon = shared.st
st2 = shared.st
`)

	result := LoadWorkspace(defPath)
	if len(result.Documents) != 2 {
		t.Fatalf("expected duplicate source paths to be de-duplicated, got %d documents", len(result.Documents))
	}
	if result.Documents[1].Path != filepath.Clean(sharedPath) {
		t.Fatalf("unexpected source path %q", result.Documents[1].Path)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %d", len(result.Diagnostics))
	}
}

func TestLoadWorkspaceSupportsWindowsStyleSeparators(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "win", "fighter.def")
	cmdPath := filepath.Join(root, "win", "cmd", "hero.cmd")
	stPath := filepath.Join(root, "win", "st", "state.st")

	writeTextFile(t, cmdPath, `[Command]
name = "special"
`)
	writeTextFile(t, stPath, `[State 20]
type = ChangeState
value = 20
`)
	writeTextFile(t, defPath, `[Files]
cmd = \cmd\hero.cmd
st = \st\state.st
`)

	result := LoadWorkspace(defPath)
	if len(result.Documents) != 3 {
		t.Fatalf("expected three documents, got %d", len(result.Documents))
	}
	for _, doc := range result.Documents {
		if filepath.Base(doc.Path) == "fighter.def" {
			continue
		}
		if _, err := os.Stat(doc.Path); err != nil {
			t.Fatalf("document path should exist: %q", doc.Path)
		}
	}
}
func TestLoadWorkspaceFallsBackToGameRootSource(t *testing.T) {
	root := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(origWd); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir to workspace root: %v", err)
	}

	defPath := filepath.Join(root, "chars", "hero.def")
	fallbackPath := filepath.Join(root, "hero.st")

	writeTextFile(t, fallbackPath, `[State 100]
`)
	writeTextFile(
		t,
		defPath,
		`[Files]
st = hero.st
`,
	)

	result := LoadWorkspace(defPath)
	if len(result.Documents) != 2 {
		t.Fatalf("expected fallback source to resolve from game root, got %d", len(result.Documents))
	}
	if result.Documents[1].Path != filepath.Clean(fallbackPath) {
		t.Fatalf("expected resolved source path %q, got %q", filepath.Clean(fallbackPath), result.Documents[1].Path)
	}
}

func TestLoadWorkspaceFallsBackToDataDirectory(t *testing.T) {
	root := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(origWd); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir to workspace root: %v", err)
	}

	defPath := filepath.Join(root, "chars", "hero.def")
	dataSource := filepath.Join(root, "data", "hero.st")

	writeTextFile(t, dataSource, `[State 100]
`)
	writeTextFile(
		t,
		defPath,
		`[Files]
st = hero.st
`,
	)

	result := LoadWorkspace(defPath)
	if len(result.Documents) != 2 {
		t.Fatalf("expected fallback source to resolve from data dir, got %d", len(result.Documents))
	}
	if result.Documents[1].Path != filepath.Clean(dataSource) {
		t.Fatalf("expected resolved source path %q, got %q", filepath.Clean(dataSource), result.Documents[1].Path)
	}
}

func TestLoadWorkspacePreservesAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	absPath := filepath.Join(root, "absolute", "hero.st")
	defPath := filepath.Join(root, "hero.def")

	writeTextFile(t, absPath, `[State 100]
`)
	writeTextFile(
		t,
		defPath,
		`[Files]
st = `+absPath+`
`,
	)

	result := LoadWorkspace(defPath)
	if len(result.Documents) != 2 {
		t.Fatalf("expected absolute source to resolve, got %d", len(result.Documents))
	}
	if result.Documents[1].Path != filepath.Clean(absPath) {
		t.Fatalf("expected absolute source path %q, got %q", filepath.Clean(absPath), result.Documents[1].Path)
	}
}

func TestLoadWorkspaceResolvesCmdAndNumberedStSources(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "slice.def")
	cmdPath := filepath.Join(root, "slice.cmd")
	stPath := filepath.Join(root, "slice.st")
	st3Path := filepath.Join(root, "slice3.st")

	writeTextFile(t, cmdPath, `[Command]
name = "x"
`)
	writeTextFile(t, stPath, `[State 300]
type = ChangeState
value = 300
`)
	writeTextFile(t, st3Path, `[State 301]
type = SelfState
value = 301
`)
	writeTextFile(t, defPath, `[files]
cmd = slice.cmd
st = slice.st
st3 = slice3.st
`)

	result := LoadWorkspace(defPath)
	if len(result.Documents) != 4 {
		t.Fatalf("expected 4 documents (def, cmd, st, st3), got %d", len(result.Documents))
	}
	if result.Documents[0].Path != filepath.Clean(defPath) {
		t.Fatalf("expected root document first")
	}
}

func TestLoadWorkspaceDoesNotReloadRootDefinition(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "hero.def")
	writeTextFile(t, defPath, `[Files]
st = hero.def
`)

	result := LoadWorkspace(defPath)
	if len(result.Documents) != 1 {
		t.Fatalf("expected self-referencing root to load once, got %d documents", len(result.Documents))
	}
}

func writeTextFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write file %q: %v", path, err)
	}
}
