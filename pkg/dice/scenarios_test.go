package dice

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scenarioResolver(m map[string]int) func(string) (int, bool) {
	return func(name string) (int, bool) {
		v, ok := m[name]
		return v, ok
	}
}

func scenarioRoll(t *testing.T, formula string, resolve func(string) (int, bool), faces ...int) Result {
	t.Helper()
	e, err := Parse(formula)
	require.NoError(t, err)
	require.NotNil(t, e)
	rng := newFakeRNG(t, faces...)
	r, err := e.Roll(rng, resolve)
	require.NoError(t, err)
	assert.Equal(t, len(faces), rng.pos, "every scripted face should be consumed")
	return r
}

func asGroup(t *testing.T, n Node) DiceGroupNode {
	t.Helper()
	g, ok := n.(DiceGroupNode)
	require.True(t, ok, "expected DiceGroupNode, got %T", n)
	return g
}

func asBinary(t *testing.T, n Node, op string) OperatorNode {
	t.Helper()
	o, ok := n.(OperatorNode)
	require.True(t, ok, "expected OperatorNode, got %T", n)
	require.Equal(t, op, o.Op)
	require.Len(t, o.Operands, 2)
	return o
}

// S1: Roll 4d6 keep highest 3.
func TestScenarioS1(t *testing.T) {
	r := scenarioRoll(t, "4d6kh3", nil, 2, 5, 3, 6)
	assert.Equal(t, 14, r.Total)
	g := asGroup(t, r.Tree)
	assert.Equal(t, []int{2, 5, 3, 6}, dieValues(g.Dice), "all four dice in roll order")
	assert.Equal(t, []DieState{DieDropped, DieKept, DieKept, DieKept}, dieStates(g.Dice))
	assert.Equal(t, 14, g.Value)
}

// S2: Variable modifier.
func TestScenarioS2(t *testing.T) {
	r := scenarioRoll(t, "1d8+@str", scenarioResolver(map[string]int{"str": 3}), 5)
	assert.Equal(t, 8, r.Total)
	op := asBinary(t, r.Tree, "+")
	g := asGroup(t, op.Operands[0])
	assert.Equal(t, []int{5}, dieValues(g.Dice))
	assert.Equal(t, []DieState{DieKept}, dieStates(g.Dice))
	v, ok := op.Operands[1].(VariableNode)
	require.True(t, ok, "expected VariableNode, got %T", op.Operands[1])
	assert.Equal(t, "str", v.Name)
	assert.Equal(t, 3, v.Value)
}

// S3: Unknown variable.
func TestScenarioS3(t *testing.T) {
	e, err := Parse("1d8+@dex")
	require.NoError(t, err)
	r, err := e.Roll(rand.New(rand.NewPCG(1, 1)), scenarioResolver(map[string]int{"str": 3}))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnknownVariable)
	assert.Contains(t, err.Error(), "dex")
	assert.Equal(t, Result{}, r, "no partial result on error")

	// A nil resolver behaves the same way.
	r, err = e.Roll(rand.New(rand.NewPCG(1, 1)), nil)
	assert.ErrorIs(t, err, ErrUnknownVariable)
	assert.Equal(t, Result{}, r)
}

// S4: Repeated rolls from one parse.
func TestScenarioS4(t *testing.T) {
	e, err := Parse("2d6+3")
	require.NoError(t, err)
	run := func() []Result {
		rng := rand.New(rand.NewPCG(2026, 11))
		out := make([]Result, 0, 100)
		for range 100 {
			r, err := e.Roll(rng, nil)
			require.NoError(t, err)
			require.GreaterOrEqual(t, r.Total, 5)
			require.LessOrEqual(t, r.Total, 15)
			out = append(out, r)
		}
		return out
	}
	first := run()

	// Each explanation tree is independent: mutating one leaves others intact.
	distinct := map[int]bool{}
	for _, r := range first {
		distinct[r.Total] = true
	}
	assert.Greater(t, len(distinct), 1, "100 seeded rolls should not all be equal")
	before := asGroup(t, asBinary(t, first[1].Tree, "+").Operands[0])
	saved := append([]Die(nil), before.Dice...)
	asGroup(t, asBinary(t, first[0].Tree, "+").Operands[0]).Dice[0].Value = 99
	assert.Equal(t, saved, asGroup(t, asBinary(t, first[1].Tree, "+").Operands[0]).Dice)

	// Same seed reproduces the same sequence.
	again := run()
	first[0] = again[0] // undo the mutation above for comparison
	assert.Equal(t, first, again)
}

