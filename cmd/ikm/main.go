package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/ikemen-engine/ikemen-devtools/internal/contract"
	"github.com/ikemen-engine/ikemen-devtools/internal/corpus"
	"github.com/ikemen-engine/ikemen-devtools/internal/index"
	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/lsp"
	"github.com/ikemen-engine/ikemen-devtools/internal/mcp"
	"github.com/ikemen-engine/ikemen-devtools/internal/mutation"
	"github.com/ikemen-engine/ikemen-devtools/internal/operations"
	"github.com/ikemen-engine/ikemen-devtools/internal/oracle"
	"github.com/ikemen-engine/ikemen-devtools/internal/patch"
	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
	"github.com/ikemen-engine/ikemen-devtools/internal/release"
	"github.com/ikemen-engine/ikemen-devtools/internal/report"
	"github.com/ikemen-engine/ikemen-devtools/internal/semantics"
	"github.com/ikemen-engine/ikemen-devtools/internal/trace"
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
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "config":
		return runConfig(args[1:], stdout, stderr)
	case "capabilities":
		return runCapabilities(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "index":
		return runIndex(args[1:], stdout, stderr)
	case "corpus":
		return runCorpus(args[1:], stdout, stderr)
	case "workspace":
		return runWorkspace(args[1:], stdout, stderr)
	case "metadata":
		return runMetadata(args[1:], stdout, stderr)
	case "lsp":
		return runLSP(args[1:], stdin, stdout, stderr)
	case "mcp":
		return runMCP(args[1:], stdin, stdout, stderr)
	case "patch":
		return runPatch(args[1:], stdout, stderr)
	case "rename", "fix":
		return runMutationPrepare(subcommand, args[1:], stdout, stderr)
	case "runtime":
		if len(args) > 1 && args[1] == "oracle" {
			return runRuntimeOracle(args[2:], stdout, stderr)
		}
		return runRuntime(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		if len(args) > 1 && args[1] == "--json" {
			payload := map[string]any{"operations": []string{"workspace", "query.diagnostics", "query.symbols", "query.definition", "query.references", "query.search", "graph.dependencies", "graph.dependents", "graph.path", "graph.impact", "inspect.workspace", "inspect.character", "inspect.stage", "inspect.file", "export.jsonl", "export.scip", "export.sql", "rename.prepare", "fix.prepare", "patch.plan", "patch.diff", "patch.apply", "patch.recover", "runtime.trace"}, "exitCodes": map[string]int{"ok": 0, "findings": 1, "usage": 2, "input": 3, "internal": 4, "budget": 5, "conflict": 6, "runtime": 7}, "authority": "read-only"}
			b, _ := json.Marshal(payload)
			fmt.Fprintln(stdout, string(b))
			return 0
		}
		printUsage(stdout)
		return 0
	case "query", "graph", "inspect", "export":
		return runReadOnly(subcommand, args[1:], stdout, stderr)
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
	files := multiStringFlag{}
	fs.Var(&files, "file", "document to preload (repeatable)")
	root := fs.String("root", "", "workspace root or entry definition")
	allowWrite := fs.Bool("allow-write", false, "advertise guarded mutation tools")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if len(fs.Args()) != 0 {
		printUsage(stderr)
		return 1
	}
	server := mcp.NewServerWithPolicy(releaseVersion, *allowWrite)
	ctx := context.Background()
	if strings.TrimSpace(*root) != "" {
		if err := server.SetRoot(*root); err != nil {
			fmt.Fprintln(stderr, "ikm mcp: invalid root:", err)
			return 3
		}
		loaded := workspace.LoadWorkspaceWithProfile(*root, profile.NewDistributionProfile(""))
		for _, d := range loaded.Documents {
			if b, e := os.ReadFile(d.Path); e == nil {
				_ = server.SetDocument(ctx, d.Path, b)
			}
		}
		for _, d := range loaded.Diagnostics {
			fmt.Fprintf(stderr, "ikm mcp: %s: %s\n", d.Code, d.Message)
		}
	}
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

func runWorkspace(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workspace", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "explicit workspace root")
	profileName := fs.String("profile", "", "profile override")
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return contract.ExitCode(contract.FailureUsage, false)
	}
	if strings.TrimSpace(*root) == "" || len(fs.Args()) != 1 {
		return contract.ExitCode(contract.FailureUsage, false)
	}
	cfg, err := workspace.ResolveConfig(*root, workspace.ConfigFlags{Profile: *profileName})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return contract.ExitCode(contract.FailureInput, false)
	}
	d, err := workspace.Discover(cfg.Root, cfg)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return contract.ExitCode(contract.FailureInternal, false)
	}
	snap := d.Snapshot()
	if *asJSON {
		out := contract.Envelope{Operation: "workspace." + fs.Args()[0], Tool: "ikm", Status: contract.StatusComplete, Workspace: contract.Workspace{Root: "", Profile: cfg.Profile, Configuration: cfg.Digest()}, Snapshot: contract.Snapshot{ID: snap.ID}, Result: map[string]any{"files": len(d.Files), "orphans": d.Orphans}, Page: contract.Page{Returned: len(d.Files)}, Truncation: contract.Truncation{Truncated: d.Truncated, Reasons: d.Reasons}}
		b, e := out.CanonicalJSON()
		if e != nil {
			return contract.ExitCode(contract.FailureInternal, false)
		}
		fmt.Fprintln(stdout, string(b))
		return contract.ExitOK
	}

	fmt.Fprintf(stdout, "Root: %s\nProfile: %s\nFiles: %d\nOrphans: %d\nSnapshot: %s\n", cfg.Root, cfg.Profile, len(d.Files), d.Orphans, snap.ID)
	return contract.ExitOK
}
func runReadOnly(group string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return contract.ExitUsage
	}
	action := strings.ToLower(args[0])
	fs := flag.NewFlagSet(group, flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "workspace root or select.def")
	profileName := fs.String("profile", "", "compatibility profile")
	path := fs.String("path", "", "path filter")
	name := fs.String("name", "", "symbol name")
	kind := fs.String("kind", "", "kind/classification")
	code := fs.String("code", "", "diagnostic code")
	severity := fs.String("severity", "", "diagnostic severity")
	query := fs.String("query", "", "text query")
	identity := fs.String("identity", "", "stable identity")
	snapshot := fs.String("snapshot", "", "snapshot id")
	cursor := fs.String("cursor", "", "opaque page cursor")
	limit := fs.Int("limit", 100, "page size")
	jsonOut := fs.Bool("json", false, "canonical JSON")
	jsonl := fs.Bool("jsonl", false, "JSONL output")
	includeDecl := fs.Bool("include-declarations", false, "include declaration references")
	if err := fs.Parse(args[1:]); err != nil {
		return contract.ExitUsage
	}
	if strings.TrimSpace(*root) == "" {
		fmt.Fprintln(stderr, "--root is required")
		return contract.ExitUsage
	}
	o := operations.Options{Root: *root, Profile: *profileName, Path: *path, Name: *name, Kind: *kind, Code: *code, Severity: *severity, Query: *query, Identity: *identity, Snapshot: *snapshot, Cursor: *cursor, Limit: *limit, IncludeDeclarations: *includeDecl}
	ctx := context.Background()
	var r operations.Result
	var err error
	switch group {
	case "query":
		switch action {
		case "diagnostics":
			r, err = operations.Diagnostics(ctx, o)
		case "symbols":
			r, err = operations.Symbols(ctx, o)
		case "definition", "references":
			r, err = operations.References(ctx, o)
		case "search":
			r, err = operations.Search(ctx, o)
		default:
			fmt.Fprintln(stderr, "unknown query operation")
			return contract.ExitUsage
		}
	case "graph":
		r, err = operations.Graph(ctx, o)
	case "inspect":
		r, err = operations.Inspect(ctx, o)
	case "export":
		r, err = operations.Export(ctx, o, action)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return contract.ExitBudget
		}
		return contract.ExitInput
	}
	if *jsonl {
		if v, ok := r.Envelope.Result.([]any); ok {
			for _, x := range v {
				b, _ := json.Marshal(x)
				fmt.Fprintln(stdout, string(b))
			}
		} else {
			b, _ := r.Envelope.CanonicalJSON()
			fmt.Fprintln(stdout, string(b))
		}
	} else if *jsonOut {
		if err := operations.WriteJSON(stdout, r); err != nil {
			return contract.ExitInternal
		}
	} else {
		fmt.Fprintln(stdout, string(mustJSON(r.Envelope.Result)))
	}
	for _, d := range r.Envelope.Diagnostics {
		if d.Severity == string(ir.SeverityError) {
			return contract.ExitFindings
		}
	}
	return contract.ExitOK
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: ikm workspace --root <path> (scan|status) [--json]")
	fmt.Fprintln(w, "usage: ikm version [--json]")
	fmt.Fprintln(w, "usage: ikm config show --effective [--root <path>] [--json]")
	fmt.Fprintln(w, "usage: ikm capabilities [--transport cli|mcp] [--json]")
	fmt.Fprintln(w, "usage: ikm doctor --root <path> [--cache <path>] [--json] [--fix --rebuild-cache]")
	fmt.Fprintln(w, "usage: ikm check [--json] <path>")
	fmt.Fprintln(w, "usage: ikm index [--output <file>] <path>")
	fmt.Fprintln(w, "usage: ikm corpus [--json] [--output <file>] <select-path>")
	fmt.Fprintln(w, "usage: ikm metadata --timestamp <RFC3339> [--file <path> ...]")
	fmt.Fprintln(w, "usage: ikm lsp")
	fmt.Fprintln(w, "usage: ikm mcp [--file <path> ...]")
	fmt.Fprintln(w, "usage: ikm query diagnostics|symbols|definition|references|search --root <path> [flags]")
	fmt.Fprintln(w, "usage: ikm graph dependencies|dependents|path|impact --root <path> [flags]")
	fmt.Fprintln(w, "usage: ikm inspect workspace|character|stage|file --root <path> [flags]")
	fmt.Fprintln(w, "usage: ikm export jsonl|scip|sql --root <path> [--json]")
}
func readPatchPlan(path string) (patch.PatchPlan, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return patch.PatchPlan{}, err
	}
	var plan patch.PatchPlan
	if err := json.Unmarshal(b, &plan); err != nil {
		return plan, err
	}
	if err := plan.Validate(); err != nil {
		return plan, err
	}
	return plan, nil
}

