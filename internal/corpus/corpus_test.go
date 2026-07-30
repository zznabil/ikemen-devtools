package corpus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
)

func TestBuildManifestBuildsEntriesAndResolvesSources(t *testing.T) {
	root := t.TempDir()
	charsDir := filepath.Join(root, "chars")
	if err := os.MkdirAll(charsDir, 0o755); err != nil {
		t.Fatalf("mkdir chars: %v", err)
	}

	writeTextFile(t, filepath.Join(charsDir, "alpha.def"), `[Info]
name = "Alpha"
`)
	writeTextFile(t, filepath.Join(charsDir, "beta.def"), `[Info]
name = "Beta"
`)
	writeTextFile(
		t,
		filepath.Join(root, "select.def"),
		`[Characters]
; roster lines
randomselect
----
chars/alpha.def, order=1
chars/beta.def,order=2
chars/alpha.def, order=3
chars/does-not-exist.def, bonus=1
`,
	)

	m := BuildManifest(filepath.Join(root, "select.def"), profile.NewDistributionProfile(""))
	if m.DeclaredSourceCount != 4 {
		t.Fatalf("expected 4 declared entries, got %d", m.DeclaredSourceCount)
	}
	if m.ResolvedSourceCount != 2 {
		t.Fatalf("expected 2 resolved sources, got %d", m.ResolvedSourceCount)
	}
	if m.ErrorCount != 1 {
		t.Fatalf("expected 1 error diagnostic, got %d", m.ErrorCount)
	}
	if len(m.Entries) != 4 {
		t.Fatalf("expected 4 manifest entries, got %d", len(m.Entries))
	}
	if m.Entries[0].Status != entryStatusResolved {
		t.Fatalf("expected first entry resolved, got %q", m.Entries[0].Status)
	}
	if m.Entries[2].Status != entryStatusDeduplicated {
		t.Fatalf("expected duplicate entry deduplicated, got %q", m.Entries[2].Status)
	}
	if m.Entries[3].Status != entryStatusMissing {
		t.Fatalf("expected missing entry missing, got %q", m.Entries[3].Status)
	}
	if m.Entries[0].Span.Start.Line != 5 || m.Entries[0].Span.Start.Column <= 0 {
		t.Fatalf("expected positive span for first entry, got %+v", m.Entries[0].Span)
	}
	if len(m.Entries[0].Options) != 1 || m.Entries[0].Options[0] != "order=1" {
		t.Fatalf("expected options parsed for first entry, got %v", m.Entries[0].Options)
	}
}

func TestBuildManifestResolvesLeadingEqualsPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "="), 0o755); err != nil {
		t.Fatalf("mkdir equals dir: %v", err)
	}
	writeTextFile(t, filepath.Join(root, "=", "eq.def"), `[Info]
name = "Equals"
`)

	writeTextFile(
		t,
		filepath.Join(root, "select.def"),
		`[Characters]
=/eq.def
`,
	)

	m := BuildManifest(filepath.Join(root, "select.def"), profile.NewDistributionProfile(""))
	if m.ErrorCount != 0 {
		t.Fatalf("expected leading-equals source to resolve, got %d errors", m.ErrorCount)
	}
	if m.Entries[0].ResolvedPath != filepath.Clean(filepath.Join(root, "=", "eq.def")) {
		t.Fatalf("expected leading-equals path to resolve to equals directory, got %q", m.Entries[0].ResolvedPath)
	}
}

func TestBuildManifestResolvesDistributionRosterFromCharsDirectory(t *testing.T) {
	root := t.TempDir()
	charDir := filepath.Join(root, "chars", "Hero")
	if err := os.MkdirAll(charDir, 0o755); err != nil {
		t.Fatalf("mkdir character directory: %v", err)
	}
	writeTextFile(t, filepath.Join(charDir, "Hero.def"), `[Info]
name = "Hero"
`)
	selectPath := filepath.Join(root, "data", "select.def")
	if err := os.MkdirAll(filepath.Dir(selectPath), 0o755); err != nil {
		t.Fatalf("mkdir data directory: %v", err)
	}
	writeTextFile(t, selectPath, `[Characters]
Hero/Hero.def
`)

	m := BuildManifest(selectPath, profile.NewDistributionProfile(""))
	if m.ErrorCount != 0 {
		t.Fatalf("expected chars-directory roster entry to resolve, got %d errors", m.ErrorCount)
	}
	if got, want := m.Entries[0].ResolvedPath, filepath.Join(root, "chars", "Hero", "Hero.def"); got != want {
		t.Fatalf("expected resolved path %q, got %q", want, got)
	}
}

