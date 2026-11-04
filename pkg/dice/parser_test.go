package dice

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// render prints an AST ignoring positions, fully parenthesising operators.
// Variables are printed as @name whether or not the stored name keeps the '@'.
func render(n node) string {
	switch v := n.(type) {
	case nil:
		return "<nil>"
	case *literal:
		return render(*v)
	case *variable:
		return render(*v)
	case *unary:
		return render(*v)
	case *binary:
		return render(*v)
	case *diceTerm:
		return render(*v)
	case *funcCall:
		return render(*v)
	case literal:
		return fmt.Sprint(v.value)
	case variable:
		s := "@" + strings.TrimPrefix(v.name, "@")
		if v.label != "" {
			s += "[" + v.label + "]"
		}
		return s
	case unary:
		return "(" + string(v.op) + render(v.operand) + ")"
	case binary:
		return "(" + render(v.left) + string(v.op) + render(v.right) + ")"
	case diceTerm:
		s := render(v.count) + "d"
		if v.fudge {
			s += "F"
		} else {
			s += render(v.sides)
		}
		if v.label != "" {
			s += "[" + v.label + "]"
		}
		return s
	case funcCall:
		args := make([]string, len(v.args))
		for i, a := range v.args {
			args[i] = render(a)
		}
		return v.name + "(" + strings.Join(args, ",") + ")"
	default:
		return fmt.Sprintf("<%T>", n)
	}
}

func mustParse(t *testing.T, in string) *Expr {
	t.Helper()
	e, err := Parse(in)
	require.NoError(t, err, "Parse(%q)", in)
	require.NotNil(t, e, "Parse(%q)", in)
	return e
}

func TestParseAST(t *testing.T) {
	tests := []struct{ in, want string }{
		// literals and arithmetic
		{"42", "42"},
		{" 7 ", "7"},
		{"2+3", "(2+3)"},
		{"2+3*4", "(2+(3*4))"},
		{"2*3+4", "((2*3)+4)"},
		{"(2+3)*4", "((2+3)*4)"},
		{"-2*3", "((-2)*3)"},
		{"1-2-3", "((1-2)-3)"},
		{"8/4/2", "((8/4)/2)"},
		{"8/2*3", "((8/2)*3)"},
		{"1-(2-3)", "(1-(2-3))"},
		{"-(1+2)", "(-(1+2))"},
		{"--3", "(-(-3))"},
		{"2*-3", "(2*(-3))"},
		{"((5))", "5"},
		// dice
		{"1d6", "1d6"},
		{"3d8", "3d8"},
		{"d20", "1d20"},
		{"d%", "1d100"},
		{"2d%", "2d100"},
		{"dF", "1dF"},
		{"4dF", "4dF"},
		{"1d1", "1d1"},
		{"2d6+3", "(2d6+3)"},
		{"2d6*2", "(2d6*2)"},
		{"-1d6", "(-1d6)"},
		{"1d6+1d8-d4", "((1d6+1d8)-1d4)"},
		// variables and labels
		{"@str", "@str"},
		{"1d20+@str", "(1d20+@str)"},
		{"1d6[fire]", "1d6[fire]"},
		{"2d6[slash]+1d4[fire]", "(2d6[slash]+1d4[fire])"},
		{"@str[strength]", "@str[strength]"},
		{"1d20+@dex[dex]", "(1d20+@dex[dex])"},
		{"dF[luck]", "1dF[luck]"},
		{"d%[pct]", "1d100[pct]"},
		// functions
		{"min(1,2)", "min(1,2)"},
		{"max(1d6,1d8)", "max(1d6,1d8)"},
		{"floor(7)", "floor(7)"},
		{"ceil(@x)", "ceil(@x)"},
		{"max(1, 2+3)*2", "(max(1,(2+3))*2)"},
		{"min(max(1,2),floor(3))", "min(max(1,2),floor(3))"},
		{"1+ceil(2)", "(1+ceil(2))"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			e := mustParse(t, tc.in)
			assert.Equal(t, tc.want, render(e.root))
		})
	}
}

func TestParseFudgeIsMarked(t *testing.T) {
	e := mustParse(t, "dF")
	d, ok := e.root.(*diceTerm)
	if !ok {
		dv, ok2 := e.root.(diceTerm)
		require.True(t, ok2, "dF root is %T, want diceTerm", e.root)
		d = &dv
	}
	assert.True(t, d.fudge, "dF must be a fudge die")

	e = mustParse(t, "1d3")
	assert.NotContains(t, render(e.root), "F", "1d3 must not be a fudge die")
}

func TestParseLargeButValidDice(t *testing.T) {
	l := Limits{MaxDice: 1 << 30, MaxSides: 1 << 30}
	e, err := ParseWithLimits("123456d987654", l)
	require.NoError(t, err)
	assert.Equal(t, "123456d987654", render(e.root))
}

func TestParseZeroDiceIsSyntaxError(t *testing.T) {
	for _, in := range []string{"1d0", "d0", "0d6", "0d0", "2+0d6", "0dF", "0d%"} {
		t.Run(in, func(t *testing.T) {
			e, err := Parse(in)
			assert.Nil(t, e)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrSyntax)
			assert.Contains(t, err.Error(), "position")
		})
	}
}

