// Package inmem implements the domain's repository ports against process
// memory: a map behind a [sync.RWMutex], living and dying with the process.
//
// It is the odd one out under internal/infra. Every other subdirectory here is
// named for a technology outside the process and owns the connection to it;
// this one owns no connection and has nothing to Open or Close. It belongs
// beside them anyway, because placement follows the *role* a package plays
// rather than the machinery it happens to need: this is an adapter answering
// "how is a User stored", it is swapped for [github.com/samwisebuze/dmost/internal/infra/sqlite]
// at the composition root without either the domain or the use cases noticing,
// and it is exactly as much an implementation detail of this program as a
// storage backend that talks to a database.
//
// It is the backend cmd/dmostd wires today, and it is not a test double: the
// concurrency, the uniqueness scan, and the compare-and-set on Version are the
// real behavior a repository is expected to have, which is what makes
// [UserRepository.Save] the reference implementation of the versioning rule
// described in [github.com/samwisebuze/dmost/pkg/domain].
//
// The scenarios that pin that rule down for characters no longer live in this
// package's tests. They moved to
// [github.com/samwisebuze/dmost/internal/test/repotest] when
// [github.com/samwisebuze/dmost/internal/infra/sqlite] grew a second
// implementation of the same port, so that both answer to one definition
// instead of two that drift; this package's test file runs them from there.
package inmem
