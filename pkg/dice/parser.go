package dice

import "errors"

// Parse parses formula with the default limits.
func Parse(formula string) (*Expr, error) {
	return ParseWithLimits(formula, DefaultLimits())
}

// ParseWithLimits parses formula, enforcing l at parse time.
func ParseWithLimits(formula string, l Limits) (*Expr, error) {
	return nil, errors.New("dice: parser not implemented")
}
