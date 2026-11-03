package dice

// Node is an explanation tree node.
type Node interface{}

// Result is an evaluation result.
type Result struct {
	Total int
	Tree  Node
}

// DieState is the state of a die.
type DieState int

const (
	DieKept DieState = iota
	DieDropped
	DieRerolled
	DieExploded
)

// Die is one rolled die.
type Die struct {
	Value        int
	State        DieState
	RerolledFrom int
	ExplodedFrom int
}
