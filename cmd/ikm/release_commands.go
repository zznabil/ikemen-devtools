package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ikemen-engine/ikemen-devtools/internal/capability"
	"github.com/ikemen-engine/ikemen-devtools/internal/contract"
	"github.com/ikemen-engine/ikemen-devtools/internal/doctor"
	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/workspace"
)

type versionResult struct {
	SchemaVersion    string            `json:"schemaVersion"`
	BinaryVersion    string            `json:"binaryVersion"`
	Commit           string            `json:"commit"`
	BuildTime        string            `json:"buildTime"`
	GoVersion        string            `json:"goVersion"`
	OS               string            `json:"os"`
	Arch             string            `json:"arch"`
	SchemaVersions   map[string]string `json:"schemaVersions"`
	ProtocolVersions map[string]string `json:"protocolVersions"`
	FeatureGates     map[string]bool   `json:"featureGates"`
}

func runVersion(args []string, stdout, stderr interface{ Write([]byte) (int, error) }) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable version")
	if err := fs.Parse(args); err != nil || len(fs.Args()) != 0 {
		return contract.ExitUsage
	}
	revision, _, vcsTime := buildVCSMetadata()
	result := versionResult{SchemaVersion: contract.SchemaVersion, BinaryVersion: releaseVersion, Commit: revision, BuildTime: vcsTime, GoVersion: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH,
		SchemaVersions: map[string]string{"envelope": contract.SchemaVersion, "identity": ir.IdentityContractVersion}, ProtocolVersions: map[string]string{"mcp": "2025-06-18", "lsp": "3.17"}, FeatureGates: map[string]bool{"readOnly": true, "mutation": false, "runtime": false}}
	if *asJSON {
		b, _ := json.Marshal(result)
		fmt.Fprintln(stdout, string(b))
		return contract.ExitOK
	}
	fmt.Fprintf(stdout, "ikm %s (%s/%s, %s)\n", result.BinaryVersion, result.OS, result.Arch, result.GoVersion)
	return contract.ExitOK
}

func runConfig(args []string, stdout, stderr interface{ Write([]byte) (int, error) }) int {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	fs.SetOutput(stderr)
	effective := fs.Bool("effective", false, "show resolved values")
	asJSON := fs.Bool("json", false, "emit JSON")
	root := fs.String("root", ".", "workspace root")
	if err := fs.Parse(args); err != nil || (len(fs.Args()) != 0 && (len(fs.Args()) != 1 || fs.Args()[0] != "show")) || !*effective {
		return contract.ExitUsage
	}
	cfg, err := workspace.ResolveConfig(*root, workspace.ConfigFlags{})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return contract.ExitInput
	}
	payload := map[string]any{"schemaVersion": contract.SchemaVersion, "operation": "config.show", "status": "complete", "config": cfg, "digest": cfg.Digest(), "provenance": map[string]string{"defaults": "built-in", "workspace": filepath.Join(cfg.Root, workspace.ConfigDirName, workspace.ConfigFileName), "flags": "none"}}
	if *asJSON {
		b, _ := json.Marshal(payload)
		fmt.Fprintln(stdout, string(b))
		return contract.ExitOK
	}
	fmt.Fprintf(stdout, "root=%s profile=%s cache=%s digest=%s\n", cfg.Root, cfg.Profile, cfg.Cache, cfg.Digest())
	return contract.ExitOK
}

func runCapabilities(args []string, stdout, stderr interface{ Write([]byte) (int, error) }) int {
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON")
	transport := fs.String("transport", "cli", "transport: cli or mcp")
	if err := fs.Parse(args); err != nil || len(fs.Args()) != 0 {
		return contract.ExitUsage
	}
	if *transport != "cli" && *transport != "mcp" {
		fmt.Fprintln(stderr, "unsupported transport")
		return contract.ExitUsage
	}
	registry := capability.DefaultRegistry()
	payload := map[string]any{"schemaVersion": contract.SchemaVersion, "operation": "capabilities", "status": "complete", "transport": *transport, "capabilities": registry.Filter(capability.Availability{Read: true})}
	if *asJSON {
		b, _ := json.Marshal(payload)
		fmt.Fprintln(stdout, string(b))
		return contract.ExitOK
	}
	for _, d := range registry.Filter(capability.Availability{Read: true}) {
		fmt.Fprintln(stdout, d.Name)
	}
	return contract.ExitOK
}

func runDoctor(args []string, stdout, stderr interface{ Write([]byte) (int, error) }) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON")
	root := fs.String("root", ".", "workspace root")
	cache := fs.String("cache", "", "cache path")
	fix := fs.Bool("fix", false, "apply explicitly selected reversible maintenance action")
	rebuild := fs.Bool("rebuild-cache", false, "rebuild selected cache")
	if err := fs.Parse(args); err != nil || len(fs.Args()) != 0 {
		return contract.ExitUsage
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return contract.ExitInput
	}
	if st, statErr := os.Stat(abs); statErr != nil || !st.IsDir() {
		fmt.Fprintln(stderr, "workspace root is unavailable")
		return contract.ExitInput
	}
	cfg, cfgErr := workspace.ResolveConfig(abs, workspace.ConfigFlags{})
	cachePath := *cache
	if cachePath == "" && cfgErr == nil && cfg.Cache == "disk" {
		cachePath = filepath.Join(abs, workspace.ConfigDirName, "cache")
	}
	findings := []contract.Diagnostic{}
	if cfgErr != nil {
		findings = append(findings, contract.Diagnostic{Code: "CONFIG_INVALID", Severity: "error", Message: cfgErr.Error(), Evidence: map[string]any{"root": abs}})
	}
	if cachePath != "" {
		report := doctor.Inspect(cachePath)
		if report.Health != "healthy" {
			findings = append(findings, contract.Diagnostic{Code: "CACHE_" + strings.ToUpper(report.Health), Severity: "warning", Message: "cache is not healthy; rebuild from source", Path: "", Evidence: report})
		}
		if *fix && *rebuild {
			if err := doctor.Rebuild(cachePath); err != nil {
				findings = append(findings, contract.Diagnostic{Code: "CACHE_REBUILD_FAILED", Severity: "error", Message: err.Error()})
			}
		}
	}
	payload := contract.Envelope{SchemaVersion: contract.SchemaVersion, Operation: "doctor", Tool: "ikm", Status: contract.StatusComplete, Result: map[string]any{"workspace": abs, "cache": cachePath, "readOnly": !(*fix && *rebuild)}, Diagnostics: findings}
	if *asJSON {
		b, _ := payload.CanonicalJSON()
		fmt.Fprintln(stdout, string(b))
	} else {
		if len(findings) == 0 {
			fmt.Fprintln(stdout, "doctor: healthy")
		} else {
			for _, f := range findings {
				fmt.Fprintf(stdout, "%s %s: %s\n", f.Severity, f.Code, f.Message)
			}
		}
	}
	if len(findings) > 0 {
		return contract.ExitFindings
	}
	return contract.ExitOK
}
