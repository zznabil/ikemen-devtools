package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ikemen-engine/ikemen-devtools/internal/corpus"
	"github.com/ikemen-engine/ikemen-devtools/internal/lsp"
	"github.com/ikemen-engine/ikemen-devtools/internal/report"
)

func TestCheckCommandValidFixture(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "hero.def")
	cmdPath := filepath.Join(root, "hero.cmd")
	stPath := filepath.Join(root, "hero.st")

	writeTextFile(t, cmdPath, `[Command]
name = "jump"
`)
	writeTextFile(t, stPath, `[Statedef 100]
[State 100]
type = ChangeState
value = 100
trigger1 = command = "jump"
`)
	writeTextFile(
		t,
		defPath,
		`[Info]
name = "Hero"
[Files]
cmd = hero.cmd
st = hero.st
`,
	)

	stdout, stderr, status := runCLI(t, []string{"check", defPath})

	if status != 0 {
		t.Fatalf("expected success status for valid fixture, got %d", status)
	}
	if strings.TrimSpace(stdout) == "" || !strings.Contains(stdout, "No diagnostics.") {
		t.Fatalf("expected human success output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected clean stderr on success, got %q", stderr)
	}
}

func TestCheckCommandReportsMissingSource(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "fighter.def")
	cmdPath := filepath.Join(root, "fighter.cmd")

	writeTextFile(t, cmdPath, `[Command]
name = "run"
`)
	writeTextFile(
		t,
		defPath,
		`[Info]
name = "Fighter"
[Files]
cmd = fighter.cmd
st = missing.st
`,
	)

	stdout, stderr, status := runCLI(t, []string{"check", defPath})

	if status == 0 {
		t.Fatalf("expected non-zero status for missing source")
	}
	if !strings.Contains(stdout, "missing-source") {
		t.Fatalf("expected missing-source diagnostic, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr output for check diagnostics, got %q", stderr)
	}
}

func TestCheckCommandReportsUndefinedState(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "slice.def")
	cmdPath := filepath.Join(root, "slice.cmd")
	stPath := filepath.Join(root, "slice.st")

	writeTextFile(t, cmdPath, `[Command]
name = "jump"
`)
	writeTextFile(t, stPath, `[Statedef 100]
[State 100]
type = ChangeState
value = 999
trigger1 = command = "jump"
`)
	writeTextFile(
		t,
		defPath,
		`[Info]
[Files]
cmd = slice.cmd
st = slice.st
`,
	)

	stdout, stderr, status := runCLI(t, []string{"check", defPath})

	if status == 0 {
		t.Fatalf("expected non-zero status for undefined state")
	}
	if !strings.Contains(stdout, "undefined-state") {
		t.Fatalf("expected undefined-state diagnostic, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr output for check diagnostics, got %q", stderr)
	}
}

func TestCheckCommandJSONOutput(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "hero.def")
	cmdPath := filepath.Join(root, "hero.cmd")
	stPath := filepath.Join(root, "hero.st")

	writeTextFile(t, cmdPath, `[Command]
name = "jump"
`)
	writeTextFile(t, stPath, `[Statedef 100]
`)
	writeTextFile(
		t,
		defPath,
		`[Files]
cmd = hero.cmd
st = hero.st
`,
	)

	first, firstErr, firstStatus := runCLI(t, []string{"check", "--json", defPath})
	if firstStatus != 0 {
		t.Fatalf("expected success status, got %d", firstStatus)
	}
	if firstErr != "" {
		t.Fatalf("expected no stderr for successful JSON check, got %q", firstErr)
	}

	var payload report.Report
	if err := json.Unmarshal([]byte(strings.TrimSpace(first)), &payload); err != nil {
		t.Fatalf("expected valid JSON payload, got %v", err)
	}

	second, _, _ := runCLI(t, []string{"check", "--json", defPath})
	if first != second {
		t.Fatalf("expected deterministic JSON output, first=%q second=%q", first, second)
	}
}

