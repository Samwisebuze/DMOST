package dice

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// propCase is a corpus formula. When success is non-nil, every top-level dice
// group of the formula uses success counting with that comparison, so a
// group's value is the number of counting dice meeting it rather than their sum.
type propCase struct {
	formula string
	success *propSuccess
}

type propSuccess struct {
	cmp    string
	target int
}

// propCorpus covers every feature: literals, NdM, implicit count, d%, dF,
// arithmetic with precedence, parentheses, unary minus, variables, labels,
// functions, keep/drop (k/p with h/l selectors and dh/dl aliases), rerolls
// (rr/ro/r), explodes (e/!), success counting and grouped keep/drop.
var propCorpus = []propCase{
	{formula: "7"},
	{formula: "1d6"},
	{formula: "d20"},
	{formula: "3d8"},
	{formula: "2d6+3"},
	{formula: "1d20+4d6kh3+2"},
	{formula: "2d6*2-1d4"},
	{formula: "(1d6+2)*3"},
	{formula: "10/(1d4+1)"},
	{formula: "-1d6+10"},
	{formula: "-(2d4)"},
	{formula: "1d8+@str"},
	{formula: "1d20+@dex[dex]"},
	{formula: "2d6[slash]+1d4[fire]"},
	{formula: "1d%"},
	{formula: "2d%"},
	{formula: "4dF"},
	{formula: "1dF[luck]+2"},
	{formula: "min(1d6,1d8)"},
	{formula: "max(1d6,3)+floor(1d4)-ceil(2)"},
	{formula: "4d6kh3"},
	{formula: "4d6kl1"},
	{formula: "4d6ph1"},
	{formula: "4d6pl1"},
	{formula: "4d6dh1"},
	{formula: "4d6dl1"},
	{formula: "2d20kh1"},
	{formula: "2d20kl1"},
	{formula: "1d6rr1"},
	{formula: "2d6rr<=2"},
	{formula: "2d6ro<3"},
	{formula: "1d6ro=1"},
	{formula: "1d6r1"},
	{formula: "1d6r>=5"},
	{formula: "1d6e"},
	{formula: "3d6e+2"},
	{formula: "1d6e>=5"},
	{formula: "1d6!"},
	{formula: "4d6!>4"},
	{formula: "2d6ro1kh1"},
	{formula: "4d6epl1"},
	{formula: "4d6rr1kh3"},
	{formula: "(1d6+1d8)kh1"},
	{formula: "(1d6+1d8)kl1+2"},
	{formula: "(2d6+1d8)dh1"},
	{formula: "(1d6+1d8)pl1*2"},
	{formula: "(1d20+5)kh1"},
	{formula: "(1d6+1d6+1)kh2"},
	{formula: "3d6>=4", success: &propSuccess{">=", 4}},
	{formula: "6d10>8", success: &propSuccess{">", 8}},
	{formula: "4d10kl2>=5", success: &propSuccess{">=", 5}},
	{formula: "5d6=6", success: &propSuccess{"=", 6}},
	{formula: "5d6<3", success: &propSuccess{"<", 3}},
	{formula: "5d6<=2", success: &propSuccess{"<=", 2}},
}

func propResolve(name string) (int, bool) {
	switch name {
	case "str":
		return 3, true
	case "dex":
		return -1, true
	}
	return 0, false
}

func propCompare(v int, cmp string, target int) bool {
	switch cmp {
	case "<":
		return v < target
	case "<=":
		return v <= target
	case "=":
		return v == target
	case ">=":
		return v >= target
	case ">":
		return v > target
	}
	panic("unknown comparison " + cmp)
}

func propFloorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// recomputeTree recomputes a value from the explanation tree alone: dice
// groups contribute the sum of their DieKept and DieExploded dice (or, with
// success counting, the number of those meeting the comparison), combined
// through operator and function nodes. It does not trust any Value field
// except for literals and variables.
func recomputeTree(t *testing.T, n Node, sc *propSuccess) int {
	t.Helper()
	switch x := n.(type) {
	case LiteralNode:
		return x.Value
	case VariableNode:
		return x.Value
	case DiceGroupNode:
		v := 0
		for _, d := range x.Dice {
			if d.State != DieKept && d.State != DieExploded {
				continue
			}
			if sc != nil {
				if propCompare(d.Value, sc.cmp, sc.target) {
					v++
				}
				continue
			}
			v += d.Value
		}
		return v
	case OperatorNode:
		if len(x.Operands) == 1 {
			require.Equal(t, "-", x.Op, "unary operator")
			return -recomputeTree(t, x.Operands[0], sc)
		}
		require.Len(t, x.Operands, 2, "binary operator %q", x.Op)
		a := recomputeTree(t, x.Operands[0], sc)
		b := recomputeTree(t, x.Operands[1], sc)
		switch x.Op {
		case "+":
			return a + b
		case "-":
			return a - b
		case "*":
			return a * b
		case "/":
			require.NotZero(t, b, "division by zero in a successful roll")
			return propFloorDiv(a, b)
		}
		t.Fatalf("unknown operator %q", x.Op)
	case FunctionNode:
		args := make([]int, len(x.Args))
		for i, a := range x.Args {
			args[i] = recomputeTree(t, a, sc)
		}
		switch x.Name {
		case "min":
			return min(args[0], args[1])
		case "max":
			return max(args[0], args[1])
		case "floor", "ceil":
			return args[0]
		}
		t.Fatalf("unknown function %q", x.Name)
	default:
		t.Fatalf("unexpected node type %T", n)
	}
	return 0
}

// checkDieInvariants asserts structural invariants of every dice group.
func checkDieInvariants(t *testing.T, n Node) {
	t.Helper()
	switch x := n.(type) {
	case DiceGroupNode:
		require.NotEmpty(t, x.Dice)
		for i, d := range x.Dice {
			switch d.State {
			case DieKept, DieDropped, DieRerolled, DieExploded:
			default:
				t.Fatalf("die %d has invalid state %d", i, d.State)
			}
			if d.RerolledFrom != -1 {
				require.True(t, d.RerolledFrom >= 0 && d.RerolledFrom < i, "RerolledFrom %d of die %d", d.RerolledFrom, i)
				assert.Equal(t, DieRerolled, x.Dice[d.RerolledFrom].State, "reroll source must be marked rerolled")
			}
			if d.ExplodedFrom != -1 {
				require.True(t, d.ExplodedFrom >= 0 && d.ExplodedFrom < i, "ExplodedFrom %d of die %d", d.ExplodedFrom, i)
			}
		}
	case OperatorNode:
		for _, o := range x.Operands {
			checkDieInvariants(t, o)
		}
	case FunctionNode:
		for _, a := range x.Args {
			checkDieInvariants(t, a)
		}
	}
}

func TestPropertyTotalMatchesTree(t *testing.T) {
	const seeds = 300
	for _, pc := range propCorpus {
		t.Run(pc.formula, func(t *testing.T) {
			e, err := Parse(pc.formula)
			require.NoError(t, err)
			for seed := uint64(1); seed <= seeds; seed++ {
				rng := rand.New(rand.NewPCG(seed, seed))
				res, err := e.Roll(rng, propResolve)
				require.NoError(t, err, "seed %d", seed)
				require.NotNil(t, res.Tree, "seed %d", seed)
				checkDieInvariants(t, res.Tree)
				got := recomputeTree(t, res.Tree, pc.success)
				require.Equal(t, got, res.Total, "seed %d: total disagrees with explanation tree %s", seed, fmt.Sprintf("%#v", res.Tree))
			}
		})
	}
}

// TestPropertyFeatureCoverage makes sure the seeded rolls actually exercise
// every die state, so the property test is not vacuous.
func TestPropertyFeatureCoverage(t *testing.T) {
	seen := map[DieState]bool{}
	var walk func(Node)
	walk = func(n Node) {
		switch x := n.(type) {
		case DiceGroupNode:
			for _, d := range x.Dice {
				seen[d.State] = true
			}
		case OperatorNode:
			for _, o := range x.Operands {
				walk(o)
			}
		case FunctionNode:
			for _, a := range x.Args {
				walk(a)
			}
		}
	}
	for _, pc := range propCorpus {
		e, err := Parse(pc.formula)
		require.NoError(t, err, pc.formula)
		for seed := uint64(1); seed <= 50; seed++ {
			res, err := e.Roll(rand.New(rand.NewPCG(seed, seed)), propResolve)
			require.NoError(t, err, pc.formula)
			walk(res.Tree)
		}
	}
	for _, s := range []DieState{DieKept, DieDropped, DieRerolled, DieExploded} {
		assert.True(t, seen[s], "die state %d never produced by the corpus", s)
	}
}

