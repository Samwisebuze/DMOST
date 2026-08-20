// Package spike anchors this reference spike's code generation directives, the
// way internal/generate.go does for the real type tree.
//
// Regenerate (required before the first `go test` — see README.md):
//
//	go generate ./...
package spike

// The extension build. Generated from character/schema, whose 14 files are the
// production schemas byte-for-byte except for the `goJSONSchema` keywords in
// spellcasting.schema.json — `diff` the two directories to see the whole change.
//
// The schemas live *inside* the package directory because character/schema.go
// embeds them: go:embed cannot reach above its own package. The generator is
// pointed at the same copy so that one set of files serves both the generated
// types and the compiled schema, which is the arrangement being demonstrated.
//
//go:generate sh -c "go tool go-jsonschema -p character --resolve-extension json -o character/character.gen.go character/schema/*.schema.json"

// The control, generated straight from the production schemas — no copy, so it
// cannot drift from what the repo actually ships. Everything that differs
// between the two generated packages is caused by the extension keywords and
// nothing else.
//
//go:generate sh -c "go tool go-jsonschema -p baseline --resolve-extension json -o baseline/baseline.gen.go ../../../jsonschema/character/v1alpha/*.schema.json"
