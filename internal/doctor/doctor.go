// Package doctor reports disposable cache health without mutating source files.
package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Report struct {
	Path          string `json:"path"`
	Schema        string `json:"schema,omitempty"`
	Snapshot      string `json:"snapshot,omitempty"`
	Health        string `json:"health"`
	LockOwner     string `json:"lockOwner,omitempty"`
	Size          int64  `json:"size"`
	RebuildReason string `json:"rebuildReason,omitempty"`
}

func Inspect(path string) Report {
	r := Report{Path: filepath.Clean(path), Health: "missing"}
	indexPath := filepath.Join(path, "index.sqlite")
	if st, err := os.Stat(indexPath); err == nil {
		r.Size = st.Size()
		header := make([]byte, 16)
		f, readErr := os.Open(indexPath)
		if readErr == nil {
			_, readErr = f.Read(header)
			_ = f.Close()
		}
		if readErr != nil || string(header) != "SQLite format 3\x00" {
			r.Health = "corrupt"
			r.RebuildReason = "index is not a SQLite database"
		} else {
			r.Health = "healthy"
		}
	} else if !os.IsNotExist(err) {
		r.Health = "unreadable"
		r.RebuildReason = err.Error()
	}
	if data, err := os.ReadFile(filepath.Join(path, "identity.json")); err == nil {
		var v struct {
			Schema   string `json:"schema"`
			Snapshot string `json:"snapshot"`
		}
		if json.Unmarshal(data, &v) == nil {
			r.Schema = v.Schema
			r.Snapshot = v.Snapshot
		}
	}
	if data, err := os.ReadFile(filepath.Join(path, "writer.lock")); err == nil {
		r.LockOwner = string(data)
		if r.Health == "healthy" {
			r.Health = "locked"
		}
	}
	return r
}
func (r Report) JSON() string { b, _ := json.Marshal(r); return string(b) }
func (r Report) Human() string {
	return fmt.Sprintf("cache %s: %s (size=%d, schema=%s, snapshot=%s, lock=%s, reason=%s)", r.Path, r.Health, r.Size, r.Schema, r.Snapshot, r.LockOwner, r.RebuildReason)
}
func Rebuild(path string) error {
	if filepath.Clean(path) == "." || filepath.Clean(path) == string(filepath.Separator) {
		return fmt.Errorf("doctor: refusing unsafe cache path")
	}
	for _, name := range []string{"index.sqlite", "index.sqlite-shm", "index.sqlite-wal", "identity.json"} {
		if err := os.Remove(filepath.Join(path, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func Cleanup(path string) error {
	if filepath.Clean(path) == "." || filepath.Clean(path) == string(filepath.Separator) {
		return fmt.Errorf("doctor: refusing unsafe cache path")
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "index.sqlite.quarantine-") {
			if err := os.Remove(filepath.Join(path, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
