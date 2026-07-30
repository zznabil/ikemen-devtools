package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ikemen-engine/ikemen-devtools/internal/document"
	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"github.com/ikemen-engine/ikemen-devtools/internal/profile"
	"github.com/ikemen-engine/ikemen-devtools/internal/semantics"
	"github.com/ikemen-engine/ikemen-devtools/internal/workspace"
)

// ServiceResult holds analysis outputs returned by the coordinator.
type ServiceResult struct {
	Workspace   workspace.LoadResult
	Semantics   semantics.ResolveResult
	Diagnostics []ir.Diagnostic
}

// Option customizes coordinator behavior.
type Option func(*Coordinator)

// WithReadFile replaces file reads (tests only).
func WithReadFile(readFile func(string) ([]byte, error)) Option {
	return func(c *Coordinator) {
		if readFile != nil {
			c.readFile = readFile
		}
	}
}

// WithProfile sets workspace resolution behavior.
func WithProfile(profile profile.CompatibilityProfile) Option {
	return func(c *Coordinator) {
		c.profile = profile
	}
}

// ErrEmptyWorkspacePath indicates no definition path was supplied.
var ErrEmptyWorkspacePath = errors.New("service: empty workspace path")

// Coordinator caches workspace analyses and invalidates dependents on changes.
type Coordinator struct {
	mu       sync.Mutex
	readFile func(string) ([]byte, error)
	profile  profile.CompatibilityProfile

	roots      map[string]*rootCacheEntry
	documents  map[string]*documentCacheEntry
	dependents map[string]map[string]struct{}
	inFlight   map[string]context.CancelFunc
}

// NewCoordinator creates an empty coordinator.
func NewCoordinator(opts ...Option) *Coordinator {
	coord := &Coordinator{
		readFile:   os.ReadFile,
		profile:    profile.NewStrictPortableProfile(""),
		roots:      make(map[string]*rootCacheEntry),
		documents:  make(map[string]*documentCacheEntry),
		dependents: make(map[string]map[string]struct{}),
		inFlight:   make(map[string]context.CancelFunc),
	}

	for _, option := range opts {
		option(coord)
	}
	if coord.profile.Name == "" {
		coord.profile = profile.NewStrictPortableProfile("")
	}
	return coord
}

// Analyze resolves a workspace path, using cached dependencies when valid.
func (c *Coordinator) Analyze(ctx context.Context, defPath string) (ServiceResult, error) {
	if strings.TrimSpace(defPath) == "" {
		return ServiceResult{}, ErrEmptyWorkspacePath
	}
	if err := ctx.Err(); err != nil {
		return ServiceResult{}, err
	}
	canonical, err := canonicalizePath(defPath)
	if err != nil {
		return ServiceResult{}, err
	}

	analysisCtx, cancel := c.begin(canonical, ctx)
	defer func() {
		c.end(canonical)
		cancel()
	}()

	if cached, ok, err := c.hit(analysisCtx, canonical); ok {
		if err != nil {
			return ServiceResult{}, err
		}
		return cached, nil
	} else if err != nil {
		return ServiceResult{}, err
	}

	if err := analysisCtx.Err(); err != nil {
		return ServiceResult{}, err
	}
	return c.analyze(analysisCtx, canonical)
}

func (c *Coordinator) analyze(ctx context.Context, canonicalDef string) (ServiceResult, error) {
	if err := ctx.Err(); err != nil {
		return ServiceResult{}, err
	}

	loaded := workspace.LoadWorkspaceWithProfile(canonicalDef, c.profile)
	if err := ctx.Err(); err != nil {
		return ServiceResult{}, err
	}

	hashes := make(map[string]string, len(loaded.Documents))
	for _, doc := range loaded.Documents {
		if err := ctx.Err(); err != nil {
			return ServiceResult{}, err
		}
		path, err := canonicalizePath(doc.Path)
		if err != nil {
			return ServiceResult{}, err
		}
		record, err := c.snapshot(ctx, path)
		if err != nil {
			return ServiceResult{}, fmt.Errorf("snapshot %q: %w", path, err)
		}
		hashes[path] = record.hash
	}

	semInput := make([]ir.Document, 0, len(loaded.Documents))
	for _, doc := range loaded.Documents {
		semInput = append(semInput, doc)
	}
	sem := semantics.Resolve(semantics.NewMemoryWorkspace(semInput...))
	diagnostics := append(append([]ir.Diagnostic(nil), loaded.Diagnostics...), sem.Diagnostics...)
	sortDiagnostics(diagnostics)

	result := ServiceResult{
		Workspace:   cloneWorkspaceResult(loaded),
		Semantics:   cloneResolveResult(sem),
		Diagnostics: diagnostics,
	}

	c.storeResult(canonicalDef, result, hashes)
	return result, nil
}