func TestCheckCommandExitStatus(t *testing.T) {
	root := t.TempDir()
	validDefPath := filepath.Join(root, "ok.def")
	invalidDefPath := filepath.Join(root, "missing.def")
	writeTextFile(t, filepath.Join(root, "ok.cmd"), `[Command]
name = "jump"
`)
	writeTextFile(t, filepath.Join(root, "ok.st"), `[Statedef 100]
`)
	writeTextFile(
		t,
		validDefPath,
		`[Files]
cmd = ok.cmd
st = ok.st
`,
	)
	writeTextFile(t, invalidDefPath, `[Files]
cmd = missing.cmd
`)

	_, _, okStatus := runCLI(t, []string{"check", validDefPath})
	_, _, missingStatus := runCLI(t, []string{"check", invalidDefPath})

	if okStatus != 0 {
		t.Fatalf("expected status 0 for clean run, got %d", okStatus)
	}
	if missingStatus == 0 {
		t.Fatalf("expected non-zero status for missing-source error")
	}
}

func TestIndexCommandExportsSQL(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "hero.def")
	writeTextFile(t, filepath.Join(root, "hero.st"), `[Statedef 100]
`)
	writeTextFile(t, defPath, `[Files]
st = hero.st
`)

	stdout, stderr, status := runCLI(t, []string{"index", defPath})
	if status != 0 {
		t.Fatalf("expected successful index status, got %d; stderr=%q", status, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected clean stderr, got %q", stderr)
	}
	for _, want := range []string{"CREATE TABLE IF NOT EXISTS ikm_documents", "USING fts5", "COMMIT;"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected SQL output to contain %q, got %q", want, stdout)
		}
	}
}

func TestIndexCommandRejectsAliasingOutputPath(t *testing.T) {
	root := t.TempDir()
	stPath := filepath.Join(root, "hero.st")
	defPath := filepath.Join(root, "hero.def")
	writeTextFile(t, stPath, `[Statedef 100]
`)
	writeTextFile(t, defPath, `[Files]
st = hero.st
`)

	_, stderr, status := runCLI(t, []string{"index", "--output", stPath, defPath})
	if status == 0 {
		t.Fatalf("expected non-zero status for aliased output path")
	}
	if !strings.Contains(stderr, "invalid output path") {
		t.Fatalf("expected aliasing error, got %q", stderr)
	}
}

func TestIndexCommandWritesOutputToSafePath(t *testing.T) {
	root := t.TempDir()
	outputPath := filepath.Join(root, "out", "index.sql")
	defPath := filepath.Join(root, "hero.def")
	writeTextFile(t, filepath.Join(root, "hero.st"), `[Statedef 100]
`)
	writeTextFile(t, defPath, `[Files]
st = hero.st
`)

	stdout, stderr, status := runCLI(t, []string{"index", "--output", outputPath, defPath})
	if status != 0 {
		t.Fatalf("expected successful index export, got status=%d stderr=%q", status, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout output when writing file, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected clean stderr, got %q", stderr)
	}
	contents, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("expected output file %q to exist: %v", outputPath, err)
	}
	if !strings.Contains(string(contents), "CREATE TABLE IF NOT EXISTS ikm_documents") {
		t.Fatalf("expected SQL output to be written to file, got %q", string(contents))
	}
}

func TestCorpusCommandJSONOutput(t *testing.T) {
	root := t.TempDir()
	selectPath := filepath.Join(root, "select.def")
	defPath := filepath.Join(root, "chars", "hero.def")
	stPath := filepath.Join(root, "chars", "hero.st")

	if err := os.MkdirAll(filepath.Dir(defPath), 0o755); err != nil {
		t.Fatalf("mkdir chars: %v", err)
	}
	writeTextFile(t, stPath, `[Statedef 100]
`)
	writeTextFile(
		t,
		defPath,
		`[Info]
name = "Hero"
[Files]
st = hero.st
`,
	)
	writeTextFile(
		t,
		selectPath,
		`[Characters]
chars/hero.def, order=1
empty
heroes/ghost.def
chars/hero.def, order=2
`,
	)

	out, _, status := runCLI(t, []string{"corpus", "--json", selectPath})
	if status == 0 {
		t.Fatalf("expected status 1 because a missing source exists")
	}
	if len(out) == 0 {
		t.Fatalf("expected JSON output from corpus command")
	}
	var m corpus.Manifest
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &m); err != nil {
		t.Fatalf("expected valid JSON manifest, got %v; output=%q", err, out)
	}
	if m.DeclaredSourceCount != 3 {
		t.Fatalf("expected 3 declared sources, got %d", m.DeclaredSourceCount)
	}
	if m.ResolvedSourceCount != 1 {
		t.Fatalf("expected one resolved source, got %d", m.ResolvedSourceCount)
	}
	if m.ErrorCount != 1 {
		t.Fatalf("expected one error diagnostic, got %d", m.ErrorCount)
	}
}

