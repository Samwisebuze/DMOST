// Package internal anchors the repo-private code generation directives.
//
// The JSON Schema documents under docs/jsonschema are the source of truth for
// the wire contract; the Go structs under internal/dto are derived from them,
// never hand-edited. One directive per [namespace]/[version] pair: every schema
// file in a version directory is fed to the generator at once so that $ref
// between siblings resolves, and the whole set lands in a single package at
// internal/dto/[version]/[namespace].
//
// Regenerate with `go generate ./internal/...` from the repo root.
package internal

//go:generate sh -c "go tool go-jsonschema -p character --resolve-extension json -o dto/v1alpha/character/character.gen.go ../docs/jsonschema/character/v1alpha/*.schema.json"
