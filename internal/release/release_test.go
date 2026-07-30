package release

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAndCanonicalJSONRepeatable(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "b.bin")
	second := filepath.Join(root, "a.bin")
	if err := os.WriteFile(first, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := Build(Module, "1.2.3", "0.2.0", "2025-01-02T03:04:05Z", []string{first, second})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(Module, "1.2.3", "0.2.0", "2025-01-02T03:04:05Z", []string{second, first})
	if err != nil {
		t.Fatal(err)
	}
	aj, err := CanonicalJSON(a)
	if err != nil {
		t.Fatal(err)
	}
	bj, err := CanonicalJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(aj) != string(bj) {
		t.Fatalf("canonical output changed with input order: %s != %s", aj, bj)
	}
	if !strings.Contains(string(aj), `"module":"`+Module+`"`) {
		t.Fatalf("module metadata missing: %s", aj)
	}
}

func TestBuildHashChangesWhenFileChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact.bin")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := Build(Module, "1", "c", "2025-01-02T03:04:05Z", []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := Build(Module, "1", "c", "2025-01-02T03:04:05Z", []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if before.Files[0].SHA256 == after.Files[0].SHA256 {
		t.Fatal("file hash did not change")
	}
}

func TestSignAndVerify(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := Build(Module, "1", "c", "2025-01-02T03:04:05Z", nil)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := Sign(metadata, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(signed, publicKey); err != nil {
		t.Fatal(err)
	}
	signed.Version = "2"
	if err := Verify(signed, publicKey); err == nil {
		t.Fatal("tampered metadata verified")
	}
}

func TestVerifyRejectsInvalidSignature(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := Build(Module, "1", "c", "2025-01-02T03:04:05Z", nil)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := Sign(metadata, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	signed.Signature.Value = "invalid"
	if err := Verify(signed, publicKey); err == nil {
		t.Fatal("invalid signature accepted")
	}
}

func TestBuildRejectsMissingExplicitTimestamp(t *testing.T) {
	_, err := Build(Module, "1", "c", "", nil)
	if err == nil || !strings.Contains(err.Error(), "timestamp") {
		t.Fatalf("expected timestamp rejection, got %v", err)
	}
}