func TestCorpusCommandWritesOutputWhenClean(t *testing.T) {
	root := t.TempDir()
	selectPath := filepath.Join(root, "select.def")
	defPath := filepath.Join(root, "chars", "hero.def")
	stPath := filepath.Join(root, "chars", "hero.st")
	outPath := filepath.Join(root, "out", "manifest.json")

	if err := os.MkdirAll(filepath.Dir(defPath), 0o755); err != nil {
		t.Fatalf("mkdir chars: %v", err)
	}
	writeTextFile(t, stPath, `[Statedef 100]
`)
	writeTextFile(
		t,
		defPath,
		`[Files]
st = hero.st
`,
	)
	writeTextFile(
		t,
		selectPath,
		`[Characters]
chars/hero.def
`,
	)

	stdout, stderr, status := runCLI(t, []string{"corpus", "--output", outPath, "--json", selectPath})
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout when output path set, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for successful corpus command, got %q", stderr)
	}
	contents, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("expected output file %q: %v", outPath, err)
	}
	var m corpus.Manifest
	if err := json.Unmarshal(contents, &m); err != nil {
		t.Fatalf("expected manifest JSON in output file, got %v", err)
	}
	if m.ResolvedSourceCount != 1 {
		t.Fatalf("expected one resolved source, got %d", m.ResolvedSourceCount)
	}
}

func TestCorpusCommandDeduplicationAffectsResolvedCount(t *testing.T) {
	root := t.TempDir()
	selectPath := filepath.Join(root, "select.def")
	defPath := filepath.Join(root, "chars", "hero.def")
	stPath := filepath.Join(root, "chars", "hero.st")
	if err := os.MkdirAll(filepath.Dir(defPath), 0o755); err != nil {
		t.Fatalf("mkdir chars: %v", err)
	}
	writeTextFile(t, stPath, `[Statedef 100]
`)
	writeTextFile(
		t,
		defPath,
		`[Files]
st = hero.st
`,
	)
	writeTextFile(
		t,
		selectPath,
		`[Characters]
chars/hero.def
chars/./hero.def
`,
	)

	stdout, stderr, status := runCLI(t, []string{"corpus", "--json", selectPath})
	if status != 0 {
		t.Fatalf("expected clean corpus run, got status %d", status)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for clean run, got %q", stderr)
	}

	var got corpus.Manifest
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &got); err != nil {
		t.Fatalf("expected valid JSON manifest: %v", err)
	}
	if got.ResolvedSourceCount != 1 {
		t.Fatalf("expected deduped resolved source count 1, got %d", got.ResolvedSourceCount)
	}
	if got.Entries[1].Status != "deduplicated" {
		t.Fatalf("expected second entry deduplicated, got %q", got.Entries[1].Status)
	}
}

func TestCorpusCommandHumanOutput(t *testing.T) {
	root := t.TempDir()
	selectPath := filepath.Join(root, "select.def")
	defDir := filepath.Join(root, "chars")
	if err := os.MkdirAll(defDir, 0o755); err != nil {
		t.Fatalf("mkdir chars: %v", err)
	}
	writeTextFile(t, filepath.Join(defDir, "alpha.def"), `[Info]
name = "Alpha"
`)
	writeTextFile(t, filepath.Join(defDir, "beta.def"), `[Info]
name = "Beta"
`)
	writeTextFile(
		t,
		selectPath,
		`[Characters]
chars/alpha.def, option=fast
chars/beta.def, option=keep
`,
	)

	out, _, status := runCLI(t, []string{"corpus", selectPath})
	if status != 0 {
		t.Fatalf("expected success for valid corpus manifest, got status %d", status)
	}
	if !strings.Contains(out, "Profile: distribution") {
		t.Fatalf("expected human summary profile, got %q", out)
	}
	if !strings.Contains(out, "Entries:") {
		t.Fatalf("expected human entries section, got %q", out)
	}
	first := strings.Index(out, "- chars/alpha.def (resolved):")
	second := strings.Index(out, "- chars/beta.def (resolved):")
	if first == -1 || second == -1 {
		t.Fatalf("expected both entries in human output, got %q", out)
	}
	if first > second {
		t.Fatalf("expected entries to preserve declaration order")
	}
}

