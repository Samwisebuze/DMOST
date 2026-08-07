// Package app is the use-case layer: it sits between transport adapters
// (pkg/http) and the domain, and it is the only layer that knows how a
// request turns into domain operations.
//
// # Interfaces here, implementations in services
//
// This package declares the use-case interfaces ([UserService] and friends in
// service.go); the concrete implementations live in the child package
// [github.com/samwisebuze/dmost/pkg/app/services]. The split is what lets
// callers depend on the narrow interface while services depends on the
// domain — pkg/http handlers hold an app.UserService and tests substitute a
// fake, without either importing services. Adding a use case means a method
// on an interface here plus a type in services, in that order.
//
// # App is the composition root's carrier
//
// [App] is a plain struct of service interfaces — the assembled set of
// use-cases that a server is handed. [New] wires the default all-in-memory
// stack and is the convenient path; cmd/dmostd builds its own App literal
// instead, so it can choose adapters. Nothing here reaches for a global.
//
// Note that pkg/http captures the *App pointer at construction time and
// mutates it through Server.SetApp; an App handed to a server must be filled
// in through that method rather than reassigned.
//
// # Wire types in the use-case signatures
//
// Inbound data is, usually, in a wire type (see pkg/dto).
// This leaves validation and domain translation up to app instead of the transport adapters.
//
// Services return domain types outward. Translating those back to a wire
// shape is the transport's job, not this layer's.
package app
