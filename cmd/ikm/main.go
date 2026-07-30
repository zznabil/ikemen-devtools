package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/ikemen-engine/ikemen-devtools/internal/corpus"
	"github.com/ikemen-engine/ikemen-devtools/internal/index"
	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/lsp"
	"github.com/ikemen-engine/ikemen-devtools/internal/mcp"
	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
	"github.com/ikemen-engine/ikemen-devtools/internal/release"
	"github.com/ikemen-engine/ikemen-devtools/internal/report"
	"github.com/ikemen-engine/ikemen-devtools/internal/semantics"
	"github.com/ikemen-engine/ikemen-devtools/internal/workspace"
)

var releaseVersion = "0.0.0-dev"

func main() {
	os.Exit(runWithIO(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWithIO(args, os.Stdin, stdout, stderr)
}

func runWithIO(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}

	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	switch subcommand {
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "index":
		return runIndex(args[1:], stdout, stderr)
	case "corpus":
		return runCorpus(args[1:], stdout, stderr)
	case "metadata":
		return runMetadata(args[1:], stdout, stderr)
	case "lsp":
		return runLSP(args[1:], stdin, stdout, stderr)
	case "mcp":
		return runMCP(args[1:], stdin, stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		printUsage(stderr)
		return 1
	}
}

func runLSP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		printUsage(stderr)
		return 1
	}
	if err := lsp.NewServer().Serve(context.Background(), stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "ikm lsp: %v\n", err)
		return 1
	}
	return 0
}

func runMCP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printUsage(stderr) }
	files := multiStringFlag{}
	fs.Var(&files, "file", "document to preload (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if len(fs.Args()) != 0 {
		printUsage(stderr)
		return 1
	}

	server := mcp.NewServerWithVersion(releaseVersion)
	ctx := context.Background()
	for _, path := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "ikm mcp: read %s: %v\n", path, err)
			return 1
		}
		if err := server.SetDocument(ctx, filepath.Clean(path), source); err != nil {
			fmt.Fprintf(stderr, "ikm mcp: preload %s: %v\n", path, err)
			return 1
		}
	}
	if err := server.Serve(ctx, stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "ikm mcp: %v\n", err)
		return 1
	}
	return 0
}

func loadWorkspace(defPath string) workspace.LoadResult {
	return workspace.LoadWorkspaceWithProfile(defPath, profile.NewDistributionProfile(""))
}

func runMetadata(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("metadata", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printUsage(stderr) }
	timestamp := fs.String("timestamp", "", "explicit RFC3339 build timestamp (required)")
	module := fs.String("module", release.Module, "module path")
	contract := fs.String("contract", ir.IdentityContractVersion, "metadata contract version")
	files := multiStringFlag{}
	fs.Var(&files, "file", "file to hash (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if len(fs.Args()) != 0 {
		printUsage(stderr)
		return 1
	}
	payload, err := release.Build(*module, releaseVersion, *contract, *timestamp, files)
	if err != nil {
		fmt.Fprintln(stderr, "failed to build release metadata:", err)
		return 1
	}
	payload.Tool = "ikm"
	payload.Compiler = runtime.Compiler
	payload.GoVersion = runtime.Version()
	payload.OS = runtime.GOOS
	payload.Arch = runtime.GOARCH
	payload.VCSRevision, payload.VCSModified, payload.VCSTime = buildVCSMetadata()
	encoded, err := release.CanonicalJSON(payload)
	if err != nil {
		fmt.Fprintln(stderr, "failed to render release metadata:", err)
		return 1
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}

type multiStringFlag []string

func (f *multiStringFlag) String() string { return strings.Join(*f, ",") }
func (f *multiStringFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

type releaseMetadata = release.Metadata

func metadataPayload() releaseMetadata {
	return releaseMetadata{Module: release.Module, Version: releaseVersion, Contract: ir.IdentityContractVersion}
}
func buildVCSMetadata() (revision, modified, vcsTime string) {
	revision, modified, vcsTime = "unknown", "unknown", "unknown"
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.modified":
				modified = setting.Value
			case "vcs.time":
				vcsTime = setting.Value
			}
		}
	}
	return revision, modified, vcsTime
}

func runIndex(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printUsage(stderr)
	}
	outputPath := fs.String("output", "", "write SQL to a file instead of stdout")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	pos := fs.Args()
	if len(pos) != 1 || strings.TrimSpace(pos[0]) == "" {
		printUsage(stderr)
		return 1
	}
	ws := loadWorkspace(pos[0])
	sem := semantics.Resolve(semantics.NewMemoryWorkspace(ws.Documents...))
	sql := index.Export(ws, sem)
	diagnostics := append(append([]ir.Diagnostic{}, ws.Diagnostics...), sem.Diagnostics...)
	hasErrors := hasErrorDiagnostics(diagnostics)
	if strings.TrimSpace(*outputPath) != "" {
		if hasErrors {
			return 1
		}
		protected := make([]string, 0, len(ws.Documents))
		for _, doc := range ws.Documents {
			protected = append(protected, doc.Path)
		}
		if err := validateOutputPath(*outputPath, protected...); err != nil {
			fmt.Fprintln(stderr, "invalid output path:", err)
			return 1
		}
		if err := writeSQLToFile(*outputPath, sql); err != nil {
			fmt.Fprintln(stderr, "failed to write SQL:", err)
			return 1
		}
	} else {
		fmt.Fprintln(stdout, sql)
	}
	if hasErrors {
		return 1
	}
	return 0
}