func runPatch(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: ikm patch plan|diff|apply|recover <plan>")
		return 2
	}
	op, path := strings.ToLower(args[0]), args[1]
	plan, err := readPatchPlan(path)
	if err != nil {
		fmt.Fprintln(stderr, "invalid patch plan:", err)
		return 3
	}
	root := plan.WorkspaceRoot
	switch op {
	case "plan", "diff":
		result, err := patch.PreviewPatch(root, patch.Patch{Edits: plan.Edits})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 6
		}
		return writeJSON(stdout, map[string]any{"schemaVersion": contract.SchemaVersion, "operation": "patch." + op, "workspace": root, "result": result})
	case "apply":
		fs := flag.NewFlagSet("patch apply", flag.ContinueOnError)
		fs.SetOutput(stderr)
		allow := fs.Bool("allow-write", false, "authorize writes")
		if err := fs.Parse(args[2:]); err != nil {
			return 2
		}
		if !*allow {
			fmt.Fprintln(stderr, "policy refusal: patch apply requires --allow-write")
			return 6
		}
		auth := mutation.NewAuthorizer(true)
		token, err := auth.Issue(plan)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 6
		}
		result, err := (mutation.Service{Authorizer: auth}).Apply(context.Background(), root, token, plan.InputSnapshot)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 6
		}
		return writeJSON(stdout, map[string]any{"schemaVersion": contract.SchemaVersion, "operation": "patch.apply", "workspace": root, "result": result})
	case "recover":
		if err := patch.Recover(root); err != nil {
			fmt.Fprintln(stderr, err)
			return 6
		}
		return writeJSON(stdout, map[string]any{"schemaVersion": contract.SchemaVersion, "operation": "patch.recover", "workspace": root, "result": "recovered"})
	default:
		fmt.Fprintln(stderr, "unknown patch operation:", op)
		return 2
	}
}

