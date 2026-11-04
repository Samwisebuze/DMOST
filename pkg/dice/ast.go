package dice

// Expr is a parsed, immutable expression.
type Expr struct {
	root   node
	src    string
	limits Limits // limits in force, set at parse time
}

type node interface{ isNode() }

type literal struct{ value int }

type variable struct {
	name  string
	label string
	pos   int
}

type unary struct {
	op      byte
	operand node
	pos     int
}

type binary struct {
	op          byte
	left, right node
	pos         int
}

// diceTerm is NdS with optional modifiers.
type diceTerm struct {
	count, sides node
	label        string
	fudge        bool // dF: faces -1, 0, +1; sides is unused
	mods         []diceMod
	pos          int
}

// diceMod is a keep/drop, reroll, explode or success-counting modifier;
// see the mod* constants for kinds.
type diceMod struct {
	kind string
	cmp  string
	arg  node
	once bool
}

type funcCall struct {
	name string
	args []node
	pos  int
}

// groupKeep is a group of terms with keep/drop applied.
type groupKeep struct {
	items []node
	kind  string // "kh", "kl", "ph", "pl"
	count node
	label string
	pos   int
}

func (literal) isNode()   {}
func (variable) isNode()  {}
func (unary) isNode()     {}
func (binary) isNode()    {}
func (diceTerm) isNode()  {}
func (funcCall) isNode()  {}
func (groupKeep) isNode() {}
