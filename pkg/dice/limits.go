package dice

// Default limits.
const (
	DefaultMaxDice       = 1000
	DefaultMaxSides      = 10000
	DefaultMaxRerolls    = 100
	DefaultMaxExplosions = 100
	DefaultMaxNodes      = 256
)

// Limits bounds evaluation. A zero field falls back to its default.
type Limits struct {
	MaxDice, MaxSides, MaxRerolls, MaxExplosions, MaxNodes int
}

// DefaultLimits returns the default limits.
func DefaultLimits() Limits {
	return Limits{DefaultMaxDice, DefaultMaxSides, DefaultMaxRerolls, DefaultMaxExplosions, DefaultMaxNodes}
}

// withDefaults replaces zero fields with defaults.
func (l Limits) withDefaults() Limits {
	d := DefaultLimits()
	if l.MaxDice == 0 {
		l.MaxDice = d.MaxDice
	}
	if l.MaxSides == 0 {
		l.MaxSides = d.MaxSides
	}
	if l.MaxRerolls == 0 {
		l.MaxRerolls = d.MaxRerolls
	}
	if l.MaxExplosions == 0 {
		l.MaxExplosions = d.MaxExplosions
	}
	if l.MaxNodes == 0 {
		l.MaxNodes = d.MaxNodes
	}
	return l
}
