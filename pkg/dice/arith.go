package dice

import (
	"fmt"
	"math"
)

func addInt(a, b int) (int, error) {
	if (b > 0 && a > math.MaxInt-b) || (b < 0 && a < math.MinInt-b) {
		return 0, fmt.Errorf("%w: %d + %d", ErrOverflow, a, b)
	}
	return a + b, nil
}

func subInt(a, b int) (int, error) {
	if (b < 0 && a > math.MaxInt+b) || (b > 0 && a < math.MinInt+b) {
		return 0, fmt.Errorf("%w: %d - %d", ErrOverflow, a, b)
	}
	return a - b, nil
}

func mulInt(a, b int) (int, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}
	c := a * b
	if (a == -1 && b == math.MinInt) || (b == -1 && a == math.MinInt) || c/b != a {
		return 0, fmt.Errorf("%w: %d * %d", ErrOverflow, a, b)
	}
	return c, nil
}

// divInt is floor division.
func divInt(a, b int) (int, error) {
	if b == 0 {
		return 0, fmt.Errorf("%w: %d / 0", ErrDivisionByZero, a)
	}
	if a == math.MinInt && b == -1 {
		return 0, fmt.Errorf("%w: %d / %d", ErrOverflow, a, b)
	}
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q, nil
}

func negInt(a int) (int, error) {
	if a == math.MinInt {
		return 0, fmt.Errorf("%w: -(%d)", ErrOverflow, a)
	}
	return -a, nil
}
