package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func putManifestFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestStartupManifestFullClosureMixedPathsAndInlineStage(t *testing.T) {
	root := t.TempDir()
	putManifestFile(t, root, "save/config.json", `{"System":"DATA\\system.def","Motif":"data/motif/system.def","StartStage":"=data/stages/start.def","TrainingChar":"chars\\hero.def","CommonStates":"common/common.def"}`)
	putManifestFile(t, root, "data/system.def", `[Files]
select = "DATA\\select.def"
fight = =data/fight.def
fonts = data/fonts/arc.fnt
`)
	putManifestFile(t, root, "data/motif/system.def", `[Files]
select = "../select.def"
assets = "../assets/ui.air"
`)
	putManifestFile(t, root, "data/select.def", `[Characters]
"chars/HERO.def", STAGES\\arena.def
[Stages]
stages/arena.def
`)
	putManifestFile(t, root, "chars/hero.def", `[Files]
cmd = "hero.cmd"
cns = =hero.cns
st = states/hero.st
air = hero.air
sff = hero.sff
snd = hero.snd
`)
	putManifestFile(t, root, "chars/hero.cmd", "[Command]\nname = x\n")
	putManifestFile(t, root, "chars/hero.cns", "[Statedef 0]\ntype = S\n")
	putManifestFile(t, root, "chars/states/hero.st", "[Statedef 1]\n")
	putManifestFile(t, root, "chars/hero.air", "")
	putManifestFile(t, root, "chars/hero.sff", "")
	putManifestFile(t, root, "chars/hero.snd", "")
	putManifestFile(t, root, "stages/arena.def", `[Files]
sff = arena.sff
snd = arena.snd
assets = "gfx\\arena.air"
`)
	for _, p := range []string{"data/fight.def", "data/fonts/arc.fnt", "data/assets/ui.air", "data/stages/start.def", "common/common.def", "stages/arena.sff", "stages/arena.snd", "stages/gfx/arena.air"} {
		putManifestFile(t, root, p, "")
	}
	files, diags := resolveStartupManifest(root)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	must := []string{"data/select.def", "chars/hero.def", "chars/hero.cmd", "chars/hero.cns", "chars/states/hero.st", "stages/arena.def", "stages/arena.sff", "stages/arena.snd"}
	set := map[string]bool{}
	for _, p := range files {
		set[strings.ToLower(filepath.ToSlash(p))] = true
	}
	for _, p := range must {
		if !set[strings.ToLower(filepath.ToSlash(p))] {
			t.Errorf("closure missing %s", p)
		}
	}
	if len(files) < 15 {
		t.Fatalf("expected broad closure, got %d files", len(files))
	}
}

func TestStartupManifestResolvesSelectCharacterConventions(t *testing.T) {
	root := t.TempDir()
	putManifestFile(t, root, "save/config.json", `{"System":"data/system.def"}`)
	putManifestFile(t, root, "data/system.def", "[Files]\nselect = select.def\n")
	putManifestFile(t, root, "data/select.def", `[Characters]
empty
randomselect
Hero
Villain\Villain.def
chars/Boss/Boss.def
Hero, order=1
[Stages]
stages/arena.def
`)
	putManifestFile(t, root, "chars/Hero/Hero.def", "")
	putManifestFile(t, root, "chars/Villain/Villain.def", "")
	putManifestFile(t, root, "chars/Boss/Boss.def", "")
	putManifestFile(t, root, "stages/arena.def", "")

	files, diags := resolveStartupManifest(root)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	joined := strings.ToLower(strings.Join(files, "\n"))
	for _, want := range []string{"chars/hero/hero.def", "chars/villain/villain.def", "chars/boss/boss.def", "stages/arena.def"} {
		if !strings.Contains(joined, want) {
			t.Errorf("closure missing %s", want)
		}
	}
}

func TestStartupManifestPrefersLiteralEqualsDirectoryBeforeLegacyPrefix(t *testing.T) {
	root := t.TempDir()
	putManifestFile(t, root, "save/config.json", `{"TrainingChar":"chars/Hero/Hero.def"}`)
	putManifestFile(t, root, "data/select.def", "")
	putManifestFile(t, root, "chars/Hero/Hero.def", "[Files]\ncmd = =/cmd.cmd\ncns = =legacy.cns\nstcommon = common1.cns\n")
	putManifestFile(t, root, "chars/Hero/=/cmd.cmd", "")
	putManifestFile(t, root, "chars/Hero/legacy.cns", "")

	files, diags := resolveStartupManifest(root)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	joined := strings.ToLower(strings.Join(files, "\n"))
	for _, want := range []string{"chars/hero/=/cmd.cmd", "chars/hero/legacy.cns"} {
		if !strings.Contains(joined, want) {
			t.Errorf("closure missing %s", want)
		}
	}
}

func TestStartupManifestDiagnosticsContainmentAndDeterminism(t *testing.T) {
	root := t.TempDir()
	putManifestFile(t, root, "save/config.json", `{"System":"system.def"}`)
	putManifestFile(t, root, "system.def", `[Files]\nselect = missing.def\nassets = ../escape.air\n`)
	putManifestFile(t, root, "select.def", `[Characters]\nhero.def, stages/inline.def\n`)
	putManifestFile(t, root, "hero.def", "[Files]\ncns = hero.cns\n")
	putManifestFile(t, root, "hero.cns", "")
	a, ad := resolveStartupManifest(root)
	b, bd := resolveStartupManifest(root)
	if strings.Join(a, "\n") != strings.Join(b, "\n") || strings.Join(ad, "\n") != strings.Join(bd, "\n") {
		t.Fatal("manifest resolution is nondeterministic")
	}
	joined := strings.Join(ad, " ")
	if !strings.Contains(joined, "missing-manifest-file") {
		t.Fatalf("missing diagnostic absent: %v", ad)
	}
	if strings.Contains(strings.Join(a, " "), "escape") {
		t.Fatal("root escape entered manifest")
	}
	cfg, _ := ResolveConfig(root, ConfigFlags{})
	d1, _ := Discover(root, cfg)
	d2, _ := Discover(root, cfg)
	if d1.Snapshot().ID != d2.Snapshot().ID || cfg.Digest() != cfg.Digest() {
		t.Fatal("identity is nondeterministic")
	}
	raw, _ := json.Marshal(cfg)
	if !strings.Contains(string(raw), "startupManifest") {
		t.Fatal("config identity omitted startup manifest")
	}
}

func TestExplicitEntryPointsPrecedeStartupDerivationAndDefaultsExcluded(t *testing.T) {
	root := t.TempDir()
	putManifestFile(t, root, ".ikm/config.json", `{"version":"0.1","profile":"strict/portable","entryPoints":["authored.def"]}`)
	putManifestFile(t, root, "authored.def", "")
	putManifestFile(t, root, "save/config.json", `{"System":"missing.def"}`)
	putManifestFile(t, root, "_upstream/noise.def", "")
	putManifestFile(t, root, "tooling/noise.def", "")
	putManifestFile(t, root, "cache/noise.def", "")
	putManifestFile(t, root, "logs/noise.def", "")
	putManifestFile(t, root, "generated/noise.def", "")
	cfg, err := ResolveConfig(root, ConfigFlags{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.EntryPoints) != 1 || cfg.EntryPoints[0] != "authored.def" {
		t.Fatalf("explicit entrypoints lost: %#v", cfg.EntryPoints)
	}
	d, err := Discover(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range d.Files {
		if strings.Contains(f.Path, "noise.def") {
			t.Fatalf("excluded default discovered: %s", f.Path)
		}
	}
}