func TestCorpusCommandRejectsOutputAliasingSelectPath(t *testing.T) {
	root := t.TempDir()
	selectPath := filepath.Join(root, "select.def")
	defPath := filepath.Join(root, "chars", "hero.def")
	if err := os.MkdirAll(filepath.Dir(defPath), 0o755); err != nil {
		t.Fatalf("mkdir chars: %v", err)
	}
	writeTextFile(
		t,
		defPath,
		`[Info]
name = "Hero"
`,
	)
	writeTextFile(t, selectPath, `[Characters]
chars/hero.def
`)

	_, stderr, status := runCLI(t, []string{"corpus", "--output", selectPath, "--json", selectPath})
	if status == 0 {
		t.Fatalf("expected output path alias check to fail")
	}
	if !strings.Contains(stderr, "invalid output path") {
		t.Fatalf("expected invalid output path error, got %q", stderr)
	}
}
func TestCorpusCommandDoesNotWriteOutputOnError(t *testing.T) {
	root := t.TempDir()
	selectPath := filepath.Join(root, "select.def")
	outPath := filepath.Join(root, "out", "manifest.json")
	writeTextFile(
		t,
		selectPath,
		`[Characters]
does-not-exist.def
`,
	)

	out, _, status := runCLI(t, []string{"corpus", "--output", outPath, "--json", selectPath})
	if status == 0 {
		t.Fatalf("expected non-zero status when manifest has errors")
	}
	if out != "" {
		t.Fatalf("expected no stdout output when --output is used, got %q", out)
	}
	if _, err := os.Stat(outPath); err == nil {
		t.Fatalf("expected output file to not be created on error")
	}
}

func TestMetadataCommandOutputsDeterministicJSON(t *testing.T) {
	args := []string{"metadata", "--timestamp", "2025-01-02T03:04:05Z"}
	firstOut, firstErr, firstStatus := runCLI(t, args)
	if firstStatus != 0 {
		t.Fatalf("expected metadata command success, got %d (%s)", firstStatus, firstErr)
	}
	secondOut, _, secondStatus := runCLI(t, args)
	if secondStatus != 0 {
		t.Fatalf("expected second metadata command success, got %d", secondStatus)
	}
	if firstOut != secondOut {
		t.Fatalf("expected deterministic metadata output, first=%q second=%q", firstOut, secondOut)
	}
	var payload releaseMetadata
	if err := json.Unmarshal([]byte(strings.TrimSpace(firstOut)), &payload); err != nil {
		t.Fatalf("expected valid metadata JSON: %v", err)
	}
	if payload.Module == "" || payload.Contract == "" || payload.BuildTimestamp != "2025-01-02T03:04:05Z" {
		t.Fatalf("missing release metadata fields: %#v", payload)
	}
}

func TestMetadataCommandRejectsMissingTimestamp(t *testing.T) {
	_, stderr, status := runCLI(t, []string{"metadata"})
	if status == 0 || !strings.Contains(stderr, "timestamp") {
		t.Fatalf("expected missing timestamp rejection, status=%d stderr=%q", status, stderr)
	}
}

func TestMetadataCommandRejectsArguments(t *testing.T) {
	_, stderr, status := runCLI(t, []string{"metadata", "extra.def"})
	if status != 1 {
		t.Fatalf("expected non-zero status for metadata arguments, got %d", status)
	}
	if !strings.Contains(stderr, "usage: ikm metadata") {
		t.Fatalf("expected usage output on invalid metadata invocation, got %q", stderr)
	}
}

