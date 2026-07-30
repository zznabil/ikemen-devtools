package expr

import "testing"

func TestParseArithmeticAndEvaluate(t *testing.T) {
	node, errs := Parse(`1 + 2 * 3 - 4 / 2 + (5 - 1)`)
	if len(errs) != 0 {
		t.Fatalf("unexpected parse errors: %#v", errs)
	}

	res := Evaluate(node, Inputs{})
	if res.Class != EvalFinite {
		t.Fatalf("expected finite result, got %s", res.Class)
	}
	if res.Value.Kind != ValueInt {
		t.Fatalf("expected int result, got %#v", res.Value.Kind)
	}
	if res.Value.Int != 9 {
		t.Fatalf("expected result 9, got %d", res.Value.Int)
	}
}

func TestParseConditionalsAndRuntimeInputs(t *testing.T) {
	node, errs := Parse(`cond(command = "jump" && pos y > 10, var(1) + 1, 0)`)
	if len(errs) != 0 {
		t.Fatalf("unexpected parse errors: %#v", errs)
	}

	cmd := "jump"
	posY := int64(20)
	result := Evaluate(node, Inputs{Command: &cmd, PosY: &posY, Vars: map[int]int64{1: 4}})
	if result.Class != EvalFinite {
		t.Fatalf("expected finite result, got %s", result.Class)
	}
	if result.Value.Kind != ValueInt || result.Value.Int != 5 {
		t.Fatalf("expected result 5 from cond true branch, got %#v", result)
	}

	node2, errs2 := Parse(`cond(command = "jump", 1, 0)`)
	if len(errs2) != 0 {
		t.Fatalf("unexpected parse errors: %#v", errs2)
	}

	result2 := Evaluate(node2, Inputs{Command: strPtr("other")})
	if result2.Class != EvalFinite || result2.Value.Kind != ValueInt || result2.Value.Int != 0 {
		t.Fatalf("expected false branch value 0, got %#v", result2)
	}

	if _, ok := node.(*CondExpr); !ok {
		t.Fatalf("expected root cond node, got %T", node)
	}
}

func TestDynamicAndUnsupportedClassification(t *testing.T) {
	node, errs := Parse(`var(7) + 3`)
	if len(errs) != 0 {
		t.Fatalf("unexpected parse errors: %#v", errs)
	}
	res := Evaluate(node, Inputs{})
	if res.Class != EvalDynamic {
		t.Fatalf("expected dynamic from missing var input, got %s", res.Class)
	}

	node2, errs2 := Parse(`1 / 0`)
	if len(errs2) != 0 {
		t.Fatalf("unexpected parse errors: %#v", errs2)
	}
	res2 := Evaluate(node2, Inputs{})
	if res2.Class != EvalUnsupported {
		t.Fatalf("expected unsupported from division by zero, got %s", res2.Class)
	}

	node3, errs3 := Parse(`unknown(1, 2)`)
	if len(errs3) != 0 {
		t.Fatalf("unexpected parse errors: %#v", errs3)
	}
	res3 := Evaluate(node3, Inputs{})
	if res3.Class != EvalUnsupported {
		t.Fatalf("expected unsupported for unknown function, got %s", res3.Class)
	}
}

func TestMalformedExpressions(t *testing.T) {
	tests := []string{
		`1 +`,
		`cond(1, 2)`,
		`pos`,
	}

	for _, expr := range tests {
		node, errs := Parse(expr)
		if len(errs) == 0 {
			t.Fatalf("expected parse errors for %q, got none", expr)
		}
		if node != nil {
			t.Fatalf("expected nil node for malformed expression %q", expr)
		}
	}
}

func TestASTNodeSpans(t *testing.T) {
	node, errs := Parse(`1 + var(2)`)
	if len(errs) != 0 {
		t.Fatalf("unexpected parse errors: %#v", errs)
	}

	root, ok := node.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected binary root, got %T", node)
	}
	if root.Span().Start.Line != 1 || root.Left.(*IntLiteral).Span().Start.Column != 1 {
		t.Fatalf("unexpected root/left span: %#v", root.Span())
	}

	varExpr, ok := root.Right.(*VarExpr)
	if !ok {
		t.Fatalf("expected var node on rhs, got %T", root.Right)
	}
	if varExpr.Span().Start.Column != 5 {
		t.Fatalf("unexpected var span start, got %#v", varExpr.Span())
	}
}

func strPtr(value string) *string {
	s := value
	return &s
}
