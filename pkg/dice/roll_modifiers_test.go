package dice

import (
	"errors"
	"reflect"
	"testing"
)

const (
	RR = DieRerolled
	X  = DieExploded
)

// constRNG always returns the same face and counts draws; it never runs out,
// so runaway rerolls/explosions are caught by the draw cap instead of hanging.
type constRNG struct {
	t     *testing.T
	face  int
	draws int
	cap   int
}

func (c *constRNG) IntN(n int) int {
	c.t.Helper()
	c.draws++
	if c.draws > c.cap {
		c.t.Fatalf("constRNG: more than %d draws; iteration limit not enforced", c.cap)
	}
	if c.face > n {
		return n - 1
	}
	return c.face - 1
}

// prefixRNG returns scripted faces, then repeats the last one forever (up to a cap).
type prefixRNG struct {
	t     *testing.T
	faces []int
	draws int
	cap   int
}

func (p *prefixRNG) IntN(n int) int {
	p.t.Helper()
	p.draws++
	if p.draws > p.cap {
		p.t.Fatalf("prefixRNG: more than %d draws; limit not enforced", p.cap)
	}
	i := p.draws - 1
	if i >= len(p.faces) {
		i = len(p.faces) - 1
	}
	return p.faces[i] - 1
}

func checkLinks(t *testing.T, g DiceGroupNode, rerolledFrom, explodedFrom []int) {
	t.Helper()
	var rf, ef []int
	for _, d := range g.Dice {
		rf = append(rf, d.RerolledFrom)
		ef = append(ef, d.ExplodedFrom)
	}
	if rerolledFrom != nil && !reflect.DeepEqual(rf, rerolledFrom) {
		t.Errorf("RerolledFrom = %v, want %v", rf, rerolledFrom)
	}
	if explodedFrom != nil && !reflect.DeepEqual(ef, explodedFrom) {
		t.Errorf("ExplodedFrom = %v, want %v", ef, explodedFrom)
	}
}

func rollErr(t *testing.T, e *Expr, rng RNG) (Result, error) {
	t.Helper()
	return e.Roll(rng, nil)
}

func mustParseLimits(t *testing.T, f string, l Limits) *Expr {
	t.Helper()
	e, err := ParseWithLimits(f, l)
	if err != nil {
		t.Fatalf("ParseWithLimits(%q): %v", f, err)
	}
	return e
}

// --- ro ---

func TestRerollOnceExample(t *testing.T) {
	r := rollFaces(t, "1d6ro<=2", 1, 4)
	if r.Total != 4 {
		t.Errorf("Total = %d, want 4", r.Total)
	}
	g := rootGroup(t, r)
	checkDice(t, g, []int{1, 4}, []DieState{RR, K})
	checkLinks(t, g, []int{-1, 0}, []int{-1, -1})
	if g.Value != 4 {
		t.Errorf("group Value = %d, want 4", g.Value)
	}
}

func TestRerollOnceOnlyOnce(t *testing.T) {
	r := rollFaces(t, "1d6ro<=2", 1, 2)
	if r.Total != 2 {
		t.Errorf("Total = %d, want 2", r.Total)
	}
	g := rootGroup(t, r)
	checkDice(t, g, []int{1, 2}, []DieState{RR, K})
	checkLinks(t, g, []int{-1, 0}, []int{-1, -1})
}

func TestRerollOnceNoMatch(t *testing.T) {
	r := rollFaces(t, "1d6ro1", 3)
	if r.Total != 3 {
		t.Errorf("Total = %d, want 3", r.Total)
	}
	g := rootGroup(t, r)
	checkDice(t, g, []int{3}, []DieState{K})
	checkLinks(t, g, []int{-1}, []int{-1})
}

func TestRerollOnceAppendsSibling(t *testing.T) {
	r := rollFaces(t, "2d6ro1", 1, 3, 5)
	if r.Total != 8 {
		t.Errorf("Total = %d, want 8", r.Total)
	}
	g := rootGroup(t, r)
	checkDice(t, g, []int{1, 3, 5}, []DieState{RR, K, K})
	checkLinks(t, g, []int{-1, -1, 0}, []int{-1, -1, -1})
}

