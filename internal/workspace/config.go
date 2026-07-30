package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
)

const (
	ConfigVersion  = "0.1"
	ConfigDirName  = ".ikm"
	ConfigFileName = "config.json"
)

type Budgets struct {
	MaxFiles      int   `json:"maxFiles"`
	MaxBytes      int64 `json:"maxBytes"`
	MaxItems      int   `json:"maxItems"`
	MaxDepth      int   `json:"maxDepth"`
	MaxDurationMS int   `json:"maxDurationMs"`
}

type WorkspaceConfig struct {
	Version             string   `json:"version"`
	Root                string   `json:"root"`
	Profile             string   `json:"profile"`
	EntryPoints         []string `json:"entryPoints"`
	StartupManifest     []string `json:"startupManifest,omitempty"`
	StartupDiagnostics  []string `json:"startupDiagnostics,omitempty"`
	Cache               string   `json:"cache"`
	Adapters            []string `json:"adapters"`
	ExternalRoots       []string `json:"externalRoots"`
	Includes            []string `json:"includes"`
	Excludes            []string `json:"excludes"`
	Budgets             Budgets  `json:"budgets"`
	entryPointsExplicit bool
}

type ConfigFlags struct {
	Root          string
	Profile       string
	Cache         string
	EntryPoints   []string
	Adapters      []string
	ExternalRoots []string
	Includes      []string
	Excludes      []string
	Budgets       *Budgets
}

type ConfigError struct {
	Code, Field string
	Err         error
}

func (e *ConfigError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("configuration.%s: %v", e.Code, e.Err)
	}
	return fmt.Sprintf("configuration.%s (%s): %v", e.Code, e.Field, e.Err)
}
func (e *ConfigError) Unwrap() error { return e.Err }

var defaultBudgets = Budgets{MaxFiles: 100000, MaxBytes: 1 << 30, MaxItems: 10000, MaxDepth: 64, MaxDurationMS: 120000}

func DefaultWorkspaceConfig() WorkspaceConfig {
	return WorkspaceConfig{Version: ConfigVersion, Profile: "strict/portable", Cache: "memory", Budgets: defaultBudgets}
}

// ResolveConfig reads only root/.ikm/config.json and applies flags over config over defaults.
func ResolveConfig(root string, flags ConfigFlags) (WorkspaceConfig, error) {
	root = strings.TrimSpace(root)
	if flags.Root != "" {
		root = strings.TrimSpace(flags.Root)
	}
	if root == "" {
		return WorkspaceConfig{}, &ConfigError{Code: "invalid-root", Err: errors.New("explicit workspace root is required")}
	}
	abs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return WorkspaceConfig{}, &ConfigError{Code: "invalid-root", Err: err}
	}
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		return WorkspaceConfig{}, &ConfigError{Code: "invalid-root", Err: err}
	}
	cfg := DefaultWorkspaceConfig()
	path := filepath.Join(abs, ConfigDirName, ConfigFileName)
	if data, readErr := os.ReadFile(path); readErr == nil {
		dec := json.NewDecoder(strings.NewReader(string(data)))
		dec.DisallowUnknownFields()
		var fileCfg WorkspaceConfig
		if err := dec.Decode(&fileCfg); err != nil {
			return WorkspaceConfig{}, &ConfigError{Code: "invalid", Err: err}
		}
		if fileCfg.Version != "" {
			cfg.Version = fileCfg.Version
		}
		if fileCfg.Profile != "" {
			cfg.Profile = fileCfg.Profile
		}
		if fileCfg.Cache != "" {
			cfg.Cache = fileCfg.Cache
			cfg.entryPointsExplicit = fileCfg.EntryPoints != nil
		}
		if fileCfg.EntryPoints != nil {
			cfg.EntryPoints = fileCfg.EntryPoints
		}
		if fileCfg.Adapters != nil {
			cfg.Adapters = fileCfg.Adapters
		}
		if fileCfg.ExternalRoots != nil {
			cfg.ExternalRoots = fileCfg.ExternalRoots
		}
		if fileCfg.Includes != nil {
			cfg.Includes = fileCfg.Includes
		}
		if fileCfg.Excludes != nil {
			cfg.Excludes = fileCfg.Excludes
		}
		if fileCfg.Budgets != (Budgets{}) {
			cfg.Budgets = fileCfg.Budgets
		}
	}
	if len(cfg.EntryPoints) == 0 {
		manifest, diags := deriveGameEntryPoints(abs)
		cfg.StartupManifest = append([]string(nil), manifest...)
		cfg.StartupDiagnostics = diags
		for _, path := range manifest {
			if isSemanticSourcePath(path) {
				cfg.EntryPoints = append(cfg.EntryPoints, path)
			}
		}
	}
	if flags.Profile != "" {
		cfg.Profile = flags.Profile
	}
	if flags.Cache != "" {
		cfg.Cache = flags.Cache
	}
	if flags.EntryPoints != nil {
		cfg.EntryPoints = flags.EntryPoints
		cfg.entryPointsExplicit = true
	}
	if flags.ExternalRoots != nil {
		cfg.ExternalRoots = flags.ExternalRoots
	}
	if flags.Includes != nil {
		cfg.Includes = flags.Includes
	}
	if flags.Excludes != nil {
		cfg.Excludes = flags.Excludes
	}
	if flags.Budgets != nil {
		cfg.Budgets = *flags.Budgets
	}
	cfg.Root = abs
	_ = flags.Root
	if err := validateConfig(cfg); err != nil {
		return WorkspaceConfig{}, err
	}

	return normalizeConfig(cfg), nil
}

