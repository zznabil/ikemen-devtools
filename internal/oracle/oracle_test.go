package oracle

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRunResult struct {
	run []byte
	err error
}

type fakeRunner struct {
	run chan fakeRunResult
}

func (f fakeRunner) Run(_ context.Context, _ Request) (RunResult, error) {
	result := <-f.run
	return RunResult{Stdout: result.run, Stderr: nil, ExitCode: 0}, result.err
}

func TestCompareCanonicalizesAndMatchesEquivalentJSON(t *testing.T) {
	t.Parallel()

	requestQueue := make(chan fakeRunResult, 1)
	runner := fakeRunner{run: requestQueue}
	requestQueue <- fakeRunResult{
		run: []byte(`{
		"payload": {
			"flags": [1, 2, 3],
			"mode": "release"
		},
		"runtime": {
			"ok": true,
			"build": 7
		}
		}`),
	}

	got, err := Compare(context.Background(), Request{
		Command:    "ikemen-runtime",
		Args:       []string{"--snapshot", "--json"},
		WorkingDir: "C:/games/ikemen",
		Timeout:    0,
	}, []byte(`{"runtime": {"build":7,"ok":true},"payload":{"mode":"release","flags":[1,2,3]}}`), runner)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !got.Match() {
		t.Fatalf("expected matching snapshots")
	}
	if len(got.Mismatches) != 0 {
		t.Fatalf("expected no mismatches, got %d", len(got.Mismatches))
	}
}

func TestCompareEmitsActionableMismatchDiagnostics(t *testing.T) {
	t.Parallel()

	requestQueue := make(chan fakeRunResult, 1)
	runner := fakeRunner{run: requestQueue}
	requestQueue <- fakeRunResult{
		run: []byte(`{
		"settings": {
			"mode": "debug",
			"flags": ["alpha"]
		},
		"extra": "present"
		}`),
	}

	got, err := Compare(context.Background(), Request{Command: "ikemen-runtime"}, []byte(`{
		"settings": {
			"mode": "release",
			"flags": ["alpha", "bravo"]
		},
		"version": 42
		}`), runner)
	if err != nil {
		t.Fatalf("expected no execution error, got %v", err)
	}
	if got.Match() {
		t.Fatalf("expected mismatches")
	}

	paths := map[string]struct{}{}
	for _, diag := range got.Mismatches {
		paths[diag.Path] = struct{}{}
	}
	for _, wantPath := range []string{"$.settings.mode", "$.settings.flags", "$.version", "$.extra"} {
		if _, ok := paths[wantPath]; !ok {
			t.Fatalf("missing diagnostic for path %s: %#v", wantPath, got.Mismatches)
		}
	}
	for _, diag := range got.Mismatches {
		if !strings.Contains(diag.Message, diag.Path) {
			t.Fatalf("expected message to include path, got %q", diag.Message)
		}
	}
}

func TestComparePassesCommandConfigurationToRunner(t *testing.T) {
	t.Parallel()

	captured := make(chan Request, 1)
	runner := RunnerFunc(func(_ context.Context, request Request) (RunResult, error) {
		captured <- request
		return RunResult{Stdout: []byte(`{"ok":true}`)}, nil
	})

	request := Request{
		Command:    "snapshotter",
		Args:       []string{"--capture", "--seed", "123"},
		WorkingDir: "D:/mugen",
		Timeout:    3 * time.Second,
	}
	if _, err := Compare(context.Background(), request, []byte(`{"ok":true}`), runner); err != nil {
		t.Fatalf("expected match without error, got %v", err)
	}

	got := <-captured
	if got.Command != request.Command {
		t.Fatalf("expected command %q, got %q", request.Command, got.Command)
	}
	if got.WorkingDir != request.WorkingDir {
		t.Fatalf("expected working dir %q, got %q", request.WorkingDir, got.WorkingDir)
	}
	if got.Timeout != request.Timeout {
		t.Fatalf("expected timeout passthrough, got %v", got.Timeout)
	}
	if len(got.Args) != len(request.Args) {
		t.Fatalf("expected %d args, got %d", len(request.Args), len(got.Args))
	}
	for i, arg := range got.Args {
		if arg != request.Args[i] {
			t.Fatalf("expected arg %d = %q, got %q", i, request.Args[i], arg)
		}
	}
}

func TestCompareHonorsCallerCancellation(t *testing.T) {
	t.Parallel()

	runner := RunnerFunc(func(ctx context.Context, request Request) (RunResult, error) {
		<-ctx.Done()
		return RunResult{}, ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Compare(ctx, Request{Command: "timeout-check"}, []byte(`{"ok":true}`), runner)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
}

func TestCompareHonorsTimeoutSetting(t *testing.T) {
	t.Parallel()

	runner := RunnerFunc(func(ctx context.Context, request Request) (RunResult, error) {
		<-ctx.Done()
		return RunResult{}, ctx.Err()
	})

	_, err := Compare(context.Background(), Request{Command: "slow", Timeout: 25 * time.Millisecond}, []byte(`{"ok":true}`), runner)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

type RunnerFunc func(context.Context, Request) (RunResult, error)

func (fn RunnerFunc) Run(ctx context.Context, request Request) (RunResult, error) {
	return fn(ctx, request)
}
