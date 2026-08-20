// Package internal anchors the repo-private code generation directives.
//
// The JSON Schema documents under pkg/domain/[namespace]/schema are the source
// of truth for the wire contract; the Go structs under internal/dto are derived
// from them, never hand-edited. They sit beside the domain because the domain
// embeds and compiles the same files to validate against
// (pkg/domain/character/schema/v1alpha), and a second copy under docs/ would be
// a second answer to what a sheet is. One directive per [namespace]/[version]
// pair: every schema file in a version directory is fed to the generator at
// once so that $ref between siblings resolves, and the whole set lands in a
// single package at internal/dto/[version]/[namespace].
//
// Regenerate with `go generate ./internal/...` from the repo root.
package internal

//go:generate sh -c "go tool go-jsonschema -p character --resolve-extension json -o dto/v1alpha/character/character.gen.go ../pkg/domain/character/schema/v1alpha/*.schema.json"
