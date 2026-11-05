package dice

import (
	"math/rand/v2"
	"testing"
)

var _ RNG = (*rand.Rand)(nil)

type recT struct {
	testing.TB
	failed bool
}

func (r *recT) Helper() { r.TB.Helper() }
func (r *recT) Fatalf(s string, args ...any) {
	r.failed = true
	r.TB.Logf(s, args...)
}

func TestFakeRNGScript(t *testing.T) {
	f := newFakeRNG(t, 3, 1, 6)
	for _, want := range []int{2, 0, 5} {
		if got := f.IntN(6); got != want {
			t.Fatalf("IntN = %d, want %d", got, want)
		}
	}
	var _ RNG = f
}

func TestFakeRNGExhausted(t *testing.T) {
	r := &recT{TB: t}
	f := newFakeRNG(r, 2)
	f.IntN(6)
	if r.failed {
		t.Fatal("failed too early")
	}
	f.IntN(6)
	if !r.failed {
		t.Fatal("expected failure on exhausted script")
	}
}
