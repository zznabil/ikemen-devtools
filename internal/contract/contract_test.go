package contract

import "testing"

func TestCanonicalJSONGoldenAndStable(t *testing.T) {
	e := Envelope{
		Operation: "workspace.scan", Tool: "ikm", Workspace: Workspace{Root: `C:\games\ikemen`, Profile: "distribution", Configuration: "abc"},
		Snapshot: Snapshot{ID: "snap-1"}, Result: map[string]any{"count": 2},
		Diagnostics: []Diagnostic{{Code: "W1", Severity: "warning", Message: "check", Path: `C:\games\ikemen\chars\hero.def`}},
		Page:        Page{Limit: 10, Returned: 1}, Truncation: Truncation{Reasons: []string{}},
	}
	want := `{"schemaVersion":"0.1.0","operation":"workspace.scan","tool":"ikm","status":"complete","workspace":{"profile":"distribution","configurationDigest":"abc"},"snapshot":{"id":"snap-1"},"result":{"count":2},"diagnostics":[{"code":"W1","severity":"warning","message":"check"}],"page":{"limit":10,"returned":1},"truncated":{"truncated":false,"reasons":[]}}`
	got, err := e.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("canonical JSON = %s, want %s", got, want)
	}
	again, err := e.CanonicalJSON()
	if err != nil || string(again) != string(got) {
		t.Fatalf("non-deterministic canonical JSON: %s / %s", got, again)
	}
}

func TestExitCodeMappingsAndLegacy(t *testing.T) {
	cases := []struct {
		kind FailureKind
		want int
	}{
		{FailureFindings, 1}, {FailureUsage, 2}, {FailureInput, 3}, {FailureInternal, 4}, {FailureBudget, 5}, {FailureConflict, 6}, {FailureRuntime, 7}, {FailureKind("unknown"), 0},
	}
	for _, tc := range cases {
		if got := ExitCode(tc.kind, false); got != tc.want {
			t.Errorf("ExitCode(%q) = %d, want %d", tc.kind, got, tc.want)
		}
	}
	if ExitCode(FailureUsage, true) != ExitFindings || ExitCode("", true) != ExitOK {
		t.Fatal("legacy exit mapping changed")
	}
}

func TestErrorIsTyped(t *testing.T) {
	err := Error{Kind: FailureBudget, Code: "budget.exceeded", Message: "limit"}
	if err.Error() != "budget.exceeded: limit" {
		t.Fatalf("unexpected error text: %s", err)
	}
}
