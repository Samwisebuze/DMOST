package dice

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sameAST reports whether two ASTs are structurally equal, ignoring any
// struct field named "pos" (byte offsets differ between alias spellings).
func sameAST(a, b node) bool {
	return sameValue(reflect.ValueOf(a), reflect.ValueOf(b))
}

func sameValue(a, b reflect.Value) bool {
	if a.IsValid() != b.IsValid() {
		return false
	}
	if !a.IsValid() {
		return true
	}
	if a.Type() != b.Type() {
		return false
	}
	switch a.Kind() {
	case reflect.Pointer, reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		return sameValue(a.Elem(), b.Elem())
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if strings.EqualFold(a.Type().Field(i).Name, "pos") {
				continue
			}
			if !sameValue(a.Field(i), b.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Slice, reflect.Array:
		if a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !sameValue(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if a.Len() != b.Len() {
			return false
		}
		for _, k := range a.MapKeys() {
			if !sameValue(a.MapIndex(k), b.MapIndex(k)) {
				return false
			}
		}
		return true
	case reflect.Bool:
		return a.Bool() == b.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() == b.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return a.Uint() == b.Uint()
	case reflect.Float32, reflect.Float64:
		return a.Float() == b.Float()
	case reflect.String:
		return a.String() == b.String()
	default:
		return fmt.Sprint(a) == fmt.Sprint(b)
	}
}

func assertParses(t *testing.T, in string) *Expr {
	t.Helper()
	e, err := Parse(in)
	require.NoError(t, err, "Parse(%q)", in)
	require.NotNil(t, e, "Parse(%q)", in)
	return e
}

func assertSyntaxErr(t *testing.T, in string, pos int) {
	t.Helper()
	e, err := Parse(in)
	assert.Nil(t, e, "Parse(%q)", in)
	require.Error(t, err, "Parse(%q)", in)
	assert.ErrorIs(t, err, ErrSyntax, "Parse(%q)", in)
	assert.Contains(t, err.Error(), "position", "Parse(%q)", in)
	if pos >= 0 {
		assert.Contains(t, err.Error(), fmt.Sprintf("position %d", pos), "Parse(%q)", in)
	}
}

func asDiceTerm(t *testing.T, n node) *diceTerm {
	t.Helper()
	switch v := n.(type) {
	case *diceTerm:
		return v
	case diceTerm:
		return &v
	}
	t.Fatalf("root is %T, want diceTerm", n)
	return nil
}

func TestModKeepDropSelectors(t *testing.T) {
	for _, in := range []string{
		"4d6kh3", "4d6kl1", "4d6ph1", "4d6pl1", "4d6kh4", "2d20kh1", "2d20kl1",
		"4d6kh3+2", "1d20+4d6kh3", "d20kh1", "4dFkh2", "4d%kl1",
	} {
		t.Run(in, func(t *testing.T) { assertParses(t, in) })
	}
}

func TestModKeepDropDistinct(t *testing.T) {
	plain := assertParses(t, "4d6").root
	exprs := []string{"4d6kh3", "4d6kl3", "4d6ph3", "4d6pl3", "4d6kh2"}
	roots := make([]node, len(exprs))
	for i, in := range exprs {
		roots[i] = assertParses(t, in).root
		assert.False(t, sameAST(plain, roots[i]), "%q must differ from 4d6", in)
	}
	for i := range exprs {
		for j := i + 1; j < len(exprs); j++ {
			assert.False(t, sameAST(roots[i], roots[j]), "%q and %q must differ", exprs[i], exprs[j])
		}
	}
}

func TestModDropAliases(t *testing.T) {
	pairs := [][2]string{
		{"4d6dh1", "4d6ph1"},
		{"4d6dl1", "4d6pl1"},
		{"4d6dl2", "4d6pl2"},
		{"2d20dh1", "2d20ph1"},
		{"4d6dl1+3", "4d6pl1+3"},
	}
	for _, pr := range pairs {
		t.Run(pr[0], func(t *testing.T) {
			a := assertParses(t, pr[0]).root
			b := assertParses(t, pr[1]).root
			assert.True(t, sameAST(a, b), "%q must parse identically to %q", pr[0], pr[1])
		})
	}
}

func TestModBareDropIsSyntaxError(t *testing.T) {
	for _, in := range []string{"4d6d1", "4d6d", "2d20d1+3"} {
		t.Run(in, func(t *testing.T) { assertSyntaxErr(t, in, -1) })
	}
}

func TestModKeepDropMissingSelector(t *testing.T) {
	for _, in := range []string{"4d6kh", "4d6pl", "4d6dh", "4d6kh+1"} {
		t.Run(in, func(t *testing.T) { assertSyntaxErr(t, in, -1) })
	}
}

func TestModRerollSelectors(t *testing.T) {
	for _, in := range []string{
		"1d6rr1", "1d6rr<2", "1d6rr<=2", "1d6rr=1", "1d6rr>=5", "1d6rr>5",
		"1d6ro1", "1d6ro<2", "1d6ro<=2", "1d6ro=1", "1d6ro>=5", "1d6ro>5",
		"1d6r1", "1d6r<2", "1d6r<=2", "4d6rr1+2",
	} {
		t.Run(in, func(t *testing.T) { assertParses(t, in) })
	}
}

func TestModRerollAliases(t *testing.T) {
	pairs := [][2]string{
		{"1d6rr1", "1d6rr=1"},
		{"1d6ro1", "1d6ro=1"},
		{"1d6r1", "1d6rr1"},
		{"1d6r<2", "1d6rr<2"},
		{"1d6r<=2", "1d6rr<=2"},
		{"1d6r>=5", "1d6rr>=5"},
		{"1d6r=3", "1d6rr=3"},
	}
	for _, pr := range pairs {
		t.Run(pr[0], func(t *testing.T) {
			a := assertParses(t, pr[0]).root
			b := assertParses(t, pr[1]).root
			assert.True(t, sameAST(a, b), "%q must parse identically to %q", pr[0], pr[1])
		})
	}
}

func TestModRerollDistinct(t *testing.T) {
	exprs := []string{"1d6rr1", "1d6ro1", "1d6rr<2", "1d6rr<=2", "1d6rr>5", "1d6rr>=5", "1d6rr2"}
	roots := make([]node, len(exprs))
	for i, in := range exprs {
		roots[i] = assertParses(t, in).root
	}
	for i := range exprs {
		for j := i + 1; j < len(exprs); j++ {
			assert.False(t, sameAST(roots[i], roots[j]), "%q and %q must differ", exprs[i], exprs[j])
		}
	}
}

func TestModRerollMissingSelector(t *testing.T) {
	for _, in := range []string{"1d6rr", "1d6ro", "1d6r", "1d6rr<", "1d6ro>=", "1d6rr+1"} {
		t.Run(in, func(t *testing.T) { assertSyntaxErr(t, in, -1) })
	}
}

func TestModExplode(t *testing.T) {
	for _, in := range []string{
		"1d6e", "1d6e>=5", "1d6e=6", "1d6e>4", "1d6e<2", "1d6e<=2",
		"1d6!", "1d6!>=5", "1d6!>4", "1d6!=6", "3d6e+2", "3d6!+2",
	} {
		t.Run(in, func(t *testing.T) { assertParses(t, in) })
	}
}

func TestModExplodeAliases(t *testing.T) {
	pairs := [][2]string{
		{"1d6!", "1d6e"},
		{"1d6!>=5", "1d6e>=5"},
		{"1d6!>4", "1d6e>4"},
		{"1d6!=6", "1d6e=6"},
		{"3d6!+2", "3d6e+2"},
	}
	for _, pr := range pairs {
		t.Run(pr[0], func(t *testing.T) {
			a := assertParses(t, pr[0]).root
			b := assertParses(t, pr[1]).root
			assert.True(t, sameAST(a, b), "%q must parse identically to %q", pr[0], pr[1])
		})
	}
	assert.False(t, sameAST(assertParses(t, "1d6e>=5").root, assertParses(t, "1d6e>=4").root))
	assert.False(t, sameAST(assertParses(t, "1d6e").root, assertParses(t, "1d6").root))
}

func TestModExplodeUnsupportedForms(t *testing.T) {
	tests := []struct {
		in  string
		pos int
	}{
		{"2d6!p", 3},
		{"2d6!!", 3},
		{"1d6!!", 3},
		{"10d6!p", 4},
		{"1+2d6!!", 5},
		{"4d6kh3!p", 6},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) { assertSyntaxErr(t, tc.in, tc.pos) })
	}
}

