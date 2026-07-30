// Package trace provides a bounded, deterministic bridge for external runtime traces.
package trace

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"

	"strings"
	"time"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
)

const DefaultMaxOutputBytes = 1024 * 1024

// Config explicitly describes the external runtime command to launch.
type Config struct {
	Command        string
	Args           []string
	WorkingDir     string
	MaxStdoutBytes int
	MaxStderrBytes int
}

// RawResult is the process result before JSONL decoding.
type RawResult struct {
	Stdout, Stderr []byte
	ExitCode       int
}

// RuntimeBridge is the read-only consumer seam for trace collection.
type RuntimeBridge interface {
	Run(context.Context, Config) (Result, error)
}

// Runner is the injectable process boundary used by Bridge.
type Runner interface {
	Run(context.Context, Config) (RawResult, error)
}

// Event is an immutable-in-use, context-aware runtime event. Payload and Context
// are decoded values retained for consumers that need event-specific fields.
type Event struct {
	Sequence  int64          `json:"seq,omitempty"`
	Type      string         `json:"type,omitempty"`
	Timestamp string         `json:"timestamp,omitempty"`
	Path      string         `json:"path,omitempty"`
	Identity  ir.Identity    `json:"identity,omitempty"`
	Context   map[string]any `json:"context,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

// WithCorrelation returns a copy with no mutation of the parsed event.
func (e Event) WithCorrelation(c Correlation) CorrelatedEvent {
	return CorrelatedEvent{Event: e, Correlation: c}
}

// CorrelatedEvent ties an event to a workspace document and optional symbol identity.
type CorrelatedEvent struct {
	Event Event
	Correlation
}

type ParseLimits struct{ MaxLineBytes int }
type ParseError struct {
	Line int
	Code string
	Err  error
}

func (e ParseError) Error() string {
	return fmt.Sprintf("trace line %d: %s: %v", e.Line, e.Code, e.Err)
}
func (e ParseError) Unwrap() error { return e.Err }

type ParseResult struct {
	Events []Event
	Errors []ParseError
}

// ParseJSONL parses one JSON object per line, retaining valid lines in source order.
func ParseJSONL(r io.Reader, limits ParseLimits) (ParseResult, error) {
	if r == nil {
		return ParseResult{}, errors.New("nil trace reader")
	}
	out := ParseResult{}
	max := limits.MaxLineBytes
	if max <= 0 {
		max = 1024 * 1024
	}
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 1024), max+1)
	line := 0
	for s.Scan() {
		line++
		if len(s.Bytes()) > max {
			out.Errors = append(out.Errors, ParseError{Line: line, Code: "line-too-large", Err: fmt.Errorf("%d bytes exceeds limit %d", len(s.Bytes()), max)})
			continue
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(s.Bytes(), &raw); err != nil {
			out.Errors = append(out.Errors, ParseError{Line: line, Code: "malformed-json", Err: err})
			continue
		}
		if raw == nil {
			out.Errors = append(out.Errors, ParseError{Line: line, Code: "invalid-event", Err: errors.New("trace line must be a JSON object")})
			continue
		}
		var event Event
		if err := json.Unmarshal(s.Bytes(), &event); err != nil {
			out.Errors = append(out.Errors, ParseError{Line: line, Code: "invalid-event", Err: err})
			continue
		}
		if seq, ok := raw["seq"]; ok {
			if n, err := strconv.ParseInt(strings.Trim(string(seq), `"`), 10, 64); err == nil {
				event.Sequence = n
			}
		}
		out.Events = append(out.Events, event)
	}
	if err := s.Err(); err != nil {
		return out, err
	}
	return out, nil
}

// Result is the complete bounded bridge outcome.
type Result struct {
	Config      Config
	Stdout      []byte
	Stderr      []byte
	ExitCode    int
	StartedAt   time.Time
	FinishedAt  time.Time
	Events      []Event
	ParseErrors []ParseError
}

type Bridge struct {
	runner Runner
	now    func() time.Time
}

func NewBridge(runner Runner, now func() time.Time) *Bridge {
	if runner == nil {
		runner = CommandRunner{}
	}
	if now == nil {
		now = time.Now
	}
	return &Bridge{runner: runner, now: now}
}

// Run launches the configured command through the injected runner and decodes its JSONL stdout.
func (b *Bridge) Run(ctx context.Context, cfg Config) (Result, error) {
	cfg = withDefaults(cfg)
	result := Result{Config: cfg, StartedAt: b.now()}
	if strings.TrimSpace(cfg.Command) == "" {
		result.FinishedAt = b.now()
		return result, errors.New("trace command is required")
	}
	if err := ctx.Err(); err != nil {
		result.FinishedAt = b.now()
		return result, err
	}
	raw, err := b.runner.Run(ctx, cfg)
	result.Stdout = bound(raw.Stdout, cfg.MaxStdoutBytes)
	result.Stderr = bound(raw.Stderr, cfg.MaxStderrBytes)
	result.ExitCode = raw.ExitCode
	result.FinishedAt = b.now()
	parsed, parseErr := ParseJSONL(bytes.NewReader(result.Stdout), ParseLimits{MaxLineBytes: cfg.MaxStdoutBytes})
	result.Events, result.ParseErrors = parsed.Events, parsed.Errors
	if err != nil {
		return result, err
	}
	if parseErr != nil {
		return result, parseErr
	}
	if raw.ExitCode != 0 {
		return result, &ExitError{Code: raw.ExitCode}
	}
	return result, nil
}

