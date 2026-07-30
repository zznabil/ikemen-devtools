package oracle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Policy struct {
	Enabled              bool
	AllowedExecutables   []string
	MaxStdout, MaxStderr int
	Timeout              time.Duration
	DisposableRoot       string
}
type Outcome struct {
	Kind     string `json:"kind"`
	ExitCode int    `json:"exitCode"`
	Stdout   []byte `json:"stdout,omitempty"`
	Stderr   []byte `json:"stderr,omitempty"`
	TimedOut bool   `json:"timedOut"`
}

func RunGuarded(ctx context.Context, p Policy, args []string) (Outcome, error) {
	refused := func(msg string) (Outcome, error) { return Outcome{Kind: "refused"}, errors.New(msg) }
	if !p.Enabled {
		return refused("runtime capability disabled")
	}
	if len(p.AllowedExecutables) == 0 || len(args) == 0 {
		return refused("runtime executable allowlist is empty")
	}
	command, err := filepath.Abs(filepath.Clean(args[0]))
	if err != nil {
		return refused("invalid executable")
	}
	allowed := false
	for _, candidate := range p.AllowedExecutables {
		c, _ := filepath.Abs(filepath.Clean(candidate))
		if strings.EqualFold(c, command) {
			allowed = true
			break
		}
	}
	if !allowed {
		return refused("executable is not allowed")
	}
	root, err := filepath.Abs(filepath.Clean(p.DisposableRoot))
	if err != nil || root == "" {
		return refused("disposable root is required")
	}
	if st, e := os.Stat(root); e != nil || !st.IsDir() {
		return refused("invalid disposable root")
	}
	runCtx := ctx
	cancel := func() {}
	if p.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, p.Timeout)
	}
	defer cancel()
	raw, e := (DefaultRunner{}).Run(runCtx, Request{Command: command, Args: args[1:], WorkingDir: root, Timeout: p.Timeout})
	out := Outcome{Kind: "complete", ExitCode: raw.ExitCode, Stdout: bound(raw.Stdout, p.MaxStdout), Stderr: bound(raw.Stderr, p.MaxStderr)}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		out.Kind = "timed_out"
		out.TimedOut = true
	} else if e != nil {
		out.Kind = "failed"
	}
	return out, e
}
func bound(data []byte, limit int) []byte {
	if limit <= 0 || len(data) <= limit {
		return append([]byte(nil), data...)
	}
	return append([]byte(nil), data[:limit]...)
}