func (c *Coordinator) hit(ctx context.Context, root string) (ServiceResult, bool, error) {
	c.mu.Lock()
	entry, ok := c.roots[root]
	if !ok || !entry.valid {
		c.mu.Unlock()
		return ServiceResult{}, false, nil
	}
	cachedHashes := make(map[string]string, len(entry.dependencyHashes))
	for path, hash := range entry.dependencyHashes {
		cachedHashes[path] = hash
	}
	c.mu.Unlock()

	for path, expected := range cachedHashes {
		if err := ctx.Err(); err != nil {
			return ServiceResult{}, false, err
		}
		record, err := c.snapshot(ctx, path)
		if err != nil || record.hash != expected {
			c.markInvalid(root)
			return ServiceResult{}, false, nil
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	entry = c.roots[root]
	if entry == nil || !entry.valid {
		return ServiceResult{}, false, nil
	}
	return cloneServiceResult(entry.result), true, nil
}

func (c *Coordinator) markInvalid(root string) {
	c.mu.Lock()
	if entry, ok := c.roots[root]; ok {
		entry.valid = false
	}
	c.mu.Unlock()
}

func (c *Coordinator) snapshot(ctx context.Context, path string) (*documentCacheEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	canonical, err := canonicalizePath(path)
	if err != nil {
		return nil, err
	}

	info, statErr := os.Stat(canonical)
	if statErr != nil {
		record := &documentCacheEntry{path: canonical, loadErr: statErr}
		c.mu.Lock()
		prev := c.documents[canonical]
		c.documents[canonical] = record
		c.mu.Unlock()
		if prev != nil && prev.hash != "" {
			c.invalidateDependents(canonical)
		}
		return record, statErr
	}

	c.mu.Lock()
	existing, ok := c.documents[canonical]
	if ok && !isExpired(existing, info) {
		c.mu.Unlock()
		return existing, nil
	}
	c.mu.Unlock()

	bytes, err := c.readFile(canonical)
	if err != nil {
		record := &documentCacheEntry{
			path:     canonical,
			size:     info.Size(),
			mtime:    info.ModTime(),
			loadErr:  err,
			hash:     "",
			snapshot: nil,
		}
		c.mu.Lock()
		prev := c.documents[canonical]
		c.documents[canonical] = record
		c.mu.Unlock()
		if prev == nil || prev.hash != "" || prev.loadErr != nil {
			c.invalidateDependents(canonical)
		}
		return record, err
	}

	parsed, err := document.NewSnapshot(canonical, bytes)
	record := &documentCacheEntry{
		path:    canonical,
		size:    info.Size(),
		mtime:   info.ModTime(),
		loadErr: err,
	}
	if err == nil {
		record.hash = parsed.Hash()
		record.snapshot = parsed
	}
	c.mu.Lock()
	prev := c.documents[canonical]
	c.documents[canonical] = record
	c.mu.Unlock()
	if prev == nil || prev.hash != record.hash || prev.loadErr != nil {
		c.invalidateDependents(canonical)
	}
	if err != nil {
		return record, err
	}
	return record, nil
}

func (c *Coordinator) storeResult(root string, result ServiceResult, hashes map[string]string) {
	deps := make(map[string]struct{}, len(hashes))
	for path := range hashes {
		deps[path] = struct{}{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.roots[root]
	if !ok {
		entry = &rootCacheEntry{}
		c.roots[root] = entry
	}
	for oldDep := range entry.dependencies {
		if set, ok := c.dependents[oldDep]; ok {
			delete(set, root)
			if len(set) == 0 {
				delete(c.dependents, oldDep)
			}
		}
	}
	for dep := range deps {
		set, ok := c.dependents[dep]
		if !ok {
			set = make(map[string]struct{})
			c.dependents[dep] = set
		}
		set[root] = struct{}{}
	}
	entry.valid = true
	entry.dependencies = deps
	entry.dependencyHashes = make(map[string]string, len(hashes))
	for path, hash := range hashes {
		entry.dependencyHashes[path] = hash
	}
	entry.result = cloneServiceResult(result)
}

func (c *Coordinator) begin(root string, ctx context.Context) (context.Context, context.CancelFunc) {
	c.mu.Lock()
	if cancel, ok := c.inFlight[root]; ok {
		cancel()
	}
	analysisCtx, cancel := context.WithCancel(ctx)
	c.inFlight[root] = cancel
	c.mu.Unlock()
	return analysisCtx, cancel
}

func (c *Coordinator) end(root string) {
	c.mu.Lock()
	delete(c.inFlight, root)
	c.mu.Unlock()
}

func (c *Coordinator) invalidateDependents(path string) {
	c.mu.Lock()
	queue := []string{path}
	seen := map[string]struct{}{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, ok := seen[current]; ok {
			continue
		}
		seen[current] = struct{}{}
		for root := range c.dependents[current] {
			entry, ok := c.roots[root]
			if !ok {
				continue
			}
			entry.valid = false
			queue = append(queue, root)
		}
	}
	c.mu.Unlock()
}

func canonicalizePath(path string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" || clean == "." {
		return "", ErrEmptyWorkspacePath
	}
	abs, err := filepath.Abs(clean)
	if err == nil {
		clean = abs
	}
	resolved, err := filepath.EvalSymlinks(clean)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	return filepath.Clean(clean), nil
}

func isExpired(existing *documentCacheEntry, info os.FileInfo) bool {
	if existing == nil || existing.loadErr != nil {
		return true
	}
	return existing.size != info.Size() || !existing.mtime.Equal(info.ModTime())
}

func sortDiagnostics(diags []ir.Diagnostic) {
	sort.Slice(diags, func(i, j int) bool {
		a := diags[i]
		b := diags[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Start.Line != b.Start.Line {
			return a.Start.Line < b.Start.Line
		}
		if a.Start.Column != b.Start.Column {
			return a.Start.Column < b.Start.Column
		}
		if a.End.Line != b.End.Line {
			return a.End.Line < b.End.Line
		}
		if a.End.Column != b.End.Column {
			return a.End.Column < b.End.Column
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Severity != b.Severity {
			return a.Severity < b.Severity
		}
		return a.RelatedSymbol < b.RelatedSymbol
	})
}

type rootCacheEntry struct {
	result           ServiceResult
	valid            bool
	dependencies     map[string]struct{}
	dependencyHashes map[string]string
}

type documentCacheEntry struct {
	path     string
	hash     string
	size     int64
	mtime    time.Time
	loadErr  error
	snapshot *document.Snapshot
}

func cloneServiceResult(in ServiceResult) ServiceResult {
	return ServiceResult{
		Workspace:   cloneWorkspaceResult(in.Workspace),
		Semantics:   cloneResolveResult(in.Semantics),
		Diagnostics: cloneDiagnostics(in.Diagnostics),
	}
}

func cloneWorkspaceResult(in workspace.LoadResult) workspace.LoadResult {
	docs := make([]ir.Document, 0, len(in.Documents))
	for _, doc := range in.Documents {
		copyDoc := doc
		copyDoc.Sections = make([]ir.Section, 0, len(doc.Sections))
		for _, section := range doc.Sections {
			copyDoc.Sections = append(copyDoc.Sections, ir.Section{
				Header: section.Header,
				Kind:   section.Kind,
				Span:   section.Span,
				Lines:  append([]ir.SourceLine(nil), section.Lines...),
			})
		}
		copyDoc.Symbols = append([]ir.Symbol(nil), doc.Symbols...)
		copyDoc.References = append([]ir.Reference(nil), doc.References...)
		copyDoc.Diagnostics = cloneDiagnostics(doc.Diagnostics)
		docs = append(docs, copyDoc)
	}
	return workspace.LoadResult{
		Documents:   docs,
		Diagnostics: cloneDiagnostics(in.Diagnostics),
	}
}

func cloneResolveResult(in semantics.ResolveResult) semantics.ResolveResult {
	out := semantics.ResolveResult{
		Diagnostics: cloneDiagnostics(in.Diagnostics),
		References:  append([]semantics.ReferenceResolution(nil), in.References...),
		Index:       make([]semantics.SymbolIndexEntry, 0, len(in.Index)),
	}
	for _, entry := range in.Index {
		items := append([]semantics.IndexedSymbol(nil), entry.Symbols...)
		out.Index = append(out.Index, semantics.SymbolIndexEntry{Name: entry.Name, Symbols: items})
	}
	sortDiagnostics(out.Diagnostics)
	return out
}

func cloneDiagnostics(in []ir.Diagnostic) []ir.Diagnostic {
	return append([]ir.Diagnostic(nil), in...)
}
