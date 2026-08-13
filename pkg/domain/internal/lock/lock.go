// Package lock holds the capability token that gates the domain mutations
// belonging to persistence rather than to callers.
//
// Some state on an aggregate is not the caller's to change — a version is the
// standing example. It advances when a repository stores the aggregate, and at
// no other time. While every aggregate lived in one package that rule cost
// nothing to enforce: the mutator was unexported, and only the factory beside
// it could reach it. Splitting the domain into common and per-aggregate
// packages broke that, because a method the factory can call from another
// package is a method any adapter can call.
//
// [Key] restores the boundary at the import graph instead. This package sits
// under pkg/domain/internal, so only code rooted at pkg/domain can import it —
// a mutator taking a Key is therefore unreachable from pkg/inmem, pkg/app or
// pkg/http, which must go through an aggregate's factory as before.
package lock

// sealed is what makes [Key] unforgeable, and it has to be a field of a type
// declared here. A Key defined as a bare empty struct would not be enough: an
// untyped struct{}{} is assignable to any named type sharing its underlying
// type, so an adapter could conjure one without importing this package at all.
// No struct written elsewhere can be identical to one holding a sealed.
type sealed struct{}

// Key is proof that the caller is domain code. It carries no information — the
// only thing it says is that whoever holds it could import this package.
type Key struct{ _ sealed }

// New mints a token, and is reachable only from pkg/domain/...
func New() Key { return Key{} }