func TestModSuccessCounting(t *testing.T) {
	for _, in := range []string{
		"6d10>=8", "6d10>8", "6d10<3", "6d10<=3", "6d10=10",
		"6d10kh3>=8", "6d10e>=8", "6d10rr1>7", "6d10!>=8", "6d10>=8+1",
	} {
		t.Run(in, func(t *testing.T) { assertParses(t, in) })
	}
	a := assertParses(t, "6d10>=8").root
	assert.False(t, sameAST(a, assertParses(t, "6d10").root), "success counting must change the AST")
	assert.False(t, sameAST(a, assertParses(t, "6d10>8").root), ">= and > must differ")
	assert.False(t, sameAST(a, assertParses(t, "6d10>=7").root), "targets must differ")
}

func TestModComparisonBetweenExpressionsIsSyntaxError(t *testing.T) {
	for _, in := range []string{
		"3>=2", "1<2", "@a>1", "(1d6+2)>=3", "2d6+3>=4", "min(1,2)>0", "1=1", "6d10>=", "6d10>=8>=2",
	} {
		t.Run(in, func(t *testing.T) { assertSyntaxErr(t, in, -1) })
	}
}

func TestModGroupedKeepDrop(t *testing.T) {
	for _, in := range []string{
		"(1d6+1d8)kh1", "(1d6+1d8)kl1", "(1d6+1d8)ph1", "(1d6+1d8)pl1",
		"(1d6+1d8)dh1", "(1d6+1d8)dl1", "(1d20+5)kh1", "(1d6+1d8)kh1+2",
	} {
		t.Run(in, func(t *testing.T) {
			e := assertParses(t, in)
			if strings.HasSuffix(in, "+2") {
				return
			}
			switch e.root.(type) {
			case *groupKeep, groupKeep:
			default:
				t.Errorf("root is %T, want groupKeep", e.root)
			}
		})
	}
	a := assertParses(t, "(1d6+1d8)dh1").root
	b := assertParses(t, "(1d6+1d8)ph1").root
	assert.True(t, sameAST(a, b), "grouped dh must alias ph")
	assert.False(t, sameAST(assertParses(t, "(1d6+1d8)kh1").root, assertParses(t, "(1d6+1d8)kl1").root))
}

