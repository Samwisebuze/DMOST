package dice

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

// recRNG records every IntN argument and returns scripted face-1 values.
type recRNG struct {
	t     *testing.T
	faces []int
	args  []int
}

func (r *recRNG) IntN(n int) int {
	r.t.Helper()
	if len(r.args) >= len(r.faces) {
		r.t.Fatalf("recRNG script exhausted after %d draws", len(r.args))
		return 0
	}
	f := r.faces[len(r.args)]
	r.args = append(r.args, n)
	return f - 1
}

// rollFaces parses f and rolls it with the scripted faces, requiring every
// scripted face to be consumed.
func rollFaces(t *testing.T, f string, faces ...int) Result {
	t.Helper()
	e, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse(%q): %v", f, err)
	}
	rng := newFakeRNG(t, faces...)
	r, err := e.Roll(rng, nil)
	if err != nil {
		t.Fatalf("Roll(%q): %v", f, err)
	}
	if rng.pos != len(faces) {
		t.Fatalf("Roll(%q) drew %d dice, want %d", f, rng.pos, len(faces))
	}
	return r
}

// diceGroups collects every DiceGroupNode in the tree in depth-first,
// left-to-right order, whatever node types wrap them.
func diceGroups(n Node) []DiceGroupNode {
	var out []DiceGroupNode
	var walk func(v reflect.Value)
	nodeT := reflect.TypeOf((*Node)(nil)).Elem()
	walk = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		switch v.Kind() {
		case reflect.Interface, reflect.Pointer:
			if v.IsNil() {
				return
			}
			walk(v.Elem())
			return
		case reflect.Slice:
			if v.Type().Elem() == reflect.TypeOf(Die{}) {
				return
			}
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
			return
		case reflect.Struct:
			if v.Type() == reflect.TypeOf(DiceGroupNode{}) {
				out = append(out, v.Interface().(DiceGroupNode))
				return
			}
			for i := 0; i < v.NumField(); i++ {
				f := v.Field(i)
				if f.Kind() == reflect.Interface && !f.Type().Implements(nodeT) && f.Type() != nodeT {
					continue
				}
				walk(f)
			}
		}
	}
	walk(reflect.ValueOf(n))
	return out
}

func rootGroup(t *testing.T, r Result) DiceGroupNode {
	t.Helper()
	switch g := r.Tree.(type) {
	case DiceGroupNode:
		return g
	case *DiceGroupNode:
		return *g
	}
	t.Fatalf("tree = %#v, want DiceGroupNode", r.Tree)
	return DiceGroupNode{}
}

func dieValues(ds []Die) []int {
	out := make([]int, len(ds))
	for i, d := range ds {
		out[i] = d.Value
	}
	return out
}

func dieStates(ds []Die) []DieState {
	out := make([]DieState, len(ds))
	for i, d := range ds {
		out[i] = d.State
	}
	return out
}

func checkDice(t *testing.T, g DiceGroupNode, values []int, states []DieState) {
	t.Helper()
	if got := dieValues(g.Dice); !reflect.DeepEqual(got, values) {
		t.Errorf("die values = %v, want %v", got, values)
	}
	if got := dieStates(g.Dice); !reflect.DeepEqual(got, states) {
		t.Errorf("die states = %v, want %v", got, states)
	}
}

const (
	K = DieKept
	D = DieDropped
)

func TestRollNdMCallsIntNPerDie(t *testing.T) {
	e, err := Parse("3d8")
	if err != nil {
		t.Fatal(err)
	}
	rng := &recRNG{t: t, faces: []int{7, 1, 8}}
	r, err := e.Roll(rng, nil)
	if err != nil {
		t.Fatalf("Roll: %v", err)
	}
	if !reflect.DeepEqual(rng.args, []int{8, 8, 8}) {
		t.Errorf("IntN args = %v, want [8 8 8]", rng.args)
	}
	if r.Total != 16 {
		t.Errorf("total = %d, want 16", r.Total)
	}
	g := rootGroup(t, r)
	checkDice(t, g, []int{7, 1, 8}, []DieState{K, K, K})
	if g.Value != 16 {
		t.Errorf("group value = %d, want 16", g.Value)
	}
}

func TestRollImplicitCountAndPercent(t *testing.T) {
	e, err := Parse("d% + d20")
	if err != nil {
		t.Fatal(err)
	}
	rng := &recRNG{t: t, faces: []int{100, 1}}
	r, err := e.Roll(rng, nil)
	if err != nil {
		t.Fatalf("Roll: %v", err)
	}
	if !reflect.DeepEqual(rng.args, []int{100, 20}) {
		t.Errorf("IntN args = %v, want [100 20]", rng.args)
	}
	if r.Total != 101 {
		t.Errorf("total = %d, want 101", r.Total)
	}
	gs := diceGroups(r.Tree)
	if len(gs) != 2 {
		t.Fatalf("got %d dice groups, want 2", len(gs))
	}
	checkDice(t, gs[0], []int{100}, []DieState{K})
	checkDice(t, gs[1], []int{1}, []DieState{K})
}

