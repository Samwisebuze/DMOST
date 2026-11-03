package dice

import "errors"

// Sentinel errors. Position-carrying errors wrap these so errors.Is works.
var (
	ErrSyntax          = errors.New("dice: syntax error")
	ErrUnknownVariable = errors.New("dice: unknown variable")
	ErrDivisionByZero  = errors.New("dice: division by zero")
	ErrOverflow        = errors.New("dice: integer overflow")
	ErrLimitExceeded   = errors.New("dice: limit exceeded")
	ErrKeepCount       = errors.New("dice: invalid keep/drop count")
)

// limitError is a specific limit error that also matches ErrLimitExceeded.
type limitError struct{ msg string }

func (e *limitError) Error() string { return e.msg }

func (e *limitError) Is(target error) bool { return target == ErrLimitExceeded }

// Specific limit errors; all match ErrLimitExceeded via errors.Is.
var (
	ErrTooManyDice        error = &limitError{"dice: too many dice"}
	ErrTooManySides       error = &limitError{"dice: too many sides"}
	ErrTooManyRerolls     error = &limitError{"dice: too many rerolls"}
	ErrTooManyExplosions  error = &limitError{"dice: too many explosions"}
	ErrExpressionTooLarge error = &limitError{"dice: expression too large"}
)
