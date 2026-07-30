// Package conformance provides black-box protocol checks for the compiled ikm binary.
package conformance

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

type WireResult struct {
	Stdout []byte
	Stderr []byte
	Err    error
}

// Run starts the compiled binary and sends newline-delimited JSON-RPC frames.
// A fresh process per scenario keeps protocol state and failures isolated.
func Run(ctx context.Context, binary string, input []byte) WireResult {
	cmd := exec.CommandContext(ctx, binary, "mcp")
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return WireResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), Err: err}
}

func Frames(raw []byte) ([][]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), 16<<20+1)
	frames := make([][]byte, 0)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			return nil, errors.New("blank stdout frame")
		}
		if !json.Valid(line) {
			return nil, fmt.Errorf("invalid JSON frame: %q", line)
		}
		frames = append(frames, append([]byte(nil), line...))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return frames, nil
}

func Decode(frame []byte) (map[string]json.RawMessage, error) {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(frame, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func AssertResponseShape(frame []byte) error {
	obj, err := Decode(frame)
	if err != nil {
		return err
	}
	if string(obj["jsonrpc"]) != `"2.0"` {
		return errors.New("response missing jsonrpc 2.0")
	}
	_, result := obj["result"]
	_, failure := obj["error"]
	if result == failure {
		return errors.New("response must contain exactly one of result or error")
	}
	if id, ok := obj["id"]; ok && string(id) == "" {
		return errors.New("response id is empty")
	}
	return nil
}

func AssertNoStdoutPollution(result WireResult) error {
	frames, err := Frames(result.Stdout)
	if err != nil {
		return err
	}
	for _, frame := range frames {
		if err := AssertResponseShape(frame); err != nil {
			return err
		}
	}
	return nil
}

func AssertNotificationOnly(result WireResult) error {
	frames, err := Frames(result.Stdout)
	if err != nil {
		return err
	}
	if len(frames) != 0 {
		return fmt.Errorf("notification produced %d response frames", len(frames))
	}
	return nil
}

func AssertBatchIDs(frame []byte, want []json.Number) error {
	var values []map[string]json.RawMessage
	if err := json.Unmarshal(frame, &values); err != nil {
		return err
	}
	if len(values) != len(want) {
		return fmt.Errorf("batch responses = %d, want %d", len(values), len(want))
	}
	for i, value := range values {
		if err := AssertResponseShape(mustJSON(value)); err != nil {
			return err
		}
		var got json.Number
		if err := json.Unmarshal(value["id"], &got); err != nil || got != want[i] {
			return fmt.Errorf("batch response %d id = %q, want %q", i, got, want[i])
		}
	}
	return nil
}

func ValidateObjectSchema(schema json.RawMessage) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(schema, &object); err != nil {
		return err
	}
	if string(object["type"]) != `"object"` {
		return errors.New("schema is not an object schema")
	}
	return nil
}

func mustJSON(value any) []byte { encoded, _ := json.Marshal(value); return encoded }
