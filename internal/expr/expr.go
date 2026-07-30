package expr

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
)

// Node represents a parsed expression node.
type Node interface {
	NodeType() NodeType
	Span() ir.SourceSpan
}

// NodeType identifies a parsed expression node.
type NodeType string

const (
	NodeInt         NodeType = "int"
	NodeString      NodeType = "string"
	NodeUnary       NodeType = "unary"
	NodeBinary      NodeType = "binary"
	NodeVar         NodeType = "var"
	NodeRandom      NodeType = "random"
	NodeCommand     NodeType = "command"
	NodePos         NodeType = "pos"
	NodeStateType   NodeType = "statetype"
	NodeCond        NodeType = "cond"
	NodeUnsupported NodeType = "unsupported"
)

// Operator denotes the parsed operator.
type Operator string

const (
	OpAdd      Operator = "+"
	OpSub      Operator = "-"
	OpMul      Operator = "*"
	OpDiv      Operator = "/"
	OpMod      Operator = "%"
	OpEq       Operator = "=="
	OpNeq      Operator = "!="
	OpAssignEq Operator = "="
	OpLt       Operator = "<"
	OpLte      Operator = "<="
	OpGt       Operator = ">"
	OpGte      Operator = ">="
	OpAnd      Operator = "&&"
	OpOr       Operator = "||"
	OpNot      Operator = "!"
)

// ParseError is a deterministic parser diagnostic.
type ParseError struct {
	Code    string
	Message string
	Span    ir.SourceSpan
}

