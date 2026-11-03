package dice

import "errors"

type tokenKind int

const (
	tokInt      tokenKind = iota // integer literal
	tokD                         // d
	tokDPercent                  // d%
	tokDF                        // dF
	tokPlus
	tokMinus
	tokStar
	tokSlash
	tokLParen
	tokRParen
	tokComma
	tokIdent // @name
	tokLabel // [text]
	tokFunc  // min, max, floor, ceil
	tokMod   // k p h l r rr ro e ! dh dl
	tokCmp   // < <= = >= >
)

// lexToken is a lexeme; text is the raw source slice and pos its byte offset.
type lexToken struct {
	kind tokenKind
	text string
	pos  int
}

// lex splits input into tokens, skipping whitespace.
func lex(input string) ([]lexToken, error) { return nil, errors.New("not implemented") }