func TestModGroupedKeepDropWithoutDice(t *testing.T) {
	for _, in := range []string{"(1+2)kh1", "(3)kl1", "(@a+@b)kh1", "(1+2)dl1", "5kh1", "@a kh1"} {
		t.Run(in, func(t *testing.T) { assertSyntaxErr(t, in, -1) })
	}
}

func TestModKeepCount(t *testing.T) {
	for _, in := range []string{
		"4d6kh0", "4d6kl0", "4d6ph0", "4d6pl0", "4d6dh0", "4d6dl0",
		"4d6kh7", "4d6kl5", "4d6ph5", "4d6pl9", "1d20kh2", "d20kh2",
		"4d6rr1kh7", "(1d6+1d8)kh0",
	} {
		t.Run(in, func(t *testing.T) {
			e, err := Parse(in)
			assert.Nil(t, e)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrKeepCount)
		})
	}
}

func TestModUnknownLetters(t *testing.T) {
	tests := []struct {
		in  string
		pos int
	}{
		{"4d6x", 3},
		{"4d6z3", 3},
		{"4d6h3", 3},
		{"4d6l1", 3},
		{"4d6kx3", 4},
		{"4d6kh3q", 6},
		{"4d6kh3h1", 6},
		{"1d6rr1y", 6},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) { assertSyntaxErr(t, tc.in, tc.pos) })
	}
}

func TestModChainingOrder(t *testing.T) {
	for _, in := range []string{"4d6rr1kh3", "4d6kh3rr1", "4d6ro1e", "8d6rr1ro2!kh3", "4d6epl1", "4d6rr1kh3>=4"} {
		t.Run(in, func(t *testing.T) { assertParses(t, in) })
	}
	a := assertParses(t, "4d6rr1kh3").root
	b := assertParses(t, "4d6kh3rr1").root
	assert.False(t, sameAST(a, b), "modifier order must be preserved")

	d := asDiceTerm(t, a)
	require.Len(t, d.mods, 2, "4d6rr1kh3 must carry two modifiers in written order")
	assert.Equal(t, "rr", d.mods[0].kind)
	assert.Equal(t, "kh", d.mods[1].kind)

	d = asDiceTerm(t, b)
	require.Len(t, d.mods, 2)
	assert.Equal(t, "kh", d.mods[0].kind)
	assert.Equal(t, "rr", d.mods[1].kind)
}
