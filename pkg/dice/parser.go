package dice

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse parses formula with the default limits.
//
// Grammar (lowest to highest precedence):
//
//	expr    := term (('+' | '-') term)*
//	term    := unary (('*' | '/') unary)*
//	unary   := ('-' | '+') unary | primary
//	primary := INT | [INT] 'd' INT | [INT] 'd%' | [INT] 'dF'
//	         | '@' name | func '(' args ')' | '(' expr ')'
//
// A dice term or variable may carry a [label] suffix. d% is d100 and dF is a
// Fudge die with faces -1, 0 and +1. The functions are min(a,b), max(a,b),
// floor(x) and ceil(x). Because all values are integers, floor and ceil are
// the identity on integers; they exist for formula compatibility.
func Parse(formula string) (*Expr, error) {
	return ParseWithLimits(formula, DefaultLimits())
}

// ParseWithLimits parses formula, enforcing l at parse time. Zero fields of l
// fall back to their defaults. The effective limits are stored on the Expr for
// use at roll time. On error the returned *Expr is nil.
func ParseWithLimits(formula string, l Limits) (*Expr, error) {
	l = l.withDefaults()
	toks, err := lex(formula)
	if err != nil {
		return nil, err
	}
	p := &exprParser{toks: toks, src: formula, limits: l}
	root, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if t, ok := p.peek(); ok {
		return nil, p.unexpected(t)
	}
	return &Expr{root: root, src: formula, limits: l}, nil
}

type exprParser struct {
	toks   []lexToken
	i      int
	src    string
	limits Limits
	nodes  int
	depth  int
}

func (p *exprParser) peek() (lexToken, bool) {
	if p.i < len(p.toks) {
		return p.toks[p.i], true
	}
	return lexToken{}, false
}

func (p *exprParser) next() lexToken {
	t := p.toks[p.i]
	p.i++
	return t
}

func (p *exprParser) syntaxErr(pos int, format string, args ...any) error {
	return fmt.Errorf("%w: %s at position %d", ErrSyntax, fmt.Sprintf(format, args...), pos)
}

func (p *exprParser) unexpected(t lexToken) error {
	return p.syntaxErr(t.pos, "unexpected %q", t.text)
}

func (p *exprParser) eof(what string) error {
	return p.syntaxErr(len(p.src), "unexpected end of input, expected %s", what)
}

// addNode accounts for one AST node and enforces MaxNodes.
func (p *exprParser) addNode(pos int) error {
	p.nodes++
	if p.nodes > p.limits.MaxNodes {
		return fmt.Errorf("%w: more than %d nodes at position %d", ErrExpressionTooLarge, p.limits.MaxNodes, pos)
	}
	return nil
}

// expect consumes a token of kind k or returns a syntax error.
func (p *exprParser) expect(k tokenKind, what string) (lexToken, error) {
	t, ok := p.peek()
	if !ok {
		return t, p.eof(what)
	}
	if t.kind != k {
		return t, p.syntaxErr(t.pos, "unexpected %q, expected %s", t.text, what)
	}
	return p.next(), nil
}

func (p *exprParser) parseExpr() (node, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > p.limits.MaxNodes {
		pos := len(p.src)
		if t, ok := p.peek(); ok {
			pos = t.pos
		}
		return nil, fmt.Errorf("%w: nesting too deep at position %d", ErrExpressionTooLarge, pos)
	}
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	for {
		t, ok := p.peek()
		if !ok || (t.kind != tokPlus && t.kind != tokMinus) {
			return left, nil
		}
		p.next()
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		if err := p.addNode(t.pos); err != nil {
			return nil, err
		}
		left = &binary{op: t.text[0], left: left, right: right, pos: t.pos}
	}
}

func (p *exprParser) parseTerm() (node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		t, ok := p.peek()
		if !ok || (t.kind != tokStar && t.kind != tokSlash) {
			return left, nil
		}
		p.next()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		if err := p.addNode(t.pos); err != nil {
			return nil, err
		}
		left = &binary{op: t.text[0], left: left, right: right, pos: t.pos}
	}
}