func TestRollFudge(t *testing.T) {
	e, err := Parse("4dF")
	if err != nil {
		t.Fatal(err)
	}
	// IntN results 0,1,2,0 -> -1,0,+1,-1.
	rng := &recRNG{t: t, faces: []int{1, 2, 3, 1}}
	r, err := e.Roll(rng, nil)
	if err != nil {
		t.Fatalf("Roll: %v", err)
	}
	if !reflect.DeepEqual(rng.args, []int{3, 3, 3, 3}) {
		t.Errorf("IntN args = %v, want [3 3 3 3]", rng.args)
	}
	if r.Total != -1 {
		t.Errorf("total = %d, want -1", r.Total)
	}
	checkDice(t, rootGroup(t, r), []int{-1, 0, 1, -1}, []DieState{K, K, K, K})
}

func TestRollKeepHighest(t *testing.T) {
	r := rollFaces(t, "4d6kh3", 2, 5, 3, 6)
	if r.Total != 14 {
		t.Errorf("total = %d, want 14", r.Total)
	}
	g := rootGroup(t, r)
	checkDice(t, g, []int{2, 5, 3, 6}, []DieState{D, K, K, K})
	if g.Value != 14 {
		t.Errorf("group value = %d, want 14", g.Value)
	}
}

func TestRollKeepDropVariants(t *testing.T) {
	cases := []struct {
		f      string
		faces  []int
		total  int
		states []DieState
	}{
		{"4d6kl1", []int{4, 2, 6, 3}, 2, []DieState{D, K, D, D}},
		{"4d6kl3", []int{4, 2, 6, 3}, 9, []DieState{K, K, D, K}},
		{"4d6ph1", []int{4, 2, 6, 3}, 9, []DieState{K, K, D, K}},
		{"4d6pl1", []int{4, 2, 6, 3}, 13, []DieState{K, D, K, K}},
		{"4d6dh1", []int{4, 2, 6, 3}, 9, []DieState{K, K, D, K}},
		{"4d6dl1", []int{4, 2, 6, 3}, 13, []DieState{K, D, K, K}},
		{"4d6dl2", []int{4, 2, 6, 3}, 10, []DieState{K, D, K, D}},
		{"4d6dh2", []int{4, 2, 6, 3}, 5, []DieState{D, K, D, K}},
		{"4d6kh4", []int{4, 2, 6, 3}, 15, []DieState{K, K, K, K}},
		{"2d20kh1", []int{7, 15}, 15, []DieState{D, K}},
		{"2d20kl1", []int{7, 15}, 7, []DieState{K, D}},
		{"4dFkh2", []int{1, 3, 2, 3}, 2, []DieState{D, K, D, K}},
	}
	for _, c := range cases {
		t.Run(c.f, func(t *testing.T) {
			r := rollFaces(t, c.f, c.faces...)
			if r.Total != c.total {
				t.Errorf("total = %d, want %d", r.Total, c.total)
			}
			g := rootGroup(t, r)
			if got := dieStates(g.Dice); !reflect.DeepEqual(got, c.states) {
				t.Errorf("states = %v, want %v", got, c.states)
			}
			if g.Value != c.total {
				t.Errorf("group value = %d, want %d", g.Value, c.total)
			}
		})
	}
}

// Ties are broken by roll order: kh drops the later-rolled of equal dice and
// kl keeps the earlier-rolled of equal dice.
func TestRollKeepTieBreak(t *testing.T) {
	r := rollFaces(t, "4d6kh2", 5, 3, 5, 5)
	if r.Total != 10 {
		t.Errorf("kh total = %d, want 10", r.Total)
	}
	checkDice(t, rootGroup(t, r), []int{5, 3, 5, 5}, []DieState{K, D, K, D})

	r = rollFaces(t, "4d6kh1", 6, 6, 6, 6)
	checkDice(t, rootGroup(t, r), []int{6, 6, 6, 6}, []DieState{K, D, D, D})

	r = rollFaces(t, "4d6kl2", 4, 2, 6, 2)
	if r.Total != 4 {
		t.Errorf("kl total = %d, want 4", r.Total)
	}
	checkDice(t, rootGroup(t, r), []int{4, 2, 6, 2}, []DieState{D, K, D, K})

	r = rollFaces(t, "4d6kl2", 2, 2, 4, 2)
	checkDice(t, rootGroup(t, r), []int{2, 2, 4, 2}, []DieState{K, K, D, D})

	r = rollFaces(t, "3d6kl1", 3, 3, 3)
	checkDice(t, rootGroup(t, r), []int{3, 3, 3}, []DieState{K, D, D})
}

