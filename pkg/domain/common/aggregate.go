package common

import (
	"time"

	"github.com/samwisebuze/dmost/pkg/domain/internal/lock"
)

// ID constrains the identity type of an [Aggregate].
//
// The identity value objects beside each aggregate satisfy it — see UserID in
// [github.com/samwisebuze/dmost/pkg/domain/user] for an example.
type ID interface {
	comparable
	String() string
}

// Aggregate is the root type shared by every aggregate root in the domain. It
// owns identity, creation time, and the [Version] that makes a write
// conditional. Composing it is what gives an entity all three.
type Aggregate[T ID] struct {
	id        T
	createdAt time.Time
	// version is an optimistic-locking mechanism.
	version Version
}

// NewAggregate stamps a new aggregate with the current UTC time and the first
// [Version]. The caller supplies the id because generation is per identity
// type — see NewUserID.
func NewAggregate[T ID](id T) Aggregate[T] {
	return Aggregate[T]{id: id, createdAt: time.Now().UTC(), version: NewVersion()}
}

// RehydrateAggregate skips validation. Only repositories should reach it, via
// the exported factories beside each aggregate.
func RehydrateAggregate[T ID](id T, createdAt time.Time, version Version) Aggregate[T] {
	return Aggregate[T]{id: id, createdAt: createdAt, version: version}
}

// NextVersion advances the aggregate to the version it holds once the current
// state is persisted. Versioning is a persistence concern, not a domain rule:
// the mutators leave it alone, and only a repository calls this — through the
// factories beside each aggregate, from inside the critical section that has
// already compare-and-set on [Aggregate.Version].
//
// The [lock.Key] is what keeps that narrow. Minting one means importing a
// package under pkg/domain/internal, which an adapter cannot do, so a
// repository has no way to reach this except through a factory.
func (a *Aggregate[T]) NextVersion(lock.Key) { a.version = a.version.Next() }

// Getters — domain exposes read-only access
func (a Aggregate[T]) ID() T                { return a.id }
func (a Aggregate[T]) CreatedAt() time.Time { return a.createdAt }

// Version reports the revision this aggregate was loaded at. Passing it back to
// a repository is what lets the write detect a lost update; see [ErrConflict].
func (a Aggregate[T]) Version() Version { return a.version }
