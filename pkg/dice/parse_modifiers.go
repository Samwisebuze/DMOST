package dice

import (
	"fmt"
	"strconv"
)

// Modifier kinds stored in diceMod.kind and groupKeep.kind.
//
//	kh, kl  keep highest/lowest N
//	ph, pl  drop highest/lowest N (dh/dl are aliases)
//	rr      reroll while the selector matches (r is an alias)
//	ro      reroll once if the selector matches
//	e       explode on the selector; a nil arg means the maximum face (! is an alias)
//	cs      Roll20-style success counting: count dice matching the selector
const (
	modKeepHigh  = "kh"
	modKeepLow   = "kl"
	modDropHigh  = "ph"
	modDropLow   = "pl"
	modReroll    = "rr"
	modRerollOne = "ro"
	modExplode   = "e"
	modSuccess   = "cs"
)

// keepCountErr reports an invalid keep/drop count.
func keepCountErr(pos int, format string, args ...any) error {
	return fmt.Errorf("%w: %s at position %d", ErrKeepCount, fmt.Sprintf(format, args...), pos)
}

// parseDiceMods parses the modifier chain following a dice term, in written
// order, followed by an optional trailing success-counting comparison.
func (p *exprParser) parseDiceMods(d *diceTerm) error {
	literalCount := -1
	if l, ok := d.count.(*literal); ok {
		literalCount = l.value
	}
	for {
		t, ok := p.peek()
		if !ok {
			return nil
		}
		switch t.kind {
		case tokD:
			return p.syntaxErr(t.pos, "bare drop %q is not supported, use dh or dl", t.text)
		case tokCmp:
			cmp, arg, err := p.parseSelector(true)
			if err != nil {
				return err
			}
			if err := p.addNode(t.pos); err != nil {
				return err
			}
			d.mods = append(d.mods, diceMod{kind: modSuccess, cmp: cmp, arg: arg})
			return nil
		case tokMod:
		default:
			return nil
		}
		p.next()
		var m diceMod
		switch t.text {
		case "k", "p", "dh", "dl":
			kind, count, err := p.parseKeepDrop(t)
			if err != nil {
				return err
			}
			if literalCount >= 0 && count > literalCount {
				return keepCountErr(t.pos, "%s%d exceeds the %d dice rolled", kind, count, literalCount)
			}
			m = diceMod{kind: kind, arg: &literal{value: count}}
		case "rr", "ro", "r":
			cmp, arg, err := p.parseSelector(false)
			if err != nil {
				return err
			}
			m = diceMod{kind: modReroll, cmp: cmp, arg: arg}
			if t.text == "ro" {
				m.kind = modRerollOne
				m.once = true
			}
		case "e", "!":
			m = diceMod{kind: modExplode, cmp: "="}
			if n, ok := p.peek(); ok && n.kind == tokCmp {
				cmp, arg, err := p.parseSelector(true)
				if err != nil {
					return err
				}
				m.cmp, m.arg = cmp, arg
			}
		default:
			return p.syntaxErr(t.pos, "unknown modifier %q", t.text)
		}
		if err := p.addNode(t.pos); err != nil {
			return err
		}
		d.mods = append(d.mods, m)
	}
}

// parseKeepDrop parses the selector of a keep/drop modifier whose operator
// token t has already been consumed. It returns the canonical kind and count.
func (p *exprParser) parseKeepDrop(t lexToken) (string, int, error) {
	var kind string
	switch t.text {
	case "dh":
		kind = modDropHigh
	case "dl":
		kind = modDropLow
	default:
		s, ok := p.peek()
		if !ok {
			return "", 0, p.eof("h or l selector")
		}
		if s.kind != tokMod || (s.text != "h" && s.text != "l") {
			return "", 0, p.syntaxErr(s.pos, "unexpected %q, expected h or l selector", s.text)
		}
		p.next()
		kind = t.text + s.text
	}
	nt, err := p.expect(tokInt, "keep/drop count")
	if err != nil {
		return "", 0, err
	}
	n, err := strconv.Atoi(nt.text)
	if err != nil {
		return "", 0, fmt.Errorf("%w: integer literal %q at position %d", ErrOverflow, nt.text, nt.pos)
	}
	if n <= 0 {
		return "", 0, keepCountErr(nt.pos, "%s count must be positive", kind)
	}
	return kind, n, nil
}

// parseSelector parses a comparison selector: CMP INT, or (unless cmpRequired)
// a bare INT meaning =INT.
func (p *exprParser) parseSelector(cmpRequired bool) (string, node, error) {
	t, ok := p.peek()
	if !ok {
		return "", nil, p.eof("selector")
	}
	cmp := "="
	if t.kind == tokCmp {
		p.next()
		cmp = t.text
	} else if cmpRequired || t.kind != tokInt {
		return "", nil, p.syntaxErr(t.pos, "unexpected %q, expected selector", t.text)
	}
	nt, err := p.expect(tokInt, "selector value")
	if err != nil {
		return "", nil, err
	}
	v, err := strconv.Atoi(nt.text)
	if err != nil {
		return "", nil, fmt.Errorf("%w: integer literal %q at position %d", ErrOverflow, nt.text, nt.pos)
	}
	return cmp, &literal{value: v}, nil
}

// parseGroupKeep applies a keep/drop modifier to a parenthesised expression.
// The operator token is the next token and has not been consumed.
func (p *exprParser) parseGroupKeep(inner node, pos int) (node, error) {
	t := p.next()
	switch t.text {
	case "k", "p", "dh", "dl":
	default:
		return nil, p.syntaxErr(t.pos, "modifier %q is not allowed on a group", t.text)
	}
	if !containsDice(inner) {
		return nil, p.syntaxErr(t.pos, "keep/drop on a group requires dice")
	}
	kind, count, err := p.parseKeepDrop(t)
	if err != nil {
		return nil, err
	}
	items := splitSum(inner, nil)
	if count > len(items) {
		return nil, keepCountErr(t.pos, "%s%d exceeds the %d group items", kind, count, len(items))
	}
	if err := p.addNode(t.pos); err != nil {
		return nil, err
	}
	g := &groupKeep{items: items, kind: kind, count: &literal{value: count}, pos: pos}
	g.label = p.parseLabel()
	return g, nil
}

// splitSum flattens a tree of '+' binaries into its addends.
func splitSum(n node, out []node) []node {
	if b, ok := n.(*binary); ok && b.op == '+' {
		out = splitSum(b.left, out)
		return splitSum(b.right, out)
	}
	return append(out, n)
}

// containsDice reports whether n contains any dice term.
func containsDice(n node) bool {
	switch v := n.(type) {
	case *diceTerm:
		return true
	case *unary:
		return containsDice(v.operand)
	case *binary:
		return containsDice(v.left) || containsDice(v.right)
	case *funcCall:
		for _, a := range v.args {
			if containsDice(a) {
				return true
			}
		}
	case *groupKeep:
		for _, it := range v.items {
			if containsDice(it) {
				return true
			}
		}
	}
	return false
}
