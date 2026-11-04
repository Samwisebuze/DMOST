package dice

import "fmt"

// Roll evaluates the expression. The Expr is never modified; all per-roll
// state lives in the returned Result. On error the Result is the zero value.
func (e *Expr) Roll(rng RNG, resolve func(name string) (int, bool)) (Result, error) {
	n, v, err := evalNode(e.root, rng, resolve)
	if err != nil {
		return Result{}, err
	}
	return Result{Total: v, Tree: n}, nil
}

func evalNode(n node, rng RNG, resolve func(string) (int, bool)) (Node, int, error) {
	switch x := n.(type) {
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
		cn, cv, err := evalNode(x.operand, rng, resolve)
		if err != nil {
			return nil, 0, err
		}
		v, err := negInt(cv)
		if err != nil {
			return nil, 0, err
		}
		return OperatorNode{Op: string(x.op), Operands: []Node{cn}, Value: v}, v, nil
	case *binary:
		ln, lv, err := evalNode(x.left, rng, resolve)
		if err != nil {
			return nil, 0, err
		}
		rn, rv, err := evalNode(x.right, rng, resolve)
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
			an, av, err := evalNode(a, rng, resolve)
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
