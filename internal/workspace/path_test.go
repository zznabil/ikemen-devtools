package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPathAuthorityDeniesTraversalAndAllowsExternalSubtree(t *testing.T) {
	root := t.TempDir()
	ext := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	a, err := NewPathAuthority(root, []string{ext})
	if err != nil {
		t.Fatal(err)
	}
	inside, err := a.Resolve(filepath.Join("src", "..", "src", "file.def"))
	if err != nil || inside.External || inside.Relative != "src/file.def" {
		t.Fatalf("inside=%#v err=%v", inside, err)
	}
	_, err = a.Resolve(filepath.Join("..", "outside.def"))
	var pe *PathError
	if !errors.As(err, &pe) || pe.Code != "outside-root" {
		t.Fatalf("traversal error=%v", err)
	}
	allowed, err := a.Resolve(filepath.Join(ext, "nested", "x.st"))
	if err != nil || !allowed.External || allowed.Relative != "nested/x.st" {
		t.Fatalf("external=%#v err=%v", allowed, err)
	}
}

func TestPathAuthorityDeniesSymlinkEscapeAndNormalizesFileURI(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlinks unavailable")
	}
	a, err := NewPathAuthority(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Resolve(filepath.Join("link", "secret.txt"))
	var pe *PathError
	if !errors.As(err, &pe) || pe.Code != "outside-root" {
		t.Fatalf("symlink error=%v", err)
	}
	file := filepath.Join(root, "folder", "x.def")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := a.Resolve("file://" + filepath.ToSlash(file))
	if err != nil || got.Relative != "folder/x.def" {
		t.Fatalf("file URI=%#v err=%v", got, err)
	}
	uri, err := a.WorkspaceURI(file)
	if err != nil || uri != "workspace:/folder/x.def" {
		t.Fatalf("workspace URI=%q err=%v", uri, err)
	}
}
