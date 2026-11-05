package dice

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
)

// TestRegularDiceDistribution checks the observable result of unmodified dice.
func TestRegularDiceDistribution(t *testing.T) {
	for _, sides := range []int{4, 6, 8, 10, 12, 20, 100} {
		t.Run(fmt.Sprintf("d%d", sides), func(t *testing.T) {
			const rollsPerFace = 1_000
			rolls := sides * rollsPerFace
			expr, err := Parse(fmt.Sprintf("d%d", sides))
			if err != nil {
				t.Fatal(err)
			}
			rng := rand.New(rand.NewPCG(2026, uint64(sides)))
			counts := make([]int, sides)
			for range rolls {
				result, err := expr.Roll(rng, nil)
				if err != nil {
					t.Fatal(err)
				}
				if result.Total < 1 || result.Total > sides {
					t.Fatalf("d%d rolled %d, want a face from 1 to %d", sides, result.Total, sides)
				}
				counts[result.Total-1]++
			}

			var chiSquare float64
			for _, count := range counts {
				diff := float64(count - rollsPerFace)
				chiSquare += diff * diff / rollsPerFace
			}
			// A five-standard-deviation margin accommodates sampling variation
			// while detecting substantial bias across the faces.
			degreesOfFreedom := float64(sides - 1)
			maxChiSquare := degreesOfFreedom + 5*math.Sqrt(2*degreesOfFreedom)
			if chiSquare > maxChiSquare {
				t.Errorf("d%d face counts %v: chi-square %.2f exceeds %.2f", sides, counts, chiSquare, maxChiSquare)
			}
		})
	}
}
