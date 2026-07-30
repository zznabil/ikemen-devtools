package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

var ErrStaleSnapshot = errors.New("repository: stale snapshot")

func (r *Repository) RequireGeneration(ctx context.Context, path string, generation int64) error {
	if r == nil || r.db == nil {
		return ErrNilDatabase
	}
	var got int64
	err := r.db.QueryRowContext(ctx, "SELECT generation FROM "+documentTable+" WHERE path=?", canonicalizePath(path)).Scan(&got)
	if err == nil && got != generation {
		return ErrStaleSnapshot
	}
	return err
}

// CanonicalSnapshotDigest is restart-stable for the same document hashes.
func CanonicalSnapshotDigest(hashes map[string]string) string {
	keys := make([]string, 0, len(hashes))
	for k := range hashes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte(0)
		b.WriteString(hashes[k])
		b.WriteByte(0)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
