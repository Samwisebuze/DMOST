package dice

import "errors"

// Roll evaluates the expression. Not implemented yet.
func (e *Expr) Roll(rng RNG, resolve func(name string) (int, bool)) (Result, error) {
	return Result{}, errors.New("dice: not implemented")
}
