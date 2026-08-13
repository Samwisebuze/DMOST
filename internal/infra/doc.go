// Package infra is the depository for outbound technical
// adapters: code that owns a connection to something outside the process —
// a database, a message broker, a third-party HTTP API — and nothing else.
//
// Each subdirectory is one technology, named for it. A
// subdirectory owns two kinds of thing and no others:
//
//   - the connection itself, modeled as a process: a type with exported
//     configuration fields, a constructor that returns it already runnable on
//     defaults, and Open/Close to move it in and out of a running state;
//   - the port implementations that need that connection — a
//     [github.com/samwisebuze/dmost/pkg/domain/user.UserRepository] backed by
//     SQL lives beside the *DB it queries, not in a package of its own.
//
// Nothing here is a domain rule. These packages exist because the domain
// declares ports it cannot implement itself, and every type in them is an
// answer to "how", never to "what". Dependencies point inward: infra imports
// the domain, and the domain never imports infra.
//
// # Why internal
//
// A storage backend is an implementation detail of this program, and
// publishing one under pkg/ would promise outside callers a shape that is
// expected to churn — the DSN handling, the pool defaults, and the set of
// adapters are all expected to change as persistence develops.
package infra
