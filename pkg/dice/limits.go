package dice

const (
	DefaultMaxDice       = 0
	DefaultMaxSides      = 0
	DefaultMaxRerolls    = 0
	DefaultMaxExplosions = 0
	DefaultMaxNodes      = 0
)

// Limits bounds evaluation.
type Limits struct {
	MaxDice, MaxSides, MaxRerolls, MaxExplosions, MaxNodes int
}

// DefaultLimits returns the defaults.
func DefaultLimits() Limits { return Limits{} }
