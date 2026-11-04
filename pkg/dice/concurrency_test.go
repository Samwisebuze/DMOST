package dice

import (
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentRolls rolls one parsed Expr from many goroutines, each with
// its own RNG. Run with go test -race to detect shared mutable state.
func TestConcurrentRolls(t *testing.T) {
	formulas := []string{
		"4d6kh3+1d8[slashing]+@str",
		"1d6e+2d6rr1+1d6ro<=2",
		"(1d6+1d8)kh1+min(1d4,2)",
		"3d6>=4",
		"4dF+1d%",
	}
	const workers = 16
	const rolls = 50
	for _, f := range formulas {
		t.Run(f, func(t *testing.T) {
			e, err := Parse(f)
			require.NoError(t, err)

			// Sequential reference results, per worker seed.
			want := make([][]Result, workers)
			for w := range workers {
				rng := rand.New(rand.NewPCG(uint64(w), uint64(w)))
				for range rolls {
					r, err := e.Roll(rng, propResolve)
					require.NoError(t, err)
					want[w] = append(want[w], r)
				}
			}

			got := make([][]Result, workers)
			errs := make([]error, workers)
			var wg sync.WaitGroup
			start := make(chan struct{})
			for w := range workers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					rng := rand.New(rand.NewPCG(uint64(w), uint64(w)))
					for range rolls {
						r, err := e.Roll(rng, propResolve)
						if err != nil {
							errs[w] = err
							return
						}
						got[w] = append(got[w], r)
					}
				}()
			}
			close(start)
			wg.Wait()
			for w := range workers {
				require.NoError(t, errs[w], "worker %d", w)
				assert.Equal(t, want[w], got[w], "worker %d", w)
			}
		})
	}
}
