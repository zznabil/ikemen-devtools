package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const ManifestVersion = "0.1"

type DiscoveredFile struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Active     bool   `json:"active"`
	Reason     string `json:"reason"`
	EntryPoint string `json:"entryPoint,omitempty"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
}
type DiscoveryResult struct {
	Version      string           `json:"version"`
	Root         string           `json:"root"`
	Files        []DiscoveredFile `json:"files"`
	Orphans      int              `json:"orphans"`
	Truncated    bool             `json:"truncated"`
	Reasons      []string         `json:"truncationReasons,omitempty"`
	ConfigDigest string           `json:"configurationDigest"`
}
type Snapshot struct {
	Version   string           `json:"version"`
	ID        string           `json:"id"`
	Files     []DiscoveredFile `json:"files"`
	Truncated bool             `json:"truncated"`
	Reasons   []string         `json:"truncationReasons,omitempty"`
}

func Discover(root string, cfg WorkspaceConfig) (DiscoveryResult, error) {
	a, e := NewPathAuthority(root, cfg.ExternalRoots)
	if e != nil {
		return DiscoveryResult{}, e
	}
	d := DiscoveryResult{Version: ManifestVersion, Root: a.Root(), ConfigDigest: cfg.Digest()}
	ignore := loadIgnore(a.Root())
	entries := map[string]string{}
	for _, ep := range cfg.EntryPoints {
		p, e := a.Resolve(ep)
		if e == nil {
			entries[p.Canonical] = ep
		}
	}
	var paths []string
	e = filepath.WalkDir(a.Root(), func(path string, ent fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ent.IsDir() {
			rel, _ := filepath.Rel(a.Root(), path)
			if path != a.Root() && ignored(filepath.ToSlash(rel), ignore) {
				return fs.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(a.Root(), path)
		rel = filepath.ToSlash(rel)
		if ignored(rel, ignore) || strings.HasPrefix(rel, ".ikm/") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if e != nil {
		return d, e
	}
	sort.Strings(paths)
	for _, p := range paths {
		st, e := os.Stat(p)
		if e != nil {
			continue
		}
		rel, _ := filepath.Rel(a.Root(), p)
		rel = filepath.ToSlash(rel)
		ep, active := entries[p]
		f := DiscoveredFile{Path: rel, Kind: filepath.Ext(p), Active: active, Reason: "fallback-inventory", EntryPoint: ep, Size: st.Size()}
		if st.Size() <= cfg.Budgets.MaxBytes {
			if b, x := os.ReadFile(p); x == nil {
				h := sha256.Sum256(b)
				f.SHA256 = hex.EncodeToString(h[:])
			}
		}
		d.Files = append(d.Files, f)
		if !active {
			d.Orphans++
		}
		if len(d.Files) >= cfg.Budgets.MaxFiles {
			d.Truncated = true
			d.Reasons = []string{"maxFiles"}
			break
		}
	}
	return d, nil
}
func (d DiscoveryResult) Snapshot() Snapshot {
	b, _ := json.Marshal(struct {
		Files     []DiscoveredFile `json:"files"`
		Truncated bool             `json:"truncated"`
		Reasons   []string         `json:"reasons,omitempty"`
	}{d.Files, d.Truncated, d.Reasons})
	h := sha256.Sum256(b)
	return Snapshot{Version: ManifestVersion, ID: hex.EncodeToString(h[:]), Files: d.Files, Truncated: d.Truncated, Reasons: d.Reasons}
}
func loadIgnore(root string) []string {
	b, e := os.ReadFile(filepath.Join(root, ".ikmignore"))
	if e != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out
}
func ignored(path string, pats []string) bool {
	for _, p := range pats {
		p = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(p)), "/")
		if ok, _ := filepath.Match(p, path); ok {
			return true
		}
		if strings.HasSuffix(p, "/") && strings.HasPrefix(path, p) {
			return true
		}
		if strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}