func deriveGameEntryPoints(root string) ([]string, []string) {
	manifest, diags := resolveStartupManifest(root)
	return manifest, diags
}

// LoadConfig is the explicit-root entry point for workspace configuration.
func LoadConfig(root string, flags ConfigFlags) (WorkspaceConfig, error) {
	return ResolveConfig(root, flags)
}

// CanonicalConfigDigest returns the stable digest of a normalized configuration.
func CanonicalConfigDigest(cfg WorkspaceConfig) string { return cfg.Digest() }

func validateConfig(cfg WorkspaceConfig) error {
	if cfg.Version != ConfigVersion {
		return &ConfigError{Code: "unsupported-version", Field: "version", Err: fmt.Errorf("%q", cfg.Version)}
	}
	if strings.TrimSpace(cfg.Profile) != "strict/portable" && strings.TrimSpace(cfg.Profile) != "distribution" {
		return &ConfigError{Code: "invalid-profile", Field: "profile", Err: fmt.Errorf("%q", cfg.Profile)}
	}
	if cfg.Cache != "memory" && cfg.Cache != "disk" && cfg.Cache != "disabled" {
		return &ConfigError{Code: "invalid-limit", Field: "cache", Err: errors.New("must be memory, disk, or disabled")}
	}
	b := cfg.Budgets
	if b.MaxFiles <= 0 || b.MaxBytes <= 0 || b.MaxItems <= 0 || b.MaxDepth <= 0 || b.MaxDurationMS <= 0 {
		return &ConfigError{Code: "invalid-limit", Field: "budgets", Err: errors.New("all limits must be positive")}
	}
	return nil
}

func normalizeConfig(cfg WorkspaceConfig) WorkspaceConfig {
	cfg.EntryPoints = sortedClean(cfg.EntryPoints)
	cfg.Adapters = sortedClean(cfg.Adapters)
	cfg.ExternalRoots = sortedClean(cfg.ExternalRoots)
	cfg.Includes = sortedClean(cfg.Includes)
	cfg.Excludes = sortedClean(cfg.Excludes)
	return cfg
}
func sortedClean(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func (c WorkspaceConfig) Digest() string {
	data, _ := json.Marshal(normalizeConfig(c))
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (c WorkspaceConfig) ProfileValue() profile.CompatibilityProfile {
	p := profile.NewStrictPortableProfile(c.Root)
	if c.Profile == "distribution" {
		p = profile.NewDistributionProfile(c.Root)
	}
	p.Name = c.Profile
	if len(c.ExternalRoots) > 0 {
		p.FallbackRoots = append([]string(nil), c.ExternalRoots...)
	}
	return p
}
