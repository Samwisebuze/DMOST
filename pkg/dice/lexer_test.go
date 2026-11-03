package dice

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLexTokens(t *testing.T) {
	tk := func(k tokenKind, text string, pos int) lexToken { return lexToken{k, text, pos} }
	tests := []struct {
		name string
		in   string
		want []lexToken
	}{
		{"integer", "42", []lexToken{tk(tokInt, "42", 0)}},
		{"d", "d6", []lexToken{tk(tokD, "d", 0), tk(tokInt, "6", 1)}},
		{"d percent", "2d%", []lexToken{tk(tokInt, "2", 0), tk(tokDPercent, "d%", 1)}},
		{"dF", "4dF", []lexToken{tk(tokInt, "4", 0), tk(tokDF, "dF", 1)}},
		{"operators", "1+2-3*4/5", []lexToken{
			tk(tokInt, "1", 0), tk(tokPlus, "+", 1), tk(tokInt, "2", 2), tk(tokMinus, "-", 3),
			tk(tokInt, "3", 4), tk(tokStar, "*", 5), tk(tokInt, "4", 6), tk(tokSlash, "/", 7),
			tk(tokInt, "5", 8)}},
		{"parens and comma", "(1,2)", []lexToken{
			tk(tokLParen, "(", 0), tk(tokInt, "1", 1), tk(tokComma, ",", 2),
			tk(tokInt, "2", 3), tk(tokRParen, ")", 4)}},
		{"identifier", "1+@str", []lexToken{tk(tokInt, "1", 0), tk(tokPlus, "+", 1), tk(tokIdent, "@str", 2)}},
		{"label", "1 [fire dmg]", []lexToken{tk(tokInt, "1", 0), tk(tokLabel, "[fire dmg]", 2)}},
		{"func min", "min", []lexToken{tk(tokFunc, "min", 0)}},
		{"func max", "max", []lexToken{tk(tokFunc, "max", 0)}},
		{"func floor", "floor", []lexToken{tk(tokFunc, "floor", 0)}},
		{"func ceil", "ceil", []lexToken{tk(tokFunc, "ceil", 0)}},
		{"func call", "min(1,2)", []lexToken{
			tk(tokFunc, "min", 0), tk(tokLParen, "(", 3), tk(tokInt, "1", 4),
			tk(tokComma, ",", 5), tk(tokInt, "2", 6), tk(tokRParen, ")", 7)}},
		{"mod k", "4d6k3", []lexToken{tk(tokInt, "4", 0), tk(tokD, "d", 1), tk(tokInt, "6", 2), tk(tokMod, "k", 3), tk(tokInt, "3", 4)}},
		{"mod p", "4d6p1", []lexToken{tk(tokInt, "4", 0), tk(tokD, "d", 1), tk(tokInt, "6", 2), tk(tokMod, "p", 3), tk(tokInt, "1", 4)}},
		{"mod h", "4d6h1", []lexToken{tk(tokInt, "4", 0), tk(tokD, "d", 1), tk(tokInt, "6", 2), tk(tokMod, "h", 3), tk(tokInt, "1", 4)}},
		{"mod l", "4d6l1", []lexToken{tk(tokInt, "4", 0), tk(tokD, "d", 1), tk(tokInt, "6", 2), tk(tokMod, "l", 3), tk(tokInt, "1", 4)}},
		{"mod r", "4d6r1", []lexToken{tk(tokInt, "4", 0), tk(tokD, "d", 1), tk(tokInt, "6", 2), tk(tokMod, "r", 3), tk(tokInt, "1", 4)}},
		{"mod rr", "4d6rr1", []lexToken{tk(tokInt, "4", 0), tk(tokD, "d", 1), tk(tokInt, "6", 2), tk(tokMod, "rr", 3), tk(tokInt, "1", 5)}},
		{"mod ro", "4d6ro1", []lexToken{tk(tokInt, "4", 0), tk(tokD, "d", 1), tk(tokInt, "6", 2), tk(tokMod, "ro", 3), tk(tokInt, "1", 5)}},
		{"mod e", "4d6e", []lexToken{tk(tokInt, "4", 0), tk(tokD, "d", 1), tk(tokInt, "6", 2), tk(tokMod, "e", 3)}},
		{"mod !", "4d6!", []lexToken{tk(tokInt, "4", 0), tk(tokD, "d", 1), tk(tokInt, "6", 2), tk(tokMod, "!", 3)}},
		{"mod dh", "4d6dh1", []lexToken{tk(tokInt, "4", 0), tk(tokD, "d", 1), tk(tokInt, "6", 2), tk(tokMod, "dh", 3), tk(tokInt, "1", 5)}},
		{"mod dl", "4d6dl1", []lexToken{tk(tokInt, "4", 0), tk(tokD, "d", 1), tk(tokInt, "6", 2), tk(tokMod, "dl", 3), tk(tokInt, "1", 5)}},
		{"cmp <", "<", []lexToken{tk(tokCmp, "<", 0)}},
		{"cmp <=", "<=", []lexToken{tk(tokCmp, "<=", 0)}},
		{"cmp =", "=", []lexToken{tk(tokCmp, "=", 0)}},
		{"cmp >=", ">=", []lexToken{tk(tokCmp, ">=", 0)}},
		{"cmp >", ">", []lexToken{tk(tokCmp, ">", 0)}},
		{"reroll with selector", "4d6r<2", []lexToken{
			tk(tokInt, "4", 0), tk(tokD, "d", 1), tk(tokInt, "6", 2), tk(tokMod, "r", 3),
			tk(tokCmp, "<", 4), tk(tokInt, "2", 5)}},
		{"whitespace offsets", "  1 +  2 ", []lexToken{tk(tokInt, "1", 2), tk(tokPlus, "+", 4), tk(tokInt, "2", 7)}},
		{"multibyte label offsets", "[é]+1", []lexToken{tk(tokLabel, "[é]", 0), tk(tokPlus, "+", 4), tk(tokInt, "1", 5)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lex(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLexErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		pos  int
	}{
		{"unterminated label", "1+[abc", 2},
		{"unterminated label at start", "[", 0},
		{"decimal", "1.5", 1},
		{"percent after int", "50%", 2},
		{"floor div", "7//2", 1},
		{"double equals", "1==2", 1},
		{"not equals", "1!=2", 1},
		{"double bang", "4d6!!", 3},
		{"bang p", "4d6!p", 3},
		{"ra", "4d6ra", 3},
		{"mi", "4d6mi", 3},
		{"ma", "4d6ma", 3},
		{"comment", "1 #c", 2},
		{"single quote", "1+'a'", 2},
		{"double quote", `1+"a"`, 2},
		{"unknown char", "1 $", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := lex(tt.in)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrSyntax)
			assert.Contains(t, err.Error(), fmt.Sprint(tt.pos))
		})
	}
}

func TestLexIntegerOverflow(t *testing.T) {
	_, err := lex("1+99999999999999999999")
	require.Error(t, err)
	assert.True(t, errorsIsAny(err, ErrOverflow, ErrSyntax), "got %v", err)
	assert.Contains(t, err.Error(), "2")
}

func errorsIsAny(err error, targets ...error) bool {
	for _, t := range targets {
		if errors.Is(err, t) {
			return true
		}
	}
	return false
}
