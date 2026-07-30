package patch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func editFor(t *testing.T, root, path string, start, end int, old, next string) Edit {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return Edit{Path: path, ContentHash: digest(data), IdentityContract: "0.2.0", Span: Span{ByteStart: start, ByteEnd: end}, OldText: old, NewText: next}
}

func TestPreviewIsDeterministicAndPreservesLineEndings(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.def"), []byte("one\r\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := editFor(t, root, "a.def", 5, 8, "two", "THREE")
	first, err := PreviewPatch(root, Patch{Edits: []Edit{e}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := PreviewPatch(root, Patch{Edits: []Edit{e}})
	if err != nil {
		t.Fatal(err)
	}
	if string(first.Files[0].Bytes) != "one\r\nTHREE\n" || string(second.Files[0].Bytes) != string(first.Files[0].Bytes) || first.Files[0].NewHash != second.Files[0].NewHash {
		t.Fatalf("non-deterministic preview: %#v %#v", first, second)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "a.def")); string(got) != "one\r\ntwo\n" {
		t.Fatal("preview modified source")
	}
}

func TestApplyPatchSuccessReturnsNewHash(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.def"), []byte("abc\r\ndef"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := editFor(t, root, "a.def", 5, 8, "def", "xyz")
	result, err := ApplyPatch(root, Patch{Edits: []Edit{e}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "a.def"))
	if string(got) != "abc\r\nxyz" || result.Files[0].NewHash != digest(got) || result.Files[0].NewHash == result.Files[0].OldHash {
		t.Fatalf("unexpected apply result: %#v / %q", result, got)
	}
}

func TestApplyPatchRejectsStaleHash(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.def"), []byte("abc"), 0o644)
	e := editFor(t, root, "a.def", 0, 1, "a", "x")
	os.WriteFile(filepath.Join(root, "a.def"), []byte("xbc"), 0o644)
	if _, err := ApplyPatch(root, Patch{Edits: []Edit{e}}); !errors.Is(err, ErrStaleHash) {
		t.Fatalf("expected stale hash, got %v", err)
	}
}

func TestPreviewRejectsOverlappingEdits(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.def"), []byte("abcdef"), 0o644)
	data, _ := os.ReadFile(filepath.Join(root, "a.def"))
	p := Patch{Edits: []Edit{{Path: "a.def", ContentHash: digest(data), IdentityContract: "0.2.0", Span: Span{ByteStart: 0, ByteEnd: 3}, OldText: "abc", NewText: "x"}, {Path: "a.def", ContentHash: digest(data), IdentityContract: "0.2.0", Span: Span{ByteStart: 2, ByteEnd: 4}, OldText: "cd", NewText: "y"}}}
	if _, err := PreviewPatch(root, p); !errors.Is(err, ErrOverlap) {
		t.Fatalf("expected overlap, got %v", err)
	}
}

func TestPreviewRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := PreviewPatch(root, Patch{Edits: []Edit{{Path: "../outside", Span: Span{ByteStart: 0, ByteEnd: 0}}}}); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
}

func TestApplyAtomicCommitsAllFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.def", "b.def"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	edits := []Edit{editFor(t, root, "a.def", 0, 3, "old", "new"), editFor(t, root, "b.def", 0, 3, "old", "new")}
	if _, err := ApplyAtomic(root, Patch{Edits: edits}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.def", "b.def"} {
		b, _ := os.ReadFile(filepath.Join(root, name))
		if string(b) != "new" {
			t.Fatalf("%s not replaced", name)
		}
	}
}

func TestRecoverJournalRestoresBackup(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "a.def")
	backup := target + ".ikm-backup"
	os.WriteFile(backup, []byte("old"), 0644)
	os.WriteFile(target, []byte("new"), 0644)
	j := Journal{Phase: "replacing", Root: root, Backups: []string{backup}}
	b, _ := json.Marshal(j)
	os.WriteFile(journalPath(root), b, 0600)
	if err := Recover(root); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "old" {
		t.Fatalf("recovery failed: %q", got)
	}
}
