package assets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifySignaturesAndMetadata(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		data []byte
		kind Kind
		mime string
	}{
		{"hero.sff", append([]byte("ElecbyteSpr"), 1, 2, 3, 4), KindSFF, "application/x-sff"},
		{"voice.snd", append([]byte("ElecbyteSnd"), 1, 2, 3, 4), KindSND, "audio/x-ikemen-snd"},
		{"shader.glsl", []byte("void main() {}"), KindShader, "text/x-shader"},
		{"notes.def", []byte("[Info]\nname = test\n"), KindAuthoredText, "text/plain; charset=utf-8"},
	}
	for _, tc := range cases {
		path := filepath.Join(dir, tc.name)
		if err := os.WriteFile(path, tc.data, 0600); err != nil {
			t.Fatal(err)
		}
		got := ClassifyFile(path, Limits{MaxTextBytes: 64, MaxHeaderBytes: 32})
		if got.Kind != tc.kind || got.MIME != tc.mime {
			t.Errorf("%s: got %s/%s", tc.name, got.Kind, got.MIME)
		}
	}
}

func TestLargeAuthoredFileIsBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.cmd")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(3 << 30); err != nil {
		t.Fatal(err)
	}
	f.Close()
	got := ClassifyFile(path, Limits{MaxTextBytes: 1024, MaxHeaderBytes: 128})
	if got.Kind != KindAuthoredText || !got.Truncated || got.HeaderBytes > 128 {
		t.Fatalf("unbounded result: %#v", got)
	}
}

func TestCorruptAndUnknownProduceDiagnostics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.sff")
	if err := os.WriteFile(path, []byte("ElecbyteSpr"), 0600); err != nil {
		t.Fatal(err)
	}
	got := ClassifyFile(path, DefaultLimits())
	if got.Kind != KindSFF || len(got.Diagnostics) == 0 {
		t.Fatalf("expected corrupt diagnostic: %#v", got)
	}
	unknown := filepath.Join(t.TempDir(), "asset.bin")
	if err := os.WriteFile(unknown, []byte("not-known"), 0600); err != nil {
		t.Fatal(err)
	}
	if len(ClassifyFile(unknown, DefaultLimits()).Diagnostics) == 0 {
		t.Fatal("unknown format lacked diagnostic")
	}
}
