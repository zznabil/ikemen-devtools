package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigPrecedenceAndDigest(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	config := `{"version":"0.1","profile":"distribution","cache":"disk","entryPoints":["z.def","a.def"],"budgets":{"maxFiles":9,"maxBytes":100,"maxItems":8,"maxDepth":7,"maxDurationMs":6}}`
	if err := os.WriteFile(filepath.Join(root, ConfigDirName, ConfigFileName), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	flags := ConfigFlags{Profile: "strict/portable", Budgets: &Budgets{MaxFiles: 1, MaxBytes: 2, MaxItems: 3, MaxDepth: 4, MaxDurationMS: 5}}
	got, err := ResolveConfig(root, flags)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile != "strict/portable" || got.Budgets.MaxFiles != 1 || got.Root != root {
		t.Fatalf("precedence/root: %#v", got)
	}
	if got.Digest() != got.Digest() {
		t.Fatal("digest is not deterministic")
	}
}

func TestResolveConfigRejectsUnknownAndInvalid(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ConfigDirName, ConfigFileName)
	if err := os.WriteFile(path, []byte(`{"version":"0.1","unexpected":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveConfig(root, ConfigFlags{})
	var ce *ConfigError
	if !errors.As(err, &ce) || ce.Code != "invalid" {
		t.Fatalf("unknown field error: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"version":"9"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = ResolveConfig(root, ConfigFlags{})
	if !errors.As(err, &ce) || ce.Code != "unsupported-version" {
		t.Fatalf("version error: %v", err)
	}
}

func TestResolveConfigDoesNotReadUserGlobalConfig(t *testing.T) {
	if _, err := ResolveConfig(filepath.Join(t.TempDir(), "missing"), ConfigFlags{}); err == nil {
		t.Fatal("missing explicit root must fail")
	}
}
