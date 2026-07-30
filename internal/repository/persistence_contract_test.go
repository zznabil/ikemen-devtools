package repository

import (
	"context"
	"testing"
)

func TestChangedDocumentsIncludesAddDeleteAndChange(t *testing.T) {
	got := ChangedDocuments(map[string]string{"a": "1", "gone": "x"}, map[string]string{"a": "2", "new": "3"})
	if len(got) != 3 {
		t.Fatalf("changed paths=%v", got)
	}
}

func TestWorkspaceSnapshotRejectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var r *Repository
	if err := r.UpsertWorkspaceSnapshot(ctx, nil); err != ErrNilDatabase {
		t.Fatalf("got %v", err)
	}
}