func TestRerollOnceThenKeep(t *testing.T) {
	r := rollFaces(t, "2d6ro1kh1", 1, 3, 5)
	if r.Total != 5 {
		t.Errorf("Total = %d, want 5", r.Total)
	}
	g := rootGroup(t, r)
	checkDice(t, g, []int{1, 3, 5}, []DieState{RR, D, K})
}

// --- rr / r ---

func TestRerollUntilConditionFails(t *testing.T) {
	for _, f := range []string{"1d6rr<=2", "1d6r<=2"} {
		t.Run(f, func(t *testing.T) {
			r := rollFaces(t, f, 1, 2, 5)
			if r.Total != 5 {
				t.Errorf("Total = %d, want 5", r.Total)
			}
			g := rootGroup(t, r)
			checkDice(t, g, []int{1, 2, 5}, []DieState{RR, RR, K})
			checkLinks(t, g, []int{-1, 0, 1}, []int{-1, -1, -1})
		})
	}
}

func TestRerollBareSelector(t *testing.T) {
	r := rollFaces(t, "1d6rr1", 1, 1, 6)
	if r.Total != 6 {
		t.Errorf("Total = %d, want 6", r.Total)
	}
	g := rootGroup(t, r)
	checkDice(t, g, []int{1, 1, 6}, []DieState{RR, RR, K})
	checkLinks(t, g, []int{-1, 0, 1}, nil)
}

func TestRerollInExpression(t *testing.T) {
	r := rollFaces(t, "1d6rr1+2", 1, 4)
	if r.Total != 6 {
		t.Errorf("Total = %d, want 6", r.Total)
	}
	gs := diceGroups(r.Tree)
	if len(gs) != 1 {
		t.Fatalf("got %d dice groups, want 1", len(gs))
	}
	checkDice(t, gs[0], []int{1, 4}, []DieState{RR, K})
}

// --- explode ---

func TestExplodeExample(t *testing.T) {
	for _, f := range []string{"1d6e", "1d6!"} {
		t.Run(f, func(t *testing.T) {
			r := rollFaces(t, f, 6, 6, 2)
			if r.Total != 14 {
				t.Errorf("Total = %d, want 14", r.Total)
			}
			g := rootGroup(t, r)
			checkDice(t, g, []int{6, 6, 2}, []DieState{X, X, K})
			checkLinks(t, g, []int{-1, -1, -1}, []int{-1, 0, 1})
			if g.Value != 14 {
				t.Errorf("group Value = %d, want 14", g.Value)
			}
		})
	}
}

func TestExplodeNoTrigger(t *testing.T) {
	r := rollFaces(t, "1d6e", 5)
	if r.Total != 5 {
		t.Errorf("Total = %d, want 5", r.Total)
	}
	g := rootGroup(t, r)
	checkDice(t, g, []int{5}, []DieState{K})
	checkLinks(t, g, []int{-1}, []int{-1})
}

func TestExplodeDefaultMaxFaceD8(t *testing.T) {
	r := rollFaces(t, "1d8e", 8, 6)
	if r.Total != 14 {
		t.Errorf("Total = %d, want 14", r.Total)
	}
	g := rootGroup(t, r)
	checkDice(t, g, []int{8, 6}, []DieState{X, K})
	checkLinks(t, g, nil, []int{-1, 0})
}

func TestExplodeWithCondition(t *testing.T) {
	r := rollFaces(t, "1d6e>=5", 5, 6, 3)
	if r.Total != 14 {
		t.Errorf("Total = %d, want 14", r.Total)
	}
	g := rootGroup(t, r)
	checkDice(t, g, []int{5, 6, 3}, []DieState{X, X, K})
	checkLinks(t, g, nil, []int{-1, 0, 1})
}

func TestExplodeMultipleDice(t *testing.T) {
	r := rollFaces(t, "2d6e", 6, 3, 4)
	if r.Total != 13 {
		t.Errorf("Total = %d, want 13", r.Total)
	}
	g := rootGroup(t, r)
	checkDice(t, g, []int{6, 3, 4}, []DieState{X, K, K})
	checkLinks(t, g, []int{-1, -1, -1}, []int{-1, -1, 0})
}

// --- limits ---

