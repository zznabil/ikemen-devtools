package oracle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"time"
)

// Request describes how to produce a runtime snapshot for comparison.
type Request struct {
	Command    string
	Args       []string
	WorkingDir string
	Timeout    time.Duration
}

// Runner executes snapshot collection commands.
type Runner interface {
	Run(ctx context.Context, request Request) (RunResult, error)
}

// RunResult carries the raw output from a runtime snapshot command.
type RunResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Mismatch captures an actionable difference between expected and actual runtime snapshot values.
type Mismatch struct {
	Path     string
	Kind     string
	Message  string
	Expected any
	Actual   any
}

// Comparison is the result of a runtime snapshot comparison.
type Comparison struct {
	Request          Request
	ExpectedSnapshot []byte
	ActualSnapshot   []byte
	Mismatches       []Mismatch
	Stdout           []byte
	Stderr           []byte
	ExitCode         int
}

// Match reports whether snapshots are equivalent after canonical JSON normalization.
func (c Comparison) Match() bool {
	return len(c.Mismatches) == 0
}

// DefaultRunner is the production runner using os/exec.
type DefaultRunner struct{}

// Compare executes the request and compares tool output against expectedSnapshot.
func Compare(ctx context.Context, request Request, expectedSnapshot []byte, runner Runner) (Comparison, error) {
	if runner == nil {
		runner = DefaultRunner{}
	}

	expected, err := normalizeJSON(expectedSnapshot)
	if err != nil {
		return Comparison{}, err
	}

	execCtx := ctx
	cancel := func() {}
	if request.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, request.Timeout)
	}
	defer cancel()

	raw, err := runner.Run(execCtx, request)
	if err != nil {
		return Comparison{
			Request:          request,
			ExpectedSnapshot: mustMarshalJSON(expected),
			ActualSnapshot:   raw.Stdout,
			Stdout:           raw.Stdout,
			Stderr:           raw.Stderr,
			ExitCode:         raw.ExitCode,
		}, err
	}

	actual, err := normalizeJSON(raw.Stdout)
	if err != nil {
		return Comparison{
			Request:          request,
			ExpectedSnapshot: mustMarshalJSON(expected),
			ActualSnapshot:   raw.Stdout,
			Stdout:           raw.Stdout,
			Stderr:           raw.Stderr,
			ExitCode:         raw.ExitCode,
		}, err
	}

	comparison := Comparison{
		Request:          request,
		ExpectedSnapshot: mustMarshalJSON(expected),
		ActualSnapshot:   mustMarshalJSON(actual),
		Stdout:           raw.Stdout,
		Stderr:           raw.Stderr,
		ExitCode:         raw.ExitCode,
	}

	comparison.Mismatches = diffJSON(expected, actual)
	return comparison, nil
}

func (r DefaultRunner) Run(ctx context.Context, request Request) (RunResult, error) {
	cmd := exec.CommandContext(ctx, request.Command, request.Args...)
	if request.WorkingDir != "" {
		cmd.Dir = request.WorkingDir
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = resolveExitCode(err)
		return RunResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: exitCode}, &CommandError{
			Command:    request.Command,
			Args:       request.Args,
			WorkingDir: request.WorkingDir,
			ExitCode:   exitCode,
			Stdout:     stdout.Bytes(),
			Stderr:     stderr.Bytes(),
			Err:        err,
		}
	}
	return RunResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: exitCode}, nil
}

// CommandError describes failures from a snapshot command run by DefaultRunner.
type CommandError struct {
	Command    string
	Args       []string
	WorkingDir string
	ExitCode   int
	Stdout     []byte
	Stderr     []byte
	Err        error
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("snapshot command failed: command=%q args=%v exit=%d: %v", e.Command, e.Args, e.ExitCode, e.Err)
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

func resolveExitCode(err error) int {
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if exitError.ProcessState != nil {
			return exitError.ProcessState.ExitCode()
		}
		return 1
	}
	return -1
}

func mustMarshalJSON(v any) []byte {
	payload, _ := json.Marshal(v)
	return payload
}

func normalizeJSON(payload []byte) (any, error) {
	var decoded any
	d := json.NewDecoder(bytes.NewReader(payload))
	d.UseNumber()
	if err := d.Decode(&decoded); err != nil {
		return nil, err
	}
	return normalizeValue(decoded), nil
}

func normalizeValue(v any) any {
	switch node := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(node))
		for key, value := range node {
			out[key] = normalizeValue(value)
		}
		return out
	case []any:
		out := make([]any, len(node))
		for i, value := range node {
			out[i] = normalizeValue(value)
		}
		return out
	case json.Number:
		if asInt, intErr := node.Int64(); intErr == nil {
			return asInt
		}
		if asFloat, floatErr := node.Float64(); floatErr == nil {
			return asFloat
		}
		return string(node)
	default:
		return v
	}
}

