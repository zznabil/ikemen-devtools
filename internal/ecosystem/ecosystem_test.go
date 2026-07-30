package ecosystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
)

func TestAnalyzeSelectRosterStagesAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	selectPath := filepath.Join(root, "select.def")
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "hero.def"), "[Files]\ncmd = hero.cmd\n")
	write(filepath.Join(root, "arena.def"), "")
	write(selectPath, `[Characters]
hero.def, order=1
hero.def, order=2
missing.def
randomselect
[Stages]
arena.def
missing-stage.def
[ExtraStages]
arena.def
`)

	report := AnalyzeSelect(selectPath, profile.NewStrictPortableProfile(root))
	if len(report.Characters) != 3 || len(report.Stages) != 3 {
		t.Fatalf("entries: %d chars, %d stages", len(report.Characters), len(report.Stages))
	}
	if report.Characters[0].DeclaredPath != "hero.def" || report.Characters[0].Options[0] != "order=1" {
		t.Fatalf("character options not parsed: %#v", report.Characters[0])
	}
	if report.Characters[1].Status != StatusDuplicate {
		t.Fatalf("duplicate status: %q", report.Characters[1].Status)
	}
	if report.Characters[2].Status != StatusMissing || report.Stages[1].Status != StatusMissing {
		t.Fatalf("missing statuses: %#v %#v", report.Characters[2], report.Stages[1])
	}
	if report.Stages[0].Section != "Stages" || report.Stages[2].Section != "ExtraStages" {
		t.Fatalf("stage sections: %#v", report.Stages)
	}
}

func TestAnalyzeCharacterDEFManifestPathsAndLeadingEquals(t *testing.T) {
	root := t.TempDir()
	defDir := filepath.Join(root, "chars")
	if err := os.MkdirAll(defDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hero.cmd", "hero.st", "shared.st"} {
		if err := os.WriteFile(filepath.Join(defDir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(defDir, "="), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defDir, "=", "hero.st"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defPath := filepath.Join(defDir, "hero.def")
	if err := os.WriteFile(defPath, []byte(`[Files]
cmd = hero.cmd
st = =/hero.st
st2 = shared.st
st3 = missing.st
`), 0o644); err != nil {
		t.Fatal(err)
	}
	report := AnalyzeCharacterDEF(defPath, profile.NewDistributionProfile(root))
	if len(report.Files) != 4 {
		t.Fatalf("manifest entries: %d", len(report.Files))
	}
	if report.Files[1].ResolvedPath != filepath.Join(defDir, "=", "hero.st") {
		t.Fatalf("leading equals resolved to %q", report.Files[1].ResolvedPath)
	}
	if report.Files[3].Status != StatusMissing {
		t.Fatalf("missing manifest status: %q", report.Files[3].Status)
	}
	if report.Files[2].Status != StatusResolved {
		t.Fatalf("relative manifest status: %q", report.Files[2].Status)
	}
}

func TestAnalyzeDeterministic(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "select.def")
	if err := os.WriteFile(path, []byte("[Characters]\na.def\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := profile.NewStrictPortableProfile(root)
	a, b := AnalyzeSelect(path, p), AnalyzeSelect(path, p)
	if a.JSONString() != b.JSONString() {
		t.Fatal("analysis output is not deterministic")
	}
}
func TestAnalyzeAIRValidAndMissingAssets(t *testing.T) {
	root := t.TempDir()
	airPath := filepath.Join(root, "hero.air")
	if err := os.WriteFile(filepath.Join(root, "hero.sff"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(airPath, []byte(`[Begin Action 0]
sprite = hero.sff
sound = missing.snd
0, 0, 0, 0, 1
loopstart
1, 0, 2, 3, -1
[End Action]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	report := AnalyzeAIR(airPath, profile.NewStrictPortableProfile(root))
	if len(report.Actions) != 1 || len(report.Assets) != 2 {
		t.Fatalf("AIR entries: actions=%d assets=%d", len(report.Actions), len(report.Assets))
	}
	if report.Assets[0].Status != StatusResolved || report.Assets[1].Status != StatusMissing {
		t.Fatalf("asset statuses: %#v", report.Assets)
	}
	if report.Actions[0].Rows != 2 || !report.Actions[0].LoopStart {
		t.Fatalf("action metadata: %#v", report.Actions[0])
	}
}

func TestAnalyzeAIRMalformedRowsAndDuplicateRanges(t *testing.T) {
	root := t.TempDir()
	airPath := filepath.Join(root, "bad.air")
	if err := os.WriteFile(airPath, []byte(`[Begin Action -1]
0, 0, 0
[End Action]
[Begin Action 2]
0, -1, 0, 0, 1
0, 0, 0, 0, -2
[End Action]
[Begin Action 2]
[End Action]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	report := AnalyzeAIR(airPath, profile.NewStrictPortableProfile(root))
	codes := make(map[string]bool)
	for _, d := range report.Diagnostics {
		codes[d.Code] = true
	}
	for _, code := range []string{"invalid-action-number", "malformed-action-row", "invalid-sprite-range", "invalid-duration", "duplicate-action"} {
		if !codes[code] {
			t.Fatalf("missing diagnostic %q: %#v", code, report.Diagnostics)
		}
	}
}

func TestAnalyzeAIRDeterministic(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "stable.air")
	if err := os.WriteFile(path, []byte("[Begin Action 0]\n0, 0, 0, 0, 1\n[End Action]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := profile.NewStrictPortableProfile(root)
	a, b := AnalyzeAIR(path, p), AnalyzeAIR(path, p)
	if a.JSONString() != b.JSONString() {
		t.Fatal("AIR analysis output is not deterministic")
	}
}