func TestParseLabelPlacement(t *testing.T) {
	for _, in := range []string{"3[x]", "(1d6)[x]", "(1+2)[x]", "1d6+3[x]", "min(1,2)[x]", "[x]", "[x]1d6"} {
		t.Run(in, func(t *testing.T) {
			e, err := Parse(in)
			assert.Nil(t, e)
			assert.ErrorIs(t, err, ErrSyntax)
		})
	}
}

func TestParseSyntaxErrors(t *testing.T) {
	tests := []struct {
		in  string
		pos int // -1: don't check exact position
	}{
		{"", 0},
		{"   ", -1},
		{"(1d6+", 5},
		{"(1", 2},
		{"1)", 1},
		{"((1)", 4},
		{"())", -1},
		{"()", 1},
		{"1+", 2},
		{"1*", 2},
		{"1 +", 3},
		{"*1", 0},
		{"/2", 0},
		{"1 2", 2},
		{"1d6 3", 4},
		{"@a @b", 3},
		{"1,2", 1},
		{",", 0},
		{"min(1,2", 7},
		{"min(1,)", 6},
		{"min", 3},
		{"floor()", 6},
		{"1++", 3},
		{"d", 1},
		{"2d", 2},
		{"d+1", 1},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%q", tc.in), func(t *testing.T) {
			e, err := Parse(tc.in)
			assert.Nil(t, e)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrSyntax)
			assert.Contains(t, err.Error(), "position")
			if tc.pos >= 0 {
				assert.Contains(t, err.Error(), fmt.Sprintf("position %d", tc.pos))
			}
		})
	}
}

func TestParseUnclosedReturnsNilAndPosition(t *testing.T) {
	e, err := Parse("(1d6+")
	require.Nil(t, e)
	require.True(t, errors.Is(err, ErrSyntax), "err = %v", err)
	assert.Contains(t, err.Error(), "5")
}

func TestParseLexErrorsPropagate(t *testing.T) {
	e, err := Parse("1d6 $")
	assert.Nil(t, e)
	assert.ErrorIs(t, err, ErrSyntax)
}

func TestParseLimits(t *testing.T) {
	l := Limits{MaxDice: 10, MaxSides: 20, MaxRerolls: 1, MaxExplosions: 1, MaxNodes: 8}
	tests := []struct {
		in   string
		want error
	}{
		{"10d20", nil},
		{"11d6", ErrTooManyDice},
		{"1d21", ErrTooManySides},
		{"d21", ErrTooManySides},
		{"d%", ErrTooManySides},
		{"1+11d6", ErrTooManyDice},
		{"min(1d6,1d21)", ErrTooManySides},
		{"11dF", ErrTooManyDice},
		{"1+1+1+1", nil},
		{"1+1+1+1+1+1+1+1+1+1", ErrExpressionTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			e, err := ParseWithLimits(tc.in, l)
			if tc.want == nil {
				require.NoError(t, err)
				require.NotNil(t, e)
				return
			}
			assert.Nil(t, e)
			assert.ErrorIs(t, err, tc.want)
			assert.ErrorIs(t, err, ErrLimitExceeded)
		})
	}
}

func TestParseDefaultLimits(t *testing.T) {
	_, err := Parse("1000d10000")
	require.NoError(t, err)

	e, err := Parse("1001d6")
	assert.Nil(t, e)
	assert.ErrorIs(t, err, ErrTooManyDice)

	e, err = Parse("1d10001")
	assert.Nil(t, e)
	assert.ErrorIs(t, err, ErrTooManySides)

	big := strings.Repeat("1+", 300) + "1"
	e, err = Parse(big)
	assert.Nil(t, e)
	assert.ErrorIs(t, err, ErrExpressionTooLarge)
}

func TestParseHugeDiceFailsAtParseTime(t *testing.T) {
	e, err := Parse("1000000d1000000")
	assert.Nil(t, e)
	assert.ErrorIs(t, err, ErrLimitExceeded)
	assert.True(t, errors.Is(err, ErrTooManyDice) || errors.Is(err, ErrTooManySides), "err = %v", err)
}

func TestParseZeroLimitsUseDefaults(t *testing.T) {
	_, err := ParseWithLimits("1000d6", Limits{})
	require.NoError(t, err)
	_, err = ParseWithLimits("1001d6", Limits{})
	assert.ErrorIs(t, err, ErrTooManyDice)
}

func TestParseStoresLimits(t *testing.T) {
	l := Limits{MaxDice: 50, MaxSides: 60, MaxRerolls: 7, MaxExplosions: 8, MaxNodes: 90}
	e, err := ParseWithLimits("2d6+1", l)
	require.NoError(t, err)
	assert.Equal(t, l, e.limits)

	e = mustParse(t, "2d6")
	assert.Equal(t, DefaultLimits(), e.limits)

	e, err = ParseWithLimits("1", Limits{MaxDice: 3})
	require.NoError(t, err)
	assert.Equal(t, 3, e.limits.MaxDice)
}

func TestParseStoresSource(t *testing.T) {
	e := mustParse(t, "1d6 + 2")
	assert.Equal(t, "1d6 + 2", e.src)
}

// floor and ceil must be documented as identity on integers.
func TestFloorCeilDocumented(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	found := false
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		require.NoError(t, err)
		for _, cg := range f.Comments {
			txt := strings.ToLower(cg.Text())
			if strings.Contains(txt, "floor") && strings.Contains(txt, "ceil") && strings.Contains(txt, "identity") {
				found = true
			}
		}
	}
	assert.True(t, found, "a doc comment must state that floor and ceil are identity on integers")
}
