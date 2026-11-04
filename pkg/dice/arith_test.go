package dice

import (
	"errors"
	"math"
	"testing"
)

func rollNoVars(t *testing.T, formula string) (Result, error) {
	t.Helper()
	e, err := Parse(formula)
	if err != nil {
		t.Fatalf("Parse(%q): %v", formula, err)
	}
	return e.Roll(newFakeRNG(t), func(string) (int, bool) { return 0, false })
}

func TestArithmetic(t *testing.T) {
	cases := []struct {
		f    string
		want int
	}{
		{"1+2", 3}, {"5-8", -3}, {"3*4", 12}, {"2+3*4", 14}, {"(2+3)*4", 20},
		{"-5", -5}, {"--5", 5}, {"-(2+3)", -5}, {"10/3", 3}, {"0/5", 0},
	}
	for _, c := range cases {
		r, err := rollNoVars(t, c.f)
		if err != nil || r.Total != c.want {
			t.Errorf("%s = %d, %v; want %d", c.f, r.Total, err, c.want)
		}
	}
}

func TestFloorDivision(t *testing.T) {
	cases := []struct {
		f    string
		want int
	}{
		{"-7/2", -4}, {"7/-2", -4}, {"-7/-2", 3}, {"7/2", 3}, {"-6/2", -3}, {"6/-3", -2},
	}
	for _, c := range cases {
		r, err := rollNoVars(t, c.f)
		if err != nil || r.Total != c.want {
			t.Errorf("%s = %d, %v; want %d", c.f, r.Total, err, c.want)
		}
	}
}

func TestDivisionByZero(t *testing.T) {
	for _, f := range []string{"1/0", "0/0", "5/(3-3)", "-5/0"} {
		r, err := rollNoVars(t, f)
		if !errors.Is(err, ErrDivisionByZero) {
			t.Errorf("%s: err = %v, want ErrDivisionByZero", f, err)
		}
		if r.Total != 0 || r.Tree != nil {
			t.Errorf("%s: result not zero: %+v", f, r)
		}
	}
}

func TestOverflow(t *testing.T) {
	cases := []struct {
		f string
		v int
	}{
		{"@x/-1", math.MinInt}, {"-@x", math.MinInt},
		{"@x+1", math.MaxInt}, {"@x-1", math.MinInt}, {"@x*2", math.MaxInt},
		{"@x*2", math.MinInt}, {"@x*-1", math.MinInt}, {"-1*@x", math.MinInt},
		{"@x+@x", math.MaxInt}, {"0-@x", math.MinInt},
	}
	for _, c := range cases {
		e, err := Parse(c.f)
		if err != nil {
			t.Fatal(err)
		}
		r, err := e.Roll(newFakeRNG(t), func(string) (int, bool) { return c.v, true })
		if !errors.Is(err, ErrOverflow) {
			t.Errorf("%s with x=%d: err = %v, want ErrOverflow", c.f, c.v, err)
		}
		if r.Total != 0 || r.Tree != nil {
			t.Errorf("%s: result not zero: %+v", c.f, r)
		}
	}
}

func TestNoOverflowAtBoundaries(t *testing.T) {
	cases := []struct {
		f    string
		v    int
		want int
	}{
		{"@x+0", math.MaxInt, math.MaxInt}, {"@x-0", math.MinInt, math.MinInt},
		{"@x*1", math.MinInt, math.MinInt}, {"@x/1", math.MinInt, math.MinInt},
		{"@x/-1", math.MaxInt, -math.MaxInt}, {"-@x", math.MaxInt, -math.MaxInt},
		{"@x+1", math.MinInt, math.MinInt + 1},
	}
	for _, c := range cases {
		e, _ := Parse(c.f)
		r, err := e.Roll(newFakeRNG(t), func(string) (int, bool) { return c.v, true })
		if err != nil || r.Total != c.want {
			t.Errorf("%s x=%d: %d, %v; want %d", c.f, c.v, r.Total, err, c.want)
		}
	}
}
