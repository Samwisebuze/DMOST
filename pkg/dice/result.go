package dice

// Node is an explanation tree node.
type Node interface{ explainNode() }

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

// Die is one rolled die. RerolledFrom and ExplodedFrom are indexes of the
// sibling die in the same group's Dice slice, or -1 if none.
type Die struct {
	Value        int
	State        DieState
	RerolledFrom int
	ExplodedFrom int
}

// LiteralNode is a numeric literal.
type LiteralNode struct{ Value int }

// VariableNode is a resolved variable.
type VariableNode struct {
	Name  string
	Label string
	Value int
}

// OperatorNode is a unary or binary operator.
type OperatorNode struct {
	Op       string
	Operands []Node
	Value    int
}

// FunctionNode is a function call.
type FunctionNode struct {
	Name  string
	Args  []Node
	Value int
}

// DiceGroupNode is a group of dice (e.g. 4d6kh3).
type DiceGroupNode struct {
	Label string
	Dice  []Die
	Value int
}

func (LiteralNode) explainNode()   {}
func (VariableNode) explainNode()  {}
func (OperatorNode) explainNode()  {}
func (FunctionNode) explainNode()  {}
func (DiceGroupNode) explainNode() {}
