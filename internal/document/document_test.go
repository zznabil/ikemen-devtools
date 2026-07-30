package document

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
)

func TestNewSnapshotRejectsInvalidInputs(t *testing.T) {
	t.Run("nil source", func(t *testing.T) {
		snapshot, err := NewSnapshot("foo.def", nil)
		if snapshot != nil {
			t.Fatalf("expected nil snapshot")
		}
		if !errors.Is(err, ErrNilSource) {
			t.Fatalf("expected ErrNilSource, got %v", err)
		}
	})

	t.Run("blank path", func(t *testing.T) {
		snapshot, err := NewSnapshot("   ", []byte("name = Hero"))
		if snapshot != nil {
			t.Fatalf("expected nil snapshot")
		}
		if !errors.Is(err, ErrBlankPath) {
			t.Fatalf("expected ErrBlankPath, got %v", err)
		}
	})
}

func TestNewSnapshotIsDeterministic(t *testing.T) {
	source := []byte("[Info]\r\nname = Hero\r\n[State 10]\r\n")
	snapshotA, err := NewSnapshot("./a/../a.def", source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	snapshotB, err := NewSnapshot("a.def", append([]byte(nil), source...))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if snapshotA.NormalizedPath() != "a.def" {
		t.Fatalf("unexpected normalized path: %q", snapshotA.NormalizedPath())
	}
	if snapshotA.FileType() != "def" {
		t.Fatalf("unexpected file type: %q", snapshotA.FileType())
	}
	if !snapshotA.Equal(snapshotB) {
		t.Fatalf("expected equal snapshots for equivalent normalized inputs")
	}
	if snapshotA.Hash() != snapshotB.Hash() {
		t.Fatalf("expected matching hashes")
	}
}

func TestSnapshotPreservesSourceAndLineEndings(t *testing.T) {
	source := []byte("line1\r\nline2\nline3\r\n")
	raw := append([]byte(nil), source...)
	snapshot, err := NewSnapshot("./x.cmd", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw[0] = 'X'
	if !bytes.Equal(snapshot.Bytes(), source) {
		t.Fatalf("snapshot bytes should preserve original value, got %#v", snapshot.Bytes())
	}
	if !reflect.DeepEqual(snapshot.LineEndings(), []string{"\r\n", "\n", "\r\n", ""}) {
		t.Fatalf("unexpected line endings: %#v", snapshot.LineEndings())
	}
}

func TestSnapshotStoresParserContractAndDocument(t *testing.T) {
	source := []byte("name = Hero\n")
	snapshot, err := NewSnapshot("contract.def", source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	doc := snapshot.ParsedDocument()
	if doc == nil {
		t.Fatalf("expected parsed semantic document")
	}
	if snapshot.Version() != ir.IdentityContractVersion {
		t.Fatalf("unexpected version: %q", snapshot.Version())
	}
	if snapshot.IdentityContract() != ir.IdentityContractVersion {
		t.Fatalf("unexpected identity contract: %q", snapshot.IdentityContract())
	}
	if doc.Version != ir.IdentityContractVersion {
		t.Fatalf("unexpected document version: %q", doc.Version)
	}
	if doc.Path != "contract.def" {
		t.Fatalf("unexpected document path: %q", doc.Path)
	}
}

func TestSnapshotHashIsSha256OfBytes(t *testing.T) {
	source := []byte("name = Hero\nline2 = value\n")
	snapshot, err := NewSnapshot("hash.def", source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := sha256.Sum256(source)
	if snapshot.Hash() != hex.EncodeToString(want[:]) {
		t.Fatalf("unexpected hash, got %q want %q", snapshot.Hash(), hex.EncodeToString(want[:]))
	}
}