type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("trace runtime exited with status %d", e.Code) }
func withDefaults(cfg Config) Config {
	if cfg.MaxStdoutBytes <= 0 {
		cfg.MaxStdoutBytes = DefaultMaxOutputBytes
	}
	if cfg.MaxStderrBytes <= 0 {
		cfg.MaxStderrBytes = DefaultMaxOutputBytes
	}
	return cfg
}

func bound(data []byte, limit int) []byte {
	if limit <= 0 || len(data) <= limit {
		return append([]byte(nil), data...)
	}
	return append([]byte(nil), data[:limit]...)
}

// CommandRunner executes an explicitly configured command without shell interpretation.
type CommandRunner struct{}

func (CommandRunner) Run(ctx context.Context, cfg Config) (RawResult, error) {
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Dir = cfg.WorkingDir
	var stdout, stderr bytes.Buffer
	cfg = withDefaults(cfg)
	cmd.Stdout = &limitedWriter{w: &stdout, n: cfg.MaxStdoutBytes}
	cmd.Stderr = &limitedWriter{w: &stderr, n: cfg.MaxStderrBytes}
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee := new(exec.ExitError); errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return RawResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: code}, err
}

type limitedWriter struct {
	w io.Writer
	n int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.n <= 0 {
		return len(p), nil
	}
	if len(p) > w.n {
		p = p[:w.n]
	}
	n, err := w.w.Write(p)
	w.n -= n
	return len(p), err
}

type Correlation struct {
	Matched      bool
	DocumentPath string
	Identity     ir.Identity
	SymbolID     string
	EdgeID       string
	SourceSpan   ir.SourceSpan
}

// Correlate resolves event paths and identities against workspace documents deterministically.
func Correlate(events []Event, docs []ir.Document, workspaceRoot string) []CorrelatedEvent {
	out := make([]CorrelatedEvent, 0, len(events))
	for _, event := range events {
		c := Correlation{Identity: event.Identity}
		path := canonicalPath(event.Path, workspaceRoot)
		for _, doc := range docs {
			pathMatch := event.Path == "" || path == canonicalPath(doc.Path, "")
			if !pathMatch {
				continue
			}
			for _, sym := range doc.Symbols {
				if identityMatches(event.Identity, sym.Identity) {
					c.DocumentPath, c.Matched, c.SymbolID = doc.Path, true, sym.ID
					break
				}
			}
			if !c.Matched && event.Identity.SemanticKey == "" && event.Identity.OccurrenceID == "" && event.Identity.StoreID == "" {
				c.DocumentPath, c.Matched = doc.Path, true
			}
			if c.Matched {
				break
			}
		}
		out = append(out, event.WithCorrelation(c))
	}
	return out
}
func canonicalPath(path, root string) string {
	path = filepath.Clean(strings.TrimSpace(strings.ReplaceAll(path, "\\", string(filepath.Separator))))
	if root != "" && !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return strings.ToLower(filepath.Clean(path))
}
func identityMatches(a, b ir.Identity) bool {
	if a.SemanticKey == "" && a.OccurrenceID == "" && a.StoreID == "" {
		return true
	}
	return (a.SemanticKey == "" || a.SemanticKey == b.SemanticKey) && (a.OccurrenceID == "" || a.OccurrenceID == b.OccurrenceID) && (a.StoreID == "" || a.StoreID == b.StoreID)
}

// Service is the guarded runtime-trace seam used by CLI frontends.
// It deliberately does not expose process execution policy; callers must provide
// an already-authorized Bridge and disposable Config.
type Service struct{ Bridge *Bridge }

func NewService(bridge *Bridge) *Service {
	if bridge == nil {
		bridge = NewBridge(nil, nil)
	}
	return &Service{Bridge: bridge}
}
func (s *Service) Check(ctx context.Context, cfg Config) (Result, error) {
	if s == nil || s.Bridge == nil {
		return Result{}, errors.New("trace service unavailable")
	}
	return s.Bridge.Run(ctx, cfg)
}
func (s *Service) Explain(result Result) string {
	if result.ExitCode != 0 {
		return fmt.Sprintf("runtime exited with status %d after %d loader events", result.ExitCode, len(result.Events))
	}
	if len(result.ParseErrors) > 0 {
		return fmt.Sprintf("runtime produced %d malformed trace lines", len(result.ParseErrors))
	}
	return fmt.Sprintf("runtime completed with %d loader events", len(result.Events))
}
