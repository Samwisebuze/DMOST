// Package dice parses and evaluates tabletop dice expressions such as
// "4d6kh3 + @str - 1".
//
// This comment is the design record for the package. It documents the
// grammar, the deliberate differences from other dice dialects, the
// unsupported features, the limits and the error contract.
//
// # Grammar
//
// The grammar in EBNF, from lowest to highest precedence. All values are
// integers.
//
//	expr     ::= term { ("+" | "-") term }
//	term     ::= unary { ("*" | "/") unary }
//	unary    ::= ("-" | "+") unary | primary
//	primary  ::= INT
//	           | dice
//	           | variable
//	           | function
//	           | "(" expr ")" [ keepdrop ]
//	dice     ::= [ INT ] ( "d" INT | "d%" | "dF" ) { modifier } [ success ] [ label ]
//	variable ::= "@" IDENT [ label ]
//	function ::= ( "min" | "max" ) "(" expr "," expr ")"
//	           | ( "floor" | "ceil" ) "(" expr ")"
//	modifier ::= keepdrop
//	           | ( "rr" | "ro" | "r" ) ( CMP INT | INT )
//	           | ( "e" | "!" ) [ CMP INT ]
//	keepdrop ::= ( "k" | "p" ) ( "h" | "l" ) INT
//	           | ( "dh" | "dl" ) INT
//	success  ::= CMP INT
//	CMP      ::= "=" | "<" | ">" | "<=" | ">="
//	label    ::= "[" text "]"
//
// d% is a percentile die and is exactly d100. dF is a Fudge die with faces
// -1, 0 and +1. Modifiers apply in written order. A function is one of min,
// max, floor and ceil; there is no abs function. floor and ceil are the
// identity on integers and exist for formula compatibility.
//
// Labels are accepted only on dice terms and on variables. A label is
// carried through to the result and never affects the value.
//
// The @name variable is a project extension: "@str" refers to the variable
// "str" supplied at roll time. Neither Avrae nor
// Roll20 uses this syntax for dice formulas; it keeps character-sheet
// references visually distinct from modifiers.
//
// # Differences from Avrae/d20 and Roll20
//
//	Feature            Avrae/d20            Roll20                 dice
//	-----------------  -------------------  ---------------------  ------------------------
//	keep/drop alias    kh, kl, ph, pl, dh   kh, kl, d, dh, dl      kh, kl, ph, pl, dh, dl
//	reroll alias       rr, ro, ra           r, ro                  rr, ro, plus r as alias
//	explode alias      e                    !                      e, plus ! as alias
//	success counting   not available        3d6>4 counts           3d6>4 counts successes
//	grouped keep/drop  sets only            {a,b}kh1               (1d20+1d20)kh1
//	bare drop "d3"     drop lowest (dl)     drop lowest            ErrSyntax, use dl or dh
//	division           floor, "//" is int   truncating, rounded    "/" is floor division
//	variables          bare names           @{name}                @name (extension)
//	comments           free text            free text              labels in [] only
//
// The confirmed aliases are dh/dl for ph/pl, r for rr, and ! for e. Success
// counting is a trailing comparison on a dice term: "10d10>=8" yields the
// number of dice at or above 8. A grouped keep/drop applies to the
// "+"-separated items of a parenthesised expression that contains dice:
// "(2d20+1d6)kh1" keeps the single highest item.
//
// # Unsupported features
//
// Unsupported syntax is rejected at parse time with an error matching
// ErrSyntax:
//
//   - sets such as [1,2,3] or {1d6, 2d8}
//   - decimals such as 1.5
//   - comparison operators between expressions (1d20 > 5 as a boolean);
//     comparisons exist only as modifier selectors and success counting
//   - % as a modulo operator
//   - // as integer division (use "/", which already floors)
//   - ra (reroll and add), and mi / ma (minimum and maximum die value)
//   - free-text comments after the formula; use a [label] instead
//   - !! and !p (penetrating and compounding explosions)
//
// # Limits
//
// Parse uses DefaultLimits. ParseWithLimits takes a Limits value; a zero
// field falls back to its default, so Limits{MaxDice: 50} overrides only the
// dice count. The defaults are constants:
//
//	DefaultMaxDice       = 1000   dice in one term
//	DefaultMaxSides      = 10000  sides on one die
//	DefaultMaxRerolls    = 100    rerolls per die
//	DefaultMaxExplosions = 100    explosions per die
//	DefaultMaxNodes      = 256    AST nodes and nesting depth
//
// Parse time checks: the dice count (ErrTooManyDice), the sides
// (ErrTooManySides) and the expression size and nesting depth
// (ErrExpressionTooLarge). Roll time checks: the number of dice when a grouped
// keep/drop expands to more than MaxDice dice (ErrTooManyDice), the rerolls
// per die (ErrTooManyRerolls) and the explosions per die
// (ErrTooManyExplosions). The effective limits are stored in the parsed Expr,
// so one set of limits governs both phases. Every specific limit error also
// matches ErrLimitExceeded under errors.Is.
//
// # Semantics
//
// Roll-order tie-break: when kept or dropped dice have equal values, the
// earlier-rolled die ranks first for both highest and lowest. So kh keeps the
// earlier of equal dice and drops the later one, and kl also keeps the
// earlier one. ph N is equivalent to kl (n-N) and pl N to kh (n-N).
//
// Division is floor division, rounding toward negative infinity: 7/2 is 3 and
// -7/2 is -4. Dividing by zero is ErrDivisionByZero.
//
// Keep/drop count rule: the count must be at least 1 and at most the number of
// dice. A count of 0, or one greater than the dice count, is ErrKeepCount.
// This is checked at parse time when the dice count is a literal, and at roll
// time otherwise.
//
// Integer overflow is never silent: a literal too large for an int, or an
// addition, subtraction, multiplication or negation that overflows, returns
// ErrOverflow.
//
// # Errors
//
// Errors carry a position and wrap one of these sentinels, so use errors.Is:
//
//   - ErrSyntax: the formula does not match the grammar or uses an
//     unsupported feature.
//   - ErrUnknownVariable: a variable has no value at roll time.
//   - ErrDivisionByZero: division by zero.
//   - ErrOverflow: integer overflow, including oversized literals.
//   - ErrLimitExceeded: any limit was exceeded; the specific errors
//     ErrTooManyDice, ErrTooManySides, ErrTooManyRerolls,
//     ErrTooManyExplosions and ErrExpressionTooLarge all match it.
//   - ErrKeepCount: a keep/drop count of 0 or more than the dice count.
package dice