func TestRerollLimitDefault(t *testing.T) {
	e, err := Parse("1d6rr<=6")
	if err != nil {
		t.Fatal(err)
	}
	rng := &constRNG{t: t, face: 6, cap: DefaultMaxRerolls + 10}
	r, err := rollErr(t, e, rng)
	if !errors.Is(err, ErrTooManyRerolls) {
		t.Fatalf("err = %v, want ErrTooManyRerolls", err)
	}
	if !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("err = %v, want it to match ErrLimitExceeded", err)
	}
	if !reflect.DeepEqual(r, Result{}) {
		t.Errorf("Result = %#v, want zero value on error", r)
	}
}

func TestRerollLimitCustom(t *testing.T) {
	l := Limits{MaxRerolls: 2}
	e := mustParseLimits(t, "1d6rr1", l)

	// Exactly two rerolls are allowed.
	rng := &prefixRNG{t: t, faces: []int{1, 1, 5}, cap: 3}
	r, err := e.Roll(rng, nil)
	if err != nil {
		t.Fatalf("Roll with 2 rerolls: %v", err)
	}
	if r.Total != 5 {
		t.Errorf("Total = %d, want 5", r.Total)
	}

	// A third reroll exceeds the limit.
	rng = &prefixRNG{t: t, faces: []int{1}, cap: 10}
	_, err = e.Roll(rng, nil)
	if !errors.Is(err, ErrTooManyRerolls) {
		t.Fatalf("err = %v, want ErrTooManyRerolls", err)
	}
}

func TestExplodeLimitDefault(t *testing.T) {
	e, err := Parse("1d1e")
	if err != nil {
		t.Fatal(err)
	}
	rng := &constRNG{t: t, face: 1, cap: DefaultMaxExplosions + 10}
	r, err := rollErr(t, e, rng)
	if !errors.Is(err, ErrTooManyExplosions) {
		t.Fatalf("err = %v, want ErrTooManyExplosions", err)
	}
	if !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("err = %v, want it to match ErrLimitExceeded", err)
	}
	if !reflect.DeepEqual(r, Result{}) {
		t.Errorf("Result = %#v, want zero value on error", r)
	}
}

func TestExplodeLimitCustom(t *testing.T) {
	l := Limits{MaxExplosions: 2}
	e := mustParseLimits(t, "1d6e", l)

	rng := &prefixRNG{t: t, faces: []int{6, 6, 3}, cap: 3}
	r, err := e.Roll(rng, nil)
	if err != nil {
		t.Fatalf("Roll with 2 explosions: %v", err)
	}
	if r.Total != 15 {
		t.Errorf("Total = %d, want 15", r.Total)
	}

	rng = &prefixRNG{t: t, faces: []int{6}, cap: 10}
	_, err = e.Roll(rng, nil)
	if !errors.Is(err, ErrTooManyExplosions) {
		t.Fatalf("err = %v, want ErrTooManyExplosions", err)
	}
}

func TestExplodeCappedByMaxDice(t *testing.T) {
	l := Limits{MaxDice: 3}
	e := mustParseLimits(t, "2d6e", l)

	// Two dice plus one explosion is exactly three dice.
	rng := &prefixRNG{t: t, faces: []int{6, 2, 3}, cap: 3}
	r, err := e.Roll(rng, nil)
	if err != nil {
		t.Fatalf("Roll producing 3 dice: %v", err)
	}
	if r.Total != 11 {
		t.Errorf("Total = %d, want 11", r.Total)
	}

	// A fourth die exceeds MaxDice even though explosions are within limits.
	rng = &prefixRNG{t: t, faces: []int{6, 2, 6, 6}, cap: 10}
	_, err = e.Roll(rng, nil)
	if !errors.Is(err, ErrTooManyDice) {
		t.Fatalf("err = %v, want ErrTooManyDice", err)
	}
}

func TestRerollCappedByMaxDice(t *testing.T) {
	l := Limits{MaxDice: 3}
	e := mustParseLimits(t, "2d6rr1", l)

	rng := &prefixRNG{t: t, faces: []int{1, 4, 5}, cap: 3}
	r, err := e.Roll(rng, nil)
	if err != nil {
		t.Fatalf("Roll producing 3 dice: %v", err)
	}
	if r.Total != 9 {
		t.Errorf("Total = %d, want 9", r.Total)
	}

	rng = &prefixRNG{t: t, faces: []int{1}, cap: 10}
	_, err = e.Roll(rng, nil)
	if !errors.Is(err, ErrTooManyDice) {
		t.Fatalf("err = %v, want ErrTooManyDice", err)
	}
}
