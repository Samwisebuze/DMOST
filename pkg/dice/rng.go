package dice

// RNG is a source of random ints.
type RNG interface{ IntN(n int) int }
