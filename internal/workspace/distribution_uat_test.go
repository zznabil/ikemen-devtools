package workspace

import (
	"os"
	"runtime"
	"testing"
	"time"
)

func TestDistributionUATWhenConfigured(t *testing.T) {
	root := os.Getenv("IKM_DISTRIBUTION_ROOT")
	if root == "" {
		t.Skip("set IKM_DISTRIBUTION_ROOT to exercise the development distribution")
	}
	cfg := DefaultWorkspaceConfig()
	cfg.Root = root
	cfg.Budgets.MaxFiles = 1000000
	cfg.Budgets.MaxBytes = 2 << 30
	started := time.Now()
	d, e := Discover(root, cfg)
	if e != nil {
		t.Fatal(e)
	}
	snap := d.Snapshot()
	t.Logf("distribution snapshot=%s files=%d truncated=%v machine=%s os=%s elapsed=%s", snap.ID, len(d.Files), d.Truncated, runtime.GOARCH, runtime.GOOS, time.Since(started))
	if d.Truncated {
		t.Fatalf("distribution exceeded configured inventory budget: %v", d.Reasons)
	}
}
