package dice

import "fmt"

// Roll evaluates the expression. The Expr is never modified; all per-roll
// state lives in the returned Result. On error the Result is the zero value.
func (e *Expr) Roll(rng RNG, resolve func(name string) (int, bool)) (Result, error) {
	ev := &evaluator{rng: rng, resolve: resolve}
	n, v, err := ev.eval(e.root)
	if err != nil {
		return Result{}, err
	}
	return Result{Total: v, Tree: n}, nil
}

// evaluator holds per-roll state. When replay is non-nil, dice terms take
// their already-rolled (and already marked) dice from it in evaluation order
// instead of rolling, and keep/drop is not re-applied; this recomputes a
// grouped keep/drop sub-expression from its kept dice only.
type evaluator struct {
	rng     RNG
	resolve func(string) (int, bool)
	replay  [][]Die
}

func (ev *evaluator) eval(n node) (Node, int, error) {
	resolve := ev.resolve
	switch x := n.(type) {
	case *diceTerm:
		return ev.rollDice(x)
	case *groupKeep:
		return ev.groupKeep(x)
	case *literal:
		return LiteralNode{Value: x.value}, x.value, nil
	case *variable:
		var v int
		ok := false
		if resolve != nil {
			v, ok = resolve(x.name)
		}
		if !ok {
			return nil, 0, fmt.Errorf("%w: %q", ErrUnknownVariable, x.name)
		}
		return VariableNode{Name: x.name, Label: x.label, Value: v}, v, nil
	case *unary:
		cn, cv, err := ev.eval(x.operand)
		if err != nil {
			return nil, 0, err
		}
		v, err := negInt(cv)
		if err != nil {
			return nil, 0, err
		}
		return OperatorNode{Op: string(x.op), Operands: []Node{cn}, Value: v}, v, nil
	case *binary:
		ln, lv, err := ev.eval(x.left)
		if err != nil {
			return nil, 0, err
		}
		rn, rv, err := ev.eval(x.right)
		if err != nil {
			return nil, 0, err
		}
		var v int
		switch x.op {
		case '+':
			v, err = addInt(lv, rv)
		case '-':
			v, err = subInt(lv, rv)
		case '*':
			v, err = mulInt(lv, rv)
		case '/':
			v, err = divInt(lv, rv)
		default:
			err = fmt.Errorf("dice: unknown operator %q", x.op)
		}
		if err != nil {
			return nil, 0, err
		}
		return OperatorNode{Op: string(x.op), Operands: []Node{ln, rn}, Value: v}, v, nil
	case *funcCall:
		nodes := make([]Node, 0, len(x.args))
		vals := make([]int, 0, len(x.args))
		for _, a := range x.args {
			an, av, err := ev.eval(a)
			if err != nil {
				return nil, 0, err
			}
			nodes = append(nodes, an)
			vals = append(vals, av)
		}
		var v int
		switch x.name {
		case "min":
			v = min(vals[0], vals[1])
		case "max":
			v = max(vals[0], vals[1])
		case "floor", "ceil":
			v = vals[0]
		default:
			return nil, 0, fmt.Errorf("dice: unknown function %q", x.name)
		}
		return FunctionNode{Name: x.name, Args: nodes, Value: v}, v, nil
	}
	return nil, 0, fmt.Errorf("dice: evaluation of %T not implemented", n)
}
