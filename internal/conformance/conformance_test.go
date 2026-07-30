package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCompiledStdioConformance(t *testing.T) {
	binary := buildBinary(t)
	t.Run("modern discovery", func(t *testing.T) {
		input := frame(`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"harness","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}}}`)
		r := Run(context.Background(), binary, input)
		if err := AssertNoStdoutPollution(r); err != nil {
			t.Fatal(err)
		}
		frames, err := Frames(r.Stdout)
		if err != nil || len(frames) != 1 {
			t.Fatalf("discovery frames: %d %v", len(frames), err)
		}
		obj, err := Decode(frames[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := obj["result"]; !ok {
			t.Fatalf("discovery missing result: %s", frames[0])
		}
	})

	for _, version := range []string{"2025-11-25", "2025-06-18", "2024-11-05"} {
		version := version
		t.Run("legacy "+version, func(t *testing.T) {
			r := Run(context.Background(), binary, frame(fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":%q}}`, version)))
			if err := AssertNoStdoutPollution(r); err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("result and error shapes", func(t *testing.T) {
		r := Run(context.Background(), binary, append(frame(`{"jsonrpc":"2.0","id":3,"method":"ping"}`), frame(`{"jsonrpc":"2.0","id":4,"method":"missing"}`)...))
		frames, err := Frames(r.Stdout)
		if err != nil || len(frames) != 2 {
			t.Fatalf("frames: %d %v", len(frames), err)
		}
		for _, f := range frames {
			if err := AssertResponseShape(f); err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("mixed batch correlation", func(t *testing.T) {
		r := Run(context.Background(), binary, frame(`[{"jsonrpc":"2.0","id":10,"method":"ping"},{"jsonrpc":"2.0","method":"ping"},{"jsonrpc":"2.0","id":11,"method":"missing"}]`))
		frames, err := Frames(r.Stdout)
		if err != nil || len(frames) != 1 {
			t.Fatalf("batch frames: %d %v", len(frames), err)
		}
		if err := AssertBatchIDs(frames[0], []json.Number{"10", "11"}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("notification only", func(t *testing.T) {
		r := Run(context.Background(), binary, frame(`{"jsonrpc":"2.0","method":"ping"}`))
		if err := AssertNotificationOnly(r); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("malformed and oversized", func(t *testing.T) {
		r := Run(context.Background(), binary, frame(`{"jsonrpc":`))
		frames, err := Frames(r.Stdout)
		if err != nil || len(frames) != 1 {
			t.Fatalf("parse error frames: %d %v", len(frames), err)
		}
		if err := AssertResponseShape(frames[0]); err != nil {
			t.Fatal(err)
		}
		over := frame("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\",\"params\":\"" + strings.Repeat("x", 16<<20) + "\"}")
		overResult := Run(context.Background(), binary, over)
		if len(overResult.Stdout) != 0 && overResult.Err == nil {
			t.Fatal("oversized frame was accepted")
		}
	})

	t.Run("advertised schemas", func(t *testing.T) {
		r := Run(context.Background(), binary, frame(`{"jsonrpc":"2.0","id":20,"method":"tools/list"}`))
		frames, err := Frames(r.Stdout)
		if err != nil || len(frames) != 1 {
			t.Fatalf("schema frames: %d %v", len(frames), err)
		}
		var response struct {
			Result struct {
				Tools []struct {
					InputSchema  json.RawMessage `json:"inputSchema"`
					OutputSchema json.RawMessage `json:"outputSchema"`
				} `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal(frames[0], &response); err != nil {
			t.Fatal(err)
		}
		if len(response.Result.Tools) == 0 {
			t.Fatal("tools/list advertised no tools")
		}
		for _, tool := range response.Result.Tools {
			if err := ValidateObjectSchema(tool.InputSchema); err != nil {
				t.Fatal(err)
			}
			if len(tool.OutputSchema) > 0 {
				if err := ValidateObjectSchema(tool.OutputSchema); err != nil {
					t.Fatal(err)
				}
			}
		}
	})
	t.Run("invalid request and explicit null id", func(t *testing.T) {
		r := Run(context.Background(), binary, append(frame(`{"jsonrpc":"1.0","id":null,"method":"ping"}`), frame(`{"jsonrpc":"2.0","id":21,"method":"tools/call","params":null}`)...))
		frames, err := Frames(r.Stdout)
		if err != nil || len(frames) != 2 {
			t.Fatalf("failure frames: %d %v", len(frames), err)
		}
		for _, f := range frames {
			if err := AssertResponseShape(f); err != nil {
				t.Fatal(err)
			}
		}
		var first map[string]json.RawMessage
		if err := json.Unmarshal(frames[0], &first); err != nil || string(first["id"]) != "null" {
			t.Fatalf("explicit null id was not preserved: %s", frames[0])
		}
	})
}

func buildBinary(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ikm")
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", path, "./cmd/ikm")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build ikm: %v\n%s", err, output)
	}
	return path
}

func frame(body string) []byte { return append([]byte(body), '\n') }