func TestDeterminismSeeded(t *testing.T) {
	for _, pc := range propCorpus {
		t.Run(pc.formula, func(t *testing.T) {
			e, err := Parse(pc.formula)
			require.NoError(t, err)
			for seed := uint64(1); seed <= 20; seed++ {
				a, errA := e.Roll(rand.New(rand.NewPCG(seed, seed)), propResolve)
				b, errB := e.Roll(rand.New(rand.NewPCG(seed, seed)), propResolve)
				require.NoError(t, errA)
				require.NoError(t, errB)
				assert.Equal(t, a.Total, b.Total, "seed %d", seed)
				assert.Equal(t, a.Tree, b.Tree, "seed %d", seed)
				assert.Equal(t, a, b, "seed %d", seed)
			}
		})
	}
}

func TestDeterminismSeparateParses(t *testing.T) {
	for _, pc := range propCorpus {
		e1, err := Parse(pc.formula)
		require.NoError(t, err)
		e2, err := Parse(pc.formula)
		require.NoError(t, err)
		a, err := e1.Roll(rand.New(rand.NewPCG(42, 42)), propResolve)
		require.NoError(t, err)
		b, err := e2.Roll(rand.New(rand.NewPCG(42, 42)), propResolve)
		require.NoError(t, err)
		assert.Equal(t, a, b, pc.formula)
	}
}

func TestDeterminismSequence(t *testing.T) {
	e, err := Parse("4d6kh3+1d6e+2d6rr1+(1d6+1d8)kh1")
	require.NoError(t, err)
	run := func() []Result {
		rng := rand.New(rand.NewPCG(7, 7))
		out := make([]Result, 0, 50)
		for range 50 {
			r, err := e.Roll(rng, nil)
			require.NoError(t, err)
			out = append(out, r)
		}
		return out
	}
	assert.Equal(t, run(), run())
}

func TestDeterminismScripted(t *testing.T) {
	cases := []struct {
		formula string
		faces   []int
	}{
		{"4d6kh3", []int{2, 5, 3, 6}},
		{"1d6e", []int{6, 6, 2}},
		{"1d6ro<=2", []int{1, 4}},
		{"2d6rr1+1d8[slashing]+@str", []int{1, 1, 3, 4, 7}},
		{"(1d6+1d8)kh1", []int{4, 2}},
		{"4dF", []int{1, 2, 3, 2}},
		{"3d6>=4", []int{4, 3, 6}},
	}
	for _, c := range cases {
		t.Run(c.formula, func(t *testing.T) {
			e, err := Parse(c.formula)
			require.NoError(t, err)
			a, err := e.Roll(newFakeRNG(t, c.faces...), propResolve)
			require.NoError(t, err)
			b, err := e.Roll(newFakeRNG(t, c.faces...), propResolve)
			require.NoError(t, err)
			assert.Equal(t, a.Total, b.Total)
			assert.Equal(t, a.Tree, b.Tree)
		})
	}
}

// TestRollDoesNotAliasTrees checks that each roll's explanation tree is
// independent: mutating one result must not affect another.
func TestRollDoesNotAliasTrees(t *testing.T) {
	e, err := Parse("4d6kh3+(1d6+1d8)kh1")
	require.NoError(t, err)
	a, err := e.Roll(rand.New(rand.NewPCG(3, 3)), nil)
	require.NoError(t, err)
	snapshot, err := e.Roll(rand.New(rand.NewPCG(3, 3)), nil)
	require.NoError(t, err)
	var mutate func(Node)
	mutate = func(n Node) {
		switch x := n.(type) {
		case DiceGroupNode:
			for i := range x.Dice {
				x.Dice[i].Value = -999
			}
		case OperatorNode:
			for _, o := range x.Operands {
				mutate(o)
			}
		}
	}
	mutate(a.Tree)
	b, err := e.Roll(rand.New(rand.NewPCG(3, 3)), nil)
	require.NoError(t, err)
	assert.Equal(t, snapshot, b)
}
