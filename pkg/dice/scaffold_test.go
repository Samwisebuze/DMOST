package dice

import (
	"errors"
	"reflect"
	"testing"
)

func TestSentinelsDistinct(t *testing.T) {
	all := []error{ErrSyntax, ErrUnknownVariable, ErrDivisionByZero, ErrOverflow,
		ErrLimitExceeded, ErrKeepCount, ErrTooManyDice}
	for i, a := range all {
		for j, b := range all {
			if (i == j) != errors.Is(a, b) && !(i == 6 && j == 4) {
				t.Errorf("errors.Is(%v[%d], [%d]) = %v", a, i, j, errors.Is(a, b))
			}
		}
	}
}

func TestSpecificLimitErrors(t *testing.T) {
	for _, e := range []error{ErrTooManyDice, ErrTooManySides, ErrTooManyRerolls,
		ErrTooManyExplosions, ErrExpressionTooLarge} {
		if !errors.Is(e, ErrLimitExceeded) {
			t.Errorf("%v does not match ErrLimitExceeded", e)
		}
		if errors.Is(ErrLimitExceeded, e) {
			t.Errorf("ErrLimitExceeded should not match %v", e)
		}
	}
	if errors.Is(ErrTooManyDice, ErrTooManySides) {
		t.Error("specific limit errors must be distinct")
	}
	wrapped := errors.Join(ErrTooManyDice)
	if !errors.Is(wrapped, ErrLimitExceeded) {
		t.Error("wrapped error should match ErrLimitExceeded")
	}
}

func TestDefaultConstants(t *testing.T) {
	if DefaultMaxDice != 1000 || DefaultMaxSides != 10000 || DefaultMaxRerolls != 100 ||
		DefaultMaxExplosions != 100 || DefaultMaxNodes != 256 {
		t.Fatal("wrong default constants")
	}
	want := Limits{MaxDice: 1000, MaxSides: 10000, MaxRerolls: 100, MaxExplosions: 100, MaxNodes: 256}
	if got := DefaultLimits(); got != want {
		t.Fatalf("DefaultLimits = %+v, want %+v", got, want)
	}
}

func TestDieStatesDistinct(t *testing.T) {
	seen := map[DieState]bool{}
	for _, s := range []DieState{DieKept, DieDropped, DieRerolled, DieExploded} {
		if seen[s] {
			t.Fatal("duplicate DieState")
		}
		seen[s] = true
	}
}

func TestDieAndResultFields(t *testing.T) {
	dt := reflect.TypeOf(Die{})
	for _, n := range []string{"Value", "RerolledFrom", "ExplodedFrom"} {
		f, ok := dt.FieldByName(n)
		if !ok || f.Type.Kind() != reflect.Int {
			t.Errorf("Die.%s missing or not int", n)
		}
	}
	if f, ok := dt.FieldByName("State"); !ok || f.Type != reflect.TypeOf(DieState(0)) {
		t.Error("Die.State missing or wrong type")
	}
	rt := reflect.TypeOf(Result{})
	if f, ok := rt.FieldByName("Total"); !ok || f.Type.Kind() != reflect.Int {
		t.Error("Result.Total missing")
	}
	if _, ok := rt.FieldByName("Tree"); !ok {
		t.Error("Result.Tree missing")
	}
}

func TestNoJSONTagsAndNoExportedExprFields(t *testing.T) {
	for _, ty := range []reflect.Type{reflect.TypeOf(Result{}), reflect.TypeOf(Die{}), reflect.TypeOf(Limits{})} {
		for i := 0; i < ty.NumField(); i++ {
			if tag := ty.Field(i).Tag; tag != "" {
				t.Errorf("%s.%s has tag %q", ty, ty.Field(i).Name, tag)
			}
		}
	}
	et := reflect.TypeOf(Expr{})
	if et.Kind() != reflect.Struct || et.NumField() == 0 {
		t.Fatal("Expr must be a struct wrapping the unexported AST")
	}
	for i := 0; i < et.NumField(); i++ {
		if et.Field(i).IsExported() {
			t.Errorf("Expr.%s is exported", et.Field(i).Name)
		}
	}
}
