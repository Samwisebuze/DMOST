package dice

type tb interface {
	Helper()
	Fatalf(format string, args ...any)
}

// fakeRNG returns scripted face values in order; IntN(n) = face-1.
type fakeRNG struct {
	t     tb
	faces []int
	pos   int
}

func newFakeRNG(t tb, faces ...int) *fakeRNG { return &fakeRNG{t: t, faces: faces} }

func (f *fakeRNG) IntN(n int) int {
	f.t.Helper()
	if f.pos >= len(f.faces) {
		f.t.Fatalf("fake RNG script exhausted after %d draws", f.pos)
		return 0
	}
	face := f.faces[f.pos]
	f.pos++
	return face - 1
}