func TestRollKeepInArithmetic(t *testing.T) {
	r := rollFaces(t, "1d20+4d6kh3+2", 10, 2, 5, 3, 6)
	if r.Total != 26 {
		t.Errorf("total = %d, want 26", r.Total)
	}
	gs := diceGroups(r.Tree)
	if len(gs) != 2 {
		t.Fatalf("got %d dice groups, want 2", len(gs))
	}
	checkDice(t, gs[0], []int{10}, []DieState{K})
	checkDice(t, gs[1], []int{2, 5, 3, 6}, []DieState{D, K, K, K})
}

func TestRollKeepCountExceedsDiceAtRollTime(t *testing.T) {
	for _, f := range []string{"(1d6+1d6+1)kh3", "(1d6+2+3)kl2", "(1d6+1d8+4)dh3"} {
		t.Run(f, func(t *testing.T) {
			e, err := Parse(f)
			if err != nil {
				t.Fatalf("Parse(%q): %v", f, err)
			}
			r, err := e.Roll(newFakeRNG(t, 1, 2, 3, 4, 5, 6), nil)
			if !errors.Is(err, ErrKeepCount) {
				t.Fatalf("err = %v, want ErrKeepCount", err)
			}
			if r.Total != 0 || r.Tree != nil {
				t.Errorf("result not zero on error: %+v", r)
			}
		})
	}
}

func TestRollSuccessCounting(t *testing.T) {
	cases := []struct {
		f     string
		faces []int
		total int
	}{
		{"6d10>=8", []int{8, 3, 10, 7, 9, 1}, 3},
		{"6d10>8", []int{8, 3, 10, 7, 9, 1}, 2},
		{"6d10<3", []int{8, 3, 10, 7, 9, 1}, 1},
		{"6d10<=3", []int{8, 3, 10, 7, 9, 1}, 2},
		{"6d10=7", []int{8, 3, 10, 7, 9, 7}, 2},
		{"3d6>=7", []int{6, 6, 6}, 0},
	}
	for _, c := range cases {
		t.Run(c.f, func(t *testing.T) {
			r := rollFaces(t, c.f, c.faces...)
			if r.Total != c.total {
				t.Errorf("total = %d, want %d", r.Total, c.total)
			}
			g := rootGroup(t, r)
			if g.Value != c.total {
				t.Errorf("group value = %d, want %d", g.Value, c.total)
			}
			want := make([]DieState, len(c.faces))
			checkDice(t, g, c.faces, want)
		})
	}
}

func TestRollSuccessCountingOnlyKeptDice(t *testing.T) {
	// kh3 of 8,3,10,7,9,1 keeps 8,10,9; two of those are >= 9.
	r := rollFaces(t, "6d10kh3>=9", 8, 3, 10, 7, 9, 1)
	if r.Total != 2 {
		t.Errorf("total = %d, want 2", r.Total)
	}
	checkDice(t, rootGroup(t, r), []int{8, 3, 10, 7, 9, 1}, []DieState{K, D, K, D, K, D})

	// Dropped dice that meet the comparison are not counted.
	r = rollFaces(t, "4d10kl2>=5", 9, 1, 10, 6)
	if r.Total != 1 {
		t.Errorf("total = %d, want 1", r.Total)
	}
	checkDice(t, rootGroup(t, r), []int{9, 1, 10, 6}, []DieState{D, K, D, K})

	r = rollFaces(t, "6d10>=8+1", 8, 3, 10, 7, 9, 1)
	if r.Total != 4 {
		t.Errorf("total = %d, want 4", r.Total)
	}
}

func TestRollGroupedKeep(t *testing.T) {
	// Dice pooled across the group: d6s 3,5 and d8 6; keep highest 2 -> 5+6.
	r := rollFaces(t, "(2d6+1d8)kh2", 3, 5, 6)
	if r.Total != 11 {
		t.Errorf("total = %d, want 11", r.Total)
	}
	gs := diceGroups(r.Tree)
	if len(gs) != 2 {
		t.Fatalf("got %d dice groups, want 2", len(gs))
	}
	checkDice(t, gs[0], []int{3, 5}, []DieState{D, K})
	checkDice(t, gs[1], []int{6}, []DieState{K})
	if gs[0].Value != 5 || gs[1].Value != 6 {
		t.Errorf("group values = %d, %d, want 5, 6", gs[0].Value, gs[1].Value)
	}
}