func runMutationPrepare(kind string, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintf(stderr, "usage: ikm %s prepare ...\n", kind)
		return 2
	}
	if args[0] != "prepare" {
		fmt.Fprintln(stderr, "only preview preparation is available; use '<kind> prepare'")
		return 2
	}
	return writeJSON(stdout, map[string]any{"schemaVersion": contract.SchemaVersion, "operation": kind + ".prepare", "result": map[string]any{"status": "requires semantic provider", "readOnly": true}})
}

func runRuntime(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] != "trace" {
		fmt.Fprintln(stderr, "usage: ikm runtime trace --allow-runtime --workdir <dir> --timeout <duration> --max-stdout <bytes> --max-stderr <bytes> <command> [args...]")
		return 2
	}
	fs := flag.NewFlagSet("runtime trace", flag.ContinueOnError)
	fs.SetOutput(stderr)
	allow := fs.Bool("allow-runtime", false, "authorize runtime execution")
	workdir := fs.String("workdir", "", "working directory")
	timeout := fs.Duration("timeout", 30*time.Second, "execution timeout")
	maxOut := fs.Int("max-stdout", trace.DefaultMaxOutputBytes, "stdout budget")
	maxErr := fs.Int("max-stderr", trace.DefaultMaxOutputBytes, "stderr budget")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if !*allow {
		fmt.Fprintln(stderr, "policy refusal: runtime execution requires --allow-runtime")
		return 6
	}
	if *timeout <= 0 || *maxOut <= 0 || *maxErr <= 0 || len(fs.Args()) == 0 {
		fmt.Fprintln(stderr, "runtime requires positive budgets and a command")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	result, err := trace.NewService(trace.NewBridge(nil, nil)).Check(ctx, trace.Config{Command: fs.Args()[0], Args: fs.Args()[1:], WorkingDir: *workdir, MaxStdoutBytes: *maxOut, MaxStderrBytes: *maxErr})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 7
	}
	return writeJSON(stdout, map[string]any{"schemaVersion": contract.SchemaVersion, "operation": "runtime.trace", "result": result})
}
func runRuntimeOracle(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("runtime oracle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	allow := fs.Bool("allow-runtime", false, "authorize runtime execution")
	workdir := fs.String("workdir", "", "working directory")
	timeout := fs.Duration("timeout", 30*time.Second, "execution timeout")
	expectedPath := fs.String("expected", "", "expected JSON snapshot")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !*allow {
		fmt.Fprintln(stderr, "policy refusal: runtime oracle requires --allow-runtime")
		return 6
	}
	if *timeout <= 0 || *expectedPath == "" || len(fs.Args()) == 0 {
		fmt.Fprintln(stderr, "runtime oracle requires --expected, positive timeout, and command")
		return 2
	}
	expected, err := os.ReadFile(*expectedPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 3
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	cmp, err := oracle.Compare(ctx, oracle.Request{Command: fs.Args()[0], Args: fs.Args()[1:], WorkingDir: *workdir, Timeout: *timeout}, expected, nil)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 7
	}
	code := 0
	if !cmp.Match() {
		code = 1
	}
	if writeJSON(stdout, map[string]any{"schemaVersion": contract.SchemaVersion, "operation": "runtime.oracle", "result": cmp}) != 0 {
		return 4
	}
	return code
}

func writeJSON(w io.Writer, value any) int {
	b, err := json.Marshal(value)
	if err != nil {
		return 4
	}
	fmt.Fprintln(w, string(b))
	return 0
}
