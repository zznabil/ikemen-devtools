package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
	"github.com/ikemen-engine/ikemen-devtools/internal/workspace"
)

// These budgets are intentionally generous enough for slow CI while still
// catching accidental order-of-magnitude allocation regressions.
const (
	benchmarkMaxRosterAllocsPerOp    = 1_000_000
	benchmarkMaxCharacterAllocsPerOp = 100_000
	benchmarkFixtureEntries          = 48
)

type corpusBenchmarkReport struct {
	entries      int
	documents    int
	diagnostics  int
	elapsed      time.Duration
	allocBytes   uint64
	peakHeapByte uint64
}

// BenchmarkCorpusSelectRoster measures the complete select roster path,
// including source resolution and character workspace loading.
func BenchmarkCorpusSelectRoster(b *testing.B) {
	fixture := makeBenchmarkFixture(b)
	profile := profile.NewDistributionProfile("")
	if allocs := testing.AllocsPerRun(3, func() {
		_ = BuildManifest(fixture.selectPath, profile)
	}); allocs > benchmarkMaxRosterAllocsPerOp {
		b.Fatalf("select roster allocation budget exceeded: %.0f > %d allocs/op", allocs, benchmarkMaxRosterAllocsPerOp)
	}

	b.ReportAllocs()
	b.ResetTimer()
	var total corpusBenchmarkReport
	for i := 0; i < b.N; i++ {
		r := measureCorpusOperation(func() corpusBenchmarkReport {
			manifest := BuildManifest(fixture.selectPath, profile)
			return corpusBenchmarkReport{
				entries:     len(manifest.Entries),
				documents:   manifest.ResolvedSourceCount,
				diagnostics: manifest.ErrorCount + manifest.WarningCount,
			}
		})
		total.entries += r.entries
		total.documents += r.documents
		total.diagnostics += r.diagnostics
		total.elapsed += r.elapsed
		total.allocBytes += r.allocBytes
		if r.peakHeapByte > total.peakHeapByte {
			total.peakHeapByte = r.peakHeapByte
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(total.entries)/float64(b.N), "entries/op")
	b.ReportMetric(float64(total.documents)/float64(b.N), "documents/op")
	b.ReportMetric(float64(total.diagnostics)/float64(b.N), "diagnostics/op")
	b.ReportMetric(float64(total.elapsed)/float64(b.N), "elapsed-ns/op")
	b.ReportMetric(float64(total.allocBytes)/float64(b.N), "alloc-bytes/op")
	b.ReportMetric(float64(total.peakHeapByte), "peak-heap-bytes")
}

// BenchmarkCorpusCharacterDefinition measures parsing a representative
// character definition and its referenced command/state documents.
func BenchmarkCorpusCharacterDefinition(b *testing.B) {
	fixture := makeBenchmarkFixture(b)
	if allocs := testing.AllocsPerRun(3, func() {
		_ = workspace.LoadWorkspaceWithProfile(fixture.characterPath, profile.NewDistributionProfile(""))
	}); allocs > benchmarkMaxCharacterAllocsPerOp {
		b.Fatalf("character definition allocation budget exceeded: %.0f > %d allocs/op", allocs, benchmarkMaxCharacterAllocsPerOp)
	}

	p := profile.NewDistributionProfile("")
	b.ReportAllocs()
	b.ResetTimer()
	var total corpusBenchmarkReport
	for i := 0; i < b.N; i++ {
		r := measureCorpusOperation(func() corpusBenchmarkReport {
			loaded := workspace.LoadWorkspaceWithProfile(fixture.characterPath, p)
			return corpusBenchmarkReport{
				entries:     1,
				documents:   len(loaded.Documents),
				diagnostics: len(loaded.Diagnostics),
			}
		})
		total.entries += r.entries
		total.documents += r.documents
		total.diagnostics += r.diagnostics
		total.elapsed += r.elapsed
		total.allocBytes += r.allocBytes
		if r.peakHeapByte > total.peakHeapByte {
			total.peakHeapByte = r.peakHeapByte
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(total.entries)/float64(b.N), "entries/op")
	b.ReportMetric(float64(total.documents)/float64(b.N), "documents/op")
	b.ReportMetric(float64(total.diagnostics)/float64(b.N), "diagnostics/op")
	b.ReportMetric(float64(total.elapsed)/float64(b.N), "elapsed-ns/op")
	b.ReportMetric(float64(total.allocBytes)/float64(b.N), "alloc-bytes/op")
	b.ReportMetric(float64(total.peakHeapByte), "peak-heap-bytes")
}

type benchmarkFixture struct {
	selectPath    string
	characterPath string
}

func makeBenchmarkFixture(tb testing.TB) benchmarkFixture {
	if fixture, ok := findBenchmarkCorpus(); ok {
		return fixture
	}

	root := tb.TempDir()
	charsDir := filepath.Join(root, "chars")
	if err := os.MkdirAll(charsDir, 0o755); err != nil {
		tb.Fatalf("create benchmark fixture: %v", err)
	}

	var roster strings.Builder
	roster.WriteString("[Characters]\n")
	for i := 0; i < benchmarkFixtureEntries; i++ {
		name := fmt.Sprintf("hero%02d", i)
		roster.WriteString("chars/")
		roster.WriteString(name)
		roster.WriteString("/")
		roster.WriteString(name)
		roster.WriteString(".def, order=")
		roster.WriteString(fmt.Sprint((i % 10) + 1))
		roster.WriteByte('\n')
		charDir := filepath.Join(charsDir, name)
		if err := os.MkdirAll(charDir, 0o755); err != nil {
			tb.Fatalf("create character directory: %v", err)
		}
		writeBenchmarkFile(tb, filepath.Join(charDir, name+".def"), fmt.Sprintf("[Info]\nname = \"Hero %02d\"\n[Files]\ncmd = %s.cmd\ncns = %s.cns\n", i, name, name))
		writeBenchmarkFile(tb, filepath.Join(charDir, name+".cmd"), "[Command]\nname = \"basic\"\ncommand = x\n")
		writeBenchmarkFile(tb, filepath.Join(charDir, name+".cns"), "[Statedef 0]\ntype = S\n[State 0, Null]\ntype = Null\n")
	}
	selectPath := filepath.Join(root, "select.def")
	writeBenchmarkFile(tb, selectPath, roster.String())
	return benchmarkFixture{
		selectPath:    selectPath,
		characterPath: filepath.Join(charsDir, "hero00", "hero00.def"),
	}
}

func findBenchmarkCorpus() (benchmarkFixture, bool) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		return benchmarkFixture{}, false
	}
	selectPath := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "..", "Ikemen-GO", "data", "select.def"))
	data, err := os.ReadFile(selectPath)
	if err != nil {
		return benchmarkFixture{}, false
	}
	entries, _ := parseSelectEntries(string(data), selectPath)
	p := profile.NewDistributionProfile("")
	for _, entry := range entries {
		characterPath := resolveCharacterPath(selectPath, entry.declaredPath, p)
		if characterPath != "" {
			if info, statErr := os.Stat(characterPath); statErr == nil && !info.IsDir() {
				return benchmarkFixture{selectPath: selectPath, characterPath: characterPath}, true
			}
		}
	}
	return benchmarkFixture{}, false
}

func writeBenchmarkFile(tb testing.TB, path, source string) {
	tb.Helper()
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		tb.Fatalf("write benchmark fixture %q: %v", path, err)
	}
}

func measureCorpusOperation(operation func() corpusBenchmarkReport) corpusBenchmarkReport {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	report := operation()
	report.elapsed = time.Since(start)
	runtime.ReadMemStats(&after)
	report.allocBytes = after.TotalAlloc - before.TotalAlloc
	report.peakHeapByte = report.allocBytes
	if after.HeapAlloc > before.HeapAlloc {
		report.peakHeapByte += after.HeapAlloc - before.HeapAlloc
	}
	return report
}