func TestRollGroupedKeepVariants(t *testing.T) {
	cases := []struct {
		f      string
		faces  []int
		total  int
		states [][]DieState
	}{
		{"(1d6+1d8)kh1", []int{4, 2}, 4, [][]DieState{{K}, {D}}},
		{"(1d6+1d8)kl1", []int{4, 2}, 2, [][]DieState{{D}, {K}}},
		{"(1d6+1d8)ph1", []int{4, 7}, 4, [][]DieState{{K}, {D}}},
		{"(1d6+1d8)pl1", []int{4, 7}, 7, [][]DieState{{D}, {K}}},
		{"(1d6+1d8)dh1", []int{4, 7}, 4, [][]DieState{{K}, {D}}},
		{"(1d6+1d8)dl1", []int{4, 7}, 7, [][]DieState{{D}, {K}}},
		{"(2d6+1d8)dh1", []int{3, 5, 6}, 8, [][]DieState{{K, K}, {D}}},
		{"(1d6+1d8)kh1+2", []int{4, 2}, 6, [][]DieState{{K}, {D}}},
		{"(1d6+1d8)kh1*2", []int{4, 2}, 8, [][]DieState{{K}, {D}}},
	}
	for _, c := range cases {
		t.Run(c.f, func(t *testing.T) {
			r := rollFaces(t, c.f, c.faces...)
			if r.Total != c.total {
				t.Errorf("total = %d, want %d", r.Total, c.total)
			}
			gs := diceGroups(r.Tree)
			if len(gs) != len(c.states) {
				t.Fatalf("got %d dice groups, want %d", len(gs), len(c.states))
			}
			for i, g := range gs {
				if got := dieStates(g.Dice); !reflect.DeepEqual(got, c.states[i]) {
					t.Errorf("group %d states = %v, want %v", i, got, c.states[i])
				}
			}
		})
	}
}

func TestRollOverflow(t *testing.T) {
	for _, f := range []string{"2d6", "3d6kh2"} {
		t.Run(f, func(t *testing.T) {
			e, err := Parse(f)
			if err != nil {
				t.Fatal(err)
			}
			r, err := e.Roll(newFakeRNG(t, math.MaxInt, math.MaxInt, 1), nil)
			if !errors.Is(err, ErrOverflow) {
				t.Fatalf("err = %v, want ErrOverflow", err)
			}
			if r.Total != 0 || r.Tree != nil {
				t.Errorf("result not zero on error: %+v", r)
			}
		})
	}
	// Dropped dice are not summed, so no overflow.
	r := rollFaces(t, "3d6kl2", math.MaxInt, 1, 2)
	if r.Total != 3 {
		t.Errorf("total = %d, want 3", r.Total)
	}
	r = rollFaces(t, "2d6kh1", math.MaxInt, math.MaxInt)
	if r.Total != math.MaxInt {
		t.Errorf("total = %d, want MaxInt", r.Total)
	}
}

func TestRollDiceGroupLabel(t *testing.T) {
	r := rollFaces(t, "4d6kh3[Strength]", 2, 5, 3, 6)
	g := rootGroup(t, r)
	if g.Label != "Strength" {
		t.Errorf("label = %q, want Strength", g.Label)
	}
	r = rollFaces(t, "1d20[Attack] + 1d4", 12, 3)
	gs := diceGroups(r.Tree)
	if len(gs) != 2 {
		t.Fatalf("got %d dice groups, want 2", len(gs))
	}
	if gs[0].Label != "Attack" || gs[1].Label != "" {
		t.Errorf("labels = %q, %q, want Attack, empty", gs[0].Label, gs[1].Label)
	}
	if r.Total != 15 {
		t.Errorf("total = %d, want 15", r.Total)
	}
}

func TestRollDiceRepeatable(t *testing.T) {
	e, err := Parse("(2d6+1d8)kh2 + 4d6kh3>=4")
	if err != nil {
		t.Fatal(err)
	}
	r1, err := e.Roll(newFakeRNG(t, 3, 5, 6, 2, 5, 3, 6), nil)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := e.Roll(newFakeRNG(t, 3, 5, 6, 2, 5, 3, 6), nil)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Total != r2.Total || !reflect.DeepEqual(r1.Tree, r2.Tree) {
		t.Errorf("rolls differ: %+v vs %+v", r1, r2)
	}
	fresh, _ := Parse("(2d6+1d8)kh2 + 4d6kh3>=4")
	if !sameAST(e.root, fresh.root) {
		t.Errorf("expr changed by Roll")
	}
	if r1.Total != 13 {
		t.Errorf("total = %d, want 13", r1.Total)
	}
}