func TestHelpListsProtocolServers(t *testing.T) {
	stdout, stderr, status := runCLI(t, []string{"help"})
	if status != 0 || stderr != "" {
		t.Fatalf("help status=%d stderr=%q", status, stderr)
	}
	for _, command := range []string{"ikm lsp", "ikm mcp"} {
		if !strings.Contains(stdout, command) {
			t.Fatalf("help missing %q: %q", command, stdout)
		}
	}
}

func TestMCPCommandPreloadsExplicitFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "hero.cmd")
	writeTextFile(t, path, "[Command]\nname = \"jump\"\n")
	requests := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"document_symbols","arguments":{"uri":"` + filepath.ToSlash(path) + `"}}}`,
		"",
	}, "\n")
	var stdout, stderr bytes.Buffer
	status := runWithIO([]string{"mcp", "--file", path}, strings.NewReader(requests), &stdout, &stderr)
	if status != 0 || stderr.Len() != 0 {
		t.Fatalf("mcp status=%d stderr=%q", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "command:jump") || strings.Contains(stdout.String(), "Content-Length:") {
		t.Fatalf("unexpected MCP output %q", stdout.String())
	}
}

func TestLSPCommandUsesContentLengthFraming(t *testing.T) {
	var input bytes.Buffer
	if err := lsp.WriteFrame(&input, []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	status := runWithIO([]string{"lsp"}, &input, &stdout, &stderr)
	if status != 0 || stderr.Len() != 0 {
		t.Fatalf("lsp status=%d stderr=%q", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Content-Length:") {
		t.Fatalf("missing LSP framing: %q", stdout.String())
	}
}

func runCLI(t *testing.T, args []string) (string, string, int) {
	t.Helper()
	var out bytes.Buffer
	var errOut bytes.Buffer

	status := run(args, &out, &errOut)
	return out.String(), errOut.String(), status
}

func writeTextFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
func TestReadOnlyCLIGoldenParity(t *testing.T) {
	root := t.TempDir()
	def := filepath.Join(root, "hero.def")
	writeTextFile(t, filepath.Join(root, "hero.st"), "[Statedef 100]\n")
	writeTextFile(t, def, "[Files]\nst = hero.st\n")
	for _, args := range [][]string{{"query", "symbols", "--root", def, "--json"}, {"query", "diagnostics", "--root", def, "--json"}, {"graph", "dependencies", "--root", def, "--json"}, {"inspect", "file", "--root", def, "--json"}, {"export", "jsonl", "--root", def, "--json"}, {"export", "scip", "--root", def, "--json"}, {"export", "sql", "--root", def, "--json"}} {
		first, stderr, status := runCLI(t, args)
		if status != 0 || stderr != "" {
			t.Fatalf("%v status=%d stderr=%q", args, status, stderr)
		}
		second, _, _ := runCLI(t, args)
		if first != second {
			t.Fatalf("non-deterministic output for %v", args)
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(first), &payload); err != nil {
			t.Fatalf("invalid JSON for %v: %v", args, err)
		}
		for _, k := range []string{"schemaVersion", "operation", "status", "workspace", "snapshot", "result", "page", "truncated"} {
			if _, ok := payload[k]; !ok {
				t.Fatalf("missing %s for %v", k, args)
			}
		}
	}
}
func TestAuthorizedRuntimeCommandRequiresDisposableRootAndAllowlist(t *testing.T) {
	root := t.TempDir()
	command := filepath.Join(root, "engine.exe")
	if _, err := authorizedRuntimeCommand(command, root, nil); err == nil {
		t.Fatal("expected empty allowlist refusal")
	}
	if _, err := authorizedRuntimeCommand(command, "", []string{command}); err == nil {
		t.Fatal("expected missing disposable root refusal")
	}
	got, err := authorizedRuntimeCommand(command, root, []string{command})
	if err != nil || got != command {
		t.Fatalf("expected authorized command, got %q, %v", got, err)
	}
	if _, err := authorizedRuntimeCommand(filepath.Join(root, "other.exe"), root, []string{command}); err == nil {
		t.Fatal("expected command outside allowlist refusal")
	}
}
