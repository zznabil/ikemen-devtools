package workspace

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type PathAuthority struct {
	root        string
	external    []string
	insensitive bool
}
type AuthorizedPath struct {
	Canonical string
	Relative  string
	External  bool
}
type PathError struct {
	Code, Path string
	Err        error
}

func (e *PathError) Error() string { return fmt.Sprintf("path.%s: %s: %v", e.Code, e.Path, e.Err) }
func (e *PathError) Unwrap() error { return e.Err }

func NewPathAuthority(root string, externalRoots []string) (*PathAuthority, error) {
	canonical, err := canonicalPath(root)
	if err != nil {
		return nil, &PathError{Code: "invalid-root", Path: root, Err: err}
	}
	if st, err := os.Stat(canonical); err != nil || !st.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		return nil, &PathError{Code: "invalid-root", Path: root, Err: err}
	}
	a := &PathAuthority{root: canonical, insensitive: runtime.GOOS == "windows"}
	for _, raw := range externalRoots {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		p, err := canonicalPath(raw)
		if err != nil {
			return nil, &PathError{Code: "invalid-external-root", Path: raw, Err: err}
		}
		if st, e := os.Stat(p); e != nil || !st.IsDir() {
			if e == nil {
				e = errors.New("not a directory")
			}
			return nil, &PathError{Code: "invalid-external-root", Path: raw, Err: e}
		}
		a.external = append(a.external, p)
	}
	return a, nil
}

func (a *PathAuthority) Root() string {
	if a == nil {
		return ""
	}
	return a.root
}
func (a *PathAuthority) Resolve(raw string) (AuthorizedPath, error) {
	if a == nil {
		return AuthorizedPath{}, &PathError{Code: "invalid-authority", Err: errors.New("nil authority")}
	}
	value, err := decodePath(raw)
	if err != nil {
		return AuthorizedPath{}, err
	}
	if value == "" {
		return AuthorizedPath{}, &PathError{Code: "empty", Path: raw, Err: errors.New("path is empty")}
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(a.root, value)
	}
	canonical, err := canonicalPath(value)
	if err != nil {
		return AuthorizedPath{}, &PathError{Code: "invalid", Path: raw, Err: err}
	}
	base := a.root
	external := false
	if !within(canonical, base, a.insensitive) {
		for _, ext := range a.external {
			if within(canonical, ext, a.insensitive) {
				base = ext
				external = true
				break
			}
		}
	}
	if !within(canonical, base, a.insensitive) {
		return AuthorizedPath{}, &PathError{Code: "outside-root", Path: raw, Err: errors.New("path escapes authorized roots")}
	}
	rel, err := filepath.Rel(base, canonical)
	if err != nil {
		return AuthorizedPath{}, err
	}
	if rel == "." {
		rel = ""
	}
	return AuthorizedPath{Canonical: canonical, Relative: filepath.ToSlash(rel), External: external}, nil
}
func (a *PathAuthority) WorkspaceURI(raw string) (string, error) {
	p, err := a.Resolve(raw)
	if err != nil {
		return "", err
	}
	if p.External {
		return "external:/" + url.PathEscape(p.Relative), nil
	}
	return "workspace:/" + p.Relative, nil
}

func decodePath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(value), "file://") {
		u, err := url.Parse(value)
		if err != nil {
			return "", &PathError{Code: "invalid-uri", Path: raw, Err: err}
		}
		if u.Scheme != "file" {
			return "", &PathError{Code: "invalid-uri", Path: raw, Err: errors.New("unsupported URI scheme")}
		}
		value = u.Path
		if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
			if len(u.Host) == 2 && u.Host[1] == ':' {
				value = u.Host + value
			} else {
				value = `\\` + u.Host + value
			}
		}
		if runtime.GOOS == "windows" && len(value) >= 3 && value[0] == '/' && value[2] == ':' {
			value = value[1:]
		}
		value, err = url.PathUnescape(value)
		if err != nil {
			return "", err
		}
	}
	value = strings.ReplaceAll(value, "\\", string(filepath.Separator))
	return value, nil
}
func canonicalPath(raw string) (string, error) {
	p, err := filepath.Abs(filepath.Clean(raw))
	if err != nil {
		return "", err
	}
	if resolved, e := filepath.EvalSymlinks(p); e == nil {
		return filepath.Clean(resolved), nil
	}
	parts := []string{}
	for {
		if _, e := os.Lstat(p); e == nil {
			break
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		parts = append(parts, filepath.Base(p))
		p = parent
	}
	resolved, e := filepath.EvalSymlinks(p)
	if e != nil {
		return "", e
	}
	for i := len(parts) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, parts[i])
	}
	return filepath.Clean(resolved), nil
}
func within(path, root string, insensitive bool) bool {
	if insensitive {
		path = strings.ToLower(path)
		root = strings.ToLower(root)
	}
	if path == root {
		return true
	}
	return strings.HasPrefix(path, strings.TrimRight(root, string(filepath.Separator))+string(filepath.Separator))
}