func (p *exprParser) parseUnary() (node, error) {
	t, ok := p.peek()
	if ok && (t.kind == tokMinus || t.kind == tokPlus) {
		p.next()
		p.depth++
		defer func() { p.depth-- }()
		if p.depth > p.limits.MaxNodes {
			return nil, fmt.Errorf("%w: nesting too deep at position %d", ErrExpressionTooLarge, t.pos)
		}
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		if t.kind == tokPlus {
			return operand, nil // unary plus is a no-op
		}
		if err := p.addNode(t.pos); err != nil {
			return nil, err
		}
		return &unary{op: '-', operand: operand, pos: t.pos}, nil
	}
	return p.parsePrimary()
}

func (p *exprParser) parsePrimary() (node, error) {
	t, ok := p.peek()
	if !ok {
		return nil, p.eof("expression")
	}
	switch t.kind {
	case tokInt:
		p.next()
		v, err := strconv.Atoi(t.text)
		if err != nil {
			return nil, fmt.Errorf("%w: integer literal %q at position %d", ErrOverflow, t.text, t.pos)
		}
		if n, ok := p.peek(); ok && (n.kind == tokD || n.kind == tokDPercent || n.kind == tokDF) {
			return p.parseDice(v, t.pos)
		}
		if err := p.addNode(t.pos); err != nil {
			return nil, err
		}
		return &literal{value: v}, nil
	case tokD, tokDPercent, tokDF:
		return p.parseDice(1, t.pos)
	case tokIdent:
		p.next()
		if err := p.addNode(t.pos); err != nil {
			return nil, err
		}
		v := &variable{name: strings.TrimPrefix(t.text, "@"), pos: t.pos}
		v.label = p.parseLabel()
		return v, nil
	case tokFunc:
		return p.parseFunc()
	case tokLParen:
		p.next()
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokRParen, "')'"); err != nil {
			return nil, err
		}
		return inner, nil
	default:
		return nil, p.unexpected(t)
	}
}

// parseLabel consumes an optional [label] token and returns its text.
func (p *exprParser) parseLabel() string {
	if t, ok := p.peek(); ok && t.kind == tokLabel {
		p.next()
		return t.text[1 : len(t.text)-1]
	}
	return ""
}

// parseDice parses the d/d%/dF part of a dice term whose count is already known.
func (p *exprParser) parseDice(count, pos int) (node, error) {
	dt := p.next()
	if count <= 0 {
		return nil, p.syntaxErr(pos, "dice count must be positive")
	}
	if count > p.limits.MaxDice {
		return nil, fmt.Errorf("%w: %d dice exceeds limit %d at position %d", ErrTooManyDice, count, p.limits.MaxDice, pos)
	}
	d := &diceTerm{count: &literal{value: count}, pos: pos}
	sides := 0
	switch dt.kind {
	case tokDPercent:
		sides = 100
	case tokDF:
		d.fudge = true
	default:
		st, err := p.expect(tokInt, "number of sides")
		if err != nil {
			return nil, err
		}
		sides, err = strconv.Atoi(st.text)
		if err != nil {
			return nil, fmt.Errorf("%w: integer literal %q at position %d", ErrOverflow, st.text, st.pos)
		}
		if sides <= 0 {
			return nil, p.syntaxErr(st.pos, "dice must have at least one side")
		}
	}
	if !d.fudge {
		if sides > p.limits.MaxSides {
			return nil, fmt.Errorf("%w: %d sides exceeds limit %d at position %d", ErrTooManySides, sides, p.limits.MaxSides, pos)
		}
		d.sides = &literal{value: sides}
	}
	if err := p.addNode(pos); err != nil {
		return nil, err
	}
	d.label = p.parseLabel()
	return d, nil
}

// funcArity is the number of arguments each function takes.
// floor and ceil are identity on integers (all dice values are integers).
var funcArity = map[string]int{"min": 2, "max": 2, "floor": 1, "ceil": 1}

func (p *exprParser) parseFunc() (node, error) {
	ft := p.next()
	arity := funcArity[ft.text]
	if _, err := p.expect(tokLParen, "'('"); err != nil {
		return nil, err
	}
	if err := p.addNode(ft.pos); err != nil {
		return nil, err
	}
	fc := &funcCall{name: ft.text, pos: ft.pos}
	for i := 0; i < arity; i++ {
		if i > 0 {
			if _, err := p.expect(tokComma, "','"); err != nil {
				return nil, err
			}
		}
		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		fc.args = append(fc.args, arg)
	}
	if _, err := p.expect(tokRParen, "')'"); err != nil {
		return nil, err
	}
	return fc, nil
}
