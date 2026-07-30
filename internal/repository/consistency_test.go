package repository

import "testing"

func TestTransitiveDependents(t *testing.T) {
	got := TransitiveDependents([]string{"a"}, []DependencyEdge{{SourcePath: "b", TargetPath: "a"}, {SourcePath: "c", TargetPath: "b"}})
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("%v", got)
	}
}
func TestCanonicalSnapshotDigestStable(t *testing.T) {
	a := CanonicalSnapshotDigest(map[string]string{"b": "2", "a": "1"})
	b := CanonicalSnapshotDigest(map[string]string{"a": "1", "b": "2"})
	if a != b {
		t.Fatal("digest not stable")
	}
}
