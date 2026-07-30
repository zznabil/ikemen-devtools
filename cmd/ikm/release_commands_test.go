package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestVersionJSONContract(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runVersion([]string{"--json"}, &out, &errOut); code != 0 {
		t.Fatalf("version exit=%d stderr=%s", code, errOut.String())
	}
	var got versionResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.BinaryVersion == "" || got.SchemaVersion == "" || got.OS == "" || got.Arch == "" {
		t.Fatalf("incomplete version: %+v", got)
	}
}

func TestConfigEffectiveRedactsPathsByContract(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".ikm"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := runConfig([]string{"--effective", "--json", "--root", root}, &out, &errOut); code != 0 {
		t.Fatalf("config exit=%d stderr=%s", code, errOut.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["digest"] == nil || payload["provenance"] == nil {
		t.Fatalf("missing effective config contract: %s", out.String())
	}
}

func TestDoctorCorruptCacheIsActionableAndReadOnly(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "cache")
	if err := os.Mkdir(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "index.sqlite"), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := runDoctor([]string{"--json", "--root", root, "--cache", cache}, &out, &errOut)
	if code == 0 {
		t.Fatal("corrupt cache must report findings")
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["diagnostics"]; !ok {
		t.Fatalf("missing diagnostics: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(cache, "index.sqlite")); err != nil {
		t.Fatalf("read-only doctor mutated cache: %v", err)
	}
}
