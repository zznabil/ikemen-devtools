// Package contract defines the versioned operation result and CLI error contract.
package contract

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

const SchemaVersion = "0.1.0"

type Status string

const (
	StatusComplete Status = "complete"
	StatusPartial  Status = "partial"
	StatusBlocked  Status = "blocked"
	StatusFailed   Status = "failed"
)

type Workspace struct {
	Root          string `json:"root,omitempty"`
	Profile       string `json:"profile,omitempty"`
	Configuration string `json:"configurationDigest,omitempty"`
}
type Snapshot struct {
	ID string `json:"id,omitempty"`
}
type Page struct {
	Limit      int    `json:"limit"`
	Returned   int    `json:"returned"`
	NextCursor string `json:"nextCursor,omitempty"`
}
type Truncation struct {
	Truncated bool     `json:"truncated"`
	Reasons   []string `json:"reasons"`
}
type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
	Evidence any    `json:"evidence,omitempty"`
}

type Envelope struct {
	SchemaVersion string       `json:"schemaVersion"`
	Operation     string       `json:"operation"`
	Tool          string       `json:"tool"`
	Status        Status       `json:"status"`
	Workspace     Workspace    `json:"workspace"`
	Snapshot      Snapshot     `json:"snapshot"`
	Result        any          `json:"result"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
	Page          Page         `json:"page"`
	Truncation    Truncation   `json:"truncated"`
}

// CanonicalJSON renders stable JSON without a trailing newline. Host paths are
// omitted unless they have already been converted to workspace-relative paths.
func (e Envelope) CanonicalJSON() ([]byte, error) {
	if e.SchemaVersion == "" {
		e.SchemaVersion = SchemaVersion
	}
	if e.Status == "" {
		e.Status = StatusComplete
	}
	if e.Diagnostics == nil {
		e.Diagnostics = []Diagnostic{}
	}
	if e.Truncation.Reasons == nil {
		e.Truncation.Reasons = []string{}
	}
	e.Workspace.Root = relativePath(e.Workspace.Root)
	for i := range e.Diagnostics {
		e.Diagnostics[i].Path = relativePath(e.Diagnostics[i].Path)
	}
	return json.Marshal(e)
}

func relativePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) || (len(path) >= 2 && path[1] == ':') || strings.HasPrefix(path, `\\`) {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(path))
}

type FailureKind string

const (
	FailureFindings FailureKind = "findings"
	FailureUsage    FailureKind = "usage"
	FailureInput    FailureKind = "input"
	FailureInternal FailureKind = "internal"
	FailureBudget   FailureKind = "budget"
	FailureConflict FailureKind = "conflict"
	FailureRuntime  FailureKind = "runtime"
)

type Error struct {
	Kind    FailureKind `json:"kind"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details any         `json:"details,omitempty"`
}

func (e Error) Error() string { return e.Code + ": " + e.Message }

const (
	ExitOK       = 0
	ExitFindings = 1
	ExitUsage    = 2
	ExitInput    = 3
	ExitInternal = 4
	ExitBudget   = 5
	ExitConflict = 6
	ExitRuntime  = 7
)

func ExitCode(kind FailureKind, legacy bool) int {
	if legacy {
		if kind == "" {
			return ExitOK
		}
		return ExitFindings
	}
	switch kind {
	case FailureFindings:
		return ExitFindings
	case FailureUsage:
		return ExitUsage
	case FailureInput:
		return ExitInput
	case FailureInternal:
		return ExitInternal
	case FailureBudget:
		return ExitBudget
	case FailureConflict:
		return ExitConflict
	case FailureRuntime:
		return ExitRuntime
	default:
		return ExitOK
	}
}
