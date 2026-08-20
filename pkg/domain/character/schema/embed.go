// Package schema carries the character sheet schema as embedded bytes.
//
// The documents beside this file are the source of truth for what a character
// sheet is. Two consumers read them, and they must read the same bytes:
//
//   - [github.com/samwisebuze/dmost/pkg/domain/character] compiles them at
//     startup and validates every sheet entering the domain against them.
//   - internal/generate.go runs go-jsonschema over them to produce the schema
//     wire types in internal/dto/v1alpha/character.
package schema

import "embed"

// documents holds every *.schema.json in this directory. The set is compiled as
// a batch, not one file at a time: character.schema.json is only the assembly,
// and its $refs point across the other files. See
// [github.com/samwisebuze/dmost/pkg/domain/character] for the compile.
//
//go:embed *.schema.json
var documents embed.FS

// _root names the document that assembles the others — the one a whole character
// sheet is validated against. The rest are reachable from it by $ref.
const _root = "character.schema.json"
