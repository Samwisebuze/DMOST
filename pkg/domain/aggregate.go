package domain

import "time"

// ID constrains the identity type of an [Aggregate]. The identity value objects
// in vo.go — UserID and friends — satisfy it.
type ID interface {
	comparable
	String() string
}

// Aggregate is the root type shared by every aggregate root in the domain. It
// owns identity and creation time; entities compose it by embedding, which
// promotes ID and CreatedAt with the embedder's concrete identity type.
//
// Its fields are unexported like the entities that embed it, and are set only
// through newAggregate or, for adapters reconstructing persisted state,
// rehydrateAggregate.
type Aggregate[T ID] struct {
	id        T
	createdAt time.Time
}

// newAggregate stamps a new aggregate with the current UTC time. The caller
// supplies the id because generation is per identity type — see [NewUserID].
func newAggregate[T ID](id T) Aggregate[T] {
	return Aggregate[T]{id: id, createdAt: time.Now().UTC()}
}

// rehydrateAggregate skips validation. Only repositories should reach it, via
// the exported factories in factory.go.
func rehydrateAggregate[T ID](id T, createdAt time.Time) Aggregate[T] {
	return Aggregate[T]{id: id, createdAt: createdAt}
}

// Getters — domain exposes read-only access
func (a Aggregate[T]) ID() T                { return a.id }
func (a Aggregate[T]) CreatedAt() time.Time { return a.createdAt }
