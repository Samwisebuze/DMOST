package dice

import (
	"errors"
	"testing"
)

func FuzzParse(f *testing.F) {
	seeds := []string{
		"", " ", "1", "-1", "d6", "1d6", "4d6kh3", "4d6pl1", "4d6dh1", "4d6dl1",
		"1d20+4d6kh3+2", "2d6*2-1d4/2", "-(1d6)", "((1))", "(1d6+",
		"1d8+@str", "1d8[slashing]+@str[ability]", "1d6[unterminated",
		"1d%", "4dF", "min(1,2)", "max(1d6,3)", "floor(1)", "ceil(1d4)",
		"1d6rr1", "1d6ro<=2", "1d6r>=5", "1d6e", "1d6!", "1d6e>=5",
		"2d6!p", "2d6!!", "3d6>=4", "4d10kl2>=5", "(1d6+1d8)kh1", "(1d6+1d8)kh0",
		"4d6kh7", "1d0", "0d6", "1000000d1000000", "1d1e", "1d6rr<=6",
		"9223372036854775807+1", "99999999999999999999", "9223372036854775807*2",
		"1d6 3", "1.5", "1d6//2", "1%2", "{1,2}", "1d6ra", "1d6mi2", "@", "[x]",
		"((((((((((((((((((((1))))))))))))))))))))",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, formula string) {
		e, err := Parse(formula)
		if err != nil {
			if e != nil {
				t.Fatalf("Parse(%q) returned a non-nil Expr with error %v", formula, err)
			}
			if !errors.Is(err, ErrSyntax) && !errors.Is(err, ErrLimitExceeded) &&
				!errors.Is(err, ErrKeepCount) && !errors.Is(err, ErrOverflow) {
				t.Fatalf("Parse(%q) returned unexpected error %v", formula, err)
			}
			return
		}
		if e == nil {
			t.Fatalf("Parse(%q) returned nil Expr and nil error", formula)
		}
	})
}