// S5: Exploding die.
func TestScenarioS5(t *testing.T) {
	r := scenarioRoll(t, "1d6e", nil, 6, 6, 2)
	assert.Equal(t, 14, r.Total)
	g := asGroup(t, r.Tree)
	require.Len(t, g.Dice, 3)
	assert.Equal(t, []int{6, 6, 2}, dieValues(g.Dice))
	assert.Equal(t, DieExploded, g.Dice[0].State)
	assert.Equal(t, DieExploded, g.Dice[1].State)
	assert.True(t, g.Dice[2].State == DieKept || g.Dice[2].State == DieExploded)
	assert.Equal(t, -1, g.Dice[0].ExplodedFrom)
	assert.Equal(t, 0, g.Dice[1].ExplodedFrom)
	assert.Equal(t, 1, g.Dice[2].ExplodedFrom)
}

// S6: Reroll once.
func TestScenarioS6(t *testing.T) {
	r := scenarioRoll(t, "1d6ro<=2", nil, 1, 4)
	assert.Equal(t, 4, r.Total)
	g := asGroup(t, r.Tree)
	require.Len(t, g.Dice, 2)
	assert.Equal(t, []int{1, 4}, dieValues(g.Dice))
	assert.Equal(t, []DieState{DieRerolled, DieKept}, dieStates(g.Dice))
	assert.Equal(t, 0, g.Dice[1].RerolledFrom)
}

// S7: Runaway input rejected promptly, at parse or roll time.
func TestScenarioS7(t *testing.T) {
	cases := []struct {
		formula string
		face    int
	}{
		{"1d1e", 1},
		{"1d6rr<=6", 6},
		{"1000000d1000000", 1},
	}
	for _, c := range cases {
		t.Run(c.formula, func(t *testing.T) {
			done := make(chan error, 1)
			start := time.Now()
			go func() {
				e, err := Parse(c.formula)
				if err != nil {
					assert.Nil(t, e)
					done <- err
					return
				}
				rng := &constRNG{t: t, face: c.face, cap: DefaultMaxDice + DefaultMaxRerolls + DefaultMaxExplosions + 10}
				r, err := e.Roll(rng, nil)
				if err != nil {
					assert.Equal(t, Result{}, r)
				}
				done <- err
			}()
			select {
			case err := <-done:
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrLimitExceeded)
			case <-time.After(5 * time.Second):
				t.Fatalf("%s did not terminate promptly", c.formula)
			}
			assert.Less(t, time.Since(start), 5*time.Second)
		})
	}
}

// S8: Malformed formula.
func TestScenarioS8(t *testing.T) {
	e, err := Parse("(1d6+")
	assert.Nil(t, e)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSyntax)
	assert.Contains(t, err.Error(), "position 5")
}

// S9: Division by zero.
func TestScenarioS9(t *testing.T) {
	e, err := Parse("10/(1d4-1d4)")
	require.NoError(t, err)
	for face := 1; face <= 4; face++ {
		var r Result
		assert.NotPanics(t, func() {
			r, err = e.Roll(newFakeRNG(t, face, face), nil)
		})
		assert.ErrorIs(t, err, ErrDivisionByZero, "faces %d,%d", face, face)
		assert.Equal(t, Result{}, r)
	}
}

// S10: Labelled term.
func TestScenarioS10(t *testing.T) {
	r := scenarioRoll(t, "1d8[slashing]+@str[ability]", scenarioResolver(map[string]int{"str": 3}), 5)
	assert.Equal(t, 8, r.Total)
	op := asBinary(t, r.Tree, "+")
	g := asGroup(t, op.Operands[0])
	assert.Equal(t, "slashing", g.Label)
	assert.Equal(t, []int{5}, dieValues(g.Dice))
	v, ok := op.Operands[1].(VariableNode)
	require.True(t, ok, "expected VariableNode, got %T", op.Operands[1])
	assert.Equal(t, "str", v.Name)
	assert.Equal(t, "ability", v.Label)
	assert.Equal(t, 3, v.Value)
}

// S11: Roll20-only token rejected.
func TestScenarioS11(t *testing.T) {
	e, err := Parse("2d6!p")
	assert.Nil(t, e)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSyntax)
	assert.Contains(t, err.Error(), "position 3")
}
