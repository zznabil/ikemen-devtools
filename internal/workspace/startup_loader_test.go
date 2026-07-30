package workspace

import (
	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
	"path/filepath"
	"testing"
)

func TestLoadWorkspaceSelectUsesStartupClosure(t *testing.T) {
	root := t.TempDir()
	putManifestFile(t, root, "save/config.json", `{"Motif":"data/system.def"}`)
	putManifestFile(t, root, "data/system.def", "[Files]\nselect = select.def\n")
	putManifestFile(t, root, "data/select.def", "[Characters]\nchars/one.def, stages/one.def\n[Stages]\nstages/one.def\n")
	putManifestFile(t, root, "chars/one.def", "[Files]\ncns = one.cns\n")
	putManifestFile(t, root, "chars/one.cns", "[Statedef 0]\ntype = S\n")
	putManifestFile(t, root, "stages/one.def", "[Files]\nsff = one.sff\n")
	putManifestFile(t, root, "stages/one.sff", "\x00\xffnot ini")
	got := LoadWorkspaceWithProfile(filepath.Join(root, "data", "select.def"), profile.NewDistributionProfile(root))
	if len(got.Documents) < 5 {
		t.Fatalf("expected startup closure documents, got %d diagnostics=%d", len(got.Documents), len(got.Diagnostics))
	}
	for _, doc := range got.Documents {
		if filepath.Ext(doc.Path) == ".sff" {
			t.Fatalf("binary asset entered semantic workspace: %s", doc.Path)
		}
	}
}
