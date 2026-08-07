package domain

import "time"

// ID constrains the identity type of an [Aggregate].
//
// The identity value objects in vo.go satisfy it, see [UserID] for an example.
type ID interface {
	comparable
	String() string
}

// Aggregate is the root type shared by every aggregate root in the domain.
// It owns identity, creation time, and version;
type Aggregate[T ID] struct {
	id        T
	createdAt time.Time
	// version is an optimistic-locking mechanism.
	version uint64
}

// minVersion is the version a newly constructed aggregate carries — the one it
// holds after the insert that first persists it. Each subsequent replacement
// advances it by one, so the version doubles as a count of the writes an
// aggregate has seen.
const minVersion uint64 = 1

// newAggregate stamps a new aggregate with the current UTC time. The caller
// supplies the id because generation is per identity type — see [NewUserID].
func newAggregate[T ID](id T) Aggregate[T] {
	return Aggregate[T]{id: id, createdAt: time.Now().UTC(), version: minVersion}
}

// rehydrateAggregate skips validation. Only repositories should reach it, via
// the exported factories in factory.go.
func rehydrateAggregate[T ID](id T, createdAt time.Time, version uint64) Aggregate[T] {
	return Aggregate[T]{id: id, createdAt: createdAt, version: version}
}

// nextVersion advances the aggregate to the version it holds once the current
// state is persisted. Versioning is a persistence concern, not a domain rule:
// the mutators leave it alone, and only a repository calls this — through the
// factories in factory.go, from inside the critical section that
// compare-and-sets on Version.
func (a *Aggregate[T]) nextVersion() { a.version++ }

// Getters — domain exposes read-only access
func (a Aggregate[T]) ID() T                { return a.id }
func (a Aggregate[T]) CreatedAt() time.Time { return a.createdAt }

// Version reports the revision this aggregate was loaded at. Passing it back to
// a repository is what lets the write detect a lost update; see [ErrConflict].
func (a Aggregate[T]) Version() uint64 { return a.version }
