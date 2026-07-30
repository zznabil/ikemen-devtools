package trace

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
)

func TestParseJSONLDeterministicAndMalformed(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"step","seq":2,"path":"states/a.cns","identity":{"semanticKey":"state:idle"}}`,
		`not-json`,
		`{"type":"step","seq":1,"path":"states/b.cns"}`,
	}, "\n")
	got, err := ParseJSONL(strings.NewReader(input), ParseLimits{MaxLineBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != 2 || got.Events[0].Sequence != 2 || got.Events[1].Sequence != 1 {
		t.Fatalf("events=%+v", got.Events)
	}
	if len(got.Errors) != 1 || got.Errors[0].Line != 2 {
		t.Fatalf("errors=%+v", got.Errors)
	}
}

func TestParseJSONLBounds(t *testing.T) {
	got, err := ParseJSONL(strings.NewReader(`{"type":"ok","payload":"123456789"}`), ParseLimits{MaxLineBytes: 16})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != 0 || len(got.Errors) != 1 || got.Errors[0].Code != "line-too-large" {
		t.Fatalf("result=%+v", got)
	}
}

func TestBridgeCapturesBoundsAndTimestamps(t *testing.T) {
	fake := fakeRunner{result: RawResult{Stdout: []byte(`{"type":"ok"}\n`), Stderr: []byte("0123456789"), ExitCode: 3}}
	b := NewBridge(fake, func() time.Time { return time.Unix(10, 0) })
	result, err := b.Run(context.Background(), Config{Command: "fake-runtime", MaxStdoutBytes: 4, MaxStderrBytes: 5})
	if err == nil {
		t.Fatal("expected non-zero exit error")
	}
	if string(result.Stdout) != `{"ty` || string(result.Stderr) != "01234" || result.ExitCode != 3 {
		t.Fatalf("result=%+v", result)
	}
	if !result.StartedAt.Equal(time.Unix(10, 0)) || !result.FinishedAt.Equal(time.Unix(10, 0)) {
		t.Fatalf("timestamps=%+v", result)
	}
}

func TestBridgeCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := fakeRunner{err: context.Canceled}
	b := NewBridge(fake, time.Now)
	_, err := b.Run(ctx, Config{Command: "fake-runtime"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestCorrelateEventsToWorkspaceDocuments(t *testing.T) {
	doc := ir.NewDocument("/workspace/states/a.cns", "cns")
	doc.Symbols = []ir.Symbol{{ID: "idle", Identity: ir.Identity{SemanticKey: "state:idle"}, Name: "idle"}}
	result := Correlate([]Event{{Path: "states/a.cns", Identity: ir.Identity{SemanticKey: "state:idle"}}}, []ir.Document{doc}, "/workspace")
	if len(result) != 1 || !result[0].Matched || result[0].DocumentPath != doc.Path {
		t.Fatalf("result=%+v", result)
	}
}

type fakeRunner struct {
	result RawResult
	err    error
}

func (f fakeRunner) Run(context.Context, Config) (RawResult, error) { return f.result, f.err }