func runCorpus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("corpus", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printUsage(stderr)
	}
	outputPath := fs.String("output", "", "write manifest to a file instead of stdout")
	asJSON := fs.Bool("json", false, "emit JSON manifest")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	pos := fs.Args()
	if len(pos) != 1 || strings.TrimSpace(pos[0]) == "" {
		printUsage(stderr)
		return 1
	}

	selectPath := strings.TrimSpace(pos[0])
	m := corpus.BuildManifest(selectPath, profile.NewDistributionProfile(""))
	payload := m.Human()
	if *asJSON {
		bytes, err := m.JSON()
		if err != nil {
			fmt.Fprintln(stderr, "failed to render JSON:", err)
			return 1
		}
		payload = string(bytes)
	}

	hasErrors := m.ErrorCount > 0
	if strings.TrimSpace(*outputPath) != "" {
		if hasErrors {
			return 1
		}
		protected := make([]string, 0, len(m.Entries)+1)
		protected = append(protected, m.SelectPath)
		for _, entry := range m.Entries {
			if entry.ResolvedPath != "" {
				protected = append(protected, entry.ResolvedPath)
			}
		}
		if err := validateOutputPath(*outputPath, protected...); err != nil {
			fmt.Fprintln(stderr, "invalid output path:", err)
			return 1
		}
		if err := writeTextToFile(*outputPath, payload); err != nil {
			fmt.Fprintln(stderr, "failed to write manifest:", err)
			return 1
		}
	} else {
		fmt.Fprintln(stdout, payload)
	}
	if hasErrors {
		return 1
	}
	return 0
}

func writeSQLToFile(path string, sql string) error {
	return writeTextToFile(path, sql)
}

func writeTextToFile(path string, payload string) error {
	target := strings.TrimSpace(path)
	if target == "" {
		return fmt.Errorf("empty output path")
	}
	cleanTarget, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return err
	}

	tmpDir := filepath.Dir(cleanTarget)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(tmpDir, ".ikm-output-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	cleanup := func() {
		_ = os.Remove(tmpPath)
	}
	defer cleanup()

	if _, err := tmpFile.WriteString(payload + "\n"); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return err
	}
	if _, err := os.Stat(cleanTarget); err == nil {
		if err := os.Remove(cleanTarget); err != nil {
			return err
		}
	}
	if err := os.Rename(tmpPath, cleanTarget); err != nil {
		return err
	}
	cleanup = func() {}
	return nil
}

func validateOutputPath(path string, protectedPaths ...string) error {
	target := strings.TrimSpace(path)
	if target == "" {
		return fmt.Errorf("empty output path")
	}
	cleanTarget, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return err
	}

	for _, protected := range protectedPaths {
		if strings.TrimSpace(protected) == "" {
			continue
		}
		cleanProtected := strings.TrimSpace(protected)
		if cleanProtected == "" {
			continue
		}
		if absProtected, absErr := filepath.Abs(filepath.Clean(cleanProtected)); absErr == nil {
			cleanProtected = absProtected
		}
		if alias, aliasErr := pathsAlias(cleanTarget, cleanProtected); aliasErr == nil && alias {
			return fmt.Errorf("%q overlaps protected path %q", cleanTarget, cleanProtected)
		}
	}

	return nil
}

func pathsAlias(left, right string) (bool, error) {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	if pathEquals(left, right) {
		return true, nil
	}

	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr != nil || rightErr != nil {
		return false, nil
	}
	return os.SameFile(leftInfo, rightInfo), nil
}

func pathEquals(left, right string) bool {
	left = filepath.ToSlash(filepath.Clean(strings.TrimSpace(left)))
	right = filepath.ToSlash(filepath.Clean(strings.TrimSpace(right)))
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printUsage(stderr)
	}

	asJSON := fs.Bool("json", false, "emit JSON diagnostics")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	pos := fs.Args()
	if len(pos) != 1 {
		printUsage(stderr)
		return 1
	}

	path := strings.TrimSpace(pos[0])
	if path == "" {
		fmt.Fprintln(stderr, "missing path argument")
		printUsage(stderr)
		return 1
	}

	ws := loadWorkspace(path)
	sem := semantics.Resolve(semantics.NewMemoryWorkspace(ws.Documents...))
	merged := report.FromWorkspaceAndSemantics(ws, sem)

	if *asJSON {
		payload, err := merged.JSON()
		if err != nil {
			fmt.Fprintln(stderr, "failed to render JSON report:", err)
			return 1
		}
		fmt.Fprintln(stdout, string(payload))
	} else {
		fmt.Fprintln(stdout, merged.Human())
	}

	if hasErrorDiagnostics(merged.Diagnostics) {
		return 1
	}
	return 0
}

func hasErrorDiagnostics(diags []ir.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == ir.SeverityError {
			return true
		}
	}
	return false
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: ikm check [--json] <path>")
	fmt.Fprintln(w, "usage: ikm index [--output <file>] <path>")
	fmt.Fprintln(w, "usage: ikm corpus [--json] [--output <file>] <select-path>")
	fmt.Fprintln(w, "usage: ikm metadata --timestamp <RFC3339> [--file <path> ...]")
	fmt.Fprintln(w, "usage: ikm lsp")
	fmt.Fprintln(w, "usage: ikm mcp [--file <path> ...]")
}