func (e ParseError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// EvalClass reports whether an expression can be finitely evaluated.
type EvalClass string

const (
	EvalFinite      EvalClass = "finite"
	EvalDynamic     EvalClass = "dynamic"
	EvalUnsupported EvalClass = "unsupported"
)

// ValueKind reports the dynamic runtime type.
type ValueKind string

const (
	ValueInt    ValueKind = "int"
	ValueBool   ValueKind = "bool"
	ValueString ValueKind = "string"
)

// Value is a typed runtime value.
type Value struct {
	Kind   ValueKind
	Int    int64
	Bool   bool
	String string
}

// EvalResult reports classification and value.
type EvalResult struct {
	Value  Value
	Class  EvalClass
	Reason string
}

// Inputs carries optional runtime values for dynamic nodes.
type Inputs struct {
	Vars      map[int]int64
	PosY      *int64
	Command   *string
	StateType *string
	Random    *int64
}

// IntLiteral is an integer token.
type IntLiteral struct {
	Value    int64
	NodeSpan ir.SourceSpan
}

func (n *IntLiteral) NodeType() NodeType  { return NodeInt }
func (n *IntLiteral) Span() ir.SourceSpan { return n.NodeSpan }

// StringLiteral is a quoted string token.
type StringLiteral struct {
	Value    string
	NodeSpan ir.SourceSpan
}

func (n *StringLiteral) NodeType() NodeType  { return NodeString }
func (n *StringLiteral) Span() ir.SourceSpan { return n.NodeSpan }

// UnaryExpr is a unary operator expression.
type UnaryExpr struct {
	Op       Operator
	Operand  Node
	NodeSpan ir.SourceSpan
}

func (n *UnaryExpr) NodeType() NodeType  { return NodeUnary }
func (n *UnaryExpr) Span() ir.SourceSpan { return n.NodeSpan }

// BinaryExpr is a binary operator expression.
type BinaryExpr struct {
	Op       Operator
	Left     Node
	Right    Node
	NodeSpan ir.SourceSpan
}

func (n *BinaryExpr) NodeType() NodeType  { return NodeBinary }
func (n *BinaryExpr) Span() ir.SourceSpan { return n.NodeSpan }

// VarExpr represents var(index).
type VarExpr struct {
	Index    Node
	NodeSpan ir.SourceSpan
}

func (n *VarExpr) NodeType() NodeType  { return NodeVar }
func (n *VarExpr) Span() ir.SourceSpan { return n.NodeSpan }

// RandomExpr represents random.
type RandomExpr struct {
	NodeSpan ir.SourceSpan
}

func (n *RandomExpr) NodeType() NodeType  { return NodeRandom }
func (n *RandomExpr) Span() ir.SourceSpan { return n.NodeSpan }

// CommandExpr represents command.
type CommandExpr struct {
	NodeSpan ir.SourceSpan
}

func (n *CommandExpr) NodeType() NodeType  { return NodeCommand }
func (n *CommandExpr) Span() ir.SourceSpan { return n.NodeSpan }

// PosExpr represents pos y.
type PosExpr struct {
	Axis     string
	NodeSpan ir.SourceSpan
}

func (n *PosExpr) NodeType() NodeType  { return NodePos }
func (n *PosExpr) Span() ir.SourceSpan { return n.NodeSpan }

// StateTypeExpr represents statetype.
type StateTypeExpr struct {
	NodeSpan ir.SourceSpan
}

func (n *StateTypeExpr) NodeType() NodeType  { return NodeStateType }
func (n *StateTypeExpr) Span() ir.SourceSpan { return n.NodeSpan }

// CondExpr is cond(condExpr, trueExpr, falseExpr).
type CondExpr struct {
	Condition Node
	Then      Node
	Else      Node
	NodeSpan  ir.SourceSpan
}

func (n *CondExpr) NodeType() NodeType  { return NodeCond }
func (n *CondExpr) Span() ir.SourceSpan { return n.NodeSpan }

// UnsupportedExpr represents unsupported expression syntax.
type UnsupportedExpr struct {
	Name     string
	Args     []Node
	NodeSpan ir.SourceSpan
}

func (n *UnsupportedExpr) NodeType() NodeType  { return NodeUnsupported }
func (n *UnsupportedExpr) Span() ir.SourceSpan { return n.NodeSpan }

// Parse parses an expression and returns deterministic parse errors.
func Parse(source string) (Node, []ParseError) {
	p := newParser(source)
	node := p.parseOrExpr()
	if len(p.errors) > 0 {
		return nil, p.errors
	}
	if !p.expect(tokEOF, "expected-token", "unexpected trailing input") {
		return nil, p.errors
	}
	return node, nil
}

// Evaluate evaluates an AST with provided runtime inputs.
func Evaluate(node Node, in Inputs) EvalResult {
	switch v := node.(type) {
	case nil:
		return EvalResult{Class: EvalUnsupported, Reason: "nil node"}
	case *IntLiteral:
		return EvalResult{Value: Value{Kind: ValueInt, Int: v.Value}, Class: EvalFinite}
	case *StringLiteral:
		return EvalResult{Value: Value{Kind: ValueString, String: v.Value}, Class: EvalFinite}
	case *UnaryExpr:
		return evalUnary(v.Op, v.Operand, in)
	case *BinaryExpr:
		return evalBinary(v.Op, v.Left, v.Right, in)
	case *VarExpr:
		return evalVar(v.Index, in)
	case *RandomExpr:
		if in.Random == nil {
			return EvalResult{Class: EvalDynamic, Reason: "random requires runtime input"}
		}
		return EvalResult{Value: Value{Kind: ValueInt, Int: *in.Random}, Class: EvalFinite}
	case *CommandExpr:
		if in.Command == nil {
			return EvalResult{Class: EvalDynamic, Reason: "command requires runtime input"}
		}
		return EvalResult{Value: Value{Kind: ValueString, String: *in.Command}, Class: EvalFinite}
	case *PosExpr:
		if v.Axis != "y" {
			return EvalResult{Class: EvalUnsupported, Reason: "unsupported pos axis"}
		}
		if in.PosY == nil {
			return EvalResult{Class: EvalDynamic, Reason: "pos y requires runtime input"}
		}
		return EvalResult{Value: Value{Kind: ValueInt, Int: *in.PosY}, Class: EvalFinite}
	case *StateTypeExpr:
		if in.StateType == nil {
			return EvalResult{Class: EvalDynamic, Reason: "statetype requires runtime input"}
		}
		return EvalResult{Value: Value{Kind: ValueString, String: *in.StateType}, Class: EvalFinite}
	case *CondExpr:
		c := Evaluate(v.Condition, in)
		if c.Class != EvalFinite {
			return c
		}
		cond, ok := toBool(c.Value)
		if !ok {
			return EvalResult{Class: EvalUnsupported, Reason: "cond condition is not boolean-like"}
		}
		if cond {
			return Evaluate(v.Then, in)
		}
		return Evaluate(v.Else, in)
	case *UnsupportedExpr:
		return EvalResult{Class: EvalUnsupported, Reason: "unsupported expression node: " + v.Name}
	default:
		return EvalResult{Class: EvalUnsupported, Reason: "unknown node type"}
	}
}

func evalUnary(op Operator, child Node, in Inputs) EvalResult {
	r := Evaluate(child, in)
	if r.Class != EvalFinite {
		return r
	}
	switch op {
	case OpNot:
		b, ok := toBool(r.Value)
		if !ok {
			return EvalResult{Class: EvalUnsupported, Reason: "'!' expects boolean-like operand"}
		}
		return EvalResult{Value: Value{Kind: ValueBool, Bool: !b}, Class: EvalFinite}
	case OpSub:
		if r.Value.Kind != ValueInt {
			return EvalResult{Class: EvalUnsupported, Reason: "'-' expects integer operand"}
		}
		return EvalResult{Value: Value{Kind: ValueInt, Int: -r.Value.Int}, Class: EvalFinite}
	default:
		return EvalResult{Class: EvalUnsupported, Reason: "unsupported unary operator"}
	}
}

func evalBinary(op Operator, leftNode, rightNode Node, in Inputs) EvalResult {
	switch op {
	case OpAnd, OpOr:
		left := Evaluate(leftNode, in)
		if left.Class == EvalUnsupported {
			return left
		}
		if left.Class == EvalDynamic {
			return left
		}
		leftBool, ok := toBool(left.Value)
		if !ok {
			return EvalResult{Class: EvalUnsupported, Reason: "logical operand must be boolean-like"}
		}
		if op == OpAnd {
			if !leftBool {
				return EvalResult{Value: Value{Kind: ValueBool, Bool: false}, Class: EvalFinite}
			}
			right := Evaluate(rightNode, in)
			if right.Class == EvalUnsupported {
				return right
			}
			if right.Class == EvalDynamic {
				return right
			}
			rightBool, ok := toBool(right.Value)
			if !ok {
				return EvalResult{Class: EvalUnsupported, Reason: "logical rhs must be boolean-like"}
			}
			return EvalResult{Value: Value{Kind: ValueBool, Bool: leftBool && rightBool}, Class: EvalFinite}
		}

		right := Evaluate(rightNode, in)
		if right.Class == EvalUnsupported {
			return right
		}
		if right.Class == EvalDynamic {
			return right
		}
		if leftBool {
			return EvalResult{Value: Value{Kind: ValueBool, Bool: true}, Class: EvalFinite}
		}
		rightBool, ok := toBool(right.Value)
		if !ok {
			return EvalResult{Class: EvalUnsupported, Reason: "logical rhs must be boolean-like"}
		}
		return EvalResult{Value: Value{Kind: ValueBool, Bool: leftBool || rightBool}, Class: EvalFinite}
	case OpEq, OpNeq, OpAssignEq:
		left := Evaluate(leftNode, in)
		right := Evaluate(rightNode, in)
		if left.Class == EvalUnsupported || right.Class == EvalUnsupported {
			return EvalResult{Class: EvalUnsupported, Reason: "unsupported comparison operand"}
		}
		if left.Class == EvalDynamic || right.Class == EvalDynamic {
			return EvalResult{Class: EvalDynamic, Reason: "comparison requires runtime input"}
		}
		if left.Value.Kind != right.Value.Kind {
			return EvalResult{Class: EvalUnsupported, Reason: "mixed types in comparison"}
		}
		var equal bool
		switch left.Value.Kind {
		case ValueInt:
			equal = left.Value.Int == right.Value.Int
		case ValueBool:
			equal = left.Value.Bool == right.Value.Bool
		case ValueString:
			equal = left.Value.String == right.Value.String
		default:
			return EvalResult{Class: EvalUnsupported, Reason: "unsupported comparison kind"}
		}
		if op == OpNeq {
			equal = !equal
		}
		return EvalResult{Value: Value{Kind: ValueBool, Bool: equal}, Class: EvalFinite}
	case OpLt, OpLte, OpGt, OpGte:
		left := Evaluate(leftNode, in)
		right := Evaluate(rightNode, in)
		if left.Class == EvalUnsupported || right.Class == EvalUnsupported {
			return EvalResult{Class: EvalUnsupported, Reason: "unsupported comparison operand"}
		}
		if left.Class == EvalDynamic || right.Class == EvalDynamic {
			return EvalResult{Class: EvalDynamic, Reason: "comparison requires runtime input"}
		}
		if left.Value.Kind != ValueInt || right.Value.Kind != ValueInt {
			return EvalResult{Class: EvalUnsupported, Reason: "comparison requires integers"}
		}
		l, r := left.Value.Int, right.Value.Int
		var out bool
		switch op {
		case OpLt:
			out = l < r
		case OpLte:
			out = l <= r
		case OpGt:
			out = l > r
		case OpGte:
			out = l >= r
		}
		return EvalResult{Value: Value{Kind: ValueBool, Bool: out}, Class: EvalFinite}
	case OpAdd, OpSub, OpMul, OpDiv, OpMod:
		left := Evaluate(leftNode, in)
		right := Evaluate(rightNode, in)
		if left.Class == EvalUnsupported || right.Class == EvalUnsupported {
			return EvalResult{Class: EvalUnsupported, Reason: "unsupported arithmetic operand"}
		}
		if left.Class == EvalDynamic || right.Class == EvalDynamic {
			return EvalResult{Class: EvalDynamic, Reason: "arithmetic requires runtime input"}
		}
		if left.Value.Kind != ValueInt || right.Value.Kind != ValueInt {
			return EvalResult{Class: EvalUnsupported, Reason: "arithmetic expects integers"}
		}
		l, r := left.Value.Int, right.Value.Int
		var out int64
		switch op {
		case OpAdd:
			out = l + r
		case OpSub:
			out = l - r
		case OpMul:
			out = l * r
		case OpDiv:
			if r == 0 {
				return EvalResult{Class: EvalUnsupported, Reason: "division by zero"}
			}
			out = l / r
		case OpMod:
			if r == 0 {
				return EvalResult{Class: EvalUnsupported, Reason: "mod by zero"}
			}
			out = l % r
		}
		return EvalResult{Value: Value{Kind: ValueInt, Int: out}, Class: EvalFinite}
	default:
		return EvalResult{Class: EvalUnsupported, Reason: "unsupported binary operator"}
	}
}

func evalVar(indexExpr Node, in Inputs) EvalResult {
	index := Evaluate(indexExpr, in)
	if index.Class != EvalFinite {
		return index
	}
	if index.Value.Kind != ValueInt {
		return EvalResult{Class: EvalUnsupported, Reason: "var index must be integer"}
	}
	if in.Vars == nil {
		return EvalResult{Class: EvalDynamic, Reason: "vars table not supplied"}
	}
	v, ok := in.Vars[int(index.Value.Int)]
	if !ok {
		return EvalResult{Class: EvalDynamic, Reason: "var index unresolved"}
	}
	return EvalResult{Value: Value{Kind: ValueInt, Int: v}, Class: EvalFinite}
}

func toBool(value Value) (bool, bool) {
	switch value.Kind {
	case ValueBool:
		return value.Bool, true
	case ValueInt:
		return value.Int != 0, true
	default:
		return false, false
	}
}

// ---------------- lexer/parser ----------------

type tokenType int

const (
	tokEOF tokenType = iota
	tokInt
	tokString
	tokIdent
	tokPlus
	tokMinus
	tokStar
	tokSlash
	tokPercent
	tokEqual
	tokEqEq
	tokNotEq
	tokNot
	tokLess
	tokLessEq
	tokGreater
	tokGreaterEq
	tokAndAnd
	tokOrOr
	tokLParen
	tokRParen
	tokComma
)

type token struct {
	kind tokenType
	text string
	span ir.SourceSpan
}

type parser struct {
	source string
	tokens []token
	pos    int
	errors []ParseError
}

func newParser(source string) *parser {
	toks, errs := lex(source)
	return &parser{source: source, tokens: toks, errors: errs}
}

func (p *parser) parseOrExpr() Node {
	left := p.parseAndExpr()
	if left == nil {
		return nil
	}
	for p.match(tokOrOr) {
		op := p.prev().span
		right := p.parseAndExpr()
		if right == nil {
			p.addError("expected-expression", "expected right expression after '||'", op)
			return nil
		}
		left = &BinaryExpr{Op: OpOr, Left: left, Right: right, NodeSpan: spanCover(left.Span(), right.Span())}
	}
	return left
}

func (p *parser) parseAndExpr() Node {
	left := p.parseEqualityExpr()
	if left == nil {
		return nil
	}
	for p.match(tokAndAnd) {
		op := p.prev().span
		right := p.parseEqualityExpr()
		if right == nil {
			p.addError("expected-expression", "expected right expression after '&&'", op)
			return nil
		}
		left = &BinaryExpr{Op: OpAnd, Left: left, Right: right, NodeSpan: spanCover(left.Span(), right.Span())}
	}
	return left
}

func (p *parser) parseEqualityExpr() Node {
	left := p.parseCmpExpr()
	if left == nil {
		return nil
	}
	for {
		switch p.peek().kind {
		case tokEqEq, tokNotEq, tokEqual:
			opTok := p.advance()
			right := p.parseCmpExpr()
			if right == nil {
				p.addError("expected-expression", "expected right expression", p.peek().span)
				return nil
			}
			op := OpEq
			switch opTok.kind {
			case tokNotEq:
				op = OpNeq
			case tokEqual:
				op = OpAssignEq
			}
			left = &BinaryExpr{Op: op, Left: left, Right: right, NodeSpan: spanCover(left.Span(), right.Span())}
		default:
			return left
		}
	}
}

func (p *parser) parseCmpExpr() Node {
	left := p.parseAddExpr()
	if left == nil {
		return nil
	}
	for {
		switch p.peek().kind {
		case tokLess, tokLessEq, tokGreater, tokGreaterEq:
			opTok := p.advance()
			right := p.parseAddExpr()
			if right == nil {
				p.addError("expected-expression", "expected right expression", p.peek().span)
				return nil
			}
			op := OpLt
			switch opTok.kind {
			case tokLessEq:
				op = OpLte
			case tokGreater:
				op = OpGt
			case tokGreaterEq:
				op = OpGte
			}
			left = &BinaryExpr{Op: op, Left: left, Right: right, NodeSpan: spanCover(left.Span(), right.Span())}
		default:
			return left
		}
	}
}

func (p *parser) parseAddExpr() Node {
	left := p.parseMulExpr()
	if left == nil {
		return nil
	}
	for {
		switch p.peek().kind {
		case tokPlus, tokMinus:
			opTok := p.advance()
			right := p.parseMulExpr()
			if right == nil {
				p.addError("expected-expression", "expected right expression", p.peek().span)
				return nil
			}
			op := OpAdd
			if opTok.kind == tokMinus {
				op = OpSub
			}
			left = &BinaryExpr{Op: op, Left: left, Right: right, NodeSpan: spanCover(left.Span(), right.Span())}
		default:
			return left
		}
	}
}

func (p *parser) parseMulExpr() Node {
	left := p.parseUnaryExpr()
	if left == nil {
		return nil
	}
	for {
		switch p.peek().kind {
		case tokStar, tokSlash, tokPercent:
			opTok := p.advance()
			right := p.parseUnaryExpr()
			if right == nil {
				p.addError("expected-expression", "expected right expression", p.peek().span)
				return nil
			}
			op := OpMul
			if opTok.kind == tokSlash {
				op = OpDiv
			}
			if opTok.kind == tokPercent {
				op = OpMod
			}
			left = &BinaryExpr{Op: op, Left: left, Right: right, NodeSpan: spanCover(left.Span(), right.Span())}
		default:
			return left
		}
	}
}

func (p *parser) parseUnaryExpr() Node {
	tok := p.peek()
	switch tok.kind {
	case tokNot:
		p.advance()
		expr := p.parseUnaryExpr()
		if expr == nil {
			p.addError("expected-expression", "expected expression after '!'", p.peek().span)
			return nil
		}
		return &UnaryExpr{Op: OpNot, Operand: expr, NodeSpan: spanCover(tok.span, expr.Span())}
	case tokMinus:
		p.advance()
		expr := p.parseUnaryExpr()
		if expr == nil {
			p.addError("expected-expression", "expected expression after '-'", p.peek().span)
			return nil
		}
		return &UnaryExpr{Op: OpSub, Operand: expr, NodeSpan: spanCover(tok.span, expr.Span())}
	default:
		return p.parsePrimary()
	}
}

func (p *parser) parsePrimary() Node {
	tok := p.peek()
	if tok.kind == tokEOF {
		p.addError("expected-expression", "unexpected end of expression", tok.span)
		return nil
	}

	switch tok.kind {
	case tokInt:
		p.advance()
		v, _ := strconv.ParseInt(tok.text, 10, 64)
		return &IntLiteral{Value: v, NodeSpan: tok.span}
	case tokString:
		p.advance()
		value, err := strconv.Unquote(tok.text)
		if err != nil {
			p.addError("invalid-string", "malformed quoted string", tok.span)
			return nil
		}
		return &StringLiteral{Value: value, NodeSpan: tok.span}
	case tokLParen:
		p.advance()
		expr := p.parseOrExpr()
		if expr == nil {
			return nil
		}
		if !p.expect(tokRParen, "expected-token", "expected ')' after grouped expression") {
			return nil
		}
		// Keep inner expression; parenthesized span is available in caller via this wrapper's context.
		_ = tok
		return expr
	case tokIdent:
		p.advance()
		name := strings.ToLower(tok.text)
		switch name {
		case "var":
			if !p.expect(tokLParen, "expected-token", "expected '(' after var") {
				return nil
			}
			arg := p.parseOrExpr()
			if arg == nil {
				p.addError("expected-expression", "expected var index", p.peek().span)
				return nil
			}
			if !p.expect(tokRParen, "expected-token", "expected ')' after var index") {
				return nil
			}
			close := p.prev().span
			return &VarExpr{Index: arg, NodeSpan: spanCover(tok.span, close)}
		case "random":
			// random may be used as bare identifier or call.
			if p.match(tokLParen) {
				if !p.match(tokRParen) {
					p.addError("random-arity", "random accepts no arguments", p.peek().span)
					// consume until right paren for deterministic recovery
					for p.peek().kind != tokEOF && p.peek().kind != tokRParen {
						p.advance()
					}
					p.expect(tokRParen, "expected-token", "expected ')' after random")
				}
			}
			return &RandomExpr{NodeSpan: tok.span}
		case "command":
			return &CommandExpr{NodeSpan: tok.span}
		case "pos":
			next := p.peek()
			if next.kind != tokIdent {
				p.addError("expected-token", "expected axis after pos", next.span)
				return nil
			}
			p.advance()
			return &PosExpr{Axis: strings.ToLower(next.text), NodeSpan: spanCover(tok.span, next.span)}
		case "statetype":
			return &StateTypeExpr{NodeSpan: tok.span}
		case "cond":
			if !p.expect(tokLParen, "expected-token", "expected '(' after cond") {
				return nil
			}
			condExpr := p.parseOrExpr()
			if condExpr == nil {
				p.addError("expected-expression", "expected cond condition", p.peek().span)
				return nil
			}
			if !p.expect(tokComma, "expected-token", "expected first comma in cond") {
				return nil
			}
			thenExpr := p.parseOrExpr()
			if thenExpr == nil {
				p.addError("expected-expression", "expected cond true branch", p.peek().span)
				return nil
			}
			if !p.expect(tokComma, "expected-token", "expected second comma in cond") {
				return nil
			}
			elseExpr := p.parseOrExpr()
			if elseExpr == nil {
				p.addError("expected-expression", "expected cond false branch", p.peek().span)
				return nil
			}
			if !p.expect(tokRParen, "expected-token", "expected ')' after cond") {
				return nil
			}
			close := p.prev().span
			return &CondExpr{Condition: condExpr, Then: thenExpr, Else: elseExpr, NodeSpan: spanCover(tok.span, close)}
		default:
			if p.match(tokLParen) {
				args := make([]Node, 0)
				if p.peek().kind != tokRParen {
					arg := p.parseOrExpr()
					if arg != nil {
						args = append(args, arg)
					}
					for p.match(tokComma) {
						arg = p.parseOrExpr()
						if arg != nil {
							args = append(args, arg)
						}
					}
				}
				if !p.expect(tokRParen, "expected-token", "expected ')' after function call") {
					close := p.prev().span
					return &UnsupportedExpr{Name: name, Args: args, NodeSpan: spanCover(tok.span, close)}
				}
				close := p.prev().span
				return &UnsupportedExpr{Name: name, Args: args, NodeSpan: spanCover(tok.span, close)}
			}
			return &UnsupportedExpr{Name: name, NodeSpan: tok.span}
		}
	default:
		p.addError("unexpected-token", "unexpected token", tok.span)
		p.advance()
		return nil
	}
}

func (p *parser) parseExpression() Node { return p.parseOrExpr() }

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{kind: tokEOF, span: lastSpan(p.source)}
	}
	return p.tokens[p.pos]
}

