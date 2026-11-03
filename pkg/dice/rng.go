package dice

// RNG is a source of random ints; *rand.Rand from math/rand/v2 satisfies it.
type RNG interface{ IntN(n int) int }
