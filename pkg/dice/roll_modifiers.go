package dice

import "fmt"

// groupRoller rolls additional dice for one dice group, enforcing the
// MaxDice, MaxRerolls and MaxExplosions limits across the whole group.
type groupRoller struct {
	ev         *evaluator
	sides      int
	fudge      bool
	pos        int
	rerolls    int
	explosions int
}

// face draws one die face (one rng.IntN(sides) call).
func (g *groupRoller) face() (int, error) {
	r := g.ev.rng.IntN(g.sides)
	if g.fudge {
		return r - 1, nil // 0,1,2 -> -1,0,+1
	}
	return addInt(r, 1)
}

// maxFace is the highest face of the group's dice.
func (g *groupRoller) maxFace() int {
	if g.fudge {
		return 1
	}
	return g.sides
}

// add rolls a new die and appends it, checking MaxDice first.
func (g *groupRoller) add(dice []Die, rerolledFrom, explodedFrom int) ([]Die, error) {
	if len(dice) >= g.ev.limits.MaxDice {
		return nil, fmt.Errorf("%w: more than %d dice at position %d", ErrTooManyDice, g.ev.limits.MaxDice, g.pos)
	}
	f, err := g.face()
	if err != nil {
		return nil, err
	}
	return append(dice, Die{Value: f, State: DieKept, RerolledFrom: rerolledFrom, ExplodedFrom: explodedFrom}), nil
}

// reroll applies rr (reroll while matching) or ro (reroll once). Each
// matching kept die is marked DieRerolled and a replacement is appended as
// a sibling with RerolledFrom set. For rr, replacements are checked again.
func (g *groupRoller) reroll(dice []Die, m *diceMod) ([]Die, error) {
	_, target, err := g.ev.eval(m.arg)
	if err != nil {
		return nil, err
	}
	end := len(dice)
	for i := 0; i < len(dice); i++ {
		if m.once && i >= end {
			break
		}
		if dice[i].State != DieKept || !compare(dice[i].Value, m.cmp, target) {
			continue
		}
		g.rerolls++
		if g.rerolls > g.ev.limits.MaxRerolls {
			return nil, fmt.Errorf("%w: more than %d rerolls at position %d", ErrTooManyRerolls, g.ev.limits.MaxRerolls, g.pos)
		}
		dice[i].State = DieRerolled
		if dice, err = g.add(dice, i, -1); err != nil {
			return nil, err
		}
	}
	return dice, nil
}

// explode applies e: each kept die matching the selector (default: the
// maximum face) is marked DieExploded, still counts toward the total, and
// triggers an appended sibling die with ExplodedFrom set. Explosions chain.
func (g *groupRoller) explode(dice []Die, m *diceMod) ([]Die, error) {
	target := g.maxFace()
	if m.arg != nil {
		var err error
		if _, target, err = g.ev.eval(m.arg); err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(dice); i++ {
		if dice[i].State != DieKept || !compare(dice[i].Value, m.cmp, target) {
			continue
		}
		g.explosions++
		if g.explosions > g.ev.limits.MaxExplosions {
			return nil, fmt.Errorf("%w: more than %d explosions at position %d", ErrTooManyExplosions, g.ev.limits.MaxExplosions, g.pos)
		}
		dice[i].State = DieExploded
		var err error
		if dice, err = g.add(dice, -1, i); err != nil {
			return nil, err
		}
	}
	return dice, nil
}

// counts reports whether a die in this state contributes to the group value
// and is eligible for keep/drop: kept and exploded dice do, dropped and
// rerolled dice do not.
func (s DieState) counts() bool { return s == DieKept || s == DieExploded }
