// Package cache manages disposable workspace SQLite caches.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ikemen-engine/ikemen-devtools/internal/repository"
)

type Identity struct {
	Schema   string `json:"schema"`
	Tool     string `json:"tool"`
	IR       string `json:"ir"`
	Profile  string `json:"profile"`
	Config   string `json:"config"`
	Root     string `json:"root"`
	Snapshot string `json:"snapshot"`
}
type Cache struct {
	Dir, DBPath, LockPath string
	dbClose               func() error
	lock                  *os.File
	Ephemeral             bool
}

var ErrLocked = errors.New("cache: writer lock is held")

func Open(ctx context.Context, dir string, identity Identity, ephemeral bool) (*Cache, error) {
	if ephemeral {
		return &Cache{Ephemeral: true}, nil
	}
	dir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(dir, "writer.lock")
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return nil, ErrLocked
		}
		return nil, err
	}
	c := &Cache{Dir: dir, DBPath: filepath.Join(dir, "index.sqlite"), LockPath: lockPath, lock: lock}
	if _, err := lock.WriteString(fmt.Sprintf("pid=%d\ntime=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))); err != nil {
		c.Close()
		return nil, err
	}
	db, err := repository.OpenSQLite(ctx, c.DBPath)
	if err != nil {
		c.quarantine()
		return nil, err
	}
	repo, err := repository.Open(ctx, db)
	if err != nil {
		db.Close()
		c.quarantine()
		return nil, err
	}
	_ = repo
	c.dbClose = db.Close
	identityPath := filepath.Join(dir, "identity.json")
	data, _ := json.Marshal(identity)
	if old, readErr := os.ReadFile(identityPath); readErr == nil && string(old) != string(data) {
		db.Close()
		c.quarantine()
		return Open(ctx, dir, identity, false)
	}
	if err := os.WriteFile(identityPath, data, 0600); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}
func (c *Cache) Close() error {
	if c == nil {
		return nil
	}
	if c.dbClose != nil {
		_ = c.dbClose()
	}
	if c.lock != nil {
		_ = c.lock.Close()
		_ = os.Remove(c.LockPath)
	}
	return nil
}
func (c *Cache) quarantine() {
	if c == nil {
		return
	}
	_ = os.Rename(c.DBPath, c.DBPath+".quarantine-"+time.Now().UTC().Format("20060102T150405.000000000Z"))
	_ = os.Remove(c.LockPath)
}
