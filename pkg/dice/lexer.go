package dice

import (
	"fmt"
	"strconv"
	"strings"
)

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
func lex(input string) ([]lexToken, error) {
	var toks []lexToken
	n := len(input)
	i := 0
	bad := func(pos int, what string) error {
		return fmt.Errorf("%w: %s at position %d", ErrSyntax, what, pos)
	}
	emit := func(k tokenKind, length int) {
		toks = append(toks, lexToken{k, input[i : i+length], i})
		i += length
	}
	at := func(j int) byte {
		if j < n {
			return input[j]
		}
		return 0
	}
	for i < n {
		c := input[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c >= '0' && c <= '9':
			j := i
			for j < n && input[j] >= '0' && input[j] <= '9' {
				j++
			}
			if _, err := strconv.Atoi(input[i:j]); err != nil {
				return nil, fmt.Errorf("%w: integer literal %q at position %d", ErrOverflow, input[i:j], i)
			}
			emit(tokInt, j-i)
		case c == '+':
			emit(tokPlus, 1)
		case c == '-':
			emit(tokMinus, 1)
		case c == '*':
			emit(tokStar, 1)
		case c == '/':
			if at(i+1) == '/' {
				return nil, bad(i, "unsupported //")
			}
			emit(tokSlash, 1)
		case c == '(':
			emit(tokLParen, 1)
		case c == ')':
			emit(tokRParen, 1)
		case c == ',':
			emit(tokComma, 1)
		case c == '@':
			j := i + 1
			for j < n && (input[j] == '_' || input[j] >= 'a' && input[j] <= 'z' || input[j] >= 'A' && input[j] <= 'Z' || input[j] >= '0' && input[j] <= '9') {
				j++
			}
			if j == i+1 {
				return nil, bad(i, "empty identifier")
			}
			emit(tokIdent, j-i)
		case c == '[':
			end := strings.IndexByte(input[i:], ']')
			if end < 0 {
				return nil, bad(i, "unterminated label")
			}
			emit(tokLabel, end+1)
		case c == '<' || c == '>':
			if at(i+1) == '=' {
				emit(tokCmp, 2)
			} else {
				emit(tokCmp, 1)
			}
		case c == '=':
			if at(i+1) == '=' {
				return nil, bad(i, "unsupported ==")
			}
			emit(tokCmp, 1)
		case c == '!':
			if d := at(i + 1); d == '!' || d == 'p' || d == '=' {
				return nil, bad(i, "unsupported modifier")
			}
			emit(tokMod, 1)
		case c == 'd':
			switch at(i + 1) {
			case '%':
				emit(tokDPercent, 2)
			case 'F':
				emit(tokDF, 2)
			case 'h', 'l':
				emit(tokMod, 2)
			default:
				emit(tokD, 1)
			}
		case c == 'k' || c == 'p' || c == 'h' || c == 'l' || c == 'e':
			emit(tokMod, 1)
		case c == 'r':
			switch at(i + 1) {
			case 'r', 'o':
				emit(tokMod, 2)
			case 'a':
				return nil, bad(i, "unsupported modifier ra")
			default:
				emit(tokMod, 1)
			}
		default:
			matched := false
			for _, f := range []string{"min", "max", "floor", "ceil"} {
				if strings.HasPrefix(input[i:], f) {
					emit(tokFunc, len(f))
					matched = true
					break
				}
			}
			if !matched {
				return nil, bad(i, fmt.Sprintf("unexpected character %q", c))
			}
		}
	}
	return toks, nil
}