func diffJSON(expected, actual any) []Mismatch {
	var mismatches []Mismatch
	diffJSONValues("$", expected, actual, &mismatches)
	return mismatches
}

func diffJSONValues(path string, expected, actual any, out *[]Mismatch) {
	switch lhs := expected.(type) {
	case map[string]any:
		rhs, ok := actual.(map[string]any)
		if !ok {
			*out = append(*out, Mismatch{
				Path:     path,
				Kind:     "type",
				Message:  formatMismatchMessage(path, "type", lhs, actual),
				Expected: lhs,
				Actual:   actual,
			})
			return
		}
		expectedKeys := make([]string, 0, len(lhs))
		for key := range lhs {
			expectedKeys = append(expectedKeys, key)
		}
		sort.Strings(expectedKeys)

		seen := make(map[string]struct{}, len(rhs))
		for key := range rhs {
			seen[key] = struct{}{}
		}

		for _, key := range expectedKeys {
			expectedValue := lhs[key]
			actualValue, ok := rhs[key]
			delete(seen, key)
			if !ok {
				*out = append(*out, Mismatch{
					Path:     jsonPointer(path, key),
					Kind:     "missing",
					Message:  formatMismatchMessage(jsonPointer(path, key), "missing", nil, expectedValue),
					Expected: expectedValue,
					Actual:   nil,
				})
				continue
			}
			diffJSONValues(jsonPointer(path, key), expectedValue, actualValue, out)
		}

		remaining := make([]string, 0, len(seen))
		for key := range seen {
			remaining = append(remaining, key)
		}
		sort.Strings(remaining)
		for _, key := range remaining {
			*out = append(*out, Mismatch{
				Path:     jsonPointer(path, key),
				Kind:     "unexpected",
				Message:  formatMismatchMessage(jsonPointer(path, key), "unexpected", rhs[key], nil),
				Expected: nil,
				Actual:   rhs[key],
			})
		}

	case []any:
		rhs, ok := actual.([]any)
		if !ok {
			*out = append(*out, Mismatch{
				Path:     path,
				Kind:     "type",
				Message:  formatMismatchMessage(path, "type", lhs, actual),
				Expected: lhs,
				Actual:   actual,
			})
			return
		}
		if len(lhs) != len(rhs) {
			*out = append(*out, Mismatch{
				Path:     path,
				Kind:     "length",
				Message:  formatMismatchMessage(path, "length", len(lhs), len(rhs)),
				Expected: len(lhs),
				Actual:   len(rhs),
			})
		}
		common := len(lhs)
		if len(rhs) < common {
			common = len(rhs)
		}
		for index := 0; index < common; index++ {
			diffJSONValues(fmt.Sprintf("%s[%d]", path, index), lhs[index], rhs[index], out)
		}
		if len(lhs) > len(rhs) {
			for index := common; index < len(lhs); index++ {
				*out = append(*out, Mismatch{
					Path:     fmt.Sprintf("%s[%d]", path, index),
					Kind:     "missing",
					Message:  formatMismatchMessage(fmt.Sprintf("%s[%d]", path, index), "missing", lhs[index], nil),
					Expected: lhs[index],
					Actual:   nil,
				})
			}
		} else if len(rhs) > len(lhs) {
			for index := common; index < len(rhs); index++ {
				*out = append(*out, Mismatch{
					Path:     fmt.Sprintf("%s[%d]", path, index),
					Kind:     "unexpected",
					Message:  formatMismatchMessage(fmt.Sprintf("%s[%d]", path, index), "unexpected", nil, rhs[index]),
					Expected: nil,
					Actual:   rhs[index],
				})
			}
		}

	default:
		if !reflect.DeepEqual(expected, actual) {
			*out = append(*out, Mismatch{
				Path:     path,
				Kind:     "value",
				Message:  formatMismatchMessage(path, "value", expected, actual),
				Expected: expected,
				Actual:   actual,
			})
		}
	}
}

func formatMismatchMessage(path, kind string, expected, actual any) string {
	if kind == "missing" {
		return fmt.Sprintf("%s: expected key exists but was missing in actual snapshot", path)
	}
	if kind == "unexpected" {
		return fmt.Sprintf("%s: unexpected key in actual snapshot", path)
	}
	if kind == "length" {
		return fmt.Sprintf("%s: lengths differ; expected=%v actual=%v", path, expected, actual)
	}
	if kind == "type" {
		return fmt.Sprintf("%s: type mismatch; expected=%T actual=%T", path, expected, actual)
	}
	return fmt.Sprintf("%s: value mismatch; expected=%v actual=%v", path, expected, actual)
}

func jsonPointer(base, key string) string {
	escaped := strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
	if base == "$" {
		return "$." + escaped
	}
	return base + "." + escaped
}