func (p *parser) prev() token {
	if p.pos == 0 {
		return token{kind: tokEOF, span: lastSpan(p.source)}
	}
	if p.pos-1 >= len(p.tokens) {
		return token{kind: tokEOF, span: lastSpan(p.source)}
	}
	return p.tokens[p.pos-1]
}

func (p *parser) match(kind tokenType) bool { return p.advanceIf(kind) }

func (p *parser) advance() token {
	current := p.peek()
	p.pos++
	return current
}

func (p *parser) advanceIf(kind tokenType) bool {
	if p.peek().kind == kind {
		p.advance()
		return true
	}
	return false
}

func (p *parser) expect(kind tokenType, code, message string) bool {
	if p.match(kind) {
		return true
	}
	p.addError(code, message, p.peek().span)
	return false
}

func (p *parser) addError(code, message string, span ir.SourceSpan) {
	p.errors = append(p.errors, ParseError{Code: code, Message: message, Span: span})
}

func (p *parser) peekHas(kind tokenType) bool { return p.peek().kind == kind }

func lex(source string) ([]token, []ParseError) {
	var toks []token
	var errs []ParseError
	i := 0
	for i < len(source) {
		ch := source[i]
		if unicode.IsSpace(rune(ch)) {
			i++
			continue
		}

		start := i
		if isDigit(ch) {
			for i < len(source) && isDigit(source[i]) {
				i++
			}
			toks = append(toks, token{kind: tokInt, text: source[start:i], span: spanFrom(source, start, i)})
			continue
		}

		if ch == '"' {
			i++
			for i < len(source) && source[i] != '"' {
				if source[i] == '\\' && i+1 < len(source) {
					i += 2
					continue
				}
				i++
			}
			if i >= len(source) {
				errs = append(errs, ParseError{Code: "unterminated-string", Message: "unterminated string literal", Span: spanFrom(source, start, len(source))})
				break
			}
			i++
			toks = append(toks, token{kind: tokString, text: source[start:i], span: spanFrom(source, start, i)})
			continue
		}

		if isIdentStart(ch) {
			for i < len(source) && isIdentPart(source[i]) {
				i++
			}
			toks = append(toks, token{kind: tokIdent, text: source[start:i], span: spanFrom(source, start, i)})
			continue
		}

		switch ch {
		case '+':
			toks = append(toks, token{kind: tokPlus, text: "+", span: spanFrom(source, i, i+1)})
			i++
		case '-':
			toks = append(toks, token{kind: tokMinus, text: "-", span: spanFrom(source, i, i+1)})
			i++
		case '*':
			toks = append(toks, token{kind: tokStar, text: "*", span: spanFrom(source, i, i+1)})
			i++
		case '/':
			toks = append(toks, token{kind: tokSlash, text: "/", span: spanFrom(source, i, i+1)})
			i++
		case '%':
			toks = append(toks, token{kind: tokPercent, text: "%", span: spanFrom(source, i, i+1)})
			i++
		case '!':
			if i+1 < len(source) && source[i+1] == '=' {
				toks = append(toks, token{kind: tokNotEq, text: "!=", span: spanFrom(source, i, i+2)})
				i += 2
			} else {
				toks = append(toks, token{kind: tokNot, text: "!", span: spanFrom(source, i, i+1)})
				i++
			}
		case '=':
			if i+1 < len(source) && source[i+1] == '=' {
				toks = append(toks, token{kind: tokEqEq, text: "==", span: spanFrom(source, i, i+2)})
				i += 2
			} else {
				toks = append(toks, token{kind: tokEqual, text: "=", span: spanFrom(source, i, i+1)})
				i++
			}
		case '<':
			if i+1 < len(source) && source[i+1] == '=' {
				toks = append(toks, token{kind: tokLessEq, text: "<=", span: spanFrom(source, i, i+2)})
				i += 2
			} else {
				toks = append(toks, token{kind: tokLess, text: "<", span: spanFrom(source, i, i+1)})
				i++
			}
		case '>':
			if i+1 < len(source) && source[i+1] == '=' {
				toks = append(toks, token{kind: tokGreaterEq, text: ">=", span: spanFrom(source, i, i+2)})
				i += 2
			} else {
				toks = append(toks, token{kind: tokGreater, text: ">", span: spanFrom(source, i, i+1)})
				i++
			}
		case '&':
			if i+1 < len(source) && source[i+1] == '&' {
				toks = append(toks, token{kind: tokAndAnd, text: "&&", span: spanFrom(source, i, i+2)})
				i += 2
			} else {
				errs = append(errs, ParseError{Code: "unexpected-char", Message: "unexpected '&', expected &&", Span: spanFrom(source, i, i+1)})
				i++
			}
		case '|':
			if i+1 < len(source) && source[i+1] == '|' {
				toks = append(toks, token{kind: tokOrOr, text: "||", span: spanFrom(source, i, i+2)})
				i += 2
			} else {
				errs = append(errs, ParseError{Code: "unexpected-char", Message: "unexpected '|', expected ||", Span: spanFrom(source, i, i+1)})
				i++
			}
		case '(':
			toks = append(toks, token{kind: tokLParen, text: "(", span: spanFrom(source, i, i+1)})
			i++
		case ')':
			toks = append(toks, token{kind: tokRParen, text: ")", span: spanFrom(source, i, i+1)})
			i++
		case ',':
			toks = append(toks, token{kind: tokComma, text: ",", span: spanFrom(source, i, i+1)})
			i++
		default:
			errs = append(errs, ParseError{Code: "unexpected-char", Message: "unexpected character", Span: spanFrom(source, i, i+1)})
			i++
		}
	}
	return toks, errs
}

func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func isIdentStart(ch byte) bool {
	return ch == '_' || unicode.IsLetter(rune(ch))
}

func isIdentPart(ch byte) bool {
	return ch == '_' || unicode.IsLetter(rune(ch)) || isDigit(ch)
}

func spanFrom(source string, start, end int) ir.SourceSpan {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(source) {
		end = len(source)
	}
	return ir.SourceSpan{Start: positionFor(source, start), End: positionFor(source, end)}
}

func positionFor(source string, index int) ir.SourcePosition {
	if index < 0 {
		index = 0
	}
	if index > len(source) {
		index = len(source)
	}
	line := 1
	col := 1
	for i := 0; i < index; i++ {
		if source[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return ir.SourcePosition{Line: line, Column: col}
}

func lastSpan(source string) ir.SourceSpan {
	end := spanFrom(source, len(source), len(source))
	return end
}

func spanCover(a, b ir.SourceSpan) ir.SourceSpan {
	return ir.SourceSpan{Start: a.Start, End: b.End}
}
