package dice

import (
	"fmt"
	"sort"
)

// rollDice evaluates a dice term: it rolls the dice (one rng.IntN(sides)
// call per die, in order), applies keep/drop modifiers in written order and
// computes the group value: the checked sum of kept dice or, with success
// counting, the number of kept dice meeting the comparison.
func (ev *evaluator) rollDice(d *diceTerm) (Node, int, error) {
	var dice []Die
	var gr *groupRoller
	replaying := ev.replay != nil
	if replaying {
		if len(ev.replay) == 0 {
			return nil, 0, fmt.Errorf("dice: internal error: replay exhausted")
		}
		dice = ev.replay[0]
		ev.replay = ev.replay[1:]
	} else {
		_, count, err := ev.eval(d.count)
		if err != nil {
			return nil, 0, err
		}
		sides := 3
		if !d.fudge {
			if _, sides, err = ev.eval(d.sides); err != nil {
				return nil, 0, err
			}
		}
		if count <= 0 || sides <= 0 {
			return nil, 0, fmt.Errorf("%w: %dd%d at position %d", ErrSyntax, count, sides, d.pos)
		}
		gr = &groupRoller{ev: ev, sides: sides, fudge: d.fudge, pos: d.pos}
		dice = make([]Die, count)
		for i := range dice {
			face, err := gr.face()
			if err != nil {
				return nil, 0, err
			}
			dice[i] = Die{Value: face, State: DieKept, RerolledFrom: -1, ExplodedFrom: -1}
		}
	}

	var success *diceMod
	for i := range d.mods {
		m := &d.mods[i]
		switch m.kind {
		case modKeepHigh, modKeepLow, modDropHigh, modDropLow:
			if replaying {
				continue
			}
			_, n, err := ev.eval(m.arg)
			if err != nil {
				return nil, 0, err
			}
			refs := make([]*Die, 0, len(dice))
			for j := range dice {
				if dice[j].State.counts() {
					refs = append(refs, &dice[j])
				}
			}
			if err := applyKeep(refs, m.kind, n, d.pos); err != nil {
				return nil, 0, err
			}
		case modReroll, modRerollOne, modExplode:
			if replaying {
				continue
			}
			var err error
			if m.kind == modExplode {
				dice, err = gr.explode(dice, m)
			} else {
				dice, err = gr.reroll(dice, m)
			}
			if err != nil {
				return nil, 0, err
			}
		case modSuccess:
			success = m
		default:
			return nil, 0, fmt.Errorf("dice: modifier %q is not implemented at position %d", m.kind, d.pos)
		}
	}

	v, err := ev.diceValue(dice, success)
	if err != nil {
		return nil, 0, err
	}
	return DiceGroupNode{Label: d.label, Dice: dice, Value: v}, v, nil
}

// diceValue is the checked sum of kept dice, or the number of kept dice
// meeting the success comparison when success is non-nil.
func (ev *evaluator) diceValue(dice []Die, success *diceMod) (int, error) {
	if success != nil {
		_, target, err := ev.eval(success.arg)
		if err != nil {
			return 0, err
		}
		n := 0
		for _, d := range dice {
			if d.State.counts() && compare(d.Value, success.cmp, target) {
				n++
			}
		}
		return n, nil
	}
	sum := 0
	for _, d := range dice {
		if !d.State.counts() {
			continue
		}
		var err error
		if sum, err = addInt(sum, d.Value); err != nil {
			return 0, err
		}
	}
	return sum, nil
}

func compare(v int, cmp string, target int) bool {
	switch cmp {
	case ">":
		return v > target
	case ">=":
		return v >= target
	case "<":
		return v < target
	case "<=":
		return v <= target
	default:
		return v == target
	}
}

// applyKeep marks dice DieDropped according to a keep/drop modifier. dice
// must be the currently kept dice in roll order.
//
// Ties are broken by roll order: among equal values, the earlier-rolled die
// ranks first both for highest and for lowest. So kh keeps the earlier and
// drops the later-rolled of equal dice, and kl keeps the earlier-rolled of
// equal dice. ph N is equivalent to kl (len-N) and pl N to kh (len-N).
//
// A count larger than the number of dice returns ErrKeepCount.
func applyKeep(dice []*Die, kind string, n, pos int) error {
	if n < 0 || n > len(dice) {
		return keepCountErr(pos, "%s%d exceeds the %d dice available", kind, n, len(dice))
	}
	keep, high := n, true
	switch kind {
	case modKeepHigh:
	case modKeepLow:
		high = false
	case modDropHigh:
		keep, high = len(dice)-n, false
	case modDropLow:
		keep = len(dice) - n
	default:
		return fmt.Errorf("dice: unknown keep/drop %q", kind)
	}
	order := make([]int, len(dice))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		va, vb := dice[order[a]].Value, dice[order[b]].Value
		if high {
			return va > vb
		}
		return va < vb
	})
	for _, i := range order[keep:] {
		dice[i].State = DieDropped
	}
	return nil
}

// groupKeep evaluates a grouped keep/drop: the items are evaluated, keep/drop
// is applied over all kept dice in them, then the items are re-evaluated on
// those dice so every value reflects kept dice only.
func (ev *evaluator) groupKeep(g *groupKeep) (Node, int, error) {
	if ev.replay != nil {
		return ev.sumItems(g.items)
	}
	nodes := make([]Node, 0, len(g.items))
	for _, it := range g.items {
		n, _, err := ev.eval(it)
		if err != nil {
			return nil, 0, err
		}
		nodes = append(nodes, n)
	}
	var groups [][]Die
	for _, n := range nodes {
		groups = collectDice(n, groups)
	}
	var refs []*Die
	for _, ds := range groups {
		for j := range ds {
			if ds[j].State.counts() {
				refs = append(refs, &ds[j])
			}
		}
	}
	_, count, err := ev.eval(g.count)
	if err != nil {
		return nil, 0, err
	}
	if err := applyKeep(refs, g.kind, count, g.pos); err != nil {
		return nil, 0, err
	}
	re := &evaluator{rng: ev.rng, resolve: ev.resolve, replay: groups}
	if len(groups) == 0 {
		re.replay = [][]Die{}
	}
	n, v, err := re.sumItems(g.items)
	if err != nil {
		return nil, 0, err
	}
	if len(re.replay) != 0 {
		return nil, 0, fmt.Errorf("dice: internal error: %d dice groups not replayed", len(re.replay))
	}
	return n, v, nil
}

// sumItems evaluates items and sums them with checked addition, building a
// left-associative '+' tree.
func (ev *evaluator) sumItems(items []node) (Node, int, error) {
	var acc Node
	total := 0
	for i, it := range items {
		n, v, err := ev.eval(it)
		if err != nil {
			return nil, 0, err
		}
		if i == 0 {
			acc, total = n, v
			continue
		}
		if total, err = addInt(total, v); err != nil {
			return nil, 0, err
		}
		acc = OperatorNode{Op: "+", Operands: []Node{acc, n}, Value: total}
	}
	return acc, total, nil
}

// collectDice appends the Dice slice of every DiceGroupNode in n, depth-first
// left to right (the order dice terms are evaluated).
func collectDice(n Node, out [][]Die) [][]Die {
	switch x := n.(type) {
	case DiceGroupNode:
		return append(out, x.Dice)
	case OperatorNode:
		for _, o := range x.Operands {
			out = collectDice(o, out)
		}
	case FunctionNode:
		for _, a := range x.Args {
			out = collectDice(a, out)
		}
	}
	return out
}
