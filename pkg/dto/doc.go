// Package dto is the depository for this project's stable wire types: the
// versions of the contract that are published, supported, and safe for an
// outside caller to build against.
//
// Pre-release versions — live in [github.com/samwisebuze/dmost/internal/dto], which
// documents what a version package contains and the rules every version obeys.
//
// The split exists so that a package's path states its stability.
//
// A version graduates by moving here once its shape is settled.
// A published version is frozen. It is not
// edited, not reshaped, and not removed while clients depend on it; the way
// forward is always a new sibling version, and a long sunset.
package dto
