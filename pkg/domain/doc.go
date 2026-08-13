// Package domain is the pure domain layer: entities, value objects, and
// repository ports, with no dependency on any other layer in this module.
//
// DDD concepts encountered here:
//
//   - Aggregate root.
//     [github.com/samwisebuze/dmost/pkg/domain/common.Aggregate] is the generic
//     base every root composes for identity, CreatedAt, and version.
//     An aggregate is the unit of invariant enforcement and the unit a
//     repository loads and saves atomically.
//     ex. [github.com/samwisebuze/dmost/pkg/domain/user.User] is an aggregate root;
//
//   - Entity.
//     Distinguished by identity (eg.
//     [github.com/samwisebuze/dmost/pkg/domain/common.ID]), not attribute equality.
//     Fields stay unexported behind read-only getters so all mutation
//     routes through methods that can enforce invariants (e.g.
//     [github.com/samwisebuze/dmost/pkg/domain/user.User.Rename] rejects a
//     blank half of a name).
//
//   - Value object.
//     Distinguished by attribute equality, not identity, and are immutable once constructed.
//     Constructors are the only way to get a non-zero instance, so a valid
//     value in hand needs no re-validation downstream.
//
//   - Validating constructor vs. Rehydrate.
//     Constructors (ex. NewFooer) enforce invariants for state a caller is introducing;
//     Rehydration (ex. RehydrateFooer) skips them for state a repository
//     is reconstructing from storage, which is assumed already valid.
//     In a few cases Rehydration can cause an error state,
//     if the construction of a valid entity is crucial to it's functionality.
//     Factories are just objects that orchestrate this rehydration, nothing more.
//
//   - Repository (port).
//     An interface representing the management of a collection of aggregates/entities.
//     The domain only defines the contract it needs, and infrastructure (ex. internal/infra/inmem) satisfy  it.
//
//   - Optimistic concurrency.
//     Not a domain rule but a persistence concern threaded through the
//     [github.com/samwisebuze/dmost/pkg/domain/common.Version] an aggregate
//     composes: a repository compare-and-sets on it and returns
//     [github.com/samwisebuze/dmost/pkg/domain/common.ErrConflict] on a lost
//     update. Mutators never touch it; only a factory's NextVersion does, and
//     only after a successful compare-and-set.
//
//     That restriction is enforced by the compiler rather than by convention.
//     Advancing a version takes a token from pkg/domain/internal/lock, which
//     only packages under pkg/domain can import, so an adapter has no route to
//     it except through the aggregate's factory.
//
//   - Sentinel errors as ubiquitous language. Callers match with errors.Is.
package domain
