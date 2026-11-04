package dice

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func vars(m map[string]int) func(string) (int, bool) {
	return func(n string) (int, bool) { v, ok := m[n]; return v, ok }
}

func mustRoll(t *testing.T, f string, m map[string]int) Result {
	t.Helper()
	e, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse(%q): %v", f, err)
	}
	r, err := e.Roll(newFakeRNG(t), vars(m))
	if err != nil {
		t.Fatalf("Roll(%q): %v", f, err)
	}
	return r
}

func TestUnknownVariable(t *testing.T) {
	e, err := Parse("1 + @strength")
	if err != nil {
		t.Fatal(err)
	}
	r, err := e.Roll(newFakeRNG(t), vars(nil))
	if !errors.Is(err, ErrUnknownVariable) {
		t.Fatalf("err = %v, want ErrUnknownVariable", err)
	}
	if !strings.Contains(err.Error(), "strength") {
		t.Errorf("message %q does not name variable", err)
	}
	if r.Total != 0 || r.Tree != nil {
		t.Errorf("result not zero: %+v", r)
	}
}

func TestVariableNode(t *testing.T) {
	r := mustRoll(t, "@str[Strength]", map[string]int{"str": 4})
	if r.Total != 4 {
		t.Errorf("total = %d", r.Total)
	}
	v, ok := r.Tree.(VariableNode)
	if !ok {
		t.Fatalf("tree = %#v, want VariableNode", r.Tree)
	}
	if v.Name != "str" || v.Value != 4 || v.Label != "Strength" {
		t.Errorf("node = %+v", v)
	}
	r = mustRoll(t, "@str", map[string]int{"str": -2})
	if v, ok := r.Tree.(VariableNode); !ok || v.Label != "" || v.Value != -2 {
		t.Errorf("tree = %#v", r.Tree)
	}
}

func TestOperatorNodes(t *testing.T) {
	r := mustRoll(t, "@a + 3", map[string]int{"a": 2})
	op, ok := r.Tree.(OperatorNode)
	if !ok {
		t.Fatalf("tree = %#v", r.Tree)
	}
	if op.Op != "+" || op.Value != 5 || len(op.Operands) != 2 {
		t.Fatalf("op = %+v", op)
	}
	if v, ok := op.Operands[0].(VariableNode); !ok || v.Name != "a" || v.Value != 2 {
		t.Errorf("operand0 = %#v", op.Operands[0])
	}
	if l, ok := op.Operands[1].(LiteralNode); !ok || l.Value != 3 {
		t.Errorf("operand1 = %#v", op.Operands[1])
	}

	for f, sym := range map[string]string{"6-2": "-", "6*2": "*", "6/2": "/"} {
		op, ok := mustRoll(t, f, nil).Tree.(OperatorNode)
		if !ok || op.Op != sym || len(op.Operands) != 2 {
			t.Errorf("%s: %#v", f, op)
		}
	}

	r = mustRoll(t, "-@a", map[string]int{"a": 2})
	op, ok = r.Tree.(OperatorNode)
	if !ok || op.Op != "-" || op.Value != -2 || len(op.Operands) != 1 {
		t.Fatalf("unary = %#v", r.Tree)
	}
	if v, ok := op.Operands[0].(VariableNode); !ok || v.Value != 2 {
		t.Errorf("unary operand = %#v", op.Operands[0])
	}
}

func TestFunctions(t *testing.T) {
	cases := []struct {
		f    string
		want int
	}{
		{"min(3,5)", 3}, {"min(5,3)", 3}, {"min(-1,-2)", -2}, {"min(4,4)", 4},
		{"max(3,5)", 5}, {"max(5,3)", 5}, {"max(-1,-2)", -1},
		{"floor(7)", 7}, {"floor(-7)", -7}, {"ceil(7)", 7}, {"ceil(-7/2)", -4},
		{"floor(7/2)", 3}, {"max(min(1,2),0)+1", 2},
	}
	for _, c := range cases {
		if r := mustRoll(t, c.f, nil); r.Total != c.want {
			t.Errorf("%s = %d, want %d", c.f, r.Total, c.want)
		}
	}
}

func TestFunctionNode(t *testing.T) {
	r := mustRoll(t, "max(@a, 7)", map[string]int{"a": 2})
	fn, ok := r.Tree.(FunctionNode)
	if !ok {
		t.Fatalf("tree = %#v", r.Tree)
	}
	if fn.Name != "max" || fn.Value != 7 || len(fn.Args) != 2 {
		t.Fatalf("fn = %+v", fn)
	}
	if v, ok := fn.Args[0].(VariableNode); !ok || v.Value != 2 {
		t.Errorf("arg0 = %#v", fn.Args[0])
	}
	fn, ok = mustRoll(t, "floor(3)", nil).Tree.(FunctionNode)
	if !ok || fn.Name != "floor" || len(fn.Args) != 1 || fn.Value != 3 {
		t.Errorf("floor node = %#v", fn)
	}
}

func TestErrorInsideFunction(t *testing.T) {
	e, _ := Parse("max(1, 1/0)")
	r, err := e.Roll(newFakeRNG(t), vars(nil))
	if !errors.Is(err, ErrDivisionByZero) || r.Tree != nil || r.Total != 0 {
		t.Errorf("got %+v, %v", r, err)
	}
}

func TestRollDoesNotMutateExpr(t *testing.T) {
	e, err := Parse("max(@a, 2) + @b[x] * -3")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := Parse("max(@a, 2) + @b[x] * -3")
	r1, err := e.Roll(newFakeRNG(t), vars(map[string]int{"a": 1, "b": 2}))
	if err != nil {
		t.Fatal(err)
	}
	r2, err := e.Roll(newFakeRNG(t), vars(map[string]int{"a": 10, "b": 5}))
	if err != nil {
		t.Fatal(err)
	}
	if r1.Total != -4 || r2.Total != -5 {
		t.Errorf("totals = %d, %d", r1.Total, r2.Total)
	}
	if !reflect.DeepEqual(e, before) {
		t.Error("Expr changed after Roll")
	}
	r3, _ := e.Roll(newFakeRNG(t), vars(map[string]int{"a": 1, "b": 2}))
	if !reflect.DeepEqual(r1, r3) {
		t.Error("same inputs gave different results")
	}
}

func TestNoPackageLevelRand(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`\brand\.(IntN|N|Int)\b`)
	n := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		n++
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if loc := re.Find(b); loc != nil {
			t.Errorf("%s calls package-level %s", name, loc)
		}
	}
	if n == 0 {
		t.Fatal("no source files scanned")
	}
}