func TestBuildManifestDeterministicJSON(t *testing.T) {
	root := t.TempDir()
	charsDir := filepath.Join(root, "chars")
	if err := os.MkdirAll(charsDir, 0o755); err != nil {
		t.Fatalf("mkdir chars: %v", err)
	}
	writeTextFile(t, filepath.Join(charsDir, "hero.def"), `[Info]
name = "Hero"
`)
	writeTextFile(
		t,
		filepath.Join(root, "select.def"),
		`[Characters]
chars/hero.def
`,
	)

	m1 := BuildManifest(filepath.Join(root, "select.def"), profile.NewDistributionProfile(""))
	m2 := BuildManifest(filepath.Join(root, "select.def"), profile.NewDistributionProfile(""))

	b1, err1 := m1.JSON()
	if err1 != nil {
		t.Fatalf("first JSON marshal: %v", err1)
	}
	b2, err2 := m2.JSON()
	if err2 != nil {
		t.Fatalf("second JSON marshal: %v", err2)
	}

	if string(b1) != string(b2) {
		t.Fatalf("expected deterministic JSON, first=%q second=%q", b1, b2)
	}

	if err := json.Unmarshal(b1, &Manifest{}); err != nil {
		t.Fatalf("expected valid JSON payload, got %v", err)
	}
}

func TestBuildManifestWarnsWhenCharactersSectionIsMissing(t *testing.T) {
	root := t.TempDir()
	writeTextFile(t, filepath.Join(root, "select.def"), `[ExtraStages]
stages\arena.def
`)

	m := BuildManifest(filepath.Join(root, "select.def"), profile.NewDistributionProfile(""))
	if m.ErrorCount != 0 {
		t.Fatalf("missing [Characters] should warn, not error, got %d errors", m.ErrorCount)
	}
	if m.WarningCount != 1 {
		t.Fatalf("expected one warning for missing [Characters], got %d", m.WarningCount)
	}
}

func TestBuildManifestSkipsCommentAndRandomRowsInCharacters(t *testing.T) {
	root := t.TempDir()
	charsDir := filepath.Join(root, "chars")
	if err := os.MkdirAll(charsDir, 0o755); err != nil {
		t.Fatalf("mkdir chars: %v", err)
	}

	writeTextFile(t, filepath.Join(charsDir, "alpha.def"), `[Info]
name = "Alpha"
`)
	writeTextFile(t, filepath.Join(charsDir, "beta.def"), `[Info]
name = "Beta"
`)
	writeTextFile(
		t,
		filepath.Join(root, "select.def"),
		`# top comment
[Characters]
; ignored comment
chars/alpha.def, order=1 ; inline comment

randomselect
-----
chars/beta.def, option=keep
# tail comment
`,
	)

	m := BuildManifest(filepath.Join(root, "select.def"), profile.NewDistributionProfile(""))
	if len(m.Entries) != 2 {
		t.Fatalf("expected 2 parsed entries, got %d", len(m.Entries))
	}
	if m.Entries[0].DeclaredPath != filepath.ToSlash(filepath.Join("chars", "alpha.def")) {
		t.Fatalf("unexpected first entry path %q", m.Entries[0].DeclaredPath)
	}
	if m.Entries[1].DeclaredPath != filepath.ToSlash(filepath.Join("chars", "beta.def")) {
		t.Fatalf("unexpected second entry path %q", m.Entries[1].DeclaredPath)
	}
	if m.Entries[0].Span.Start.Line != 4 {
		t.Fatalf("expected first entry at line 4, got %+v", m.Entries[0].Span)
	}
	if m.Entries[1].Span.Start.Line != 8 {
		t.Fatalf("expected second entry at line 8, got %+v", m.Entries[1].Span)
	}
	if len(m.Entries[0].Options) != 1 || m.Entries[0].Options[0] != "order=1" {
		t.Fatalf("expected options for first entry, got %v", m.Entries[0].Options)
	}
}

func TestBuildManifestDeduplicatesResolvedSourcesInInputOrder(t *testing.T) {
	root := t.TempDir()
	writeTextFile(t, filepath.Join(root, "chars.def"), `[Info]
name = "Chars"
`)
	writeTextFile(
		t,
		filepath.Join(root, "select.def"),
		`[Characters]
chars.def
./chars.def
`,
	)

	m := BuildManifest(filepath.Join(root, "select.def"), profile.NewDistributionProfile(""))
	if len(m.Entries) != 2 {
		t.Fatalf("expected 2 manifest entries, got %d", len(m.Entries))
	}
	if m.Entries[0].Status != entryStatusResolved {
		t.Fatalf("expected first entry resolved, got %q", m.Entries[0].Status)
	}
	if m.Entries[1].Status != entryStatusDeduplicated {
		t.Fatalf("expected second entry deduplicated, got %q", m.Entries[1].Status)
	}
	if m.ResolvedSourceCount != 1 {
		t.Fatalf("expected deduped resolved count 1, got %d", m.ResolvedSourceCount)
	}
}

func TestSplitLineSupportsHashComments(t *testing.T) {
	code, comment := splitLine(`chars/hero.def # comment`)
	if code != "chars/hero.def" {
		t.Fatalf("expected trimmed code, got %q", code)
	}
	if comment != "comment" {
		t.Fatalf("expected comment to capture trailing hash comment, got %q", comment)
	}
}

func writeTextFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
