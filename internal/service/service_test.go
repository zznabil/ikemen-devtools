package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
)

func TestCoordinatorCacheHitByDependencyHashes(t *testing.T) {
	rootDir := t.TempDir()
	defPath := filepath.Join(rootDir, "hero.def")
	cmdPath := filepath.Join(rootDir, "hero.cmd")

	writeTextFile(t, defPath, `[files]
cmd = hero.cmd`)
	writeTextFile(t, cmdPath, `[Command]\nname = "jump"\n`)

	reader := &countingFileReader{}
	coordinator := NewCoordinator(
		WithReadFile(reader.read),
		WithProfile(profile.NewStrictPortableProfile("")),
	)

	ctx := context.Background()
	if _, err := coordinator.Analyze(ctx, defPath); err != nil {
		t.Fatalf("first analysis failed: %v", err)
	}
	firstReads := reader.totalReads()
	if _, err := coordinator.Analyze(ctx, defPath); err != nil {
		t.Fatalf("cache-hit analysis failed: %v", err)
	}
	secondReads := reader.totalReads()
	if firstReads != secondReads {
		t.Fatalf("expected cache hit without additional reads, got first %d second %d", firstReads, secondReads)
	}
}

func TestCoordinatorInvalidatesChangedDependencies(t *testing.T) {
	rootDir := t.TempDir()
	defPath := filepath.Join(rootDir, "hero.def")
	cmdPath := filepath.Join(rootDir, "hero.cmd")

	writeTextFile(t, defPath, `[files]\ncmd = hero.cmd`)
	writeTextFile(t, cmdPath, `[Command]\nname = "jump"\n`)

	reader := &countingFileReader{}
	coordinator := NewCoordinator(WithReadFile(reader.read), WithProfile(profile.NewStrictPortableProfile("")))
	ctx := context.Background()

	first, err := coordinator.Analyze(ctx, defPath)
	if err != nil {
		t.Fatalf("first analysis failed: %v", err)
	}
	firstReads := reader.totalReads()

	if names := commandSymbolsFromResult(first); !contains(names, "command:jump") {
		t.Fatalf("expected initial command symbol in workspace result, got %#v", names)
	}

	writeTextFile(t, cmdPath, `[Command]\nname = "run"\n`)

	second, err := coordinator.Analyze(ctx, defPath)
	if err != nil {
		t.Fatalf("rerun after change failed: %v", err)
	}
	secondReads := reader.totalReads()
	if secondReads <= firstReads {
		t.Fatalf("expected dependency change to trigger reread, got first %d second %d", firstReads, secondReads)
	}

	if names := commandSymbolsFromResult(second); !contains(names, "command:run") {
		t.Fatalf("expected changed command symbol in analysis result, got %#v", names)
	}
}

func TestCoordinatorCancelsSupersededAnalysis(t *testing.T) {
	rootDir := t.TempDir()
	defPath := filepath.Join(rootDir, "hero.def")
	cmdPath := filepath.Join(rootDir, "hero.cmd")

	writeTextFile(t, defPath, `[files]\ncmd = hero.cmd`)
	writeTextFile(t, cmdPath, `[Command]\nname = "jump"\n`)

	reader := newBlockingReadFile()
	coordinator := NewCoordinator(WithReadFile(reader.read), WithProfile(profile.NewStrictPortableProfile("")))

	firstErr := make(chan error, 1)
	go func() {
		_, err := coordinator.Analyze(context.Background(), defPath)
		firstErr <- err
	}()

	reader.waitForFirstRead()

	if _, err := coordinator.Analyze(context.Background(), defPath); err != nil {
		t.Fatalf("superseding analysis failed: %v", err)
	}
	reader.releaseAll()

	if err := <-firstErr; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected first analysis to be canceled, got %v", err)
	}
}

func TestCoordinatorDiagnosticOrderIsDeterministic(t *testing.T) {
	rootDir := t.TempDir()
	defPath := filepath.Join(rootDir, "hero.def")
	cmdA := filepath.Join(rootDir, "a.cmd")
	cmdB := filepath.Join(rootDir, "b.cmd")

	writeTextFile(t, defPath, `[files]\ncmd = a.cmd\ncmd = b.cmd`)
	writeTextFile(t, cmdA, `[State 1]\ntype = HitDef\ntrigger1 = command = "alpha"\n`)
	writeTextFile(t, cmdB, `[State 2]\ntype = HitDef\ntrigger1 = command = "beta"\n`)

	coordinator := NewCoordinator(
		WithProfile(profile.NewStrictPortableProfile("")),
	)

	result, err := coordinator.Analyze(context.Background(), defPath)
	if err != nil {
		t.Fatalf("analysis failed: %v", err)
	}
	if len(result.Diagnostics) != 2 {
		t.Fatalf("expected two undefined-command diagnostics, got %d", len(result.Diagnostics))
	}

	sorted := append([]ir.Diagnostic(nil), result.Diagnostics...)
	sortDiagnostics(sorted)
	if !reflect.DeepEqual(result.Diagnostics, sorted) {
		t.Fatalf("expected deterministic diagnostic order, got %#v", result.Diagnostics)
	}
}

type countingFileReader struct {
	reads int
	mu    sync.Mutex
}

func (r *countingFileReader) read(path string) ([]byte, error) {
	r.mu.Lock()
	r.reads++
	r.mu.Unlock()
	return os.ReadFile(path)

}

func (r *countingFileReader) totalReads() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reads
}

type blockingReadFile struct {
	readStarted chan struct{}
	release     chan struct{}
	mu          sync.Mutex
	blocked     bool
}

func newBlockingReadFile() *blockingReadFile {
	return &blockingReadFile{
		readStarted: make(chan struct{}),
		release:     make(chan struct{}),
		blocked:     true,
	}
}

func (r *blockingReadFile) read(path string) ([]byte, error) {
	r.mu.Lock()
	shouldBlock := r.blocked
	if r.blocked {
		r.blocked = false
		close(r.readStarted)
	}
	r.mu.Unlock()

	if shouldBlock {
		<-r.release
	}
	return os.ReadFile(path)
}

func (r *blockingReadFile) waitForFirstRead() {
	<-r.readStarted
}

func (r *blockingReadFile) releaseAll() {
	close(r.release)
}

func writeTextFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(text, "\\n", "\n")), 0o644); err != nil {
		t.Fatalf("write file %q: %v", path, err)
	}
}

func commandSymbolsFromResult(result ServiceResult) []string {
	seen := []string{}
	for _, doc := range result.Workspace.Documents {
		for _, symbol := range doc.Symbols {
			if symbol.Kind == ir.SymbolCommand {
				seen = append(seen, symbol.Name)
			}
		}
	}
	return seen
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}
